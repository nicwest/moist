package server

import (
	"context"
	"net"
	"net/mail"
	"net/textproto"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/nicwest/moist/message"
	"github.com/nicwest/moist/store"
)

// Server represents the details of the SMTP server.
type Server struct {
	Log    log.FieldLogger
	Domain string
	addr   net.Addr

	Store *store.Store

	Ready chan struct{}
	Inbox chan message.Message

	FromBlackList []string
	ToWhiteList   []string
}

// New returns a pointer to a newly created Server.
func New(domain string, st *store.Store) *Server {
	log.SetLevel(log.DebugLevel)
	server := &Server{
		Log:    log.StandardLogger(),
		Domain: domain,

		Store: st,

		Ready: make(chan struct{}, 1),
		Inbox: make(chan message.Message),

		FromBlackList: []string{},
		ToWhiteList:   []string{},
	}

	return server
}

func split(line string) (string, string) {
	parts := strings.SplitN(line, " ", 2)
	switch len(parts) {
	case 1:
		return parts[0], ""
	case 2:
		return parts[0], parts[1]
	default:
		return "", ""
	}
}

var fromPattern = regexp.MustCompile("FROM:<(.*)>")
var rcptPattern = regexp.MustCompile("TO:<(.*)>")

func getFrom(text string) string {
	m := fromPattern.FindStringSubmatch(text)
	if m == nil {
		return ""
	}
	return m[1]
}

func getRcpt(text string) string {
	m := rcptPattern.FindStringSubmatch(text)
	if m == nil {
		return ""
	}
	return m[1]
}

func (s *Server) ehlo(conn *textproto.Conn, text string) (id uint, err error) {
	id = conn.Next()
	conn.StartResponse(id)
	defer conn.EndResponse(id)
	if err = conn.PrintfLine("250-%s greets %s", s.Domain, text); err != nil {
		return
	}

	if err = conn.PrintfLine("250-8BITMIME"); err != nil {
		return
	}

	if err = conn.PrintfLine("250 HELP"); err != nil {
		return
	}
	return
}

func (s *Server) helo(conn *textproto.Conn, text string) (id uint, err error) {
	id = conn.Next()
	conn.StartResponse(id)
	defer conn.EndResponse(id)
	if err = conn.PrintfLine("220 %s greets %s", s.Domain, text); err != nil {
		return
	}
	return
}

func (s *Server) mail(conn *textproto.Conn, text string, msg *message.Message) (id uint, ok bool, err error) {
	id = conn.Next()
	conn.StartResponse(id)
	defer conn.EndResponse(id)
	from := strings.ToLower(getFrom(text))

	for _, item := range s.FromBlackList {
		if from == strings.ToLower(item) {
			ok = false
			if err = conn.PrintfLine("221 %s Service closing transmission channel", s.Domain); err != nil {
				return
			}
			return
		}
	}

	ok = true
	msg.MailFrom = from
	if err = conn.PrintfLine("250 OK"); err != nil {
		return
	}

	return
}

func (s *Server) rcpt(conn *textproto.Conn, text string, msg *message.Message) (id uint, ok bool, err error) {
	id = conn.Next()
	conn.StartResponse(id)
	defer conn.EndResponse(id)

	rcpt := strings.ToLower(getRcpt(text))

	for _, item := range s.ToWhiteList {
		if rcpt == strings.ToLower(item) {
			ok = true
		}
	}
	if !ok {
		if err = conn.PrintfLine("550 Requested action not taken: mailbox unavailable"); err != nil {
			return
		}
		return
	}

	msg.RctpTo = rcpt
	if err = conn.PrintfLine("250 OK"); err != nil {
		return
	}
	return
}

func (s *Server) data(conn *textproto.Conn, text string, msg *message.Message) (id uint, err error) {
	id = conn.Next()
	conn.StartResponse(id)
	if err != nil {
		if err = conn.PrintfLine("502 command not implemented"); err != nil {
			conn.EndResponse(id)
			return
		}
		conn.EndResponse(id)
		return
	}

	if err = conn.PrintfLine("354 Start mail input; end with <CRLF>.<CRLF>"); err != nil {
		conn.EndResponse(id)
		return
	}
	conn.EndResponse(id)

	conn.StartRequest(id)
	dr := conn.DotReader()
	conn.EndRequest(id)

	id = conn.Next()
	conn.StartResponse(id)
	defer conn.EndResponse(id)

	mmsg, err := mail.ReadMessage(dr)

	if err != nil {
		s.Log.Warn(err)
		if werr := conn.PrintfLine("451 Requested action aborted: local error in processing"); err != nil {
			s.Log.Warn(werr)
			return
		}
		return
	}
	msg.Message = *mmsg

	if err = conn.PrintfLine("250 OK"); err != nil {
		return
	}

	return
}

func (s *Server) rset(conn *textproto.Conn, text string) (id uint, err error) {
	id = conn.Next()
	conn.StartResponse(id)
	defer conn.EndResponse(id)
	if err = conn.PrintfLine("502 command not implemented"); err != nil {
		return
	}
	return
}

func (s *Server) noop(conn *textproto.Conn, text string) (id uint, err error) {
	id = conn.Next()
	conn.StartResponse(id)
	defer conn.EndResponse(id)
	if err = conn.PrintfLine("250 OK"); err != nil {
		return
	}
	return
}

func (s *Server) quit(conn *textproto.Conn, text string) (id uint, err error) {
	id = conn.Next()
	conn.StartResponse(id)
	defer conn.EndResponse(id)
	if err = conn.PrintfLine("221 %s Service closing transmission channel", s.Domain); err != nil {
		return
	}
	return
}

func (s *Server) vrfy(conn *textproto.Conn, text string) (id uint, err error) {
	id = conn.Next()
	conn.StartResponse(id)
	defer conn.EndResponse(id)
	if err = conn.PrintfLine("502 command not implemented"); err != nil {
		return
	}
	return
}

func (s *Server) badCommand(conn *textproto.Conn, cmd, text string) (id uint, err error) {
	id = conn.Next()
	conn.StartResponse(id)
	defer conn.EndResponse(id)
	if err = conn.PrintfLine("500 command not recognised"); err != nil {
		return
	}
	return
}

func (s *Server) handle(ctx context.Context, conn net.Conn, tconn *textproto.Conn) {
	defer tconn.Close()
	id := tconn.Next()

	tconn.StartResponse(id)
	err := tconn.PrintfLine("%d %s ready", 220, s.Domain)
	tconn.EndResponse(id)

	if err != nil {
		s.Log.Error(err)
		return
	}

	var msg *message.Message

	for {
		tconn.StartRequest(id)
		line, err := tconn.ReadLine()
		s.Log.WithFields(log.Fields{
			"id": id,
		}).Debug(line)
		tconn.EndRequest(id)

		if err != nil {
			s.Log.Error(err)
			return
		}

		cmd, text := split(line)
		s.Log.WithFields(log.Fields{
			"cmd": cmd,
		}).Debug(text)

		var ok bool
		var domain string

		switch cmd {
		case "EHLO":
			id, err = s.ehlo(tconn, text)
			if err != nil {
				s.Log.Debug(err)
			}
			domain = text
		case "HELO":
			id, err = s.helo(tconn, text)
			if err != nil {
				s.Log.Debug(err)
			}
			domain = text
		case "MAIL":
			msg = message.New(domain, conn.RemoteAddr())
			id, ok, err = s.mail(tconn, text, msg)
			if err != nil {
				msg = nil
				s.Log.Debug(err)
			}
			if !ok {
				msg = nil
				s.Log.Debug("from address on black list")
				return
			}
		case "RCPT":
			id, ok, err = s.rcpt(tconn, text, msg)
			if err != nil {
				s.Log.Debug(err)
			}
			if !ok {
				s.Log.Debug("RCPT not recognised")
			}
		case "DATA":
			id, err = s.data(tconn, text, msg)
			if err != nil {
				s.Log.Debug(err)
			} else {
				s.Inbox <- *msg
			}
		case "RSET":
			id, err = s.rset(tconn, text)
			if err != nil {
				s.Log.Debug(err)
			}
		case "NOOP":
			id, err = s.noop(tconn, text)
			if err != nil {
				s.Log.Debug(err)
			}
		case "QUIT":
			id, err = s.quit(tconn, text)
			if err != nil {
				s.Log.Debug(err)
			}
			return
		case "VRFY":
			id, err = s.vrfy(tconn, text)
			if err != nil {
				s.Log.Debug(err)
			}
		default:
			id, err = s.badCommand(tconn, cmd, text)
			if err != nil {
				s.Log.Debug(err)
			}
		}
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
	conns := make(chan net.Conn)
	cctx, cancel := context.WithCancel(ctx)

	go func() {
		for {
			select {
			case <-cctx.Done():
				return
			default:
				conn, err := ln.Accept()
				if err != nil {
					s.Log.Error(err)
					continue
				}
				conns <- conn
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			s.Log.Info("server shutting down")
			cancel()
			return
		case conn := <-conns:
			tconn := textproto.NewConn(conn)
			go s.handle(ctx, conn, tconn)
		}
	}
}
