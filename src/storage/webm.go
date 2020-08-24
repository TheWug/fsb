package storage

import (
	tgtypes "github.com/thewug/gogram/data"

	"database/sql"
	"strings"
)

func FindCachedMp4ForWebm(md5 string, settings UpdaterSettings) (*tgtypes.FileID, error) {
	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return nil, settings.Transaction.err }

	query := "SELECT telegram_id FROM webms_converted_for_telegram WHERE md5 = $1"

	row := tx.QueryRow(query, strings.ToLower(md5))

	var out tgtypes.FileID
	err := row.Scan(&out)
	if err == sql.ErrNoRows {
		return nil, nil
	}

	return &out, nil
}

func SaveCachedMp4ForWebm(md5 string, id tgtypes.FileID, settings UpdaterSettings) error {
	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

	query := "INSERT INTO webms_converted_for_telegram (md5, telegram_id) VALUES ($1, $2) ON CONFLICT (md5) DO UPDATE SET telegram_id = EXCLUDED.telegram_id"

	_, err := tx.Exec(query, strings.ToLower(md5), id)

	settings.Transaction.commit = mine && (err == nil)
	return err
}
