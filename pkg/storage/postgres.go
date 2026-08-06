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

var defaultPostgresSchemas = []string{"auth_schema", "plan_schema", "checkin_schema", "profile_schema", "statistics_schema"}

type PostgresSchemaTarget struct {
	Schema string
	Role   string
}

func OpenPostgres(ctx context.Context, cfg config.Config) (*gorm.DB, error) {
	if cfg.DBDSN == "" {
		return nil, errors.New("database DSN is required")
	}
	connectCtx, cancel := context.WithTimeout(ctx, postgresConnectTimeout)
	defer cancel()
	db, err := gorm.Open(postgres.Open(cfg.DBDSN), &gorm.Config{DisableAutomaticPing: true})
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

// InitPostgresSchemas is the local migration entry point. Pass explicit targets
// for a service so it only needs its own schema role.
func InitPostgresSchemas(ctx context.Context, db *gorm.DB, targets ...PostgresSchemaTarget) error {
	if db == nil {
		return errors.New("database is required")
	}
	if len(targets) == 0 {
		for _, schema := range defaultPostgresSchemas {
			targets = append(targets, PostgresSchemaTarget{Schema: schema})
		}
	}
	for _, target := range targets {
		if target.Schema == "" {
			return errors.New("schema is required")
		}
		if err := db.WithContext(ctx).Exec("CREATE SCHEMA IF NOT EXISTS \"" + target.Schema + "\"").Error; err != nil {
			return fmt.Errorf("create schema %s: %w", target.Schema, err)
		}
	}
	return nil
}

func PostgresSchemaExists(ctx context.Context, db *gorm.DB, schema string) (bool, error) {
	if db == nil {
		return false, errors.New("database is required")
	}
	if schema == "" {
		return false, errors.New("schema is required")
	}
	var count int64
	err := db.WithContext(ctx).Raw("SELECT count(*) FROM information_schema.schemata WHERE schema_name = ?", schema).Scan(&count).Error
	return count > 0, err
}
