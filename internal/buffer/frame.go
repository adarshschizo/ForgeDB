package buffer

import "forgedb/internal/storage"

type Frame struct {
	PageID   uint32
	Page     *storage.Page
	Dirty    bool
	PinCount int
}
