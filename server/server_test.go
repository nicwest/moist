package server

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/smtp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func sendTestMail(addr, to, from, body string) (err error) {
	c, err := smtp.Dial(addr)
	if err != nil {
		return
	}
	err = c.Mail(to)
	if err != nil {
		return
	}
	err = c.Rcpt(from)
	if err != nil {
		return
	}

	wc, err := c.Data()
	if err != nil {
		return
	}
	_, err = fmt.Fprintf(wc, body)
	if err != nil {
		return
	}
	err = wc.Close()
	if err != nil {
		return
	}

	err = c.Quit()
	if err != nil {
		return
	}
	return nil
}

func TestServer(t *testing.T) {
	ctx := context.Background()
	cctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	server := New("mail.example.com")
	go func() {
		err := server.Listen(cctx, ":0")
		if err != nil {
			t.Fatal(err)
		}
	}()
	<-server.Ready
	go func() {
		err := sendTestMail(
			server.addr.String(),
			"fred@example.com",
			"bob@example.com",
			"hello world!",
		)
		if err != nil {
			t.Fatal(err)
		}
	}()

	select {
	case msg := <-server.Inbox:
		body, err := ioutil.ReadAll(msg.Body)
		if assert.Nil(t, err) {
			assert.Equal(t, "hello world!", body)
		}
	case <-cctx.Done():
		t.Fatal("context timed out")
	}

}
