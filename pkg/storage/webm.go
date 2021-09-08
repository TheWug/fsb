package storage

import (
	tgtypes "github.com/thewug/gogram/data"

	"database/sql"
	"strings"
)

func FindCachedMp4ForWebm(d DBLike, md5 string) (*tgtypes.FileID, error) {
	query := "SELECT telegram_id FROM webms_converted_for_telegram WHERE md5 = $1"
	out := new(tgtypes.FileID)

	err := d.Enter(func(tx Queryable) error { return tx.QueryRow(query, strings.ToLower(md5)).Scan(&out) })

	if err != nil {
		out = nil
		if err == sql.ErrNoRows {
			err = nil
		}
	}
	return out, err
}

func SaveCachedMp4ForWebm(d DBLike, md5 string, id tgtypes.FileID) error {
	query := "INSERT INTO webms_converted_for_telegram (md5, telegram_id) VALUES ($1, $2) ON CONFLICT (md5) DO UPDATE SET telegram_id = EXCLUDED.telegram_id"

	return d.Enter(func(tx Queryable) error { return WrapExec(tx.Exec(query, strings.ToLower(md5), id)) })
}
