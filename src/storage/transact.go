package storage

import (
	"database/sql"
)

type rollbackError struct {
	what string
}

func (r rollbackError) Error() string { return r.what }
func (r rollbackError) Masked()       {}

type commitError rollbackError

func (c commitError) Error() string { return c.what }
func (c commitError) Yielded()      {}

// RollbackAndMaskError produces an error which implements RollbackAndMask.
func RollbackAndMaskError(message string) error {
	return rollbackError{what: message}
}

// CommitAndYieldError produces an error which implements CommitAndYield.
func CommitAndYieldError(message string) error {
	return commitError{what: message}
}

// Returning a RollbackAndMask from within a function called by Transact produces special behavior, described by Transact's docs.
type RollbackAndMask interface {
	Masked()
}

// Returning a CommitAndYield from within a function called by Transact produces special behavior, described by Transact's docs.
type CommitAndYield interface {
	Yielded()
}

// Usage: storage.Transact(Database, func(tx *sql.Tx) error { var err error; output, err = storage.CallSomeFunction(tx, foo, bar, etc, other); return err })
// Takes a callback which accesses the database and wraps it in a transaction.
// the callback may return an error:
// - If the error is nil or implements CommitAndYield, the transaction will be committed.
// - Otherwise, the transaction will be rolled back.
// - If the error is nil or implements RollbackAndMask, a nil error will be returned to the caller, indicating success.
// - Otherwise, the error will be returned to the caller.
// - If tx.Commit (or tx.Rollback, whichever is used) fails with an error, that error will be returned instead, superceding the previous options.
func Transact(db_connection *sql.DB, callback func(*sql.Tx) error) error {
	tx, err := db_connection.Begin()
	if err != nil {
		return err
	}

	err = callback(tx)

	if err == nil {
		err = tx.Commit()
	} else if err.(RollbackAndMask) != nil {
		err = tx.Rollback()
	} else if err.(CommitAndYield) != nil {
		innerErr := tx.Commit()
		if innerErr != nil {
			return innerErr
		}
	} else {
		innerErr := tx.Rollback()
		if innerErr != nil {
			return innerErr
		}
	}
	return err
}

