package transaction

import (
	"fmt"
	"sync"
)

type State uint8

const (
	Active State = iota
	Committed
	Aborted
)

type Transaction struct {
	ID    uint64
	State State
}

type Manager struct {
	mu     sync.Mutex
	nextID uint64
	active map[uint64]*Transaction
	locks  *LockManager
}

func NewManager() *Manager {
	return &Manager{
		nextID: 1,
		active: make(map[uint64]*Transaction),
		locks:  NewLockManager(),
	}
}

func (m *Manager) Begin() *Transaction {
	m.mu.Lock()
	defer m.mu.Unlock()
	txn := &Transaction{ID: m.nextID, State: Active}
	m.nextID++
	m.active[txn.ID] = txn
	return txn
}

func (m *Manager) Commit(txn *Transaction) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.active[txn.ID]
	if !ok {
		return fmt.Errorf("transaction %d is not active", txn.ID)
	}
	if current.State != Active {
		return fmt.Errorf("transaction %d is not active", txn.ID)
	}
	current.State = Committed
	delete(m.active, txn.ID)
	m.locks.Release(txn.ID)
	txn.State = Committed
	return nil
}

func (m *Manager) Abort(txn *Transaction) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.active[txn.ID]
	if !ok {
		return fmt.Errorf("transaction %d is not active", txn.ID)
	}
	current.State = Aborted
	delete(m.active, txn.ID)
	m.locks.Release(txn.ID)
	txn.State = Aborted
	return nil
}

func (m *Manager) Lock() *LockManager { return m.locks }
