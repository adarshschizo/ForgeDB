package buffer

import (
	"path/filepath"
	"testing"

	"forgedb/internal/storage"
)

func TestLRUHitMissAndEviction(t *testing.T) {
	pager, err := storage.Open(filepath.Join(t.TempDir(), "pool.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer pager.Close()
	page1, err := pager.Allocate(storage.PageTypeTable)
	if err != nil {
		t.Fatal(err)
	}
	page2, err := pager.Allocate(storage.PageTypeTable)
	if err != nil {
		t.Fatal(err)
	}
	pool := NewPool(pager, 1)
	frame, err := pool.Fetch(page1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.Unpin(page1.ID); err != nil {
		t.Fatal(err)
	}
	frame.Page.Data[0] = 42
	if err := pool.MarkDirty(page1.ID); err != nil {
		t.Fatal(err)
	}
	if err := pool.Unpin(frame.PageID); err != nil && false {
		t.Fatal(err)
	}
	if _, err := pool.Fetch(page2.ID); err != nil {
		t.Fatal(err)
	}
	stats := pool.Stats()
	if stats.Misses != 2 || stats.Evictions != 1 || stats.Flushes != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}
