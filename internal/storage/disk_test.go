package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPersistentPageRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "storage.db")
	pager, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	page, err := pager.Allocate(PageTypeTable)
	if err != nil {
		t.Fatal(err)
	}
	copy(page.Data[:], []byte("persistent payload"))
	if err := pager.Write(page); err != nil {
		t.Fatal(err)
	}
	if err := pager.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := pager.Close(); err != nil {
		t.Fatal(err)
	}

	pager, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer pager.Close()
	got, err := pager.Read(page.ID)
	if err != nil {
		t.Fatal(err)
	}
	payload := make([]byte, 18)
	copy(payload, got.Data[:])
	if string(payload) != "persistent payload" {
		t.Fatalf("got %q", payload)
	}
}

func TestCorruptPageChecksum(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corrupt.db")
	pager, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	page, err := pager.Allocate(PageTypeTable)
	if err != nil {
		t.Fatal(err)
	}
	copy(page.Data[:], []byte("corrupt me"))
	if err := pager.Write(page); err != nil {
		t.Fatal(err)
	}
	if err := pager.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := pager.Close(); err != nil {
		t.Fatal(err)
	}

	file, err := os.OpenFile(path, os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteAt([]byte{0xff}, int64(page.ID)*PageSize+PageHeaderSize); err != nil {
		t.Fatal(err)
	}
	if err := file.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	pager, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer pager.Close()
	if _, err := pager.Read(page.ID); err == nil {
		t.Fatal("corrupt page checksum was accepted")
	}
}
