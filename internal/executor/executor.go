package executor

import (
	"fmt"
	"forgedb/internal/buffer"
	"forgedb/internal/catalog"
	"forgedb/internal/record"
	"forgedb/internal/sql/ast"
	"forgedb/internal/storage"
	"forgedb/internal/transaction"
	"forgedb/internal/wal"
	"os"
	"sort"
	"strings"
	"sync"
)

type Result struct {
	Columns []string
	Rows    []record.Row
	Message string
}

type Engine struct {
	mu           sync.RWMutex
	pager        *storage.Pager
	path         string
	catalog      *catalog.Catalog
	txCatalog    *catalog.Catalog
	txRows       map[string][]record.Row
	txActive     bool
	transactions *transaction.Manager
	activeTxn    *transaction.Transaction
	walLog       *wal.Log
	bufferPool   *buffer.Pool
}

func Open(path string) (*Engine, error) {
	pager, err := storage.Open(path)
	if err != nil {
		return nil, err
	}
	log, err := wal.Open(path + ".wal")
	if err != nil {
		pager.Close()
		return nil, err
	}
	e := &Engine{
		pager:        pager,
		path:         path,
		catalog:      catalog.New(),
		txRows:       make(map[string][]record.Row),
		transactions: transaction.NewManager(),
		walLog:       log,
	}
	e.bufferPool = buffer.NewPool(pager, 64)
	records, err := log.ReadAll()
	if err != nil {
		log.Close()
		pager.Close()
		return nil, err
	}
	if err := wal.ReplayCommitted(pager, records); err != nil {
		log.Close()
		pager.Close()
		return nil, err
	}
	st, err := os.Stat(path)
	if err != nil {
		pager.Close()
		return nil, err
	}
	if st.Size() > storage.PageSize {
		p, err := pager.Read(1)
		if err != nil {
			pager.Close()
			return nil, err
		}
		data := strings.TrimRight(string(p.Data[:]), "\x00")
		if data != "\"\"" {
			if err := e.catalog.Decode([]byte(data)); err != nil {
				pager.Close()
				return nil, err
			}
		}
	} else {
		p, err := pager.Allocate(storage.PageTypeMetadata)
		if err != nil {
			pager.Close()
			return nil, err
		}
		p.Data = [storage.PageSize - storage.PageHeaderSize]byte{}
		copy(p.Data[:], []byte(`{"tables":{}}`))
		if err := pager.Write(p); err != nil {
			pager.Close()
			return nil, err
		}
		if err := pager.Sync(); err != nil {
			pager.Close()
			return nil, err
		}
	}
	return e, nil
}

func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.saveCatalogLocked(); err != nil {
		return err
	}
	if err := e.bufferPool.FlushAll(); err != nil {
		return err
	}
	if err := e.pager.Sync(); err != nil {
		return err
	}
	if err := e.walLog.Sync(); err != nil {
		return err
	}
	if err := e.walLog.Close(); err != nil {
		return err
	}
	return e.pager.Close()
}

func (e *Engine) Execute(stmt ast.Statement) (Result, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	switch s := stmt.(type) {
	case *ast.Explain:
		selectStmt, ok := s.Statement.(*ast.Select)
		if !ok {
			return Result{}, fmt.Errorf("EXPLAIN requires a SELECT statement")
		}
		if selectStmt.Where == nil {
			return Result{Message: fmt.Sprintf("Seq Scan on %s\nProjection", selectStmt.Table)}, nil
		}
		return Result{Message: fmt.Sprintf("Filter (%s)\n  Seq Scan on %s", selectStmt.Where.Operator, selectStmt.Table)}, nil
	case *ast.Transaction:
		switch s.Action {
		case "BEGIN":
			e.txActive = true
			e.txCatalog = e.cloneCatalog()
			e.txRows = make(map[string][]record.Row)
			e.activeTxn = e.transactions.Begin()
			if _, err := e.walLog.Append(wal.RecordBegin, e.activeTxn.ID, 0, nil); err != nil {
				return Result{}, err
			}
			if err := e.walLog.Sync(); err != nil {
				return Result{}, err
			}
			for name := range e.catalog.Tables {
				rows, err := e.readRowsLocked(name)
				if err != nil {
					return Result{}, err
				}
				e.txRows[name] = rows
			}
			return Result{Message: "transaction begun"}, nil
		case "COMMIT":
			if !e.txActive {
				return Result{}, fmt.Errorf("no active transaction")
			}
			e.txActive = false
			if err := e.saveCatalogLocked(); err != nil {
				return Result{}, err
			}
			if _, err := e.walLog.Append(wal.RecordCommit, e.activeTxn.ID, 0, nil); err != nil {
				return Result{}, err
			}
			if err := e.walLog.Sync(); err != nil {
				return Result{}, err
			}
			if err := e.transactions.Commit(e.activeTxn); err != nil {
				return Result{}, err
			}
			e.activeTxn = nil
			return Result{Message: "committed"}, nil
		case "ROLLBACK":
			if !e.txActive {
				return Result{}, fmt.Errorf("no active transaction")
			}
			e.catalog = e.txCatalog
			e.txActive = false
			for name, rows := range e.txRows {
				if err := e.writeRowsLocked(name, rows); err != nil {
					return Result{}, err
				}
			}
			if err := e.saveCatalogLocked(); err != nil {
				return Result{}, err
			}
			if _, err := e.walLog.Append(wal.RecordAbort, e.activeTxn.ID, 0, nil); err != nil {
				return Result{}, err
			}
			if err := e.walLog.Sync(); err != nil {
				return Result{}, err
			}
			if err := e.transactions.Abort(e.activeTxn); err != nil {
				return Result{}, err
			}
			e.activeTxn = nil
			return Result{Message: "rolled back"}, nil
		}
	case *ast.CreateTable:
		cols := make([]catalog.Column, len(s.Columns))
		for i, c := range s.Columns {
			cols[i] = catalog.Column{Name: c.Name, Type: c.Type}
		}
		if err := e.catalog.CreateTable(s.Name, cols); err != nil {
			return Result{}, err
		}
		page, err := e.pager.Allocate(storage.PageTypeTable)
		if err != nil {
			return Result{}, err
		}
		e.catalog.Tables[s.Name].Pages = []uint32{page.ID}
		page.Data = [storage.PageSize - storage.PageHeaderSize]byte{}
		copy(page.Data[:], record.EncodeRows(nil))
		if err := e.pager.Write(page); err != nil {
			return Result{}, err
		}
		if err := e.saveCatalogLocked(); err != nil {
			return Result{}, err
		}
		return Result{Message: fmt.Sprintf("table %s created", s.Name)}, nil
	case *ast.DropTable:
		if err := e.catalog.DropTable(s.Table); err != nil {
			return Result{}, err
		}
		if err := e.saveCatalogLocked(); err != nil {
			return Result{}, err
		}
		return Result{Message: fmt.Sprintf("table %s dropped", s.Table)}, nil
	case *ast.Insert:
		table, exists := e.catalog.Tables[s.Table]
		if !exists {
			return Result{}, fmt.Errorf("table %q does not exist", s.Table)
		}
		if len(s.Values) != len(table.Columns) {
			return Result{}, fmt.Errorf("expected %d values, got %d", len(table.Columns), len(s.Values))
		}
		for i, v := range s.Values {
			if err := validate(table.Columns[i], v); err != nil {
				return Result{}, err
			}
		}
		rows, err := e.readRowsLocked(s.Table)
		if err != nil {
			return Result{}, err
		}
		rows = append(rows, record.Row{Values: s.Values})
		if err := e.writeRowsLocked(s.Table, rows); err != nil {
			return Result{}, err
		}
		return Result{Message: "1 row inserted"}, nil
	case *ast.Select:
		table, exists := e.catalog.Tables[s.Table]
		if !exists {
			return Result{}, fmt.Errorf("table %q does not exist", s.Table)
		}
		rows, err := e.readRowsLocked(s.Table)
		if err != nil {
			return Result{}, err
		}
		filtered := []record.Row{}
		for _, row := range rows {
			if s.Where == nil || e.matches(table, row, s.Where) {
				filtered = append(filtered, row)
			}
		}
		cols := []string{}
		indexes := []int{}
		if len(s.Columns) == 1 && s.Columns[0] == "*" {
			for i, c := range table.Columns {
				cols = append(cols, c.Name)
				indexes = append(indexes, i)
			}
		} else {
			for _, name := range s.Columns {
				i := columnIndex(table, name)
				if i < 0 {
					return Result{}, fmt.Errorf("unknown column %q", name)
				}
				cols = append(cols, name)
				indexes = append(indexes, i)
			}
		}
		out := make([]record.Row, 0, len(filtered))
		for _, row := range filtered {
			values := make([]record.Value, len(indexes))
			for i, index := range indexes {
				values[i] = row.Values[index]
			}
			out = append(out, record.Row{Values: values})
		}
		return Result{Columns: cols, Rows: out}, nil
	case *ast.Update:
		table, exists := e.catalog.Tables[s.Table]
		if !exists {
			return Result{}, fmt.Errorf("table %q does not exist", s.Table)
		}
		idx := columnIndex(table, s.SetColumn)
		if idx < 0 {
			return Result{}, fmt.Errorf("unknown column %q", s.SetColumn)
		}
		if err := validate(table.Columns[idx], s.SetValue); err != nil {
			return Result{}, err
		}
		rows, err := e.readRowsLocked(s.Table)
		if err != nil {
			return Result{}, err
		}
		count := 0
		for i := range rows {
			if s.Where == nil || e.matches(table, rows[i], s.Where) {
				rows[i].Values[idx] = s.SetValue
				count++
			}
		}
		if err := e.writeRowsLocked(s.Table, rows); err != nil {
			return Result{}, err
		}
		return Result{Message: fmt.Sprintf("%d row(s) updated", count)}, nil
	case *ast.Delete:
		table, exists := e.catalog.Tables[s.Table]
		if !exists {
			return Result{}, fmt.Errorf("table %q does not exist", s.Table)
		}
		rows, err := e.readRowsLocked(s.Table)
		if err != nil {
			return Result{}, err
		}
		kept := []record.Row{}
		count := 0
		for _, row := range rows {
			if s.Where == nil {
				count++
				continue
			}
			if e.matches(table, row, s.Where) {
				count++
				continue
			}
			kept = append(kept, row)
		}
		if err := e.writeRowsLocked(s.Table, kept); err != nil {
			return Result{}, err
		}
		return Result{Message: fmt.Sprintf("%d row(s) deleted", count)}, nil
	}
	return Result{}, fmt.Errorf("unsupported statement")
}

type TableInfo struct {
	Name    string
	Columns []catalog.Column
	Pages   []uint32
	Rows    int
}

type DatabaseStats struct {
	DatabaseBytes   int64
	WALBytes        int64
	PageSize        int
	PageCount       int64
	TableCount      int
	RowCount        int
	WALRecords      int
	BufferHits      uint64
	BufferMisses    uint64
	BufferEvictions uint64
	BufferFlushes   uint64
}

func (e *Engine) TableNames() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	names := make([]string, 0, len(e.catalog.Tables))
	for name := range e.catalog.Tables {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (e *Engine) TableInfo(name string) (TableInfo, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	table := e.catalog.Tables[name]
	if table == nil {
		return TableInfo{}, fmt.Errorf("table %q does not exist", name)
	}
	rows, err := e.readRowsLocked(name)
	if err != nil {
		return TableInfo{}, err
	}
	return TableInfo{
		Name:    name,
		Columns: append([]catalog.Column{}, table.Columns...),
		Pages:   append([]uint32{}, table.Pages...),
		Rows:    len(rows),
	}, nil
}

func (e *Engine) Stats() (DatabaseStats, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	dbInfo, err := os.Stat(e.path)
	if err != nil {
		return DatabaseStats{}, err
	}
	walInfo, err := os.Stat(e.path + ".wal")
	if err != nil {
		return DatabaseStats{}, err
	}
	rowCount := 0
	for name := range e.catalog.Tables {
		rows, err := e.readRowsLocked(name)
		if err != nil {
			return DatabaseStats{}, err
		}
		rowCount += len(rows)
	}
	walRecords, err := e.walLog.RecordCount()
	if err != nil {
		return DatabaseStats{}, err
	}
	bufferStats := e.bufferPool.Stats()
	return DatabaseStats{
		DatabaseBytes:   dbInfo.Size(),
		WALBytes:        walInfo.Size(),
		PageSize:        storage.PageSize,
		PageCount:       dbInfo.Size() / storage.PageSize,
		TableCount:      len(e.catalog.Tables),
		RowCount:        rowCount,
		WALRecords:      walRecords,
		BufferHits:      bufferStats.Hits,
		BufferMisses:    bufferStats.Misses,
		BufferEvictions: bufferStats.Evictions,
		BufferFlushes:   bufferStats.Flushes,
	}, nil
}

func (e *Engine) saveCatalogLocked() error {
	b, err := e.catalog.Encode()
	if err != nil {
		return err
	}
	if len(b) > storage.PageSize-storage.PageHeaderSize {
		return fmt.Errorf("catalog too large")
	}
	frame, err := e.bufferPool.Fetch(1)
	if err != nil {
		return err
	}
	frame.Page.Data = [storage.PageSize - storage.PageHeaderSize]byte{}
	copy(frame.Page.Data[:], b)
	if err := e.bufferPool.MarkDirty(frame.PageID); err != nil {
		return err
	}
	txnID := uint64(0)
	implicit := false
	var implicitTxn *transaction.Transaction
	if e.activeTxn != nil {
		txnID = e.activeTxn.ID
	} else {
		implicitTxn = e.transactions.Begin()
		txnID = implicitTxn.ID
		implicit = true
		if _, err := e.walLog.Append(wal.RecordBegin, txnID, frame.PageID, nil); err != nil {
			return err
		}
	}
	if _, err := e.walLog.Append(wal.RecordUpdate, txnID, 1, b); err != nil {
		return err
	}
	if err := e.walLog.Sync(); err != nil {
		return err
	}
	if err := e.bufferPool.Flush(frame.PageID); err != nil {
		return err
	}
	if err := e.bufferPool.Unpin(frame.PageID); err != nil {
		return err
	}
	if implicit {
		if _, err := e.walLog.Append(wal.RecordCommit, txnID, frame.PageID, nil); err != nil {
			return err
		}
		if err := e.walLog.Sync(); err != nil {
			return err
		}
		if err := e.transactions.Commit(implicitTxn); err != nil {
			return err
		}
	}
	return nil
}
func (e *Engine) readRowsLocked(name string) ([]record.Row, error) {
	table := e.catalog.Tables[name]
	if table == nil || len(table.Pages) == 0 {
		return nil, fmt.Errorf("table %q has no storage", name)
	}
	frame, err := e.bufferPool.Fetch(table.Pages[0])
	if err != nil {
		return nil, err
	}
	rows, decodeErr := record.DecodeRows(frame.Page.Data[:])
	if err := e.bufferPool.Unpin(frame.PageID); err != nil {
		return nil, err
	}
	return rows, decodeErr
}
func (e *Engine) writeRowsLocked(name string, rows []record.Row) error {
	table := e.catalog.Tables[name]
	if table == nil {
		return fmt.Errorf("table %q does not exist", name)
	}
	b := record.EncodeRows(rows)
	if len(b) > storage.PageSize-storage.PageHeaderSize {
		return fmt.Errorf("row set too large for one-page table storage")
	}
	frame, err := e.bufferPool.Fetch(table.Pages[0])
	if err != nil {
		return err
	}
	frame.Page.Data = [storage.PageSize - storage.PageHeaderSize]byte{}
	copy(frame.Page.Data[:], b)
	if err := e.bufferPool.MarkDirty(frame.PageID); err != nil {
		return err
	}
	implicit := false
	txn := e.activeTxn
	if txn == nil {
		txn = e.transactions.Begin()
		implicit = true
		if _, err := e.walLog.Append(wal.RecordBegin, txn.ID, frame.PageID, nil); err != nil {
			return err
		}
	}
	if err := e.transactions.Lock().Acquire(name, txn.ID, transaction.Exclusive); err != nil {
		return err
	}
	if _, err := e.walLog.Append(wal.RecordUpdate, txn.ID, frame.PageID, b); err != nil {
		return err
	}
	if err := e.walLog.Sync(); err != nil {
		return err
	}
	if err := e.bufferPool.Flush(frame.PageID); err != nil {
		return err
	}
	if err := e.bufferPool.Unpin(frame.PageID); err != nil {
		return err
	}
	if implicit {
		if _, err := e.walLog.Append(wal.RecordCommit, txn.ID, frame.PageID, nil); err != nil {
			return err
		}
		if err := e.walLog.Sync(); err != nil {
			return err
		}
		if err := e.transactions.Commit(txn); err != nil {
			return err
		}
	}
	return nil
}
func (e *Engine) cloneCatalog() *catalog.Catalog {
	b, _ := e.catalog.Encode()
	c := catalog.New()
	c.Decode(b)
	return c
}
func columnIndex(table *catalog.Table, name string) int {
	for i, c := range table.Columns {
		if strings.EqualFold(c.Name, name) {
			return i
		}
	}
	return -1
}
func validate(col catalog.Column, v record.Value) error {
	ok := false
	switch strings.ToUpper(col.Type) {
	case "INT", "INTEGER":
		ok = v.Kind == record.Integer
	case "TEXT", "VARCHAR":
		ok = v.Kind == record.Text
	case "BOOLEAN", "BOOL":
		ok = v.Kind == record.Boolean
	case "FLOAT", "REAL", "DOUBLE":
		ok = v.Kind == record.Integer || v.Kind == record.Float
	}
	if !ok {
		return fmt.Errorf("value has incompatible type for column %s", col.Name)
	}
	return nil
}
func (e *Engine) matches(table *catalog.Table, row record.Row, where *ast.Expression) bool {
	if where.Operator == "AND" {
		return e.matches(table, row, where.Left) && e.matches(table, row, where.Right)
	}
	if where.Operator == "OR" {
		return e.matches(table, row, where.Left) || e.matches(table, row, where.Right)
	}
	i := columnIndex(table, where.Column)
	if i < 0 {
		return false
	}
	a, b := row.Values[i], where.Value
	compare := 0
	if a.Kind == record.Integer && b.Kind == record.Integer {
		if a.I < b.I {
			compare = -1
		} else if a.I > b.I {
			compare = 1
		}
	}
	if a.Kind == record.Text && b.Kind == record.Text {
		compare = strings.Compare(a.S, b.S)
	}
	if a.Kind == record.Boolean && b.Kind == record.Boolean {
		if a.B == b.B {
			compare = 0
		} else if !a.B && b.B {
			compare = -1
		} else {
			compare = 1
		}
	}
	switch where.Operator {
	case "=":
		return compare == 0
	case "!=":
		return compare != 0
	case ">":
		return compare > 0
	case "<":
		return compare < 0
	case ">=":
		return compare >= 0
	case "<=":
		return compare <= 0
	}
	return false
}
