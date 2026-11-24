package buffer

import (
	"fmt"
	"sync"

	"forgedb/internal/storage"
)

type Stats struct {
	Hits      uint64
	Misses    uint64
	Evictions uint64
	Flushes   uint64
}

type Pool struct {
	mu       sync.Mutex
	pager    *storage.Pager
	capacity int
	frames   map[uint32]*Frame
	lru      *LRU
	stats    Stats
}

func NewPool(pager *storage.Pager, capacity int) *Pool {
	if capacity < 1 {
		capacity = 1
	}
	return &Pool{
		pager:    pager,
		capacity: capacity,
		frames:   make(map[uint32]*Frame),
		lru:      NewLRU(),
	}
}

func (p *Pool) Fetch(pageID uint32) (*Frame, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if frame, ok := p.frames[pageID]; ok {
		p.stats.Hits++
		frame.PinCount++
		p.lru.Touch(pageID)
		return frame, nil
	}
	p.stats.Misses++
	if len(p.frames) >= p.capacity {
		if err := p.evictLocked(); err != nil {
			return nil, err
		}
	}
	page, err := p.pager.Read(pageID)
	if err != nil {
		return nil, err
	}
	frame := &Frame{PageID: pageID, Page: page, PinCount: 1}
	p.frames[pageID] = frame
	p.lru.Touch(pageID)
	return frame, nil
}

func (p *Pool) Unpin(pageID uint32) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	frame, ok := p.frames[pageID]
	if !ok {
		return fmt.Errorf("page %d is not in buffer pool", pageID)
	}
	if frame.PinCount == 0 {
		return fmt.Errorf("page %d is already unpinned", pageID)
	}
	frame.PinCount--
	return nil
}

func (p *Pool) MarkDirty(pageID uint32) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	frame, ok := p.frames[pageID]
	if !ok {
		return fmt.Errorf("page %d is not in buffer pool", pageID)
	}
	frame.Dirty = true
	return nil
}

func (p *Pool) Flush(pageID uint32) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.flushLocked(pageID)
}

func (p *Pool) flushLocked(pageID uint32) error {
	frame, ok := p.frames[pageID]
	if !ok {
		return nil
	}
	if !frame.Dirty {
		return nil
	}
	if err := p.pager.Write(frame.Page); err != nil {
		return err
	}
	frame.Dirty = false
	p.stats.Flushes++
	return nil
}

func (p *Pool) evictLocked() error {
	for attempts := 0; attempts < len(p.frames); attempts++ {
		pageID, ok := p.lru.Victim()
		if !ok {
			return fmt.Errorf("buffer pool cannot find eviction victim")
		}
		frame := p.frames[pageID]
		if frame.PinCount != 0 {
			p.lru.Remove(pageID)
			p.lru.Touch(pageID)
			return fmt.Errorf("no unpinned page available for eviction")
		}
		if err := p.flushLocked(pageID); err != nil {
			return err
		}
		delete(p.frames, pageID)
		p.lru.Remove(pageID)
		p.stats.Evictions++
		return nil
	}
	return fmt.Errorf("no unpinned page available for eviction")
}

func (p *Pool) FlushAll() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for pageID := range p.frames {
		if err := p.flushLocked(pageID); err != nil {
			return err
		}
	}
	return p.pager.Sync()
}

func (p *Pool) Stats() Stats {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stats
}
