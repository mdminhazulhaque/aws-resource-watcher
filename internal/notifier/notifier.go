package notifier

import (
	"crypto/tls"
	"fmt"
	"log"
	"time"
	
	"gopkg.in/gomail.v2"
)

// Notifier handles sending notifications
type Notifier struct {
	smtpConfig *SMTPConfig
}

// SMTPConfig holds SMTP configuration
type SMTPConfig struct {
	Host      string
	Port      int
	Username  string
	Password  string
	FromEmail string
	ToEmails  []string
	UseTLS    bool
}

// ResourceChange represents a change in AWS resources
type ResourceChange struct {
	AccountID        string    `json:"account_id"`
	Timestamp        time.Time `json:"timestamp"`
	AddedResources   []string  `json:"added_resources,omitempty"`
	RemovedResources []string  `json:"removed_resources,omitempty"`
}

// NewNotifier creates a new notifier
func NewNotifier(smtpConfig *SMTPConfig) *Notifier {
	return &Notifier{
		smtpConfig: smtpConfig,
	}
}

// SendNotification sends a notification about resource changes
func (n *Notifier) SendNotification(change ResourceChange) error {
	log.Printf("Sending notification for account %s", change.AccountID)
	
	// Send email notification
	if n.smtpConfig != nil {
		log.Printf("Sending email notification")
		if err := n.sendEmail(&change); err != nil {
			log.Printf("Failed to send email notification: %v", err)
			return err
		}
		log.Printf("Email notification sent successfully")
	}
	
	return nil
}

// sendEmail sends an email notification
func (n *Notifier) sendEmail(change *ResourceChange) error {
	subject := fmt.Sprintf("AWS Resource Changes Detected - Account %s", change.AccountID)
	body := n.buildEmailBody(change)

	m := gomail.NewMessage()
	m.SetHeader("From", n.smtpConfig.FromEmail)
	m.SetHeader("To", n.smtpConfig.ToEmails...)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", body)

	d := gomail.NewDialer(n.smtpConfig.Host, n.smtpConfig.Port, n.smtpConfig.Username, n.smtpConfig.Password)
	
	if n.smtpConfig.UseTLS {
		d.TLSConfig = &tls.Config{ServerName: n.smtpConfig.Host}
	}

	if err := d.DialAndSend(m); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	log.Printf("Email notification sent successfully to %v", n.smtpConfig.ToEmails)
	return nil
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
