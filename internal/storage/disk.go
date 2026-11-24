package storage

import (
	"fmt"
	"os"
)

type DiskManager struct{ file *os.File }

func Create(path string) (*DiskManager, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	d := &DiskManager{file: f}
	if err := d.ensureHeader(); err != nil {
		f.Close()
		return nil, err
	}
	return d, nil
}

func (d *DiskManager) ensureHeader() error {
	st, err := d.file.Stat()
	if err != nil {
		return err
	}
	if st.Size() == 0 {
		header := make([]byte, PageSize)
		copy(header[0:8], "FORGEDB1")
		if _, err := d.file.WriteAt(header, 0); err != nil {
			return err
		}
		return d.file.Sync()
	}
	if st.Size() < PageSize {
		return fmt.Errorf("invalid database file: too short")
	}
	header := make([]byte, 8)
	if _, err := d.file.ReadAt(header, 0); err != nil {
		return err
	}
	if string(header) != "FORGEDB1" {
		return fmt.Errorf("invalid database file header")
	}
	return nil
}

func (d *DiskManager) ReadPage(id uint32) (*Page, error) {
	buf := make([]byte, PageSize)
	if _, err := d.file.ReadAt(buf, int64(id)*PageSize); err != nil {
		return nil, err
	}
	p := &Page{}
	if err := p.Decode(buf); err != nil {
		return nil, err
	}
	return p, nil
}

func (d *DiskManager) WritePage(p *Page) error {
	_, err := d.file.WriteAt(p.Encode(), int64(p.ID)*PageSize)
	return err
}

func (d *DiskManager) Sync() error  { return d.file.Sync() }
func (d *DiskManager) Close() error { return d.file.Close() }
