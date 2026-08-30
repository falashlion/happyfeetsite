package database

import (
	"context"
	"fmt"
	"time"

	"github.com/happyfeet/api/pkg/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB wraps pgxpool.Pool.
type DB struct{ *pgxpool.Pool }

// Connect creates and verifies a pgxpool connection.
func Connect(ctx context.Context, cfg *config.DBConfig) (*DB, error) {
	c, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("db: parse config: %w", err)
	}
	c.MaxConns = cfg.MaxConns
	c.MinConns = cfg.MinConns
	c.MaxConnLifetime = cfg.ConnMaxLifetime
	c.MaxConnIdleTime = cfg.ConnMaxIdleTime
	c.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("db: create pool: %w", err)
	}

	pctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}
	return &DB{pool}, nil
}

// Ping checks connectivity.
func (db *DB) Ping(ctx context.Context) error {
	c, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return db.Pool.Ping(c)
}
