package notifier

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
	log "github.com/sirupsen/logrus"
	"gopkg.in/gomail.v2"
)

// Notifier handles sending notifications
type Notifier struct {
	mailDriver   string
	smtpConfig   *SMTPConfig
	sesClient    *ses.Client
	emailConfig  *EmailConfig
}

// SMTPConfig holds SMTP configuration
type SMTPConfig struct {
	Host      string
	Port      int
	Username  string
	Password  string
	UseTLS    bool
}

// EmailConfig holds common email configuration
type EmailConfig struct {
	FromEmail  string
	Recipients []string
}

// ResourceChange represents a change in AWS resources
type ResourceChange struct {
	AccountID        string    `json:"account_id"`
	Timestamp        time.Time `json:"timestamp"`
	AddedResources   []string  `json:"added_resources,omitempty"`
	RemovedResources []string  `json:"removed_resources,omitempty"`
}

// NewNotifier creates a new notifier
func NewNotifier(mailDriver string, smtpConfig *SMTPConfig, sesClient *ses.Client, emailConfig *EmailConfig) *Notifier {
	return &Notifier{
		mailDriver:  mailDriver,
		smtpConfig:  smtpConfig,
		sesClient:   sesClient,
		emailConfig: emailConfig,
	}
}

// SendNotification sends a notification about resource changes
func (n *Notifier) SendNotification(ctx context.Context, change ResourceChange) error {
	log.Infof("Sending notification for account %s using %s driver", change.AccountID, n.mailDriver)

	var err error
	switch n.mailDriver {
	case "ses":
		err = n.sendSESEmail(ctx, &change)
	case "smtp":
		err = n.sendSMTPEmail(&change)
	default:
		return fmt.Errorf("unsupported mail driver: %s", n.mailDriver)
	}

	if err != nil {
		log.Errorf("Failed to send email notification: %v", err)
		return err
	}

	log.Info("Email notification sent successfully")
	return nil
}

// sendSMTPEmail sends an email notification via SMTP
func (n *Notifier) sendSMTPEmail(change *ResourceChange) error {
	if n.smtpConfig == nil || n.emailConfig == nil {
		return fmt.Errorf("SMTP configuration not provided")
	}

	subject := fmt.Sprintf("AWS Resource Changes Detected - Account %s", change.AccountID)
	body := n.buildEmailBody(change)

	m := gomail.NewMessage()
	m.SetHeader("From", n.emailConfig.FromEmail)
	m.SetHeader("To", n.emailConfig.Recipients...)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", body)

	d := gomail.NewDialer(n.smtpConfig.Host, n.smtpConfig.Port, n.smtpConfig.Username, n.smtpConfig.Password)

	if n.smtpConfig.UseTLS {
		d.TLSConfig = &tls.Config{ServerName: n.smtpConfig.Host}
	}

	if err := d.DialAndSend(m); err != nil {
		return fmt.Errorf("failed to send SMTP email: %w", err)
	}

	log.Infof("SMTP email notification sent successfully to %v", n.emailConfig.Recipients)
	return nil
}

// sendSESEmail sends an email notification via AWS SES
func (n *Notifier) sendSESEmail(ctx context.Context, change *ResourceChange) error {
	if n.sesClient == nil || n.emailConfig == nil {
		return fmt.Errorf("SES client or email configuration not provided")
	}

	subject := fmt.Sprintf("AWS Resource Changes Detected - Account %s", change.AccountID)
	body := n.buildEmailBody(change)

	input := &ses.SendEmailInput{
		Source: aws.String(n.emailConfig.FromEmail),
		Destination: &types.Destination{
			ToAddresses: n.emailConfig.Recipients,
		},
		Message: &types.Message{
			Subject: &types.Content{
				Data: aws.String(subject),
			},
			Body: &types.Body{
				Html: &types.Content{
					Data: aws.String(body),
				},
			},
		},
	}

	_, err := n.sesClient.SendEmail(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to send SES email: %w", err)
	}

	log.Infof("SES email notification sent successfully to %v", n.emailConfig.Recipients)
	return nil
}

// sendEmail sends an email notification (deprecated, kept for backward compatibility)
func (n *Notifier) sendEmail(change *ResourceChange) error {
	return n.sendSMTPEmail(change)
}

// buildEmailBody builds the HTML email body
func (n *Notifier) buildEmailBody(change *ResourceChange) string {
	html := fmt.Sprintf(`
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; }
        .header { background-color: #f8f9fa; padding: 20px; border-radius: 5px; }
        .content { margin: 20px 0; }
        .resource-list { background-color: #f8f9fa; padding: 10px; border-radius: 5px; margin: 10px 0; }
        .added { border-left: 4px solid #28a745; }
        .removed { border-left: 4px solid #dc3545; }
        .arn { font-family: monospace; font-size: 12px; }
    </style>
</head>
<body>
    <div class="header">
        <h2>AWS Resource Changes Detected</h2>
        <p><strong>Account ID:</strong> %s</p>
        <p><strong>Timestamp:</strong> %s</p>
    </div>
    
    <div class="content">
`, change.AccountID, change.Timestamp.Format(time.RFC3339))

	if len(change.AddedResources) > 0 {
		html += fmt.Sprintf(`
        <h3>Added Resources (%d)</h3>
        <div class="resource-list added">
`, len(change.AddedResources))
		for _, arn := range change.AddedResources {
			html += fmt.Sprintf(`            <div class="arn">%s</div>`, arn)
		}
		html += `        </div>`
	}

	if len(change.RemovedResources) > 0 {
		html += fmt.Sprintf(`
        <h3>Removed Resources (%d)</h3>
        <div class="resource-list removed">
`, len(change.RemovedResources))
		for _, arn := range change.RemovedResources {
			html += fmt.Sprintf(`            <div class="arn">%s</div>`, arn)
		}
		html += `        </div>`
	}

	html += `
    </div>
</body>
</html>`

	return html
}
