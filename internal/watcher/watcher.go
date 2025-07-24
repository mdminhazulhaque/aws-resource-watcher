package watcher

import (
	"aws-resource-watcher/internal/aws"
	"aws-resource-watcher/internal/config"
	"aws-resource-watcher/internal/notifier"
	"aws-resource-watcher/internal/storage"
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// Watcher monitors AWS resources for changes
type Watcher struct {
	config    *config.Config
	awsClient *aws.Client
	storage   *storage.RedisStorage
	notifier  *notifier.Notifier
	stop      chan struct{}
}

// New creates a new watcher instance
func New(cfg *config.Config) (*Watcher, error) {
	// Create AWS client
	awsClient, err := aws.NewClient(
		context.Background(),
		cfg.AWSAccessKey,
		cfg.AWSSecretKey,
		cfg.AWSRoleARN,
		cfg.AWSRegion,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS client: %w", err)
	}

	// Create Redis storage client
	redisStorage, err := storage.NewRedisStorage(cfg.RedisURI)
	if err != nil {
		return nil, fmt.Errorf("failed to create Redis storage: %w", err)
	}

	// Create notifier
	var smtpConfig *notifier.SMTPConfig
	if cfg.SMTPHost != "" {
		smtpConfig = &notifier.SMTPConfig{
			Host:      cfg.SMTPHost,
			Port:      cfg.SMTPPort,
			Username:  cfg.SMTPUsername,
			Password:  cfg.SMTPPassword,
			FromEmail: cfg.SMTPFromEmail,
			ToEmails:  cfg.SMTPToEmails,
			UseTLS:    cfg.SMTPUseTLS,
		}
	}

	notifierInstance := notifier.NewNotifier(smtpConfig)

	return &Watcher{
		config:    cfg,
		awsClient: awsClient,
		storage:   redisStorage,
		notifier:  notifierInstance,
		stop:      make(chan struct{}),
	}, nil
}

// Start starts the watcher
func (w *Watcher) Start(ctx context.Context) error {
	log.Info("Starting AWS Resource Watcher")

	// Get account ID
	accountID, err := w.awsClient.GetAccountID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get account ID: %w", err)
	}

	log.Infof("Monitoring AWS account: %s", accountID)

	// Get regions to monitor
	regions, err := w.getRegionsToMonitor(ctx)
	if err != nil {
		return fmt.Errorf("failed to get regions to monitor: %w", err)
	}

	log.Infof("Monitoring regions: %v", regions)

	// Main monitoring loop
	ticker := time.NewTicker(w.config.SleepInterval)
	defer ticker.Stop()

	// Run initial check
	if err := w.checkResources(ctx, accountID, regions); err != nil {
		log.Errorf("Initial resource check failed: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-w.stop:
			return nil
		case <-ticker.C:
			if err := w.checkResources(ctx, accountID, regions); err != nil {
				log.Errorf("Resource check failed: %v", err)
			}
		}
	}
}

// Stop stops the watcher
func (w *Watcher) Stop() {
	close(w.stop)
	if w.storage != nil {
		w.storage.Close()
	}
	log.Info("Watcher stopped")
}

// getRegionsToMonitor returns the list of regions to monitor
func (w *Watcher) getRegionsToMonitor(ctx context.Context) ([]string, error) {
	allRegions, err := w.awsClient.GetAllRegions(ctx)
	if err != nil {
		return nil, err
	}

	var regions []string

	if len(w.config.RegionsInclude) > 0 {
		// Use only included regions
		includeMap := make(map[string]bool)
		for _, region := range w.config.RegionsInclude {
			includeMap[region] = true
		}

		for _, region := range allRegions {
			if includeMap[region] {
				regions = append(regions, region)
			}
		}
	} else {
		// Use all regions except excluded ones
		excludeMap := make(map[string]bool)
		for _, region := range w.config.RegionsExclude {
			excludeMap[region] = true
		}

		for _, region := range allRegions {
			if !excludeMap[region] {
				regions = append(regions, region)
			}
		}
	}

	if len(regions) == 0 {
		return nil, fmt.Errorf("no regions to monitor")
	}

	return regions, nil
}

// checkResources checks for resource changes
func (w *Watcher) checkResources(ctx context.Context, accountID string, regions []string) error {
	log.Info("Checking for resource changes...")

	// Get current resources from all regions
	currentARNs, err := w.getAllResourceARNs(ctx, regions)
	if err != nil {
		return fmt.Errorf("failed to get current resource ARNs: %w", err)
	}

	log.Infof("Found %d resources across all regions", len(currentARNs))

	// Check if this is the first run
	isFirstRun, err := w.storage.IsFirstRun(ctx, accountID)
	if err != nil {
		return fmt.Errorf("failed to check if first run: %w", err)
	}

	if isFirstRun {
		log.Info("First run detected, storing current resources without notifications")
		if err := w.storage.SetResourceARNs(ctx, accountID, currentARNs); err != nil {
			return fmt.Errorf("failed to store initial resource ARNs: %w", err)
		}
		return nil
	}

	// Get previous resources from storage
	previousARNs, err := w.storage.GetResourceARNs(ctx, accountID)
	if err != nil {
		return fmt.Errorf("failed to get previous resource ARNs: %w", err)
	}

	// Compare resources and find changes
	addedResources, removedResources := w.compareResources(previousARNs, currentARNs)

	if len(addedResources) > 0 || len(removedResources) > 0 {
		log.Infof("Resource changes detected: %d added, %d removed", len(addedResources), len(removedResources))

		// Send notification
		change := &notifier.ResourceChange{
			AccountID:        accountID,
			Timestamp:        time.Now(),
			AddedResources:   addedResources,
			RemovedResources: removedResources,
		}

		if err := w.notifier.SendNotification(*change); err != nil {
			log.Errorf("Failed to send notification: %v", err)
		}
	} else {
		log.Info("No resource changes detected")
	}

	// Update storage with current resources
	if err := w.storage.SetResourceARNs(ctx, accountID, currentARNs); err != nil {
		return fmt.Errorf("failed to update resource ARNs in storage: %w", err)
	}

	return nil
}

// getAllResourceARNs gets all resource ARNs from all regions
func (w *Watcher) getAllResourceARNs(ctx context.Context, regions []string) ([]string, error) {
	var allARNs []string

	for _, region := range regions {
		log.Infof("Fetching resources from region: %s", region)
		
		arns, err := w.awsClient.GetResourceARNs(ctx, region)
		if err != nil {
			log.Errorf("Failed to get resources from region %s: %v", region, err)
			continue // Continue with other regions
		}

		// Filter out ARNs that match ignore patterns
		filteredARNs := w.filterARNs(arns)
		ignoredCount := len(arns) - len(filteredARNs)
		
		log.Infof("Found %d resources in region %s (%d filtered out)", len(filteredARNs), region, ignoredCount)
		allARNs = append(allARNs, filteredARNs...)
	}

	// Sort ARNs for consistent comparison
	sort.Strings(allARNs)
	return allARNs, nil
}

// compareResources compares two sets of resource ARNs and returns added and removed resources
func (w *Watcher) compareResources(previous, current []string) (added, removed []string) {
	previousSet := make(map[string]bool)
	for _, arn := range previous {
		previousSet[arn] = true
	}

	currentSet := make(map[string]bool)
	for _, arn := range current {
		currentSet[arn] = true
	}

	// Find added resources
	for _, arn := range current {
		if !previousSet[arn] {
			added = append(added, arn)
		}
	}

	// Find removed resources
	for _, arn := range previous {
		if !currentSet[arn] {
			removed = append(removed, arn)
		}
	}

	return added, removed
}

// filterARNs filters out ARNs that match the ignore patterns using AWS ARN matching logic
func (w *Watcher) filterARNs(arns []string) []string {
	if len(w.config.ARNIgnorePatterns) == 0 {
		return arns // No patterns to filter, return all ARNs
	}

	var filteredARNs []string
	for _, arn := range arns {
		shouldIgnore := false
		for _, pattern := range w.config.ARNIgnorePatterns {
			if w.matchesARNPattern(arn, pattern) {
				log.Debugf("Ignoring ARN %s (matches pattern: %s)", arn, pattern)
				shouldIgnore = true
				break
			}
		}
		
		if !shouldIgnore {
			filteredARNs = append(filteredARNs, arn)
		}
	}
	
	return filteredARNs
}

// matchesARNPattern checks if an ARN matches an ARN pattern using AWS ARN matching rules
// ARN format: arn:partition:service:region:account-id:resource-type/resource-id
// Empty fields in pattern (or just colons) match any value in that position
func (w *Watcher) matchesARNPattern(arn, pattern string) bool {
	// Split both ARN and pattern by colons
	arnParts := strings.Split(arn, ":")
	patternParts := strings.Split(pattern, ":")
	
	// Both must have at least 6 parts to be valid ARNs
	if len(arnParts) < 6 || len(patternParts) < 6 {
		log.Warnf("Invalid ARN format - ARN: %s, Pattern: %s", arn, pattern)
		return false
	}
	
	// Check each field: arn, partition, service, region, account-id, resource
	for i := 0; i < 6; i++ {
		// Empty pattern field or "*" matches any value
		if patternParts[i] == "" || patternParts[i] == "*" {
			continue
		}
		
		// For resource field (index 5), handle resource-type/resource-id or resource-type:resource-id
		if i == 5 {
			return w.matchesResourcePattern(arnParts[i], patternParts[i])
		}
		
		// Exact match required for other fields
		if arnParts[i] != patternParts[i] {
			return false
		}
	}
	
	return true
}

// matchesResourcePattern handles the resource part of ARN which can be:
// - resource-type/resource-id
// - resource-type:resource-id  
// - just resource-type
func (w *Watcher) matchesResourcePattern(arnResource, patternResource string) bool {
	// If pattern ends with /*, it matches any resource of that type
	if strings.HasSuffix(patternResource, "/*") {
		resourceType := strings.TrimSuffix(patternResource, "/*")
		return strings.HasPrefix(arnResource, resourceType+"/") || strings.HasPrefix(arnResource, resourceType+":")
	}
	
	// If pattern ends with :*, it matches any resource of that type
	if strings.HasSuffix(patternResource, ":*") {
		resourceType := strings.TrimSuffix(patternResource, ":*")
		return strings.HasPrefix(arnResource, resourceType+":") || strings.HasPrefix(arnResource, resourceType+"/")
	}
	
	// If pattern is just *, match anything
	if patternResource == "*" {
		return true
	}
	
	// Exact match
	return arnResource == patternResource
}
