package server

import (
	"context"
	"net"
	"net/mail"
	"net/textproto"
	"strings"

	log "github.com/sirupsen/logrus"
)

// Server represents the details of the SMTP server.
type Server struct {
	Log    log.FieldLogger
	Domain string
	addr   net.Addr

	Ready chan struct{}
	Inbox chan mail.Message
}

// New returns a pointer to a newly created Server.
func New(domain string) *Server {
	server := &Server{
		Log:    log.StandardLogger(),
		Domain: domain,

		Ready: make(chan struct{}, 1),
		Inbox: make(chan mail.Message),
	}

	return server
}

func split(line string) (string, string) {
	parts := strings.SplitN(line, " ", 1)
	switch len(parts) {
	case 1:
		return parts[0], ""
	case 2:
		return parts[0], parts[1]
	default:
		return "", ""
	}
}

func (s *Server) handle(ctx context.Context, conn *textproto.Conn) {
	id := conn.Next()

	conn.StartResponse(id)
	err := conn.PrintfLine("%d %s ready", 220, s.Domain)
	conn.EndResponse(id)

	if err != nil {
		s.Log.Error(err)
		return
	}

	for {
		conn.StartRequest(id)
		line, err := conn.ReadLine()
		conn.EndRequest(id)

		if err != nil {
			s.Log.Error(err)
			return
		}

		cmd, domain := split(line)

		conn.StartResponse(id)
		switch cmd {
		case "EHLO":
			if err := conn.PrintfLine("250-%s greets %s", s.Domain, domain); err != nil {
				s.Log.Error(err)
				return
			}

			if err := conn.PrintfLine("250-8BITMIME"); err != nil {
				s.Log.Error(err)
				return
			}

			if err := conn.PrintfLine("250 HELP"); err != nil {
				s.Log.Error(err)
				return
			}
		case "HELO":
			if err := conn.PrintfLine("220 %s greets %s", s.Domain, domain); err != nil {
				s.Log.Error(err)
				return
			}
		case "MAIL":
			if err := conn.PrintfLine("502 command not implemented"); err != nil {
				s.Log.Error(err)
				return
			}
		case "RCPT":
			if err := conn.PrintfLine("502 command not implemented"); err != nil {
				s.Log.Error(err)
				return
			}
		case "DATA":
			if err := conn.PrintfLine("502 command not implemented"); err != nil {
				s.Log.Error(err)
				return
			}
		case "RSET":
			if err := conn.PrintfLine("502 command not implemented"); err != nil {
				s.Log.Error(err)
				return
			}
		case "NOOP":
			if err := conn.PrintfLine("502 command not implemented"); err != nil {
				s.Log.Error(err)
				return
			}
		case "QUIT":
			if err := conn.PrintfLine("221 %s Service closing transmission channel", s.Domain); err != nil {
				s.Log.Error(err)
				return
			}
		case "VRFY":
			if err := conn.PrintfLine("502 command not implemented"); err != nil {
				s.Log.Error(err)
				return
			}
		default:
			if err := conn.PrintfLine("500 command not recognised"); err != nil {
				s.Log.Error(err)
				return
			}

		}
		conn.EndResponse(id)
	}
}

// Listen for new connections
func (s *Server) Listen(ctx context.Context, addr string) (err error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return
	}

	defer ln.Close()
	s.addr = ln.Addr()
	s.Ready <- struct{}{}

	for {
		select {
		case <-ctx.Done():
			log.Info("server shutting down")
			return
		default:
			conn, err := ln.Accept()
			if err != nil {
				log.Error(err)
			}

			tconn := textproto.NewConn(conn)
			go s.handle(ctx, tconn)
		}
	}
}
