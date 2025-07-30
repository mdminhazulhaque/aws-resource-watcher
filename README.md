# AWS Resource Watcher

A Go daemon that continuously monitors AWS resources across all regions and sends notifications when resources are added or removed.

## Configuration

All configuration is done through environment variables:

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| **AWS Configuration** ||||
| `AWS_REGION` | Default AWS region | No | us-east-1 |
| `AWS_ACCESS_KEY_ID` | AWS Access Key ID | No* | - |
| `AWS_SECRET_ACCESS_KEY` | AWS Secret Access Key | No* | - |
| `AWS_ROLE_ARN` | IAM Role ARN to assume | No* | - |
| **Region Configuration** ||||
| `REGIONS_INCLUDE` | Comma-separated list of regions to include | No | - |
| `REGIONS_EXCLUDE` | Comma-separated list of regions to exclude | No | - |
| **Storage Configuration** ||||
| `REDIS_URI` | Redis connection URI | Yes | - |
| **Monitoring Configuration** ||||
| `SLEEP_INTERVAL_SECONDS` | Sleep interval between checks in seconds | No | 300 |
| `ARN_IGNORE_PATTERNS` | Comma-separated ARN patterns to ignore | No | - |
| **Email Configuration** ||||
| `MAIL_DRIVER` | Email delivery method (`smtp` or `ses`) | No | smtp |
| `MAIL_FROM` | From email address | Yes | - |
| `MAIL_RECIPIENTS` | Comma-separated list of recipient emails | Yes | - |
| `MAIL_REGION` | AWS region for SES (when using SES driver) | No | AWS_REGION |
| **SMTP Configuration** (when MAIL_DRIVER=smtp) ||||
| `SMTP_HOST` | SMTP server hostname | Yes | - |
| `SMTP_PORT` | SMTP server port | No | 587 |
| `SMTP_USERNAME` | SMTP username | Yes | - |
| `SMTP_PASSWORD` | SMTP password | Yes | - |
| `SMTP_USE_TLS` | Use TLS for SMTP connection | No | true |

*AWS credential variables are optional if the daemon can auto-detect credentials (EC2 roles, EKS IRSA, etc.)

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
        },
        {
            "Effect": "Allow",
            "Action": [
                "ses:SendEmail",
                "ses:SendRawEmail"
            ],
            "Resource": "*"
        }
    ]
}
```

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

## License

[Add your license here]

## Contributing

[Add contributing guidelines here]
