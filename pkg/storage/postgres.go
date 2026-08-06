package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/example/fitness-checkin/pkg/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const postgresConnectTimeout = 5 * time.Second

var schemaNames = []string{"auth_schema", "plan_schema", "checkin_schema", "profile_schema", "statistics_schema"}

func OpenPostgres(ctx context.Context, cfg config.Config) (*gorm.DB, error) {
	if cfg.DBDSN == "" {
		return nil, errors.New("database DSN is required")
	}
	connectCtx, cancel := context.WithTimeout(ctx, postgresConnectTimeout)
	defer cancel()
	db, err := gorm.Open(postgres.Open(cfg.DBDSN), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open PostgreSQL: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get PostgreSQL connection: %w", err)
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	if err := sqlDB.PingContext(connectCtx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping PostgreSQL: %w", err)
	}
	return db, nil
}

func InitPostgresSchemas(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return errors.New("database is required")
	}
	for _, schema := range schemaNames {
		if err := db.WithContext(ctx).Exec("CREATE SCHEMA IF NOT EXISTS \"" + schema + "\"").Error; err != nil {
			return fmt.Errorf("create schema %s: %w", schema, err)
		}
	}
	return nil
}
