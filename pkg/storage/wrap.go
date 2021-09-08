package storage

import (
	"database/sql"
)

func WrapExec(_ sql.Result, err error) error {
	return err
}
