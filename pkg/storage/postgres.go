package storage

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/example/fitness-checkin/pkg/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const postgresConnectTimeout = 5 * time.Second

var (
	defaultPostgresTargets = []PostgresSchemaTarget{
		{Schema: "auth_schema", Role: "auth_service"},
		{Schema: "plan_schema", Role: "plan_service"},
		{Schema: "checkin_schema", Role: "checkin_service"},
		{Schema: "profile_schema", Role: "profile_service"},
		{Schema: "statistics_schema", Role: "statistics_service"},
	}
	postgresIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

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

// InitPostgresSchemas is the migration entry point. Services should pass only
// their own target; the empty-target form is reserved for admin bootstrap.
func InitPostgresSchemas(ctx context.Context, db *gorm.DB, targets ...PostgresSchemaTarget) error {
	if db == nil {
		return errors.New("database is required")
	}
	if len(targets) == 0 {
		targets = defaultPostgresTargets
	}
	for _, target := range targets {
		if err := validatePostgresTarget(target); err != nil {
			return err
		}
	}
	for _, target := range targets {
		qualifiedSchema := quotePostgresIdentifier(target.Schema)
		qualifiedRole := quotePostgresIdentifier(target.Role)
		query := fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s; ALTER SCHEMA %s OWNER TO %s", qualifiedSchema, qualifiedSchema, qualifiedRole)
		if err := db.WithContext(ctx).Exec(query).Error; err != nil {
			return fmt.Errorf("initialize schema %s for role %s: %w", target.Schema, target.Role, err)
		}
	}
	return nil
}

func validatePostgresTarget(target PostgresSchemaTarget) error {
	if !postgresIdentifier.MatchString(target.Schema) {
		return errors.New("schema must be a safe SQL identifier")
	}
	if target.Role == "" {
		return fmt.Errorf("role is required for schema %s", target.Schema)
	}
	if !postgresIdentifier.MatchString(target.Role) {
		return fmt.Errorf("role %q must be a safe SQL identifier", target.Role)
	}
	return nil
}

func quotePostgresIdentifier(identifier string) string {
	return `"` + identifier + `"`
}

func PostgresSchemaExists(ctx context.Context, db *gorm.DB, schema string) (bool, error) {
	if err := validatePostgresSchema(db, schema); err != nil {
		return false, err
	}
	var exists bool
	err := db.WithContext(ctx).Raw("SELECT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = ?)", schema).Scan(&exists).Error
	return exists, err
}

func RequirePostgresSchema(ctx context.Context, db *gorm.DB, schema string) error {
	if err := validatePostgresSchema(db, schema); err != nil {
		return err
	}
	exists, err := PostgresSchemaExists(ctx, db, schema)
	if err != nil {
		return fmt.Errorf("check PostgreSQL schema %q: %w", schema, err)
	}
	if !exists {
		return fmt.Errorf("PostgreSQL schema %q does not exist; it must be created by infrastructure before migration", schema)
	}
	return nil
}

func validatePostgresSchema(db *gorm.DB, schema string) error {
	if db == nil {
		return errors.New("database is required")
	}
	if !postgresIdentifier.MatchString(schema) {
		return errors.New("schema must be a safe SQL identifier")
	}
	return nil
}
