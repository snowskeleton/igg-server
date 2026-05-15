package email

import (
	"fmt"
	"log"
	"net"
	"net/smtp"
	"time"

	"github.com/snowskeleton/igg-server/internal/config"
)

type Sender struct {
	cfg *config.Config
}

func NewSender(cfg *config.Config) *Sender {
	return &Sender{cfg: cfg}
}

func (s *Sender) SendMagicLink(to, token string) error {
	link := fmt.Sprintf("%s/v1/auth/verify?token=%s", s.cfg.BaseURL, token)
	subject := "Sign in to I Got Gas"
	body := fmt.Sprintf(`Hello,

Click the link below to sign in to I Got Gas:

%s

This link expires in 15 minutes and can only be used once.

If you didn't request this, you can safely ignore this email.
`, link)

	return s.send(to, subject, body)
}

func (s *Sender) SendShareInvitation(to, fromEmail, carName, token string) error {
	link := fmt.Sprintf("%s/v1/shares/accept?token=%s", s.cfg.BaseURL, token)
	subject := fmt.Sprintf("%s shared a vehicle with you on I Got Gas", fromEmail)
	body := fmt.Sprintf(`Hello,

%s has shared their vehicle "%s" with you on I Got Gas.

Open the I Got Gas app or click the link below to accept:

%s

If you don't have an account, one will be created for you.
`, fromEmail, carName, link)

	return s.send(to, subject, body)
}

func (s *Sender) SendAdminMagicLink(to, token string) error {
	link := fmt.Sprintf("%s/admin/verify?token=%s", s.cfg.BaseURL, token)
	subject := "Admin Login – I Got Gas"
	body := fmt.Sprintf(`Hello,

Click the link below to sign in to the I Got Gas admin dashboard:

%s

This link expires in 15 minutes and can only be used once.

If you didn't request this, you can safely ignore this email.
`, link)

	return s.send(to, subject, body)
}

func (s *Sender) send(to, subject, body string) error {
	if s.cfg.SMTPMock {
		log.Printf("[MOCK EMAIL] To: %s\nSubject: %s\n\n%s\n", to, subject, body)
		return nil
	}

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		s.cfg.SMTPFrom, to, subject, body)

	addr := fmt.Sprintf("%s:%d", s.cfg.SMTPHost, s.cfg.SMTPPort)
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	conn.SetDeadline(time.Now().Add(10 * time.Second))

	c, err := smtp.NewClient(conn, s.cfg.SMTPHost)
	if err != nil {
		conn.Close()
		return fmt.Errorf("smtp client: %w", err)
	}
	defer c.Close()

	auth := smtp.PlainAuth("", s.cfg.SMTPUser, s.cfg.SMTPPass, s.cfg.SMTPHost)
	if err := c.Auth(auth); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}
	if err := c.Mail(s.cfg.SMTPFrom); err != nil {
		return fmt.Errorf("smtp mail: %w", err)
	}
	if err := c.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write([]byte(msg)); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close data: %w", err)
	}
	return c.Quit()
}
