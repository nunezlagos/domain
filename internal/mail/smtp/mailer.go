// Package smtpmail — HU-20.2 SMTP mailer real.
//
// Implementa interfaces invite.Mailer y otp.Mailer usando net/smtp stdlib.
// Soporta plain auth + TLS (STARTTLS opcional). Config via Config struct.
//
// Para Mailpit dev: SMTPAuth="none", SMTPTLS=false.
// Para SES/Mailgun prod: SMTPAuth="plain", SMTPTLS=true.
package smtpmail

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

type Config struct {
	Host     string
	Port     int
	Auth     string // "none" | "plain" | "login" | "cram-md5"
	User     string
	Password string
	UseTLS   bool   // STARTTLS si true
	From     string
	Timeout  time.Duration
}

type Mailer struct {
	cfg Config
}

func New(cfg Config) *Mailer {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.From == "" {
		cfg.From = "noreply@domain.local"
	}
	return &Mailer{cfg: cfg}
}

// Send envía un email plaintext. Compatible con invite.Mailer signature.
func (m *Mailer) Send(ctx context.Context, to, subject, body string) error {
	if to == "" {
		return errors.New("smtp: to required")
	}
	addr := fmt.Sprintf("%s:%d", m.cfg.Host, m.cfg.Port)
	msg := buildMessage(m.cfg.From, to, subject, body)

	// Si UseTLS, conectar STARTTLS-style
	if m.cfg.UseTLS {
		return m.sendTLS(ctx, addr, to, msg)
	}
	return m.sendPlain(ctx, addr, to, msg)
}

// SendOTP — adapter para otp.Mailer interface.
func (m *Mailer) SendOTP(ctx context.Context, to, code string, expiresIn time.Duration) error {
	subject := "Tu código de acceso a Domain"
	body := fmt.Sprintf("Tu código: %s\nVence en: %s\n\nSi no lo solicitaste, ignorá este correo.",
		code, expiresIn.String())
	return m.Send(ctx, to, subject, body)
}

func (m *Mailer) sendPlain(ctx context.Context, addr, to, msg string) error {
	dialer := &net.Dialer{Timeout: m.cfg.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	defer conn.Close()

	c, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer c.Close()

	if m.cfg.Auth == "plain" && m.cfg.User != "" {
		auth := smtp.PlainAuth("", m.cfg.User, m.cfg.Password, m.cfg.Host)
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	return sendBody(c, m.cfg.From, to, msg)
}

func (m *Mailer) sendTLS(ctx context.Context, addr, to, msg string) error {
	dialer := &net.Dialer{Timeout: m.cfg.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	defer conn.Close()

	c, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer c.Close()
	if err := c.StartTLS(&tls.Config{ServerName: m.cfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
		return fmt.Errorf("starttls: %w", err)
	}
	if m.cfg.Auth == "plain" && m.cfg.User != "" {
		auth := smtp.PlainAuth("", m.cfg.User, m.cfg.Password, m.cfg.Host)
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	return sendBody(c, m.cfg.From, to, msg)
}

func sendBody(c *smtp.Client, from, to, msg string) error {
	if err := c.Mail(from); err != nil {
		return fmt.Errorf("MAIL FROM: %w", err)
	}
	if err := c.Rcpt(to); err != nil {
		return fmt.Errorf("RCPT TO: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("DATA: %w", err)
	}
	if _, err := w.Write([]byte(msg)); err != nil {
		return fmt.Errorf("write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("close DATA: %w", err)
	}
	return c.Quit()
}

func buildMessage(from, to, subject, body string) string {
	var sb strings.Builder
	sb.WriteString("From: ")
	sb.WriteString(from)
	sb.WriteString("\r\n")
	sb.WriteString("To: ")
	sb.WriteString(to)
	sb.WriteString("\r\n")
	sb.WriteString("Subject: ")
	sb.WriteString(subject)
	sb.WriteString("\r\n")
	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	sb.WriteString("\r\n")
	sb.WriteString(body)
	return sb.String()
}
