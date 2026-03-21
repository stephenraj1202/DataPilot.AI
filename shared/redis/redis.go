package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisConfig struct {
	Host         string
	Port         string
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
}

type Client struct {
	*redis.Client
}

func NewClient(cfg RedisConfig) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%s", cfg.Host, cfg.Port),
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     getPoolSize(cfg.PoolSize),
		MinIdleConns: getMinIdleConns(cfg.MinIdleConns),
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	// Test connection with retry logic
	ctx := context.Background()
	maxRetries := 3
	var err error

	for i := 0; i < maxRetries; i++ {
		err = rdb.Ping(ctx).Err()
		if err == nil {
			break
		}
		if i == maxRetries-1 {
			return nil, fmt.Errorf("failed to connect to Redis after %d attempts: %w", maxRetries, err)
		}
		time.Sleep(time.Duration(i+1) * time.Second)
	}

	return &Client{rdb}, nil
}

func getPoolSize(size int) int {
	if size > 0 {
		return size
	}
	return 10
}

func getMinIdleConns(conns int) int {
	if conns > 0 {
		return conns
	}
	return 2
}

func (c *Client) Close() error {
	return c.Client.Close()
}
