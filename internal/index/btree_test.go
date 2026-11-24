package index

import (
	"path/filepath"
	"testing"

	"forgedb/internal/storage"
)

func openTree(t *testing.T) (*storage.Pager, *BPlusTree, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tree.db")
	pager, err := storage.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := Create(pager)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pager.Close() })
	return pager, tree, path
}

func TestInsertSearchRangeAndPersistence(t *testing.T) {
	pager, tree, path := openTree(t)
	for i := int64(1); i <= 200; i++ {
		if err := tree.Insert(i, uint32(i)); err != nil {
			t.Fatal(err)
		}
	}
	value, found, err := tree.Search(137)
	if err != nil || !found || value != 137 {
		t.Fatalf("search: value=%d found=%v err=%v", value, found, err)
	}
	values, err := tree.Range(120, 125)
	if err != nil || len(values) != 6 {
		t.Fatalf("range: values=%v err=%v", values, err)
	}
	root := tree.Root
	if err := pager.Sync(); err != nil {
		t.Fatal(err)
	}
	reopened, err := storage.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	persistent := Open(reopened, root)
	value, found, err = persistent.Search(137)
	if err != nil || !found || value != 137 {
		t.Fatalf("persistent search: value=%d found=%v err=%v", value, found, err)
	}
}

func TestDeleteAndUpdate(t *testing.T) {
	_, tree, _ := openTree(t)
	for i := int64(1); i <= 20; i++ {
		if err := tree.Insert(i, uint32(i)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tree.Insert(10, 1000); err != nil {
		t.Fatal(err)
	}
	value, found, err := tree.Search(10)
	if err != nil || !found || value != 1000 {
		t.Fatalf("update search failed: value=%d found=%v err=%v", value, found, err)
	}
	found, err = tree.Delete(10)
	if err != nil || !found {
		t.Fatalf("delete found=%v err=%v", found, err)
	}
	_, found, err = tree.Search(10)
	if err != nil || found {
		t.Fatalf("deleted key remains: found=%v err=%v", found, err)
	}
}

func TestBPlusTreeStressDeleteAndRange(t *testing.T) {
	_, tree, _ := openTree(t)
	for i := int64(1); i <= 500; i++ {
		if err := tree.Insert(i, uint32(i)); err != nil {
			t.Fatal(err)
		}
	}
	for i := int64(1); i <= 500; i += 2 {
		found, err := tree.Delete(i)
		if err != nil || !found {
			t.Fatalf("delete %d: found=%v err=%v", i, found, err)
		}
	}
	values, err := tree.Range(1, 500)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 250 {
		t.Fatalf("range returned %d values, want 250", len(values))
	}
	for _, value := range values {
		if value%2 != 0 {
			t.Fatalf("deleted odd key remains: %d", value)
		}
	}
}
