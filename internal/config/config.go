package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the AWS resource watcher
type Config struct {
	// AWS Configuration
	AWSRegion    string
	AWSAccessKey string
	AWSSecretKey string
	AWSRoleARN   string

	// Region Configuration
	RegionsInclude []string
	RegionsExclude []string

	// ARN filtering configuration
	ARNIgnorePatterns []string

	// Redis Configuration
	RedisURI string

	// Monitoring Configuration
	SleepInterval time.Duration

	// Email Configuration
	MailDriver      string
	MailRegion      string
	SMTPHost        string
	SMTPPort        int
	SMTPUsername    string
	SMTPPassword    string
	MailFrom        string
	MailRecipients  []string
	SMTPUseTLS      bool
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	// Load .env file if it exists
	_ = godotenv.Load()

	cfg := &Config{}

	// AWS Configuration
	cfg.AWSRegion = getEnvOrDefault("AWS_REGION", "us-east-1")
	cfg.AWSAccessKey = os.Getenv("AWS_ACCESS_KEY_ID")
	cfg.AWSSecretKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
	cfg.AWSRoleARN = os.Getenv("AWS_ROLE_ARN")

	// Region Configuration
	if regionsInclude := os.Getenv("REGIONS_INCLUDE"); regionsInclude != "" {
		cfg.RegionsInclude = strings.Split(regionsInclude, ",")
		for i := range cfg.RegionsInclude {
			cfg.RegionsInclude[i] = strings.TrimSpace(cfg.RegionsInclude[i])
		}
	}

	if regionsExclude := os.Getenv("REGIONS_EXCLUDE"); regionsExclude != "" {
		cfg.RegionsExclude = strings.Split(regionsExclude, ",")
		for i := range cfg.RegionsExclude {
			cfg.RegionsExclude[i] = strings.TrimSpace(cfg.RegionsExclude[i])
		}
	}

	// ARN ignore patterns Configuration
	if arnIgnorePatterns := os.Getenv("ARN_IGNORE_PATTERNS"); arnIgnorePatterns != "" {
		cfg.ARNIgnorePatterns = strings.Split(arnIgnorePatterns, ",")
		for i := range cfg.ARNIgnorePatterns {
			cfg.ARNIgnorePatterns[i] = strings.TrimSpace(cfg.ARNIgnorePatterns[i])
		}
	}

	// Redis Configuration
	cfg.RedisURI = getEnvOrDefault("REDIS_URI", "redis://localhost:6379")

	// Sleep Interval
	sleepIntervalStr := getEnvOrDefault("SLEEP_INTERVAL_SECONDS", "300")
	sleepInterval, err := strconv.Atoi(sleepIntervalStr)
	if err != nil {
		return nil, fmt.Errorf("invalid SLEEP_INTERVAL_SECONDS: %v", err)
	}
	cfg.SleepInterval = time.Duration(sleepInterval) * time.Second

	// Email Configuration
	cfg.MailDriver = getEnvOrDefault("MAIL_DRIVER", "smtp")
	cfg.MailRegion = getEnvOrDefault("MAIL_REGION", cfg.AWSRegion)
	cfg.SMTPHost = os.Getenv("SMTP_HOST")
	cfg.SMTPPort, _ = strconv.Atoi(getEnvOrDefault("SMTP_PORT", "587"))
	cfg.SMTPUsername = os.Getenv("SMTP_USERNAME")
	cfg.SMTPPassword = os.Getenv("SMTP_PASSWORD")
	cfg.MailFrom = os.Getenv("MAIL_FROM")
	cfg.SMTPUseTLS, _ = strconv.ParseBool(getEnvOrDefault("SMTP_USE_TLS", "true"))

	if recipients := os.Getenv("MAIL_RECIPIENTS"); recipients != "" {
		cfg.MailRecipients = strings.Split(recipients, ",")
		for i := range cfg.MailRecipients {
			cfg.MailRecipients[i] = strings.TrimSpace(cfg.MailRecipients[i])
		}
	}

	return cfg, cfg.validate()
}

// validate checks if the required configuration is provided
func (c *Config) validate() error {
	// AWS credentials are now optional as we have auto-detection
	// If provided, we'll use them; otherwise we'll auto-detect
	
	// Check Redis URI
	if c.RedisURI == "" {
		return fmt.Errorf("REDIS_URI is required")
	}

	// Check if notification method is configured
	if c.MailDriver == "" {
		c.MailDriver = "smtp" // default to SMTP
	}

	// Validate email configuration
	if c.MailFrom == "" || len(c.MailRecipients) == 0 {
		return fmt.Errorf("email configuration incomplete: MAIL_FROM and MAIL_RECIPIENTS are required")
	}

	// Validate SMTP configuration if using SMTP driver
	if c.MailDriver == "smtp" {
		if c.SMTPHost == "" || c.SMTPUsername == "" || c.SMTPPassword == "" {
			return fmt.Errorf("incomplete SMTP configuration: SMTP_HOST, SMTP_USERNAME, and SMTP_PASSWORD are required when using SMTP driver")
		}
	}

	// Note: For SES driver, we only need valid AWS credentials (validated elsewhere)
	
	return nil
}

// getEnvOrDefault returns the environment variable value or a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
