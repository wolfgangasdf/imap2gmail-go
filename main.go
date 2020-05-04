package main

import (
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
		Server: "localhost",
		Port:   25,
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

	chWdog := make(chan interface{})

	// watchthread, never ends.
	go func(ch chan interface{}) {
		errorstate := false
		for {
			select {
			case r := <-ch:
				log.Printf("wdog: received on ch: %v", r)
				if errorstate {
					log.Printf("wdog: was in errorstate, send email that works again!")
					SendEmail("everything is again fine", "")
					errorstate = false
				}
			case r := <-time.Tick(Conf.ConfImap.IdleTimeout * 2): // this is watchdog idle time
				log.Printf("wdog: errstate=%v received timer tick: %v", errorstate, r)
				if !errorstate {
					errorstate = true
					log.Printf("wdog: timeout, send email!")
					SendEmail("timeout", "Check that imap2gmail is working properly, possibly the IMAP or gmail server is temporarily down.\nI will send another email if it works again!")
				} else {
					log.Printf("wdog: timeout but already in errorstate, don't send email!")
				}
			}
		}
	}(chWdog)

	// from now on, the program must NOT terminate!
	SendEmail("started", "")

	// infinite loop
	for {
		log.Println("main: before imaploop")
		if err := ImapLoop(chWdog); err != nil {
			log.Println("main: error imaploop, sleeping 1 minute: ", err)
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
