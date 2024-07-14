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

func imapMoveTo(c *client.Client, seqset *imap.SeqSet, targetFolder string) error {
	// copy to moved folder
	log.Printf("imapMoveTo: copy to folder %v...", targetFolder)
	if err := c.Copy(seqset, targetFolder); err != nil {
		return err
	}

	// delete inbox msg
	log.Println("imapMoveTo: mark deleted...")
	if err := c.Store(seqset, imap.FormatFlagsOp(imap.AddFlags, false), []interface{}{imap.DeletedFlag}, nil); err != nil {
		return err
	}
	log.Println("imapMoveTo: expunge...")
	if err := c.Expunge(nil); err != nil {
		return err
	}
	return nil
}

func imapHandleFirstMsg(c *client.Client, mbox *imap.MailboxStatus) error {
	log.Println("handlefirst: begin")
	seqset := new(imap.SeqSet)
	seqset.AddNum(mbox.Messages) // newest message first
	messages := make(chan *imap.Message, 1)
	done := make(chan error, 1)
	log.Println("handlefirst: get first message envelope & size")
	go func() {
		done <- c.Fetch(seqset, []imap.FetchItem{imap.FetchEnvelope, imap.FetchRFC822Size}, messages)
	}()
	msg := <-messages
	if msg == nil {
		return errors.New("couldn't get first message envelope & size")
	}
	mSubject := msg.Envelope.Subject
	mFrom := msg.Envelope.From
	mID := msg.Envelope.MessageId
	mSize := msg.Size // note that at least for exchange servers, this is useless orig file size(s), NOT b64 encoded. joke.
	log.Printf("handlefirst:   size=%v, id=%v, subject=%v", mSize, mID, mSubject)
	if mSize > uint32(Conf.ConfImap.MaxEmailSize) {
		log.Printf("handlefirst: message too big: %v, moving to quarantine...", mSize)
		SendEmail(fmt.Sprintf("Moving message to quarantine mid=%v", mID), fmt.Sprintf("Message too big: %v\nfrom=%v\nsubject=%v", mSize, mFrom, mSubject))
		return imapMoveTo(c, seqset, Conf.ConfImap.FolderQuarantine)
	}

	log.Println("handlefirst: get first message raw")
	messages = make(chan *imap.Message, 1) // channels need to be reopened
	done = make(chan error, 1)
	section := &imap.BodySectionName{}
	go func() {
		done <- c.Fetch(seqset, []imap.FetchItem{section.FetchItem()}, messages)
	}()
	msg = <-messages
	if msg == nil {
		return errors.New("couldn't get first message raw")
	}
	r := msg.GetBody(section)
	if r == nil {
		return errors.New("server didn't returned message body")
	}
	if err := <-done; err != nil {
		return err
	}
	origlen := r.Len()

	log.Printf("handlefirst: read raw... (origlen=%v)", origlen)
	p := make([]byte, origlen)
	rlen, err := r.Read(p)
	if err != nil {
		return err
	}
	if origlen != rlen {
		return fmt.Errorf("read wrong: origlen=%v and rlen=%v", origlen, rlen)
	}

	log.Println("handlefirst: import into gmail...")
	if err := GmailImport(string(p)); err != nil {
		log.Println("handlefirst: GmailImport gave error, trying again... ", err)
		if err := GmailImport(string(p)); err != nil {
			log.Println("handlefirst: GmailImport gave error twice, moving to quarantine: ", err)
			SendEmail(fmt.Sprintf("Moving message to quarantine mid=%v", mID), fmt.Sprintf("GmailImport error: %v\nfrom=%v\nsubject=%v", err, mFrom, mSubject))
			return imapMoveTo(c, seqset, Conf.ConfImap.FolderQuarantine)
		}
	}

	log.Println("handlefirst: move to moved folder...")
	if err := imapMoveTo(c, seqset, Conf.ConfImap.FolderMoved); err != nil {
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
func ImapLoop(wdog chan error) (errres error) {

	// catch all panics
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Recovering from panic in ImapLoop, r=%v \n", r)
			errres = fmt.Errorf("ImapLoop: recovered panic: %v", r)
		}
	}()

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
				log.Printf("imaploop: handle error: %v", err)
				return err
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
