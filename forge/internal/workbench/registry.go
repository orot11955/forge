package workbench

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// SchemaVersion is the current registry.db schema version. Bump when adding
// migrations.
const SchemaVersion = 1

type RegistryEntry struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Path          string  `json:"path"`
	Type          string  `json:"type"`
	Template      *string `json:"template"`
	Status        string  `json:"status"`
	LocationType  string  `json:"location_type"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
	LastCheckedAt *string `json:"last_checked_at"`
}

type Registry struct {
	Version  int             `json:"version"`
	Projects []RegistryEntry `json:"projects"`
}

// RegistryPath/RegistryDBName are declared in workbench.go.

// EmptyRegistryJSON returns the canonical empty registry.json payload.
func EmptyRegistryJSON() string {
	return "{\n  \"version\": 1,\n  \"projects\": []\n}\n"
}

// LoadRegistry reads the registry.json project index. If an older registry.db
// exists without registry.json, it is migrated once into the documented JSON
// format for compatibility with previous builds.
func LoadRegistry(root string) (*Registry, error) {
	jsonPath := RegistryPath(root)
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return loadRegistryFallback(root)
		}
		return nil, fmt.Errorf("read registry.json: %w", err)
	}
	r := &Registry{}
	if err := json.Unmarshal(data, r); err != nil {
		return nil, fmt.Errorf("parse registry.json: %w", err)
	}
	if r.Version == 0 {
		r.Version = 1
	}
	if r.Projects == nil {
		r.Projects = []RegistryEntry{}
	}
	return r, nil
}

// SaveRegistry replaces the registry.json project index.
func SaveRegistry(root string, r *Registry) error {
	if r == nil {
		r = &Registry{}
	}
	if r.Version == 0 {
		r.Version = 1
	}
	if r.Projects == nil {
		r.Projects = []RegistryEntry{}
	}
	if err := os.MkdirAll(filepath.Dir(RegistryPath(root)), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal registry.json: %w", err)
	}
	data = append(data, '\n')
	return os.WriteFile(RegistryPath(root), data, 0o644)
}

// UpsertProject inserts or updates a single project entry in registry.json.
func UpsertProject(root string, e RegistryEntry) error {
	r, err := LoadRegistry(root)
	if err != nil {
		return err
	}
	r.Upsert(e)
	return SaveRegistry(root, r)
}

// Upsert preserves the previous in-memory Registry semantics for callers that
// already loaded the full registry.
func (r *Registry) Upsert(e RegistryEntry) {
	if r.Version == 0 {
		r.Version = 1
	}
	for i := range r.Projects {
		if r.Projects[i].ID == e.ID || r.Projects[i].Path == e.Path {
			r.Projects[i] = e
			return
		}
	}
	r.Projects = append(r.Projects, e)
}

// ---------- internals ----------

func loadRegistryFallback(root string) (*Registry, error) {
	dbPath := RegistryDBPath(root)
	if _, err := os.Stat(dbPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			r := &Registry{Version: 1, Projects: []RegistryEntry{}}
			return r, SaveRegistry(root, r)
		}
		return nil, err
	}
	db, err := openDB(root)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	r, err := loadFromDB(db)
	if err != nil {
		return nil, err
	}
	return r, SaveRegistry(root, r)
}

type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func openDB(root string) (*sql.DB, error) {
	dbPath := RegistryDBPath(root)
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open registry.db: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := applySchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := migrateFromJSONIfPresent(root, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func applySchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS schema_version (
			version INTEGER PRIMARY KEY
		)`,
		`CREATE TABLE IF NOT EXISTS projects (
			id              TEXT PRIMARY KEY,
			name            TEXT NOT NULL,
			path            TEXT NOT NULL UNIQUE,
			type            TEXT NOT NULL,
			template        TEXT,
			status          TEXT NOT NULL,
			location_type   TEXT NOT NULL,
			created_at      TEXT NOT NULL,
			updated_at      TEXT NOT NULL,
			last_checked_at TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_projects_status ON projects(status)`,
		`CREATE INDEX IF NOT EXISTS idx_projects_type   ON projects(type)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("apply schema: %w", err)
		}
	}
	if _, err := db.Exec("INSERT OR IGNORE INTO schema_version(version) VALUES (?)", SchemaVersion); err != nil {
		return err
	}
	return nil
}

func migrateFromJSONIfPresent(root string, db *sql.DB) error {
	jsonPath := RegistryPath(root)
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read legacy registry.json: %w", err)
	}
	// If projects already exist in DB, skip migration but still rotate the
	// JSON file so we don't keep re-importing on every call.
	var existing int
	if err := db.QueryRow("SELECT COUNT(*) FROM projects").Scan(&existing); err != nil {
		return err
	}
	if existing > 0 {
		_ = os.Rename(jsonPath, jsonPath+".bak")
		return nil
	}

	r := &Registry{}
	if err := json.Unmarshal(data, r); err != nil {
		return fmt.Errorf("parse legacy registry.json: %w", err)
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	for _, p := range r.Projects {
		if err := upsertProjectTx(tx, p); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migrate project %s: %w", p.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	// Rotate the JSON file out of the way.
	_ = os.Rename(jsonPath, jsonPath+".bak")
	return nil
}

func upsertProjectTx(x execer, e RegistryEntry) error {
	_, err := x.Exec(`
		INSERT INTO projects
			(id, name, path, type, template, status, location_type,
			 created_at, updated_at, last_checked_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name,
			path=excluded.path,
			type=excluded.type,
			template=excluded.template,
			status=excluded.status,
			location_type=excluded.location_type,
			updated_at=excluded.updated_at,
			last_checked_at=excluded.last_checked_at
	`, e.ID, e.Name, e.Path, e.Type, e.Template, e.Status, e.LocationType,
		e.CreatedAt, e.UpdatedAt, e.LastCheckedAt)
	return err
}

func loadFromDB(db *sql.DB) (*Registry, error) {
	rows, err := db.Query(`
		SELECT id, name, path, type, template, status, location_type,
		       created_at, updated_at, last_checked_at
		FROM projects
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	r := &Registry{Version: 1}
	for rows.Next() {
		var e RegistryEntry
		var template, lastChecked sql.NullString
		if err := rows.Scan(&e.ID, &e.Name, &e.Path, &e.Type, &template,
			&e.Status, &e.LocationType, &e.CreatedAt, &e.UpdatedAt, &lastChecked); err != nil {
			return nil, err
		}
		if template.Valid {
			tv := template.String
			e.Template = &tv
		}
		if lastChecked.Valid {
			lv := lastChecked.String
			e.LastCheckedAt = &lv
		}
		r.Projects = append(r.Projects, e)
	}
	return r, rows.Err()
}
