package mail_test

import (
	"bufio"
	"net"
	"strings"
	"testing"

	"github.com/ChristophBe/loopback/backend/internal/mail"
)

func TestLoadSMTPConfig(t *testing.T) {
	get := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	cfg, err := mail.LoadSMTPConfig(get(map[string]string{"SMTP_HOST": "mailpit", "SMTP_FROM": "a@b.example"}))
	if err != nil || cfg.Port != "1025" {
		t.Errorf("cfg = %+v, %v", cfg, err)
	}
	if _, err := mail.LoadSMTPConfig(get(map[string]string{"SMTP_FROM": "a@b.example"})); err == nil {
		t.Error("missing host accepted")
	}
	if _, err := mail.LoadSMTPConfig(get(map[string]string{"SMTP_HOST": "mailpit"})); err == nil {
		t.Error("missing from accepted")
	}
}

// fakeSMTP accepts one message and returns the DATA section.
func fakeSMTP(t *testing.T) (port string, data <-chan string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	out := make(chan string, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = c.Close() }()
		r := bufio.NewReader(c)
		reply := func(s string) { _, _ = c.Write([]byte(s + "\r\n")) }
		reply("220 fake")
		var msg strings.Builder
		inData := false
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			switch {
			case inData && line == ".\r\n":
				inData = false
				out <- msg.String()
				reply("250 ok")
			case inData:
				msg.WriteString(line)
			case strings.HasPrefix(line, "EHLO"), strings.HasPrefix(line, "HELO"):
				reply("250 fake")
			case strings.HasPrefix(line, "DATA"):
				inData = true
				reply("354 go")
			case strings.HasPrefix(line, "QUIT"):
				reply("221 bye")
				return
			default:
				reply("250 ok")
			}
		}
	}()
	_, port, _ = net.SplitHostPort(ln.Addr().String())
	return port, out
}

func TestSMTPSend(t *testing.T) {
	port, data := fakeSMTP(t)
	m := mail.NewSMTP(mail.SMTPConfig{Host: "127.0.0.1", Port: port, From: "loopback@localhost"})
	if err := m.Send(t.Context(), "a@b.example", "Hi", "line1\nline2"); err != nil {
		t.Fatal(err)
	}
	got := <-data
	for _, want := range []string{"From: loopback@localhost\r\n", "To: a@b.example\r\n", "Subject: Hi\r\n", "line1\r\nline2"} {
		if !strings.Contains(got, want) {
			t.Errorf("message lacks %q:\n%s", want, got)
		}
	}
}

func TestSMTPSendRejectsHeaderInjection(t *testing.T) {
	m := mail.NewSMTP(mail.SMTPConfig{Host: "127.0.0.1", Port: "1", From: "a@b.example"})
	if err := m.Send(t.Context(), "a@b.example\r\nBcc: x@y.example", "Hi", "x"); err == nil {
		t.Error("line break in recipient accepted")
	}
	if err := m.Send(t.Context(), "a@b.example", "Hi\nBcc: x", "x"); err == nil {
		t.Error("line break in subject accepted")
	}
}
