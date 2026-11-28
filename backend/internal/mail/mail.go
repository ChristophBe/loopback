// Package mail sends transactional email over SMTP.
package mail

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// timeout bounds one delivery.
const timeout = 30 * time.Second

// SMTPConfig is the connection to the mail server. Locally this is Mailpit.
type SMTPConfig struct {
	Host     string
	Port     string
	From     string
	Username string
	Password string
}

// LoadSMTPConfig reads SMTP_HOST (required), SMTP_PORT (default 1025),
// SMTP_FROM (required), SMTP_USERNAME and SMTP_PASSWORD with getenv.
func LoadSMTPConfig(getenv func(string) string) (SMTPConfig, error) {
	cfg := SMTPConfig{
		Host:     getenv("SMTP_HOST"),
		Port:     getenv("SMTP_PORT"),
		From:     getenv("SMTP_FROM"),
		Username: getenv("SMTP_USERNAME"),
		Password: getenv("SMTP_PASSWORD"),
	}
	if cfg.Port == "" {
		cfg.Port = "1025"
	}
	switch {
	case cfg.Host == "":
		return SMTPConfig{}, errors.New("SMTP_HOST is not set")
	case cfg.From == "":
		return SMTPConfig{}, errors.New("SMTP_FROM is not set")
	}
	return cfg, nil
}

// SMTP sends plain-text email through an SMTP server. It upgrades to TLS when
// the server offers STARTTLS and authenticates when a username is set.
type SMTP struct {
	cfg SMTPConfig
}

// NewSMTP returns an SMTP sender for the server.
func NewSMTP(cfg SMTPConfig) *SMTP { return &SMTP{cfg: cfg} }

// Send delivers the message. Header values must not contain line breaks.
func (m *SMTP) Send(ctx context.Context, to, subject, body string) error {
	for _, v := range []string{to, subject, m.cfg.From} {
		if strings.ContainsAny(v, "\r\n") {
			return errors.New("header value contains a line break")
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	msg := "From: " + m.cfg.From + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n\r\n" +
		strings.ReplaceAll(body, "\n", "\r\n")
	if err := m.deliver(ctx, to, []byte(msg)); err != nil {
		return fmt.Errorf("send mail: %w", err)
	}
	return nil
}

// deliver runs one SMTP transaction. The context and a fixed deadline bound
// the dial and every read and write.
func (m *SMTP) deliver(ctx context.Context, to string, msg []byte) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(m.cfg.Host, m.cfg.Port))
	if err != nil {
		return err
	}
	deadline, _ := ctx.Deadline()
	_ = conn.SetDeadline(deadline)
	c, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		_ = conn.Close()
		return err
	}
	defer func() { _ = c.Close() }()
	if ok, _ := c.Extension("STARTTLS"); ok {
		if err := c.StartTLS(&tls.Config{ServerName: m.cfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	}
	if m.cfg.Username != "" {
		if err := c.Auth(smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)); err != nil {
			return err
		}
	}
	if err := c.Mail(m.cfg.From); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}
