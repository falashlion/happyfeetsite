package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/happyfeet/api/pkg/config"
	"github.com/redis/go-redis/v9"
)

var ErrNotFound = errors.New("cache: not found")

// Client wraps a Redis client.
type Client struct{ rdb *redis.Client }

func New(cfg *config.RedisConfig) (*Client, error) {
	opts, err := redis.ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("cache: parse URL: %w", err)
	}
	rdb := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("cache: ping: %w", err)
	}
	return &Client{rdb}, nil
}

func (c *Client) Close() error { return c.rdb.Close() }

func (c *Client) Set(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	return c.rdb.Set(ctx, key, val, ttl).Err()
}

func (c *Client) Get(ctx context.Context, key string) ([]byte, error) {
	b, err := c.rdb.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrNotFound
	}
	return b, err
}

func (c *Client) Del(ctx context.Context, keys ...string) error {
	return c.rdb.Del(ctx, keys...).Err()
}

func (c *Client) SetNX(ctx context.Context, key string, val []byte, ttl time.Duration) (bool, error) {
	return c.rdb.SetNX(ctx, key, val, ttl).Result()
}

func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.rdb.Exists(ctx, key).Result()
	return n > 0, err
}

func (c *Client) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return c.rdb.Expire(ctx, key, ttl).Err()
}

func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	return c.rdb.Incr(ctx, key).Result()
}

func (c *Client) SetJSON(ctx context.Context, key string, v any, ttl time.Duration) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.Set(ctx, key, b, ttl)
}

func (c *Client) GetJSON(ctx context.Context, key string, dst any) error {
	b, err := c.Get(ctx, key)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}

func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// Key name constants
const (
	KeyRefreshToken     = "rt:%s"
	KeySessionBlacklist = "bl:%s"
	KeyOTPRate          = "otp_rl:%s:%s"
	KeyIdempotency      = "idm:%s"
)

func Key(format string, args ...any) string { return fmt.Sprintf(format, args...) }
