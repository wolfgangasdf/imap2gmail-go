package main

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/emersion/go-imap"
	idle "github.com/emersion/go-imap-idle"
	"github.com/emersion/go-imap/client"
)

func imapGetClient() (*client.Client, error) {
	log.Println("imapGetClient: connecting...")
	c, err := client.DialTLS(Conf.ConfImap.Server+":"+strconv.Itoa(Conf.ConfImap.Port), nil)
	if err != nil {
		return nil, errors.New("imapGetClient dial: " + err.Error())
	}
	// c.SetDebug(os.Stdout) // enable for debug
	c.Timeout = 2 * Conf.ConfImap.IdleTimeout

	if err := c.Login(Conf.ConfImap.User, Conf.ConfImap.Password); err != nil {
		return nil, errors.New("imapGetClient login: " + err.Error())
	}
	log.Println("   Logged in")
	return c, nil
}

// ImapTest xxx
func ImapTest() error {

	c, err := imapGetClient()
	if err != nil {
		return errors.New("imapTest: " + err.Error())
	}

	// List mailboxes
	mailboxes := make(chan *imap.MailboxInfo, 10)
	done := make(chan error, 1)
	go func() {
		done <- c.List("", "*", mailboxes)
	}()

	for m := range mailboxes {
		log.Println("ImapTest: Mailbox " + m.Name)
	}

	if err := <-done; err != nil {
		return errors.New("imapTest: " + err.Error())
	}
	return nil
}

func imapHandleFirstMsg(c *client.Client, mbox *imap.MailboxStatus) error {
	log.Println("handlefirst: begin")
	// get first msg
	seqset := new(imap.SeqSet)
	seqset.AddNum(1)
	messages := make(chan *imap.Message, 1)
	done := make(chan error, 1)
	section := &imap.BodySectionName{}
	items := []imap.FetchItem{section.FetchItem()}
	go func() {
		done <- c.Fetch(seqset, items, messages)
	}()

	// get body
	log.Println("handlefirst: get body...")
	msg := <-messages
	log.Println("msg: ", msg.Uid)
	r := msg.GetBody(section)
	if r == nil {
		return errors.New("Server didn't returned message body")
	}
	if err := <-done; err != nil {
		return err
	}

	// read raw email
	log.Println("handlefirst: read raw...")
	oldlen := r.Len()
	log.Println("r.len: ", oldlen)
	p := make([]byte, oldlen)
	rlen, err := r.Read(p)
	if err != nil {
		return err
	}
	if oldlen != rlen {
		return fmt.Errorf("read wrong len %v and %v", oldlen, rlen)
	}

	// import into gmail
	log.Println("handlefirst: import into gmail...")
	if err := GmailImport(string(p)); err != nil {
		return err
	}

	// copy to moved folder
	log.Println("handlefirst: copy to moved folder...")
	if err := c.Copy(seqset, Conf.ConfImap.FolderMoved); err != nil {
		return err
	}

	// delete inbox msg
	log.Println("handlefirst: mark deleted...")
	if err := c.Store(seqset, imap.FormatFlagsOp(imap.AddFlags, false), []interface{}{imap.DeletedFlag}, nil); err != nil {
		return err
	}
	log.Println("handlefirst: expunge...")
	if err := c.Expunge(nil); err != nil {
		return err
	}

	log.Println("handlefirst: done!")
	return nil
}

func imapIdleWait(c *client.Client, mbox *imap.MailboxStatus) error {
	idleClient := idle.NewClient(c)

	chChUpdates := make(chan client.Update)
	c.Updates = chChUpdates
	defer func() { c.Updates = nil }() // important, before return release this otherwise imap hangs!

	chStopIdle := make(chan struct{})
	chIdleres := make(chan error, 1)

	// run idle
	go func() {
		chIdleres <- idleClient.IdleWithFallback(chStopIdle, 0)
	}()

	log.Println("wait for idle events, msgcount: ", mbox.Messages)
	for {
		select {
		case update := <-chChUpdates: // stop idle if update
			log.Printf("idle update: %v (type %T), msgcount %v", update, update, mbox.Messages)
			go func() { chStopIdle <- struct{}{} }()
		case <-time.After(Conf.ConfImap.IdleTimeout): // stop idle after custom timeout
			log.Printf("idle stop by user timeout!")
			go func() { chStopIdle <- struct{}{} }()
		case err := <-chIdleres: // this is called after chStopIdle is notified or idle error
			log.Printf("idle done, res=%v", err)
			if err != nil {
				return err
			}
			return nil
		}
	}
}

// ImapLoop is the main loop. It returns if error or imap conn broken.
func ImapLoop(wdog chan error) error {
	c, err := imapGetClient()
	if err != nil {
		return err
	}

	// Select INBOX
	mbox, err := c.Select(Conf.ConfImap.Folder, false)
	if err != nil {
		return err
	}

	for {
		log.Printf("imaploop: have %v messages", mbox.Messages)

		// move existing messages to gmail
		for mbox.Messages > 0 {
			log.Printf("imaploop: handle first message...")
			if err := imapHandleFirstMsg(c, mbox); err != nil {
				log.Printf("imaploop: handle first error: %v", err)
				return err
				// TODO: move to quarantine?
			}
			log.Println("imaploop: import of one message done!")
		}

		// wait clever
		log.Println("imaploop: before imap idle wait...")
		if err := imapIdleWait(c, mbox); err != nil {
			return err
		}

		wdog <- nil // if no errors, silence watchdog

		log.Println("imaploop: after imap idle wait")
	}
}
