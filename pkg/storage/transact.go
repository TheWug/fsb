package storage

import (
	"context"
	"errors"
	"fmt"
	"database/sql"
)

// TODO split this into a new package and use this as its documentation

//     OVERVIEW
// This code helps make database transactions easier and more straightforward.
// The workhorse type here is `interface DBLike`, which represents a database wrapper with optional transaction.
// Since both sql.Tx and sql.DB implement the same functions for query/exec, DBLike offers the functionality of both
// behind an interface. DBLike supports both performing queries within a transaction, and performing bare queries,
// through the same interface.
//
//
//     USAGE
// If you have a collection of things you want to group together into a transaction, use Transact() to manufacture
// a DBLike which will manage the transaction for you. Transact takes a callback and passes the DBLike to it, which
// gives it both the first and the last word about how errors are handled, and thus reduces transaction management
// boilerplate in the caller's code.
//
// Errors are returned in the following order of precedence:
// - Errors which occur while opening the transaction
// - Errors which occur while closing the transaction
// - Errors which occur in the user-specified callback
// (note that in general, rolling back a transaction which encountered an error is not itself an error, so while
// close-out errors take precedence, the user-specified callback's error will usually propagate if it exists)
//
//
//     EXAMPLE:
// transaction-wrapping a function:
//
//   var some_parameter interface{}
//   var some_db       *sql.DB
//   var err            error
//
//   func InTransaction(tx DBLike, param interface{}) error {
//       ...
//   }
//
//   err = Transact(some_db, func(tx DBLike) error { return InTransaction(tx, some_parameter) })
//
// callbacks passed to Transact can return certain special errors, which are handled in special ways:
// - errors implementing RollbackAndMask cause the transaction to rollback, but Transact returns success.
// - errors implementing CommitAndYield cause the transaction to commit, but Transact returns the error.
// as previously, errors while closing the transaction (if they occur) will override these errors.
//
//
//     EXAMPLE:
// operations outside of a transaction:
//
//   var some_db *sql.DB
//
//   func NotInTransaction(tx DBLike, params ...interface{}) (err error) {
//       ...
//   }
//
//   NotInTransaction(NoTx(some_db), 1, 2, 3)
//
//
//     EXAMPLE:
// requiring an explicit transaction from within a function:
//
//   func RequiresTransaction(tx DBLike, params ...interface{}) (err error) {
//       if !tx.InTransaction() { return NewTxRequired("RequiresTransaction") }
//       ...
//   }
//
//
//     EXAMPLE:
// upgrading to a transaction from within a function:
//
//   func UpgradeToTransaction(tx DBLike, params ...interface{}) (err error) {
//       defer tx.EnsureTransaction(&err)()
//       if err != nil { return err }
//       ...
//   }
//
// If you're already in a transaction, this will be a no-op.
// If a transaction is created, it will be closed after the function returns, when deferred calls run.
// The same rules about special error types as described by Transact() are obeyed here (in fact, Transact()
// uses this mechanism internally to create transactions).
//

// base error struct for all normal errors in this file
type transactError struct {
	error
}

// named interface for unwrappable errors
type unwrappableError interface {
	error
	Unwrap() error
}

// base error struct for all unwrappable errors in this file
type transactErrorUnwrappable struct {
	unwrappableError
}

// Returning a RollbackAndMask from within a function called by Transact produces special behavior, described by Transact's docs.
type RollbackAndMask interface { Masked() }

// Returning a CommitAndYield from within a function called by Transact produces special behavior, described by Transact's docs.
type CommitAndYield interface { Yielded() }

// Used to flag to callers of functions which are called without transactions that they failed due to the absence of a transaction
// but could succeed if properly transacted.
type RequiresTransaction interface { RequiresTx() }

// Error implementations.
// These are mostly boilerplate struct and function definitions, and are all module private
// because their implementations are simple and callers shouldn't need to tamper with them
// (though they are free to implement their own if they require special features above and beyond
// those provided here).

type rollbackError transactError
func (r rollbackError) Masked() {}
type rollbackErrorUnwrappable transactErrorUnwrappable
func (r rollbackErrorUnwrappable) Masked() {}

type commitError transactError
func (c commitError) Yielded() {}
type commitErrorUnwrappable transactErrorUnwrappable
func (r commitErrorUnwrappable) Yielded() {}

type txRequiredError transactError
func (c txRequiredError) RequiresTx() {}
// txRequiredError doesn't need an unwrappable version

// NewRollbackAndMask produces an error which implements RollbackAndMask.
func NewRollbackAndMask(message string) error {
	return rollbackError{error: errors.New(message)}
}

// RollbackAndMaskErrorf produces an unwrappable error (see fmt.Errorf) which implements RollbackAndMask.
func RollbackAndMaskErrorf(format string, args ...interface{}) error {
	return rollbackErrorUnwrappable{unwrappableError: fmt.Errorf(format, args...).(unwrappableError)}
}

// NewCommitAndYield produces an error which implements CommitAndYield.
func NewCommitAndYield(message string) error {
	return commitError{error: errors.New(message)}
}

// CommitAndYieldErrorf produces an unwrappable error (see fmt.Errorf) which implements CommitAndYield.
func CommitAndYieldErrorf(format string, args ...interface{}) error {
	return commitErrorUnwrappable{unwrappableError: fmt.Errorf(format, args...).(unwrappableError)}
}

// TxRequiredError produces an error which implements RequiresTransaction.
func NewTxRequired(message string) error {
	return txRequiredError{error: errors.New(message)}
}

// Queryable is compatible with query/exec methods of both *sql.DB and *sql.Tx.
type Queryable interface {
	Exec(string, ...interface{}) (sql.Result, error)
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	Query(string, ...interface{}) (*sql.Rows, error)
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
	QueryRow(string, ...interface{}) *sql.Row
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
}

// DBLike is a generic interface that encapsulates a database, and optionally also a transaction.
type DBLike interface {
	Queryable

	InTransaction() bool
	EnsureTransaction(*error) func()
}

// queryableError implements Queryable and always returns its error from Queryable interface functions
// (or just nil from QueryRow and QueryRowContext).
type queryableError transactError

func (q queryableError) Exec(string, ...interface{}) (sql.Result, error) { return nil, q.error }
func (q queryableError) ExecContext(context.Context, string, ...interface{}) (sql.Result, error) { return nil, q.error }
func (q queryableError) Query(string, ...interface{}) (*sql.Rows, error) { return nil, q.error }
func (q queryableError) QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error) { return nil, q.error }
func (q queryableError) QueryRow(string, ...interface{}) *sql.Row { return nil }
func (q queryableError) QueryRowContext(context.Context, string, ...interface{}) *sql.Row { return nil }

// dbWrapper is the basic DBLike implementation.
type dbWrapper struct {
	Queryable

	database *sql.DB
	tx       *sql.Tx
}

// EnsureTransaction starts a transaction if one is not already started (if one is, it is a no-op).
// its argument should be a pointer to the caller's named return error variable. If an error is encountered
// while creating the transaction, it will store that error into the value pointed to by its argument. Likewise,
// if at the end of the transaction, an error occurs while committing it or rolling it back, that error will
// be stored into the error at the provided pointer. The pointer must not be nil.
//
// Certain types of errors receive special handling: see the interfaces RollbackAndMask and CommitAndYield.
func (d *dbWrapper) EnsureTransaction(parent_return *error) func() {
	if d.InTransaction() { return noop }

	txerr := d.begin()
	if txerr == nil {
		return func(){ d.onParentReturn(parent_return) }
	} else {
		*parent_return = txerr
		return noop
	}
}

// InTransaction returns true if this dbWrapper contains an ongoing, uncommitted, unrollbacked transaction.
func (d *dbWrapper) InTransaction() bool {
	return d.Queryable == d.tx
}

// begin begins a transaction. calling begin with a transaction already open will make a mess, so don't.
func (d *dbWrapper) begin() error {
	var err error
	d.tx, err = d.database.Begin()
	if err != nil {
		d.Queryable = queryableError{error: err}
	} else {
		d.Queryable = d.tx
	}
	return err
}

// rollback rolls back a transaction. calling rollback without a transaction open will make a mess, so don't.
func (d *dbWrapper) rollback() error {
	d.Queryable = d.database
	err := d.tx.Rollback()
	d.tx = nil
	return err
}

// commit commits back a transaction. calling commit without a transaction open will make a mess, so don't.
func (d *dbWrapper) commit() error {
	if !d.InTransaction() { return errors.New("not in transaction") }

	d.Queryable = d.database
	err := d.tx.Commit()
	d.tx = nil
	return err
}

// onParentReturn is the internal body of the callback returned to the caller of EnsureTransaction, if a
// new transaction is created.
func (d *dbWrapper) onParentReturn(parent_return *error) {
	if *parent_return == nil {
		*parent_return = d.commit()
	} else {
		typedErr := *parent_return
		switch typedErr.(type) {
		case RollbackAndMask:
			*parent_return = d.rollback()
		case CommitAndYield:
			innerErr := d.commit()
			if innerErr != nil { *parent_return = innerErr }
		default:
			innerErr := d.rollback()
			if innerErr != nil { *parent_return = innerErr }
		}
	}
}

// NoTx produces a DBLike around a database, without opening a transaction, allowing for untransacted queries.
func NoTx(database *sql.DB) DBLike {
	return &dbWrapper{
		q: database,
		database: database,
	}
}

// DefaultNoTx, like DefaultTransact, is a convenience wrapper for NoTx using the default database Db_pool.
func DefaultNoTx() DBLike {
	return NoTx(Db_pool)
}

// Transact wraps the provided callback in a transaction.
// See the package documentation for usage examples.
func Transact(db_connection *sql.DB, callback func(DBLike) error) (err error) {
	wrapper := NoTx(db_connection)
	defer wrapper.EnsureTransaction(&err)()
	if err != nil { return err }

	return callback(wrapper)
}

// A convenience specialization of Transact which automatically uses Db_pool.
func DefaultTransact(callback func(DBLike) error) error {
	return Transact(Db_pool, callback)
}

// noop is a dummy function which does nothing.
func noop(){}
