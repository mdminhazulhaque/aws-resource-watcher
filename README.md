# AWS Resource Watcher

A Go daemon that continuously monitors AWS resources across all regions and sends notifications when resources are added or removed.

## Features

- **Multi-Region Monitoring**: Monitors AWS resources across all regions or specific regions
- **Change Detection**: Detects when resources are added or removed from your AWS account
- **Multiple Notification Channels**: Supports email notifications
- **Redis Storage**: Uses Redis to store previous resource states for comparison
- **Docker Support**: Runs in Docker containers for easy deployment
- **Configurable**: Highly configurable through environment variables
- **IAM Role Support**: Supports both access keys and IAM roles for AWS authentication

## Architecture

The daemon follows this flow:

1. **Account Discovery**: Gets AWS account ID using STS GetCallerIdentity API
2. **Region Discovery**: Fetches all AWS regions using EC2 DescribeRegions API
3. **Resource Discovery**: Uses ResourceGroupsTaggingAPI GetResources to fetch all resource ARNs
4. **Storage**: Stores resource ARNs in Redis with account ID as the key
5. **Comparison**: Compares current resources with previously stored resources
6. **Notification**: Sends notifications for new/removed resources via email
7. **Update**: Updates Redis with the new resource list
8. **Loop**: Repeats the process after a configurable sleep interval

## Prerequisites

- Go 1.21 or higher
- Redis server
- AWS credentials (access keys or IAM role)
- Docker (optional, for containerized deployment)

## Installation

### Local Development

1. Clone the repository:
```bash
git clone <repository-url>
cd aws-resource-watcher
```

2. Install dependencies:
```bash
go mod download
```

3. Copy environment configuration:
```bash
cp .env.example .env
```

4. Edit `.env` file with your configuration
5. Build and run:
```bash
go build -o aws-resource-watcher ./cmd
./aws-resource-watcher
```

### Docker Deployment

1. Copy environment configuration:
```bash
cp .env.example .env
```

2. Edit `.env` file with your configuration
3. Run with Docker Compose:
```bash
docker-compose up -d
```

## Configuration

All configuration is done through environment variables:

### AWS Configuration

The daemon automatically detects AWS credentials in the following order:
1. **Environment variables** (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`)
2. **AWS shared credentials file** (`~/.aws/credentials`)
3. **AWS shared config file** (`~/.aws/config`)
4. **IAM roles for tasks** (ECS)
5. **IAM roles for EC2 instances**

You can also explicitly provide credentials:

| Variable | Description | Required |
|----------|-------------|----------|
| `AWS_REGION` | Default AWS region | No (default: us-east-1) |
| `AWS_ACCESS_KEY_ID` | AWS Access Key ID | No* |
| `AWS_SECRET_ACCESS_KEY` | AWS Secret Access Key | No* |
| `AWS_ROLE_ARN` | IAM Role ARN to assume | No* |

*All AWS credential variables are optional if the daemon can auto-detect credentials

### Region Configuration

| Variable | Description | Required |
|----------|-------------|----------|
| `REGIONS_INCLUDE` | Comma-separated list of regions to include | No |
| `REGIONS_EXCLUDE` | Comma-separated list of regions to exclude | No |

### Storage Configuration

| Variable | Description | Required |
|----------|-------------|----------|
| `REDIS_URI` | Redis connection URI | Yes |

### Monitoring Configuration

| Variable | Description | Required |
|----------|-------------|----------|
| `SLEEP_INTERVAL_SECONDS` | Sleep interval between checks in seconds | No (default: 300) |

### Notification Configuration

#### Email via SMTP

| Variable | Description | Required |
|----------|-------------|----------|
| `SMTP_HOST` | SMTP server hostname | No |
| `SMTP_PORT` | SMTP server port | No (default: 587) |
| `SMTP_USERNAME` | SMTP username | No |
| `SMTP_PASSWORD` | SMTP password | No |
| `SMTP_FROM_EMAIL` | From email address | No |
| `SMTP_TO_EMAILS` | Comma-separated list of recipient emails | No |
| `SMTP_USE_TLS` | Use TLS for SMTP connection | No (default: true) |

Email notification must be configured.

## AWS Permissions

The AWS credentials/role must have the following permissions:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "sts:GetCallerIdentity",
                "ec2:DescribeRegions",
                "resourcegroupstaggingapi:GetResources"
            ],
            "Resource": "*"
        }
    ]
}
```

## Notification Format

### Email

Sends an HTML email with a formatted list of added and removed resources.

## First Run Behavior

On the first run, the daemon will:
1. Discover all current resources
2. Store them in Redis
3. **Not send any notifications** (since there's no previous state to compare against)

Subsequent runs will compare against the stored state and send notifications for changes.

## Logging

The daemon uses structured JSON logging with the following levels:
- `INFO`: Normal operation logs
- `ERROR`: Error conditions
- `WARN`: Warning conditions

Logs include timestamps, levels, and contextual information.

## Development

### Project Structure

```
├── cmd/
│   └── main.go              # Application entry point
├── internal/
│   ├── aws/
│   │   └── client.go        # AWS SDK client wrapper
│   ├── config/
│   │   └── config.go        # Configuration management
│   ├── notifier/
│   │   └── notifier.go      # Notification handling
│   ├── storage/
│   │   └── redis.go         # Redis storage implementation
│   └── watcher/
│       └── watcher.go       # Main watcher logic
├── Dockerfile               # Docker container definition
├── docker-compose.yml       # Docker Compose configuration
├── go.mod                   # Go module definition
├── go.sum                   # Go module checksums
└── README.md               # This file
```

### Building

```bash
# Build for current platform
go build -o aws-resource-watcher ./cmd

# Build for Linux (for Docker)
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o aws-resource-watcher ./cmd
```

## CI/CD

### GitHub Actions

The project includes a GitHub Actions workflow (`.github/workflows/docker.yml`) that automatically:

1. **Runs tests** on every push and pull request
2. **Builds Docker images** for multiple architectures
3. **Pushes to GitHub Container Registry** (ghcr.io) on:
   - Push to `main` branch (tagged as `latest`)
   - Push to `develop` branch
   - Git tags (semantic versioning: `v1.0.0`, `v1.0`, `v1`)
4. **Generates build attestations** for supply chain security

### Workflow Triggers

- **Push to main/develop**: Builds and pushes images
- **Git tags** (v*): Creates versioned releases  
- **Pull requests**: Builds images (but doesn't push)

### Container Registry

Images are published to: `ghcr.io/mdminhazulhaque/aws-resource-watcher`

## Troubleshooting

### Common Issues

1. **AWS Credentials**: Ensure AWS credentials are properly configured
2. **Redis Connection**: Verify Redis server is running and accessible
3. **Network Access**: Ensure the application can reach AWS APIs and Redis
4. **Permissions**: Verify AWS permissions include all required actions

### Logs

Check application logs for detailed error messages:

```bash
# Docker Compose
docker-compose logs aws-resource-watcher

# Direct execution
./aws-resource-watcher
```

## Kubernetes Deployment

### Prerequisites

- EKS cluster or any Kubernetes cluster with AWS access
- `kubectl` configured to access your cluster
- `eksctl` (for EKS service account creation)

### Step 1: Create AWS Service Account (EKS)

For EKS clusters, create an IAM service account with the required permissions:

```bash
eksctl create iamserviceaccount \
    --cluster mycluster \
    --namespace myapp \
    --name aws-resource-watcher \
    --attach-policy-arn arn:aws:iam::aws:policy/ResourceGroupsandTagEditorReadOnlyAccess \
    --region us-west-2 \
    --approve
```

### Step 2: Create Namespace

```bash
kubectl apply -f k8s/namespace.yaml
```

### Step 3: Configure Application

Update the ConfigMap and Secret in `k8s/configmap.yaml` with your environment-specific values, particularly:
- SMTP configuration
- AWS credentials (for non-EKS clusters)
- Region settings
- ARN ignore patterns

### Step 4: Deploy All Resources

#### Option 1: Using Kustomize (Recommended)

```bash
kubectl apply -k k8s/
```

#### Option 2: Deploy Individual Manifests

```bash
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/redis-statefulset.yaml
kubectl apply -f k8s/app-statefulset.yaml
```

### Step 5: Verify Deployment

```bash
# Check pod status
kubectl get pods -n myapp

# Check logs
kubectl logs -f statefulset/aws-resource-watcher -n myapp

# Check Redis logs
kubectl logs -f statefulset/aws-resource-watcher-redis -n myapp
```

### Kubernetes Manifests

The Kubernetes manifests are located in the `k8s/` directory:

- `namespace.yaml` - Namespace creation
- `configmap.yaml` - Environment configuration and secrets
- `redis-statefulset.yaml` - Redis database with persistent storage
- `app-statefulset.yaml` - AWS Resource Watcher application
- `kustomization.yaml` - Kustomize configuration for easy deployment

### Important Notes for Kubernetes

1. **Storage**: Redis uses a PersistentVolumeClaim for data persistence
2. **Security**: Runs with non-root user and security contexts
3. **Resource Limits**: Configured with appropriate CPU and memory limits
4. **Health Checks**: Includes liveness and readiness probes
5. **Service Account**: Uses IAM service account for AWS access (EKS)
6. **Secrets**: SMTP password and AWS credentials stored in Kubernetes secrets

### Building Docker Image

```bash
# Build the Docker image locally
docker build -t aws-resource-watcher:latest .

# For Kubernetes deployment, tag and push to your registry
docker tag aws-resource-watcher:latest your-registry/aws-resource-watcher:latest
docker push your-registry/aws-resource-watcher:latest
```

### Using Pre-built Images

Pre-built Docker images are automatically published to GitHub Container Registry via GitHub Actions:

```bash
# Pull the latest image
docker pull ghcr.io/mdminhazulhaque/aws-resource-watcher:latest

# Use in Docker Compose (update docker-compose.yml)
# Replace 'build: .' with 'image: ghcr.io/mdminhazulhaque/aws-resource-watcher:latest'

# Use in Kubernetes (update k8s/app-statefulset.yaml)
# Set image: ghcr.io/mdminhazulhaque/aws-resource-watcher:latest
```

Available tags:
- `latest` - Latest build from main branch
- `v1.0.0` - Specific version tags
- `main` - Latest build from main branch
- `develop` - Latest build from develop branch

Update the image reference in `k8s/app-statefulset.yaml` to point to the published image.

## License

[Add your license here]

## Contributing

[Add contributing guidelines here]
