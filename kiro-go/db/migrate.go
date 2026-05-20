package db

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// ErrAlreadyAtLatest is returned when no pending schema migrations exist.
var ErrAlreadyAtLatest = errors.New("schema already at latest")

type Migration struct {
	Version  int
	Name     string
	Path     string
	SQL      string
	Checksum string
}

type MigrationState struct {
	Version  int
	Name     string
	Checksum string
	Applied  bool
}

type Status struct {
	CurrentVersion int
	Migrations     []MigrationState
}

type appliedMigration struct {
	name     string
	checksum string
}

// RunMigrations applies every pending embedded schema migration.
func RunMigrations(ctx context.Context) error {
	migrations, err := LoadMigrations()
	if err != nil {
		return err
	}
	applied, err := loadAppliedMigrations(ctx)
	if err != nil {
		return err
	}

	appliedCount := 0
	for _, m := range migrations {
		if existing, ok := applied[m.Version]; ok {
			if existing.checksum != m.Checksum {
				return fmt.Errorf("migration %04d checksum drift: db=%s embedded=%s", m.Version, existing.checksum, m.Checksum)
			}
			continue
		}
		if err := applyMigration(ctx, m); err != nil {
			return err
		}
		appliedCount++
	}
	if appliedCount == 0 {
		return ErrAlreadyAtLatest
	}
	return nil
}

// MigrationStatus returns current schema version and pending embedded migrations.
func MigrationStatus(ctx context.Context) (Status, error) {
	migrations, err := LoadMigrations()
	if err != nil {
		return Status{}, err
	}
	applied, err := loadAppliedMigrations(ctx)
	if err != nil {
		return Status{}, err
	}

	var out Status
	for _, m := range migrations {
		state := MigrationState{
			Version:  m.Version,
			Name:     m.Name,
			Checksum: m.Checksum,
		}
		if existing, ok := applied[m.Version]; ok {
			if existing.checksum != m.Checksum {
				return Status{}, fmt.Errorf("migration %04d checksum drift: db=%s embedded=%s", m.Version, existing.checksum, m.Checksum)
			}
			state.Applied = true
			if m.Version > out.CurrentVersion {
				out.CurrentVersion = m.Version
			}
		}
		out.Migrations = append(out.Migrations, state)
	}
	return out, nil
}

// LoadMigrations loads embedded SQL migrations in version order.
func LoadMigrations() ([]Migration, error) {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read embedded migrations: %w", err)
	}

	migrations := make([]Migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		version, name, err := parseMigrationName(entry.Name())
		if err != nil {
			return nil, err
		}
		path := "migrations/" + entry.Name()
		data, err := migrationsFS.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", path, err)
		}
		sum := sha256.Sum256(data)
		migrations = append(migrations, Migration{
			Version:  version,
			Name:     name,
			Path:     path,
			SQL:      string(data),
			Checksum: hex.EncodeToString(sum[:]),
		})
	}
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})
	return migrations, nil
}

func parseMigrationName(filename string) (int, string, error) {
	base := strings.TrimSuffix(filename, ".sql")
	parts := strings.SplitN(base, "_", 2)
	if len(parts) != 2 {
		return 0, "", fmt.Errorf("invalid migration filename: %s", filename)
	}
	version, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, "", fmt.Errorf("invalid migration version %q: %w", parts[0], err)
	}
	return version, parts[1], nil
}

func loadAppliedMigrations(ctx context.Context) (map[int]appliedMigration, error) {
	p := Pool()
	if p == nil {
		return nil, errors.New("postgres pool is not initialized")
	}
	rows, err := p.Query(ctx, `SELECT version, name, checksum_sha256 FROM schema_migrations`)
	if err != nil {
		if isUndefinedTable(err) {
			return map[int]appliedMigration{}, nil
		}
		return nil, fmt.Errorf("query schema_migrations: %w", err)
	}
	defer rows.Close()

	out := map[int]appliedMigration{}
	for rows.Next() {
		var version int
		var m appliedMigration
		if err := rows.Scan(&version, &m.name, &m.checksum); err != nil {
			return nil, fmt.Errorf("scan schema_migrations: %w", err)
		}
		out[version] = m
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate schema_migrations: %w", err)
	}
	return out, nil
}

func applyMigration(ctx context.Context, m Migration) error {
	p := Pool()
	if p == nil {
		return errors.New("postgres pool is not initialized")
	}
	tx, err := p.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migration %04d: %w", m.Version, err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, m.SQL); err != nil {
		return fmt.Errorf("apply migration %04d %s: %w", m.Version, m.Name, err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO schema_migrations(version, name, checksum_sha256) VALUES ($1, $2, $3)`,
		m.Version, m.Name, m.Checksum,
	); err != nil {
		return fmt.Errorf("record migration %04d: %w", m.Version, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration %04d: %w", m.Version, err)
	}
	return nil
}

func isUndefinedTable(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "42P01"
}
