package message

import (
	"net"
	"net/mail"
	"time"
)

// Message represents an email
type Message struct {
	mail.Message
	Datetime    time.Time
	MailFrom    string
	RctpTo      string
	GreetDomain string
	RemoteAddr  net.Addr
}

// New returns a pointer to a newly created message
func New(domain string, addr net.Addr) *Message {
	msg := Message{
		Datetime:    time.Now(),
		GreetDomain: domain,
		RemoteAddr:  addr,
	}
	return &msg
}
