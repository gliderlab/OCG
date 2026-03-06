package memory

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
)

// GraphStore manages the Knowledge Graph in SQLite
type GraphStore struct {
	db *sql.DB
}

// Entity represents a node in the knowledge graph
type Entity struct {
	ID          string
	Name        string
	Type        string
	Description string
	CreatedAt   int64
	UpdatedAt   int64
}

// Relation represents an edge between two entities
type Relation struct {
	ID        string
	Source    string
	Target    string
	Relation  string
	Weight    float64
	CreatedAt int64
	UpdatedAt int64
}

// NewGraphStore initializes a GraphStore and creates necessary tables
func NewGraphStore(db *sql.DB) (*GraphStore, error) {
	gs := &GraphStore{db: db}
	if err := gs.initSchema(); err != nil {
		return nil, err
	}
	return gs, nil
}

func (gs *GraphStore) initSchema() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS memory_entities (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			type TEXT NOT NULL,
			description TEXT,
			created_at INTEGER,
			updated_at INTEGER
		)`,
		`CREATE TABLE IF NOT EXISTS memory_relations (
			id TEXT PRIMARY KEY,
			source TEXT NOT NULL,
			target TEXT NOT NULL,
			relation TEXT NOT NULL,
			weight REAL DEFAULT 1.0,
			created_at INTEGER,
			updated_at INTEGER,
			UNIQUE(source, target, relation)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_entities_name ON memory_entities(name)`,
		`CREATE INDEX IF NOT EXISTS idx_relations_source ON memory_relations(source)`,
		`CREATE INDEX IF NOT EXISTS idx_relations_target ON memory_relations(target)`,
	}

	for _, query := range queries {
		if _, err := gs.db.Exec(query); err != nil {
			log.Printf("[ERROR] Graph schema init failed for query %s: %v", query, err)
			return err
		}
	}
	return nil
}

// AddEntity adds or updates an entity
func (gs *GraphStore) AddEntity(name, entityType, description string) error {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return fmt.Errorf("entity name cannot be empty")
	}

	now := time.Now().Unix()
	id := uuid.New().String()

	_, err := gs.db.Exec(`
		INSERT INTO memory_entities (id, name, type, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			type = excluded.type,
			description = excluded.description,
			updated_at = excluded.updated_at
	`, id, name, entityType, description, now, now)

	return err
}

// AddRelation adds or updates a relation between two entities
func (gs *GraphStore) AddRelation(source, target, relation string, weight float64) error {
	source = strings.ToLower(strings.TrimSpace(source))
	target = strings.ToLower(strings.TrimSpace(target))
	relation = strings.ToLower(strings.TrimSpace(relation))

	if source == "" || target == "" || relation == "" {
		return fmt.Errorf("source, target, and relation cannot be empty")
	}

	now := time.Now().Unix()
	id := uuid.New().String()

	_, err := gs.db.Exec(`
		INSERT INTO memory_relations (id, source, target, relation, weight, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(source, target, relation) DO UPDATE SET
			weight = excluded.weight,
			updated_at = excluded.updated_at
	`, id, source, target, relation, weight, now, now)

	return err
}

// GetEntity gets an entity by name
func (gs *GraphStore) GetEntity(name string) (*Entity, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	
	var e Entity
	err := gs.db.QueryRow(`
		SELECT id, name, type, description, created_at, updated_at
		FROM memory_entities WHERE name = ?
	`, name).Scan(&e.ID, &e.Name, &e.Type, &e.Description, &e.CreatedAt, &e.UpdatedAt)
	
	if err == sql.ErrNoRows {
		return nil, nil // Not found
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// SearchRelations finds relations connected to a specific entity
func (gs *GraphStore) SearchRelations(entityName string) ([]Relation, error) {
	entityName = strings.ToLower(strings.TrimSpace(entityName))
	
	rows, err := gs.db.Query(`
		SELECT id, source, target, relation, weight, created_at, updated_at
		FROM memory_relations 
		WHERE source = ? OR target = ?
		ORDER BY weight DESC, updated_at DESC
		LIMIT 50
	`, entityName, entityName)
	
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rels []Relation
	for rows.Next() {
		var r Relation
		if err := rows.Scan(&r.ID, &r.Source, &r.Target, &r.Relation, &r.Weight, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		rels = append(rels, r)
	}
	return rels, nil
}
