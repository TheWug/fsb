package storage

import (
	"context"
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
func Transact(db_connection *sql.DB, callback func(DBLike) error) error {
	tx, err := db_connection.Begin()
	if err != nil {
		return err
	}

	err = callback(dbWrapper{Queryable: tx, in_tx: true})

	if err == nil {
		err = tx.Commit()
	} else {
		typedErr := err
		switch typedErr.(type) {
		case RollbackAndMask:
			err = tx.Rollback()
		case CommitAndYield:
			innerErr := tx.Commit()
			if innerErr != nil { return innerErr }
		default:
			innerErr := tx.Rollback()
			if innerErr != nil { return innerErr }
		}
	}
	return err
}

func DefaultTransact(callback func(DBLike) error) error {
	return Transact(Db_pool, callback)
}

// broadly compatible with both *sql.DB and *sql.Tx
type Queryable interface {
	Exec(string, ...interface{}) (sql.Result, error)
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	Query(string, ...interface{}) (*sql.Rows, error)
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
	QueryRow(string, ...interface{}) *sql.Row
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
}

// a generic interface which is able to perform queries, and is aware of whether or not
// it is within a transaction.
type DBLike interface {
	Exec(string, ...interface{}) (sql.Result, error)
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	Query(string, ...interface{}) (*sql.Rows, error)
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
	QueryRow(string, ...interface{}) *sql.Row
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row

	InTransaction() bool
}

type dbWrapper struct {
	Queryable

	in_tx bool
}

func (d dbWrapper) InTransaction() bool {
	return d.in_tx
}

func NoTx(database *sql.DB) DBLike {
	return dbWrapper{Queryable: database}
}

func DefaultNoTx() DBLike {
	return NoTx(Db_pool)
}
