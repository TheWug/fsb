package storage

import (
	tgtypes "github.com/thewug/gogram/data"

	"database/sql"
	"strings"
)

func FindCachedMp4ForWebm(tx *sql.Tx, md5 string) (*tgtypes.FileID, error) {
	query := "SELECT telegram_id FROM webms_converted_for_telegram WHERE md5 = $1"

	row := tx.QueryRow(query, strings.ToLower(md5))

	var out tgtypes.FileID
	err := row.Scan(&out)
	if err == sql.ErrNoRows {
		return nil, nil
	}

	return &out, nil
}

func SaveCachedMp4ForWebm(tx *sql.Tx, md5 string, id tgtypes.FileID) error {
	query := "INSERT INTO webms_converted_for_telegram (md5, telegram_id) VALUES ($1, $2) ON CONFLICT (md5) DO UPDATE SET telegram_id = EXCLUDED.telegram_id"

	_, err := tx.Exec(query, strings.ToLower(md5), id)

	return err
}
