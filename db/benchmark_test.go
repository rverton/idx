package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestMemoryConcurrentCorrectness(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping concurrent test in short mode")
	}
	ctx := context.Background()
	tmpfile, err := os.CreateTemp("", "concurrent-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()
	defer os.Remove(tmpfile.Name())

	queries, err := Connect(ctx, tmpfile.Name())
	if err != nil {
		t.Fatal(err)
	}

	const (
		numWriters    = 25
		numReaders    = 25
		keysPerWriter = 1000
		totalKeys     = numWriters * keysPerWriter
	)

	var (
		wg          sync.WaitGroup
		writeErrors atomic.Int64
		writeCount  atomic.Int64
		readErrors  atomic.Int64
		readCount   atomic.Int64
		done        atomic.Bool
	)

	// Track which keys have been written (for reader validation)
	written := make([]atomic.Bool, totalKeys)

	start := time.Now()

	// Start writers
	for g := range numWriters {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := range keysPerWriter {
				key := fmt.Sprintf("worker-%d-key-%d", workerID, i)
				err := queries.SetMemoryKey(ctx, SetMemoryKeyParams{
					Key:        key,
					TargetType: "concurrent-test",
					TargetName: fmt.Sprintf("worker-%d", workerID),
					RunID:      sql.NullInt64{},
				})
				if err != nil {
					writeErrors.Add(1)
					t.Logf("SetMemoryKey error: %v", err)
				} else {
					written[workerID*keysPerWriter+i].Store(true)
					writeCount.Add(1)
				}
			}
		}(g)
	}

	// Start readers (run concurrently with writers)
	for g := range numReaders {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()
			for !done.Load() {
				// Pick a random worker and key to check
				for workerID := range numWriters {
					for i := range keysPerWriter {
						if done.Load() {
							return
						}
						key := fmt.Sprintf("worker-%d-key-%d", workerID, i)
						hasKey, err := queries.HasMemoryKey(ctx, key)
						if err != nil {
							readErrors.Add(1)
							t.Logf("HasMemoryKey error: %v", err)
							continue
						}
						readCount.Add(1)
						// Only flag as error if key was written but not found
						keyIdx := workerID*keysPerWriter + i
						if written[keyIdx].Load() && hasKey != 1 {
							readErrors.Add(1)
							t.Logf("Key not found after write: %s", key)
						}
					}
				}
			}
		}(g)
	}

	// Wait a bit for concurrent activity, then signal readers to stop
	time.Sleep(100 * time.Millisecond)
	done.Store(true)

	wg.Wait()
	duration := time.Since(start)

	t.Logf("Concurrent phase: %d writes, %d reads in %v",
		writeCount.Load(), readCount.Load(), duration)
	t.Logf("Write rate: %.0f keys/sec, Read rate: %.0f keys/sec",
		float64(writeCount.Load())/duration.Seconds(),
		float64(readCount.Load())/duration.Seconds())

	if writeErrors.Load() > 0 {
		t.Errorf("Got %d write errors", writeErrors.Load())
	}
	if readErrors.Load() > 0 {
		t.Errorf("Got %d read errors", readErrors.Load())
	}

	// Final verification: all keys should exist now
	var verifyErrors int64
	for workerID := range numWriters {
		for i := range keysPerWriter {
			key := fmt.Sprintf("worker-%d-key-%d", workerID, i)
			hasKey, err := queries.HasMemoryKey(ctx, key)
			if err != nil {
				verifyErrors++
				continue
			}
			if hasKey != 1 {
				verifyErrors++
			}
		}
	}

	if verifyErrors > 0 {
		t.Errorf("Final verification: %d keys missing", verifyErrors)
	}

	t.Logf("Total: %d keys written and verified successfully", writeCount.Load())
}

func TestMemoryDuplicateKeys(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping duplicate keys test in short mode")
	}
	ctx := context.Background()
	tmpfile, err := os.CreateTemp("", "duplicate-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()
	defer os.Remove(tmpfile.Name())

	queries, err := Connect(ctx, tmpfile.Name())
	if err != nil {
		t.Fatal(err)
	}

	const (
		numGoroutines = 20
		key           = "shared-key"
	)

	var wg sync.WaitGroup
	var errorCount atomic.Int64

	for g := range numGoroutines {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			err := queries.SetMemoryKey(ctx, SetMemoryKeyParams{
				Key:        key,
				TargetType: "duplicate-test",
				TargetName: fmt.Sprintf("worker-%d", workerID),
				RunID:      sql.NullInt64{},
			})
			if err != nil {
				errorCount.Add(1)
				t.Logf("SetMemoryKey error from worker %d: %v", workerID, err)
			}
		}(g)
	}

	wg.Wait()

	if errorCount.Load() > 0 {
		t.Errorf("Got %d errors writing duplicate key", errorCount.Load())
	}

	hasKey, err := queries.HasMemoryKey(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if hasKey != 1 {
		t.Error("Shared key not found after concurrent writes")
	}

	t.Log("Duplicate key handling: OK (INSERT OR IGNORE worked correctly)")
}
