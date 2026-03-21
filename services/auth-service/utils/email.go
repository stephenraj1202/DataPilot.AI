package utils

import (
	"fmt"
	"net/smtp"
	"strings"
)

type EmailConfig struct {
	SMTPHost     string
	SMTPPort     string
	SMTPUsername string
	SMTPPassword string
	FromEmail    string
	SuperAdminCC string
}

type EmailService struct {
	Config EmailConfig
}

// SendVerificationEmail sends an email verification link to the user
func (e *EmailService) SendVerificationEmail(toEmail, verificationToken string) error {
	subject := "Verify Your Email - FinOps Platform"
	verificationLink := fmt.Sprintf("http://localhost:3000/verify-email?token=%s", verificationToken)

	body := fmt.Sprintf(`
Hello,

Thank you for registering with FinOps Platform!

Please verify your email address by clicking the link below:
%s

This link will expire in 24 hours.

If you did not create an account, please ignore this email.

Best regards,
FinOps Platform Team
`, verificationLink)

	return e.sendEmail(toEmail, subject, body)
}

// SendPasswordResetEmail sends a password reset link to the user
func (e *EmailService) SendPasswordResetEmail(toEmail, resetToken string) error {
	subject := "Password Reset Request - FinOps Platform"
	resetLink := fmt.Sprintf("http://localhost:3000/reset-password?token=%s", resetToken)

	body := fmt.Sprintf(`
Hello,

We received a request to reset your password for your FinOps Platform account.

Click the link below to reset your password:
%s

This link will expire in 1 hour.

If you did not request a password reset, please ignore this email and your password will remain unchanged.

Best regards,
FinOps Platform Team
`, resetLink)

	return e.sendEmail(toEmail, subject, body)
}

// sendEmail sends an email using SMTP
func (e *EmailService) sendEmail(to, subject, body string) error {
	// Build email message
	recipients := []string{to}
	if e.Config.SuperAdminCC != "" {
		recipients = append(recipients, e.Config.SuperAdminCC)
	}

	message := e.buildEmailMessage(e.Config.FromEmail, to, e.Config.SuperAdminCC, subject, body)

	// Connect to SMTP server
	addr := fmt.Sprintf("%s:%s", e.Config.SMTPHost, e.Config.SMTPPort)

	// For development/testing without authentication
	// In production, you would use smtp.PlainAuth with credentials
	err := smtp.SendMail(addr, nil, e.Config.FromEmail, recipients, []byte(message))
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

// buildEmailMessage constructs the email message with headers
func (e *EmailService) buildEmailMessage(from, to, cc, subject, body string) string {
	var message strings.Builder

	message.WriteString(fmt.Sprintf("From: %s\r\n", from))
	message.WriteString(fmt.Sprintf("To: %s\r\n", to))

	if cc != "" {
		message.WriteString(fmt.Sprintf("Cc: %s\r\n", cc))
	}

	message.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	message.WriteString("MIME-Version: 1.0\r\n")
	message.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	message.WriteString("\r\n")
	message.WriteString(body)

	return message.String()
}
