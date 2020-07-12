package storage

import (
	"database/sql"
)

type TransactionBox struct {
	tx    *sql.Tx
	err    error
	commit bool
}

func (this *TransactionBox) Commit() error {
	return this.tx.Commit()
}

func (this *TransactionBox) Rollback() error {
	return this.tx.Rollback()
}

// This function is intended to be deferred when starting a transaction block.
// its argument is, ideally, the return value of PopulateIfEmpty. If it's true,
// the transaction will be closed out, otherwise this function is a no-op.
// in this way, you can safely handle both situations where a function creates its
// own transaction and is expected to close it, and situations where a function is
// given a pre-existing transaction and is expected to leave it open.

// is idempotent. The first call will commit the transaction, subsequent
// ones are no-ops.

// use it like this:
// mine, tx := box.PopulateIfEmpty(db)
// defer box.Finalize(mine)
func (this *TransactionBox) Finalize(is_transaction_mine bool) {
	if is_transaction_mine && this.tx != nil {
		if this.commit {
			this.Commit()
		} else {
			this.Rollback()
		}
		*this = TransactionBox{}
	} else {
	// if the transaction isn't ours, or there is no transaction (maybe because
	// this isn't the first call), do nothing.
	}
}

func (this *TransactionBox) MarkForCommit() {
	this.commit = true
}

// populates the transaction box with a new transaction. If it succeeds, it returns
// true to indicate that this context "owns" the transaction. if the box was already
// populated, this function is a no-op, and returns false to indicate that the
// transaction is owned by some other context.
func (this *TransactionBox) PopulateIfEmpty(db *sql.DB) (bool, *sql.Tx) {
	if this.tx == nil {
		this.tx, this.err = db.Begin()
		return true, this.tx
	}
	return false, this.tx
}

// creates a new transaction box, containing a fresh transaction.  If you want to use
// Finalize on a transaction created in this way in the context in which it was
// created, pass true for its argument.
func NewTxBox() (TransactionBox, error) {
	newtx, err := Db_pool.Begin()
	return TransactionBox{
		tx: newtx,
	}, err
}
