package memory

import (
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestBackfillEmbeddingDim(t *testing.T) {
	dir := t.TempDir()
	store, err := NewVectorMemoryStore(filepath.Join(dir, "vec.db"), Config{})
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer store.Close()

	vector := serializeVector([]float32{1, 2, 3})
	now := time.Now().Unix()
	_, err = store.db.Exec(`INSERT INTO vector_memories (id, text, vector, importance, category, source, embedding_dim, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, NULL, ?, ?)`,
		"id-1", "hello", vector, 0.5, "test", "manual", now, now)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	store.backfillEmbeddingDim()

	var dim int
	if err := store.db.QueryRow(`SELECT embedding_dim FROM vector_memories WHERE id = ?`, "id-1").Scan(&dim); err != nil {
		t.Fatalf("query: %v", err)
	}
	if dim != 3 {
		t.Fatalf("expected embedding_dim 3, got %d", dim)
	}
}

func TestLoadExistingVectorsSkipsDimMismatch(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "vec.db")

	store, err := NewVectorMemoryStore(dbPath, Config{EmbeddingDim: 4})
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer store.Close()

	vec := serializeVector([]float32{1, 2, 3})
	now := time.Now().Unix()
	_, err = store.db.Exec(`INSERT INTO vector_memories (id, text, vector, importance, category, source, embedding_dim, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"id-2", "text", vec, 0.5, "cat", "manual", 3, now, now)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	store.hnsw = &HNSWIndex{cfg: HNSWConfig{Dim: 4}}
	store.loadExistingVectors()

	if len(store.hnswIDs) != 0 {
		t.Fatalf("expected no hnsw ids due to dim mismatch, got %d", len(store.hnswIDs))
	}
}

// MockProvider is used to avoid external requests during tests
type MockProvider struct {
	dim int
}

func (m *MockProvider) Embed(text string) ([]float32, error) {
	// Return a static or simple vector for tests
	vec := make([]float32, m.dim)
	for i := 0; i < m.dim; i++ {
		vec[i] = float32(i) * 0.1
	}
	return vec, nil
}

func (m *MockProvider) Dim() int {
	return m.dim
}

func (m *MockProvider) Name() string {
	return "mock"
}

func TestVectorStore_CRUD(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "vec.db")

	store, err := NewVectorMemoryStore(dbPath, Config{EmbeddingDim: 3})
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer store.Close()

	store.embedding = &MockProvider{dim: 3}

	// Add
	id1, err := store.Store("test item 1", "fact", 0.8)
	if err != nil {
		t.Fatalf("Add error: %v", err)
	}

	id2, err := store.Store("test item 2", "preference", 0.9)
	if err != nil {
		t.Fatalf("Add error: %v", err)
	}

	// Search
	results, err := store.Search("test search", 10, 0.0)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	if len(results) == 0 {
		t.Fatalf("expected results but got 0")
	}

	// Delete
	_, err = store.Delete(id1)
	if err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	// Search again
	results, err = store.Search("test search", 10, 0.0)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	for _, res := range results {
		if res.Entry.ID == id1 {
			t.Fatalf("expected %s to be deleted", id1)
		}
	}

	// Get
	entry, err := store.Get(id2)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if entry.ID != id2 {
		t.Fatalf("expected entry %s, got %v", id2, entry.ID)
	}
}

func TestVectorStore_ConcurrentAccess(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "vec.db")

	store, err := NewVectorMemoryStore(dbPath, Config{EmbeddingDim: 3})
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer store.Close()

	store.embedding = &MockProvider{dim: 3}

	var wg sync.WaitGroup
	workers := 10
	itemsPerWorker := 10

	errs := make(chan error, workers*itemsPerWorker)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < itemsPerWorker; j++ {
				_, err := store.Store("concurrent text", "fact", 0.5)
				if err != nil {
					errs <- err
				}
				// Also do some reads
				_, err = store.Search("concurrent", 5, 0.0)
				if err != nil {
					errs <- err
				}
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent error: %v", err)
		}
	}

	// Verify all items added
	var count int
	err = store.db.QueryRow("SELECT COUNT(*) FROM vector_memories").Scan(&count)
	if err != nil {
		t.Fatalf("query count error: %v", err)
	}
	if count != workers*itemsPerWorker {
		t.Fatalf("expected %d items but got %d", workers*itemsPerWorker, count)
	}
}
