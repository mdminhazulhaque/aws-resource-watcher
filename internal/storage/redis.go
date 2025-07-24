package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisStorage implements storage using Redis
type RedisStorage struct {
	client *redis.Client
}

// NewRedisStorage creates a new Redis storage client
func NewRedisStorage(redisURI string) (*RedisStorage, error) {
	opts, err := redis.ParseURL(redisURI)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URI: %w", err)
	}

	client := redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisStorage{
		client: client,
	}, nil
}

// GetResourceARNs retrieves the list of resource ARNs for an account
func (r *RedisStorage) GetResourceARNs(ctx context.Context, accountID string) ([]string, error) {
	key := fmt.Sprintf("aws:resources:%s", accountID)
	
	// Get all elements from the Redis list
	result, err := r.client.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		if err == redis.Nil {
			// Key doesn't exist, return empty slice
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to get resource ARNs from Redis list: %w", err)
	}

	return result, nil
}

// SetResourceARNs stores the list of resource ARNs for an account
func (r *RedisStorage) SetResourceARNs(ctx context.Context, accountID string, arns []string) error {
	key := fmt.Sprintf("aws:resources:%s", accountID)
	
	// Use a pipeline for atomic operations
	pipe := r.client.Pipeline()
	
	// Delete the existing list first
	pipe.Del(ctx, key)
	
	// Add all ARNs to the list if there are any
	if len(arns) > 0 {
		// Convert []string to []interface{} for Redis
		values := make([]interface{}, len(arns))
		for i, arn := range arns {
			values[i] = arn
		}
		pipe.LPush(ctx, key, values...)
	}
	
	// Execute the pipeline
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to set resource ARNs in Redis list: %w", err)
	}

	return nil
}

// IsFirstRun checks if this is the first run for an account (Redis key doesn't exist)
func (r *RedisStorage) IsFirstRun(ctx context.Context, accountID string) (bool, error) {
	key := fmt.Sprintf("aws:resources:%s", accountID)
	
	exists, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check if key exists in Redis: %w", err)
	}

	return exists == 0, nil
}

// Close closes the Redis connection
func (r *RedisStorage) Close() error {
	return r.client.Close()
}
