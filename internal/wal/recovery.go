package wal

import (
	"fmt"

	"forgedb/internal/storage"
)

func CommittedTransactions(records []Record) map[uint64]bool {
	committed := make(map[uint64]bool)
	for _, record := range records {
		if record.Kind == RecordCommit {
			committed[record.TxnID] = true
		}
		if record.Kind == RecordAbort {
			delete(committed, record.TxnID)
		}
	}
	return committed
}

func ReplayCommitted(pager *storage.Pager, records []Record) error {
	committed := CommittedTransactions(records)
	for _, record := range records {
		if record.Kind != RecordUpdate || !committed[record.TxnID] {
			continue
		}
		if record.PageID == 0 {
			continue
		}
		page, err := pager.Read(record.PageID)
		if err != nil {
			return fmt.Errorf("recovery read page %d: %w", record.PageID, err)
		}
		page.Data = [storage.PageSize - storage.PageHeaderSize]byte{}
		if len(record.Payload) > len(page.Data) {
			return fmt.Errorf("recovery payload too large for page %d", record.PageID)
		}
		copy(page.Data[:], record.Payload)
		if err := pager.Write(page); err != nil {
			return fmt.Errorf("recovery write page %d: %w", record.PageID, err)
		}
	}
	return pager.Sync()
}
