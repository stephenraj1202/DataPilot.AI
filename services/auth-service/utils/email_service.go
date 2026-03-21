package utils

import (
	"crypto/tls"
	"database/sql"
	"fmt"
	"log"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// SMTPConfig holds the resolved SMTP settings for sending an email.
type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
	UseTLS   bool
}

// EnhancedEmailService sends emails using per-account custom SMTP when available,
// falling back to the platform default. It CCs the Super_Admin on every email and
// retries up to 3 times with exponential backoff on failure.
type EnhancedEmailService struct {
	DB          *sql.DB
	DefaultSMTP EmailConfig // platform-level defaults from config.ini
	AESKey      string
}

// SendVerificationEmail sends an email verification link.
func (s *EnhancedEmailService) SendVerificationEmail(toEmail, verificationToken string) error {
	subject := "Verify Your Email - FinOps Platform"
	link := fmt.Sprintf("http://localhost:3000/verify-email?token=%s", verificationToken)
	body := fmt.Sprintf(`Hello,

Thank you for registering with FinOps Platform!

Please verify your email address by clicking the link below:
%s

This link will expire in 24 hours.

If you did not create an account, please ignore this email.

Best regards,
FinOps Platform Team`, link)

	return s.send(toEmail, subject, body)
}

// SendOTPEmail sends a 6-digit OTP verification code.
func (s *EnhancedEmailService) SendOTPEmail(toEmail, otp string) error {
	subject := "Your DataPilot.AI Verification Code"
	body := fmt.Sprintf(`Your verification code is:

  %s

This code expires in 10 minutes. Do not share it with anyone.

If you did not request this code, please ignore this email.

— DataPilot.AI Team`, otp)
	return s.send(toEmail, subject, body)
}

// SendPasswordResetEmail sends a password reset link.
func (s *EnhancedEmailService) SendPasswordResetEmail(toEmail, resetToken string) error {
	subject := "Password Reset Request - FinOps Platform"
	link := fmt.Sprintf("http://localhost:3000/reset-password?token=%s", resetToken)
	body := fmt.Sprintf(`Hello,

We received a request to reset your password for your FinOps Platform account.

Click the link below to reset your password:
%s

This link will expire in 1 hour.

If you did not request a password reset, please ignore this email.

Best regards,
FinOps Platform Team`, link)

	return s.send(toEmail, subject, body)
}

// send resolves the SMTP config for the recipient's account (if any), then
// delivers the message with retry logic.
func (s *EnhancedEmailService) send(to, subject, body string) error {
	cfg := s.resolveConfig(to)

	var lastErr error
	delays := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}

	for attempt := 0; attempt <= len(delays); attempt++ {
		if attempt > 0 {
			time.Sleep(delays[attempt-1])
		}

		if err := s.deliver(cfg, to, subject, body); err != nil {
			lastErr = err
			log.Printf("[email] delivery attempt %d/%d failed to %s: %v",
				attempt+1, len(delays)+1, to, err)
			continue
		}
		return nil
	}

	log.Printf("[email] all delivery attempts failed for %s: %v", to, lastErr)
	return fmt.Errorf("email delivery failed after %d attempts: %w", len(delays)+1, lastErr)
}

// resolveConfig looks up a custom SMTP config for the account that owns toEmail.
// Falls back to the platform default when none is found.
func (s *EnhancedEmailService) resolveConfig(toEmail string) SMTPConfig {
	if s.DB != nil {
		cfg, err := s.lookupCustomSMTP(toEmail)
		if err == nil {
			return cfg
		}
	}
	return SMTPConfig{
		Host:     s.DefaultSMTP.SMTPHost,
		Port:     s.DefaultSMTP.SMTPPort,
		Username: s.DefaultSMTP.SMTPUsername,
		Password: s.DefaultSMTP.SMTPPassword,
		From:     s.DefaultSMTP.FromEmail,
		UseTLS:   false,
	}
}

// lookupCustomSMTP queries mail_settings for the account that owns toEmail.
func (s *EnhancedEmailService) lookupCustomSMTP(toEmail string) (SMTPConfig, error) {
	var host, username, encPwd, fromEmail string
	var port int
	var useTLS bool

	err := s.DB.QueryRow(`
		SELECT ms.smtp_host, ms.smtp_port, ms.smtp_username,
		       ms.encrypted_password, ms.from_email, ms.use_tls
		FROM mail_settings ms
		JOIN accounts a ON a.id = ms.account_id
		JOIN users u ON u.account_id = a.id
		WHERE u.email = ?
		LIMIT 1`, toEmail,
	).Scan(&host, &port, &username, &encPwd, &fromEmail, &useTLS)

	if err != nil {
		return SMTPConfig{}, err
	}

	password, err := Decrypt(encPwd, s.AESKey)
	if err != nil {
		return SMTPConfig{}, fmt.Errorf("failed to decrypt SMTP password: %w", err)
	}

	return SMTPConfig{
		Host:     host,
		Port:     fmt.Sprintf("%d", port),
		Username: username,
		Password: password,
		From:     fromEmail,
		UseTLS:   useTLS,
	}, nil
}

// deliver sends the email via SMTP, adding a CC to Super_Admin when configured.
func (s *EnhancedEmailService) deliver(cfg SMTPConfig, to, subject, body string) error {
	recipients := []string{to}
	cc := s.DefaultSMTP.SuperAdminCC
	if cc != "" && cc != to {
		recipients = append(recipients, cc)
	}

	msg := buildMessage(cfg.From, to, cc, subject, body)
	addr := net.JoinHostPort(cfg.Host, cfg.Port)

	if cfg.UseTLS {
		return sendWithTLS(addr, cfg, recipients, msg)
	}

	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	}
	return smtp.SendMail(addr, auth, cfg.From, recipients, []byte(msg))
}

// sendWithTLS dials a TLS connection and sends the message.
func sendWithTLS(addr string, cfg SMTPConfig, recipients []string, msg string) error {
	tlsCfg := &tls.Config{ServerName: cfg.Host}
	conn, err := tls.Dial("tcp", addr, tlsCfg)
	if err != nil {
		return fmt.Errorf("TLS dial failed: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		return fmt.Errorf("SMTP client creation failed: %w", err)
	}
	defer client.Close()

	if cfg.Username != "" {
		auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP auth failed: %w", err)
		}
	}

	if err := client.Mail(cfg.From); err != nil {
		return fmt.Errorf("SMTP MAIL FROM failed: %w", err)
	}
	for _, r := range recipients {
		if err := client.Rcpt(r); err != nil {
			return fmt.Errorf("SMTP RCPT TO %s failed: %w", r, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA failed: %w", err)
	}
	if _, err = fmt.Fprint(w, msg); err != nil {
		return fmt.Errorf("SMTP write failed: %w", err)
	}
	return w.Close()
}

// buildMessage constructs a plain-text email with proper headers.
func buildMessage(from, to, cc, subject, body string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("From: %s\r\n", from))
	b.WriteString(fmt.Sprintf("To: %s\r\n", to))
	if cc != "" && cc != to {
		b.WriteString(fmt.Sprintf("Cc: %s\r\n", cc))
	}
	b.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("\r\n")
	b.WriteString(body)
	return b.String()
}
