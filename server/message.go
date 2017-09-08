package server

import "net/mail"

type Message struct {
	From    string
	To      string
	Domain  string
	Message *mail.Message
}

func NewMessage(domain string) *Message {
	msg := Message{
		Domain: domain,
	}
	return &msg
}
