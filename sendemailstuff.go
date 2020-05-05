package main

import (
	"crypto/tls"
	"log"

	"gopkg.in/gomail.v2"
)

// https://pkg.go.dev/gopkg.in/gomail.v2?tab=doc#example-package

// SendEmail sends a notification email
func SendEmail(subject string, content string) {
	log.Printf("SendEmail: s=%v m=%v", subject, content)
	if Conf.ConfMail.Server == "" {
		return
	}
	m := gomail.NewMessage()
	m.SetHeader("From", Conf.ConfMail.From)
	m.SetHeader("To", Conf.ConfMail.To)
	m.SetHeader("Subject", "[Imap2Gmail] "+subject)
	m.SetBody("text/plain", content)

	d := gomail.NewDialer(Conf.ConfMail.Server, Conf.ConfMail.Port, Conf.ConfMail.User, Conf.ConfMail.Password)
	d.TLSConfig = &tls.Config{InsecureSkipVerify: true} // not nice, needed for localhost
	if err := d.DialAndSend(m); err != nil {
		panic(err)
	}
}
