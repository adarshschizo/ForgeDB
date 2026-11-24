package wal

import (
	"path/filepath"
	"testing"

	"forgedb/internal/storage"
)

func TestAppendSyncAndRecovery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.wal")
	log, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := log.Append(RecordBegin, 7, 0, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := log.Append(RecordUpdate, 7, 3, []byte("page-data")); err != nil {
		t.Fatal(err)
	}
	if _, err := log.Append(RecordCommit, 7, 0, nil); err != nil {
		t.Fatal(err)
	}
	if err := log.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := log.Close(); err != nil {
		t.Fatal(err)
	}

	log, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	records, err := log.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 {
		t.Fatalf("got %d records", len(records))
	}
	if !CommittedTransactions(records)[7] {
		t.Fatal("transaction 7 should be committed")
	}
}

func TestReplayCommittedPages(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "recovery.db")
	pager, err := storage.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	page1, err := pager.Allocate(storage.PageTypeMetadata)
	if err != nil {
		t.Fatal(err)
	}
	if page1.ID != 1 {
		t.Fatalf("unexpected metadata page id: %d", page1.ID)
	}
	page, err := pager.Read(1)
	if err != nil {
		t.Fatal(err)
	}
	page.Data = [4096 - 16]byte{}
	copy(page.Data[:], "old")
	if err := pager.Write(page); err != nil {
		t.Fatal(err)
	}

	records := []Record{
		{LSN: 1, TxnID: 10, Kind: RecordBegin, PageID: 1},
		{LSN: 2, TxnID: 10, Kind: RecordUpdate, PageID: 1, Payload: []byte("recovered")},
		{LSN: 3, TxnID: 10, Kind: RecordCommit, PageID: 1},
	}
	if err := ReplayCommitted(pager, records); err != nil {
		t.Fatal(err)
	}
	page, err = pager.Read(1)
	if err != nil {
		t.Fatal(err)
	}
	if string(page.Data[:9]) != "recovered" {
		t.Fatalf("unexpected recovered payload: %q", page.Data[:9])
	}
	pager.Close()
}
