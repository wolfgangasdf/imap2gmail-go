package main

import (
	"errors"
	"log"
	"time"

	"gopkg.in/ini.v1"
)

var iniPath = "imap2gmail.ini"

// ConfImap imap server settings
type ConfImap struct {
	Server           string
	Port             int `comment:"port of imap server"`
	User             string
	Password         string
	Folder           string        `comment:"imap folder to watch like INBOX"`
	FolderMoved      string        `comment:"imap folder where to move imported messages to"`
	FolderQuarantine string        `comment:"imap folder where to move failed emails"`
	MaxEmailSize     int           `comment:"larger emails are quarantined"`
	IdleTimeout      time.Duration `comment:"idle timeout minutes"`
}

// ConfMail settings for notification emails
type ConfMail struct {
	From     string `comment:"From email address"`
	To       string `comment:"email address to send notofications to"`
	Server   string `comment:"smtp mail server, leave empty to disable"`
	Port     int    `comment:"smtp port"`
	User     string `comment:"smtp user"`
	Password string
}

// ConfGmail holds gmail config
type ConfGmail struct {
	Credentials        string `comment:"credentials.json file path"`
	ProcessForCalendar bool   `comment:"process emails for calendar etc"`
}

// Config store global settings
type Config struct {
	ConfImap  `comment:"Settings for the imap server that is polled"`
	ConfMail  `comment:"Notification and warning emails settings"`
	ConfGmail `comment:"Gmail settings"`
}

// Conf holds the global settings
var Conf Config = Config{
	ConfImap: ConfImap{
		Port:             993,
		Folder:           "INBOX",
		FolderMoved:      "nowingmail",
		FolderQuarantine: "imap2gmailquarantine",
		MaxEmailSize:     20000000,
		IdleTimeout:      5 * time.Minute,
	},
	ConfMail: ConfMail{
		Port: 25,
	},
	ConfGmail: ConfGmail{
		Credentials:        "credentials.json",
		ProcessForCalendar: true,
	},
}

func doit() {

	// test connections to check settings file
	if err := GmailTest(); err != nil {
		log.Fatal(err)
	}
	if err := ImapTest(); err != nil {
		log.Fatal(err)
	}

	chWdog := make(chan error)

	// watchthread, never ends. periodically send nil to silence.
	go func(ch chan error) {
		errorstate := false
		for {
			select {
			case r := <-ch:
				log.Printf("wdog: received on ch: %v", r)
				switch r {
				case nil:
					if errorstate {
						log.Printf("wdog: was in errorstate, send email that works again!")
						SendEmail("everything is again fine", "")
						errorstate = false
					}
				default: // got an error
					if !errorstate {
						errorstate = true
						log.Printf("wdog: send email, err=%v", r)
						SendEmail("error", "Check that imap2gmail is working properly, possibly the IMAP or gmail server is temporarily down.\n"+
							"I will send another email if it works again!\nError="+r.Error())
					} else {
						log.Printf("wdog: error but already in errorstate, don't send email, error=%v", r)
					}
				}
			case r := <-time.Tick(Conf.ConfImap.IdleTimeout * 2): // this is watchdog idle time
				log.Printf("wdog: errstate=%v received timer timeout: %v", errorstate, r)
				go func() { ch <- errors.New("timeout error") }()
			}
		}
	}(chWdog)

	SendEmail("started", "")

	// from now on, the program must NOT terminate!

	// infinite loop
	for {
		log.Println("main: before imaploop")
		if err := ImapLoop(chWdog); err != nil {
			log.Println("main: error imaploop, telling watchdog and sleeping 1 minute: ", err)
			chWdog <- err
			time.Sleep(60 * time.Second) // TODO enough if thing goes mad? increase automatically if repetively?
		}
	}
}

func main() {

	if err := ini.MapTo(&Conf, iniPath); err != nil {
		log.Println("error load config, writing new...", err)
		cfg := ini.Empty()
		cfg.SaveTo("imap2gmail.ini")
		// save new
		err = ini.ReflectFrom(cfg, &Conf)
		if err = cfg.SaveTo(iniPath); err != nil {
			panic(err)
		}
		log.Fatal("Exiting. Edit configuration file and re-start!")
	}

	log.Println("Loaded configuration from ", iniPath)
	log.Printf("  test timeout var: %v, %v", Conf.ConfImap.IdleTimeout, Conf.IdleTimeout)

	doit()

}
