// Package database provides clients for the master PostgreSQL registry and tenant MongoDB databases.
package database

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/divyansh/multi-tenant-pdf-service/internal/config"
	"github.com/divyansh/multi-tenant-pdf-service/internal/models"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // PostgreSQL driver registered via side-effect.
	"github.com/sirupsen/logrus"
)

const createTenantsTableSQL = `
CREATE TABLE IF NOT EXISTS tenants (
	id           TEXT        PRIMARY KEY,
	name         TEXT        NOT NULL UNIQUE,
	mongodb_name TEXT        NOT NULL,
	bucket_name  TEXT        NOT NULL,
	status       TEXT        NOT NULL DEFAULT 'provisioning',
	created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_tenants_name   ON tenants (name);
CREATE INDEX IF NOT EXISTS idx_tenants_status ON tenants (status);
`

// PostgresClient wraps a sqlx.DB and exposes tenant registry operations.
type PostgresClient struct {
	db  *sqlx.DB
	log *logrus.Logger
}

// NewPostgresClient connects to PostgreSQL, configures the pool, and runs migrations.
func NewPostgresClient(cfg config.PostgresConfig, log *logrus.Logger) (*PostgresClient, error) {
	db, err := sqlx.Connect("postgres", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("connecting to postgres: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(30 * time.Minute)

	client := &PostgresClient{db: db, log: log}
	if err := client.migrate(); err != nil {
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	log.Info("connected to postgres and ran migrations")
	return client, nil
}

func (c *PostgresClient) migrate() error {
	_, err := c.db.Exec(createTenantsTableSQL)
	return err
}

// Ping checks the database connection.
func (c *PostgresClient) Ping(ctx context.Context) error {
	return c.db.PingContext(ctx)
}

// Close releases all connections.
func (c *PostgresClient) Close() error {
	return c.db.Close()
}

// TenantExists returns true if an active (non-deleted) tenant with the given name exists.
func (c *PostgresClient) TenantExists(ctx context.Context, name string) (bool, error) {
	var count int
	err := c.db.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM tenants WHERE name = $1 AND status != 'deleted'`, name)
	return count > 0, err
}

// GetTenant fetches a single tenant by name.
func (c *PostgresClient) GetTenant(ctx context.Context, name string) (*models.Tenant, error) {
	var t models.Tenant
	err := c.db.GetContext(ctx, &t,
		`SELECT id, name, mongodb_name, bucket_name, status, created_at, updated_at
		   FROM tenants WHERE name = $1 AND status != 'deleted'`, name)
	if err != nil {
		return nil, fmt.Errorf("getting tenant %q: %w", name, err)
	}
	return &t, nil
}

// CreateTenant inserts a new tenant record with status "provisioning".
// It derives the MongoDB DB name and MinIO bucket name from the sanitized tenant name.
func (c *PostgresClient) CreateTenant(ctx context.Context, name string) (*models.Tenant, error) {
	safe := sanitizeName(name)
	t := models.Tenant{
		ID:          uuid.NewString(),
		Name:        name,
		MongoDBName: "tenant_" + safe,
		BucketName:  "tenant-" + safe,
		Status:      models.TenantStatusProvisioning,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	_, err := c.db.NamedExecContext(ctx, `
		INSERT INTO tenants (id, name, mongodb_name, bucket_name, status, created_at, updated_at)
		VALUES (:id, :name, :mongodb_name, :bucket_name, :status, :created_at, :updated_at)`, &t)
	if err != nil {
		return nil, fmt.Errorf("inserting tenant %q: %w", name, err)
	}

	c.log.WithField("tenant", name).Info("created tenant record")
	return &t, nil
}

// ActivateTenant sets a tenant's status to "active" after provisioning completes.
func (c *PostgresClient) ActivateTenant(ctx context.Context, name string) error {
	_, err := c.db.ExecContext(ctx,
		`UPDATE tenants SET status = 'active', updated_at = NOW() WHERE name = $1`, name)
	return err
}

// DeleteTenant soft-deletes a tenant by setting its status to "deleted".
func (c *PostgresClient) DeleteTenant(ctx context.Context, name string) error {
	_, err := c.db.ExecContext(ctx,
		`UPDATE tenants SET status = 'deleted', updated_at = NOW() WHERE name = $1`, name)
	return err
}

// ListTenants returns all active tenants.
func (c *PostgresClient) ListTenants(ctx context.Context) ([]models.Tenant, error) {
	var tenants []models.Tenant
	err := c.db.SelectContext(ctx, &tenants,
		`SELECT id, name, mongodb_name, bucket_name, status, created_at, updated_at
		   FROM tenants WHERE status = 'active' ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("listing tenants: %w", err)
	}
	return tenants, nil
}

// sanitizeName lowercases the name and replaces any non-alphanumeric characters with underscores.
// MongoDB database names and MinIO bucket names have stricter character requirements than tenant display names.
var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

func sanitizeName(name string) string {
	lower := strings.ToLower(name)
	return nonAlphanumeric.ReplaceAllString(lower, "_")
}
