package main

import (
	"crypto/tls"
	"log"

	"gopkg.in/gomail.v2"
)

const canemail = true

// https://pkg.go.dev/gopkg.in/gomail.v2?tab=doc#example-package

// SendEmail sends a notification email
func SendEmail(subject string, content string) {
	if Conf.ConfMail.Server == "" {
		log.Printf("Fake SendEmail: s=%v m=%v", subject, content)
		return
	}
	m := gomail.NewMessage()
	m.SetHeader("From", Conf.ConfMail.From)
	m.SetHeader("To", Conf.ConfMail.To)
	m.SetHeader("Subject", "[Imap2Gmail] "+subject)
	m.SetBody("text/html", content)

	d := gomail.NewDialer(Conf.ConfMail.Server, Conf.ConfMail.Port, Conf.ConfMail.User, Conf.ConfMail.Password)
	d.TLSConfig = &tls.Config{InsecureSkipVerify: true} // for localhosts... TODO
	if err := d.DialAndSend(m); err != nil {
		panic(err)
	}
}
