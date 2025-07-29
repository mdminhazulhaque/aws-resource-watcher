<!-- Use this file to provide workspace-specific custom instructions to Copilot. For more details, visit https://code.visualstudio.com/docs/copilot/copilot-customization#_use-a-githubcopilotinstructionsmd-file -->

# AWS Resource Watcher Project

This is a Go daemon project that monitors AWS resources and sends notifications about changes.

## Project Context

- **Language**: Go 1.21+
- **Architecture**: Daemon/service that runs continuously
- **AWS Integration**: Uses AWS SDK for Go v2
- **Storage**: Redis for persistence
- **Notifications**: Email (SMTP or SES)
- **Deployment**: Docker containers

## Code Style Guidelines

- Follow standard Go conventions and formatting
- Use structured logging with logrus
- Handle errors gracefully with proper error wrapping
- Use context.Context for all AWS API calls
- Implement graceful shutdown patterns
- Follow dependency injection patterns for testability

## Key Dependencies

- `github.com/aws/aws-sdk-go-v2/*` - AWS SDK for Go v2
- `github.com/go-redis/redis/v8` - Redis client
- `github.com/sirupsen/logrus` - Structured logging
- `gopkg.in/gomail.v2` - Email sending
- `github.com/joho/godotenv` - Environment variable loading

## Architecture Patterns

- Configuration through environment variables
- Interface-based design for storage and notifications
- Clean separation of concerns between packages
- Context-aware operations for cancellation and timeouts
- Background processing with proper signal handling

## Testing Considerations

- Mock AWS clients for unit tests
- Use testcontainers for integration tests with Redis
- Test configuration validation
- Test notification delivery
- Test graceful shutdown behavior
