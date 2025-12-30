package idx

import (
	"context"
	"database/sql"
	"idx/db"
	"testing"
)

func TestNewMemoryStore(t *testing.T) {
	ctx := context.Background()

	queries, err := db.Connect(ctx, ":memory:")
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	runID, err := queries.InsertRun(ctx, 1234567890)
	if err != nil {
		t.Fatalf("failed to insert run: %v", err)
	}

	memory := newMemoryStore(ctx, queries, "test-target-type", "test-target-name", runID)

	t.Run("Has returns false for non-existent key", func(t *testing.T) {
		if memory.Has("non-existent-key") {
			t.Error("expected Has() to return false for non-existent key")
		}
	})

	t.Run("Set stores key and Has returns true", func(t *testing.T) {
		key := "test-key-1"

		if memory.Has(key) {
			t.Error("expected Has() to return false before Set()")
		}

		memory.Set(key)

		if !memory.Has(key) {
			t.Error("expected Has() to return true after Set()")
		}
	})

	t.Run("Set is idempotent for duplicate keys", func(t *testing.T) {
		key := "duplicate-key"

		memory.Set(key)
		memory.Set(key)

		if !memory.Has(key) {
			t.Error("expected Has() to return true after duplicate Set()")
		}
	})

	t.Run("different keys are independent", func(t *testing.T) {
		key1 := "independent-key-1"
		key2 := "independent-key-2"

		memory.Set(key1)

		if !memory.Has(key1) {
			t.Error("expected Has(key1) to return true")
		}
		if memory.Has(key2) {
			t.Error("expected Has(key2) to return false")
		}
	})
}

func TestMemoryStorePersistsAcrossInstances(t *testing.T) {
	ctx := context.Background()

	queries, err := db.Connect(ctx, ":memory:")
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	runID1, err := queries.InsertRun(ctx, 1234567890)
	if err != nil {
		t.Fatalf("failed to insert run: %v", err)
	}

	memory1 := newMemoryStore(ctx, queries, "target-type", "target-name", runID1)
	key := "persistent-key"
	memory1.Set(key)

	runID2, err := queries.InsertRun(ctx, 1234567891)
	if err != nil {
		t.Fatalf("failed to insert second run: %v", err)
	}

	memory2 := newMemoryStore(ctx, queries, "target-type", "target-name", runID2)

	if !memory2.Has(key) {
		t.Error("expected key set by first memory store to be visible to second memory store")
	}
}

func TestMemoryStoreWithDifferentTargets(t *testing.T) {
	ctx := context.Background()

	queries, err := db.Connect(ctx, ":memory:")
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	runID, err := queries.InsertRun(ctx, 1234567890)
	if err != nil {
		t.Fatalf("failed to insert run: %v", err)
	}

	memory1 := newMemoryStore(ctx, queries, "bitbucket-cloud", "target-a", runID)
	memory2 := newMemoryStore(ctx, queries, "bitbucket-cloud", "target-b", runID)

	key := "shared-key"

	memory1.Set(key)

	if !memory1.Has(key) {
		t.Error("expected memory1.Has() to return true")
	}
	if !memory2.Has(key) {
		t.Error("memory deduplication is global - key should be visible across targets")
	}
}

func TestMemoryStoreRecordsMetadata(t *testing.T) {
	ctx := context.Background()

	queries, err := db.Connect(ctx, ":memory:")
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	runID, err := queries.InsertRun(ctx, 1234567890)
	if err != nil {
		t.Fatalf("failed to insert run: %v", err)
	}

	targetType := "bitbucket-cloud"
	targetName := "my-target"
	memory := newMemoryStore(ctx, queries, targetType, targetName, runID)

	key := "metadata-test-key"
	memory.Set(key)

	hasKey, err := queries.HasMemoryKey(ctx, key)
	if err != nil {
		t.Fatalf("HasMemoryKey query failed: %v", err)
	}
	if hasKey != 1 {
		t.Error("expected key to exist in database")
	}
}

func TestMemoryStoreRequiresValidRunID(t *testing.T) {
	ctx := context.Background()

	queries, err := db.Connect(ctx, ":memory:")
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	memory := newMemoryStore(ctx, queries, "test-type", "test-name", 999)

	key := "invalid-run-key"
	memory.Set(key)

	if memory.Has(key) {
		t.Error("expected Set() to fail silently with invalid runID due to foreign key constraint")
	}
}

func TestMemoryStoreDirectQueriesIntegration(t *testing.T) {
	ctx := context.Background()

	queries, err := db.Connect(ctx, ":memory:")
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	runID, err := queries.InsertRun(ctx, 1234567890)
	if err != nil {
		t.Fatalf("failed to insert run: %v", err)
	}

	err = queries.SetMemoryKey(ctx, db.SetMemoryKeyParams{
		Key:        "direct-key",
		TargetType: "direct-type",
		TargetName: "direct-name",
		RunID:      sql.NullInt64{Int64: runID, Valid: true},
	})
	if err != nil {
		t.Fatalf("SetMemoryKey failed: %v", err)
	}

	memory := newMemoryStore(ctx, queries, "other-type", "other-name", runID)
	if !memory.Has("direct-key") {
		t.Error("expected memory store to see key inserted directly via queries")
	}
}
