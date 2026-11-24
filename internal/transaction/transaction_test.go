package transaction

import "testing"

func TestBeginCommitAndLocks(t *testing.T) {
	manager := NewManager()
	txn := manager.Begin()
	if txn.State != Active {
		t.Fatal("new transaction should be active")
	}
	if err := manager.Lock().Acquire("users", txn.ID, Exclusive); err != nil {
		t.Fatal(err)
	}
	other := manager.Begin()
	if err := manager.Lock().Acquire("users", other.ID, Shared); err == nil {
		t.Fatal("exclusive/shared lock conflict was not detected")
	}
	if err := manager.Commit(txn); err != nil {
		t.Fatal(err)
	}
	if err := manager.Lock().Acquire("users", other.ID, Shared); err != nil {
		t.Fatal(err)
	}
	if err := manager.Abort(other); err != nil {
		t.Fatal(err)
	}
}
