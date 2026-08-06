package repository

import (
	"context"
	"fmt"

	"github.com/example/fitness-checkin/services/auth-service/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const DefaultSchema = "auth_schema"

func Migrate(ctx context.Context, db *gorm.DB) error { return MigrateSchema(ctx, db, DefaultSchema) }
func MigrateSchema(ctx context.Context, db *gorm.DB, schema string) error {
	if schema == "" || schema != DefaultSchema && !validSchema(schema) {
		return fmt.Errorf("invalid auth schema")
	}
	q := quoteIdentifier(schema)
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		statements := []string{
			"CREATE SCHEMA IF NOT EXISTS " + q,
			"CREATE TABLE IF NOT EXISTS " + q + ".users (id text PRIMARY KEY, email text NOT NULL UNIQUE, password_hash text NOT NULL, created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL)",
			"CREATE TABLE IF NOT EXISTS " + q + ".refresh_tokens (id text PRIMARY KEY, user_id text NOT NULL REFERENCES " + q + ".users(id) ON DELETE CASCADE, token_hash text NOT NULL UNIQUE, expires_at timestamptz NOT NULL, revoked_at timestamptz NULL, created_at timestamptz NOT NULL)",
			"CREATE INDEX IF NOT EXISTS refresh_tokens_user_id_idx ON " + q + ".refresh_tokens(user_id)",
		}
		for _, statement := range statements {
			if err := tx.Exec(statement).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
func validSchema(value string) bool {
	if len(value) == 0 || len(value) > 50 {
		return false
	}
	for i, r := range value {
		if !(r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || i > 0 && r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}
func quoteIdentifier(value string) string { return `"` + value + `"` }

func scopedTable(schema, table string) string {
	if schema == "" {
		schema = DefaultSchema
	}
	return quoteIdentifier(schema) + "." + quoteIdentifier(table)
}

var _ = clause.Locking{}
var _ = model.User{}
