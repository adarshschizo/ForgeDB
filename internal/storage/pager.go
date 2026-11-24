package storage

import (
	"fmt"
	"os"
)

type Pager struct {
	disk *DiskManager
	next uint32
}

func Open(path string) (*Pager, error) {
	d, err := Create(path)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(path)
	if err != nil {
		d.Close()
		return nil, err
	}
	next := uint32(st.Size() / PageSize)
	if next == 0 {
		next = 1
	}
	return &Pager{disk: d, next: next}, nil
}

func (p *Pager) Allocate(typ PageType) (*Page, error) {
	page := NewPage(p.next, typ)
	p.next++
	if err := p.disk.WritePage(page); err != nil {
		return nil, err
	}
	return page, nil
}

func (p *Pager) Read(id uint32) (*Page, error) {
	if id == 0 {
		return nil, fmt.Errorf("invalid page id 0")
	}
	return p.disk.ReadPage(id)
}

func (p *Pager) Write(page *Page) error { return p.disk.WritePage(page) }
func (p *Pager) Sync() error            { return p.disk.Sync() }
func (p *Pager) Close() error           { return p.disk.Close() }
