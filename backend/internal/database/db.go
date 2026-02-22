package database

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// DB wraps the PostgreSQL connection pool
type DB struct {
	Pool *pgxpool.Pool
}

// New creates a new database connection with retry logic.
// Retries up to 10 times with a 3-second delay between attempts,
// handling the window where pg_isready passes but PostgreSQL is
// still initialising and not yet accepting connections.
func New(ctx context.Context, dsn string) (*DB, error) {
	const maxRetries = 10
	const retryDelay = 3 * time.Second

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			lastErr = fmt.Errorf("failed to create connection pool: %w", err)
		} else if pingErr := pool.Ping(ctx); pingErr != nil {
			pool.Close()
			lastErr = fmt.Errorf("failed to ping database: %w", pingErr)
		} else {
			return &DB{Pool: pool}, nil
		}

		if attempt < maxRetries {
			log.Warn().
				Err(lastErr).
				Int("attempt", attempt).
				Int("max", maxRetries).
				Msgf("Database not ready, retrying in %s...", retryDelay)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(retryDelay):
			}
		}
	}

	return nil, fmt.Errorf("failed to connect to database after %d attempts: %w", maxRetries, lastErr)
}

// Close closes the database connection pool
func (db *DB) Close() {
	db.Pool.Close()
}

// RunMigrations runs all SQL migrations in order
func (db *DB) RunMigrations(ctx context.Context) error {
	// Create migrations tracking table
	_, err := db.Pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMPTZ DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Read migration files
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	// Sort migrations by filename
	var migrations []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".sql") {
			migrations = append(migrations, entry.Name())
		}
	}
	sort.Strings(migrations)

	// Apply each migration
	for _, filename := range migrations {
		// Check if already applied
		var exists bool
		err := db.Pool.QueryRow(ctx,
			"SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)",
			filename,
		).Scan(&exists)
		if err != nil {
			return fmt.Errorf("failed to check migration status: %w", err)
		}

		if exists {
			log.Debug().Str("migration", filename).Msg("Migration already applied")
			continue
		}

		// Read and execute migration
		content, err := migrationsFS.ReadFile("migrations/" + filename)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", filename, err)
		}

		log.Info().Str("migration", filename).Msg("Applying migration")

		_, err = db.Pool.Exec(ctx, string(content))
		if err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", filename, err)
		}

		// Record migration
		_, err = db.Pool.Exec(ctx,
			"INSERT INTO schema_migrations (version) VALUES ($1)",
			filename,
		)
		if err != nil {
			return fmt.Errorf("failed to record migration %s: %w", filename, err)
		}

		log.Info().Str("migration", filename).Msg("Migration applied successfully")
	}

	return nil
}
