// Package mail sends transactional email over SMTP.
package mail

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strings"
)

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
	var auth smtp.Auth
	if m.cfg.Username != "" {
		auth = smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)
	}
	msg := "From: " + m.cfg.From + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n\r\n" +
		strings.ReplaceAll(body, "\n", "\r\n")
	addr := net.JoinHostPort(m.cfg.Host, m.cfg.Port)
	if err := smtp.SendMail(addr, auth, m.cfg.From, []string{to}, []byte(msg)); err != nil {
		return fmt.Errorf("send mail: %w", err)
	}
	return nil
}
