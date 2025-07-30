package aws

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	log "github.com/sirupsen/logrus"
)

// Client wraps AWS SDK clients
type Client struct {
	cfg                         aws.Config
	stsClient                   *sts.Client
	ec2Client                   *ec2.Client
	resourceGroupsTaggingClient *resourcegroupstaggingapi.Client
}

// NewClient creates a new AWS client with automatic credential detection
func NewClient(ctx context.Context, accessKey, secretKey, roleARN, region string) (*Client, error) {
	var cfg aws.Config
	var err error

	// Check if running in Kubernetes with IRSA
	if isRunningInKubernetes() {
		log.Info("Detected Kubernetes environment, using IRSA credentials...")
		cfg, err = autoDetectCredentials(ctx, region)
	} else if accessKey == "" && secretKey == "" && roleARN == "" {
		log.Info("No explicit credentials provided, attempting auto-detection...")
		cfg, err = autoDetectCredentials(ctx, region)
	} else {
		cfg, err = createConfigWithCredentials(ctx, accessKey, secretKey, roleARN, region)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &Client{
		cfg:                         cfg,
		stsClient:                   sts.NewFromConfig(cfg),
		ec2Client:                   ec2.NewFromConfig(cfg),
		resourceGroupsTaggingClient: resourcegroupstaggingapi.NewFromConfig(cfg),
	}, nil
}

// autoDetectCredentials attempts to detect AWS credentials automatically
func autoDetectCredentials(ctx context.Context, region string) (aws.Config, error) {
	log.Info("Auto-detecting AWS credentials...")

	// Check if running in Kubernetes with service account token
	if isRunningInKubernetes() {
		log.Info("Detected Kubernetes environment, configuring for IRSA/service account authentication")
		cfg, err := configureKubernetesCredentials(ctx, region)
		if err != nil {
			log.Warnf("Failed to configure Kubernetes credentials, falling back to default chain: %v", err)
		} else {
			return cfg, nil
		}
	}

	// Try to load default config which uses the AWS credential chain:
	// 1. Environment variables (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
	// 2. AWS shared credentials file (~/.aws/credentials)
	// 3. AWS shared config file (~/.aws/config)
	// 4. IAM roles for tasks (ECS)
	// 5. IAM roles for EC2 instances
	// 6. Web Identity Token (IRSA in EKS)
	cfg, err := config.LoadDefaultConfig(ctx, 
		config.WithRegion(region),
		config.WithEC2IMDSClientEnableState(imds.ClientDisabled), // Disable IMDS in k8s to avoid conflicts
	)
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to load AWS config with default credential chain: %w", err)
	}

	// Test the credentials by attempting to get caller identity
	stsClient := sts.NewFromConfig(cfg)
	result, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to validate AWS credentials: %w", err)
	}

	log.Infof("Successfully detected AWS credentials for account: %s", aws.ToString(result.Account))
	
	// Log the credential source if possible
	logCredentialSource()

	return cfg, nil
}

// createConfigWithCredentials creates AWS config with explicitly provided credentials
func createConfigWithCredentials(ctx context.Context, accessKey, secretKey, roleARN, region string) (aws.Config, error) {
	if roleARN != "" {
		log.Infof("Using IAM role: %s", roleARN)
		// Load default config first
		cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
		if err != nil {
			return aws.Config{}, fmt.Errorf("failed to load default AWS config: %w", err)
		}
		
		// Create STS client for role assumption
		stsClient := sts.NewFromConfig(cfg)
		
		// Use role credentials
		cfg.Credentials = stscreds.NewAssumeRoleProvider(stsClient, roleARN)
		return cfg, nil
	} else if accessKey != "" && secretKey != "" {
		log.Info("Using provided access key credentials")
		// Use access key authentication
		return config.LoadDefaultConfig(ctx,
			config.WithRegion(region),
			config.WithCredentialsProvider(aws.NewCredentialsCache(
				aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
					return aws.Credentials{
						AccessKeyID:     accessKey,
						SecretAccessKey: secretKey,
					}, nil
				}),
			)),
		)
	} else {
		log.Info("Using default credential chain")
		// Use default credential chain (environment, instance profile, etc.)
		return config.LoadDefaultConfig(ctx, config.WithRegion(region))
	}
}

// logCredentialSource logs information about the detected credential source
func logCredentialSource() {
	// Check various credential sources and log what was found
	if isRunningInKubernetes() {
		log.Info("Credential source: IRSA (IAM Roles for Service Accounts) in Kubernetes")
	} else if os.Getenv("AWS_ACCESS_KEY_ID") != "" {
		log.Info("Credential source: Environment variables (AWS_ACCESS_KEY_ID)")
	} else if os.Getenv("AWS_PROFILE") != "" {
		log.Infof("Credential source: AWS Profile (%s)", os.Getenv("AWS_PROFILE"))
	} else if fileExists(os.Getenv("HOME") + "/.aws/credentials") {
		log.Info("Credential source: AWS shared credentials file (~/.aws/credentials)")
	} else if fileExists(os.Getenv("HOME") + "/.aws/config") {
		log.Info("Credential source: AWS shared config file (~/.aws/config)")
	} else if os.Getenv("AWS_WEB_IDENTITY_TOKEN_FILE") != "" {
		log.Info("Credential source: Web Identity Token (IRSA/Kubernetes Service Account)")
	} else {
		log.Info("Credential source: IAM role or instance profile")
	}
}

// isRunningInKubernetes checks if the application is running in a Kubernetes environment
func isRunningInKubernetes() bool {
	// Check for Kubernetes service account token
	if fileExists("/var/run/secrets/kubernetes.io/serviceaccount/token") {
		return true
	}
	
	// Check for EKS service account token
	if fileExists("/var/run/secrets/eks.amazonaws.com/serviceaccount/token") {
		return true
	}
	
	// Check for Kubernetes environment variables
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		return true
	}
	
	return false
}

// configureKubernetesCredentials configures AWS credentials for Kubernetes environments
func configureKubernetesCredentials(ctx context.Context, region string) (aws.Config, error) {
	// Set environment variables for Web Identity Token if they're not already set
	if os.Getenv("AWS_WEB_IDENTITY_TOKEN_FILE") == "" {
		// Check for EKS service account token first
		if fileExists("/var/run/secrets/eks.amazonaws.com/serviceaccount/token") {
			os.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", "/var/run/secrets/eks.amazonaws.com/serviceaccount/token")
			log.Info("Using EKS service account token at /var/run/secrets/eks.amazonaws.com/serviceaccount/token")
		}
	}
	
	// Set AWS_ROLE_ARN from environment if not set (should be configured via service account annotations)
	roleArn := os.Getenv("AWS_ROLE_ARN")
	if roleArn == "" {
		log.Warn("AWS_ROLE_ARN not set, ensure your Kubernetes service account is annotated with eks.amazonaws.com/role-arn")
	} else {
		log.Infof("Using IAM role from environment: %s", roleArn)
	}
	
	// Load config with Web Identity Token support
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithEC2IMDSClientEnableState(imds.ClientDisabled), // Disable IMDS in k8s
	)
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to load AWS config for Kubernetes: %w", err)
	}
	
	return cfg, nil
}

// fileExists checks if a file exists
func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

// isRunningInKubernetes checks if the application is running in Kubernetes
func isRunningInKubernetes() bool {
	// Check for Kubernetes service account token
	return fileExists("/var/run/secrets/kubernetes.io/serviceaccount/token")
}

// GetAccountID returns the AWS account ID
func (c *Client) GetAccountID(ctx context.Context) (string, error) {
	result, err := c.stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", fmt.Errorf("failed to get caller identity: %w", err)
	}

	return aws.ToString(result.Account), nil
}

// GetConfig returns the AWS configuration
func (c *Client) GetConfig() aws.Config {
	return c.cfg
}

// GetAllRegions returns all AWS regions
func (c *Client) GetAllRegions(ctx context.Context) ([]string, error) {
	result, err := c.ec2Client.DescribeRegions(ctx, &ec2.DescribeRegionsInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to describe regions: %w", err)
	}

	regions := make([]string, len(result.Regions))
	for i, region := range result.Regions {
		regions[i] = aws.ToString(region.RegionName)
	}

	return regions, nil
}

// GetResourceARNs returns all resource ARNs in the specified region
func (c *Client) GetResourceARNs(ctx context.Context, region string) ([]string, error) {
	// Create a context with timeout to prevent hanging
	timeoutCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// Create a new config for the specific region
	regionalCfg := c.cfg.Copy()
	regionalCfg.Region = region

	client := resourcegroupstaggingapi.NewFromConfig(regionalCfg)

	var allARNs []string
	var nextToken *string
	requestCount := 0
	maxRequests := 50 // Prevent infinite loops
	consecutiveEmptyResponses := 0
	maxEmptyResponses := 1 // Stop after 3 consecutive empty responses
	seenARNs := make(map[string]bool) // Track unique ARNs to detect duplicates
	duplicateCount := 0

	for {
		requestCount++
		if requestCount > maxRequests {
			log.Warnf("Reached maximum request limit (%d) for region %s, stopping pagination", maxRequests, region)
			break
		}

		// Limit the page size to prevent hanging on large responses
		resourcesPerPage := int32(100) // AWS limit is 1-100
		input := &resourcegroupstaggingapi.GetResourcesInput{
			PaginationToken:    nextToken,
			ResourcesPerPage:   &resourcesPerPage,
		}

		log.Infof("Making GetResources request #%d for region %s (timeout: 60s)", requestCount, region)
		
		result, err := client.GetResources(timeoutCtx, input)
		if err != nil {
			log.Errorf("GetResources failed for region %s on request #%d: %v", region, requestCount, err)
			return nil, fmt.Errorf("failed to get resources in region %s (request #%d): %w", region, requestCount, err)
		}

		resourceCount := len(result.ResourceTagMappingList)
		log.Infof("Successfully received %d resources in batch #%d for region %s", resourceCount, requestCount, region)

		// Track consecutive empty responses
		if resourceCount == 0 {
			consecutiveEmptyResponses++
			log.Warnf("Received 0 resources for region %s (request #%d), consecutive empty responses: %d", region, requestCount, consecutiveEmptyResponses)
			
			// Stop if we've had too many consecutive empty responses
			if consecutiveEmptyResponses >= maxEmptyResponses {
				log.Warnf("Stopping pagination for region %s after %d consecutive empty responses", region, consecutiveEmptyResponses)
				break
			}
		} else {
			// Reset counter if we got resources
			consecutiveEmptyResponses = 0
		}

		// Track new vs duplicate ARNs
		newARNsInBatch := 0
		duplicatesInBatch := 0
		
		for _, resource := range result.ResourceTagMappingList {
			if resource.ResourceARN != nil {
				arn := aws.ToString(resource.ResourceARN)
				if seenARNs[arn] {
					duplicatesInBatch++
					duplicateCount++
				} else {
					seenARNs[arn] = true
					allARNs = append(allARNs, arn)
					newARNsInBatch++
				}
			}
		}

		log.Infof("Batch #%d for region %s: %d new ARNs, %d duplicates", requestCount, region, newARNsInBatch, duplicatesInBatch)

		// If we got mostly duplicates, it might indicate pagination issues
		if resourceCount > 0 && float64(duplicatesInBatch)/float64(resourceCount) > 0.8 {
			log.Warnf("High duplicate rate (%.1f%%) in batch #%d for region %s, potential pagination issue", 
				100.0*float64(duplicatesInBatch)/float64(resourceCount), requestCount, region)
		}

		// If we got all duplicates and no new resources, stop
		if resourceCount > 0 && newARNsInBatch == 0 {
			log.Warnf("All resources in batch #%d for region %s were duplicates, stopping pagination", requestCount, region)
			break
		}

		nextToken = result.PaginationToken
		if nextToken == nil {
			log.Infof("Pagination complete for region %s after %d requests", region, requestCount)
			break
		}
		
		log.Infof("More resources available, continuing pagination for region %s", region)
	}

	log.Infof("Total resources found in region %s: %d unique ARNs (%d duplicates encountered)", region, len(allARNs), duplicateCount)
	return allARNs, nil
}
