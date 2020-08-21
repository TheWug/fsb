package storage

import (
	"bot/types"

	tgtypes "github.com/thewug/gogram/data"

	"database/sql"
)

type UserSettings struct {
	TelegramId tgtypes.UserID
	AgeStatus types.AgeStatus
	RatingMode types.RatingMode
	BlacklistMode types.BlacklistMode
}

func GetUserSettings(settings UpdaterSettings, telegram_id tgtypes.UserID) (*UserSettings, error) {
	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return nil, settings.Transaction.err }

	query := "SELECT telegram_id, age_status, rating_mode, blacklist_mode FROM user_settings WHERE telegram_id = $1"
	row := tx.QueryRow(query, telegram_id)

	var u UserSettings
	err := row.Scan(&u.TelegramId, &u.AgeStatus, &u.RatingMode, &u.BlacklistMode)

	if err == sql.ErrNoRows {
		u.TelegramId = telegram_id
	} else if err != nil {
		return nil, err
	}

	settings.Transaction.commit = mine && (err == nil)
	return &u, nil
}

func WriteUserSettings(settings UpdaterSettings, s *UserSettings) (error) {
	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

	query := "INSERT INTO user_settings (telegram_id, age_status, rating_mode, blacklist_mode) VALUES ($1, $2, $3, $4) ON CONFLICT (telegram_id) DO UPDATE SET age_status = EXCLUDED.age_status, rating_mode = EXCLUDED.rating_mode, blacklist_mode = EXCLUDED.blacklist_mode"
	_, err := tx.Exec(query, s.TelegramId, s.AgeStatus, s.RatingMode, s.BlacklistMode)

	settings.Transaction.commit = mine && (err == nil)
	return err
}

func DeleteUserSettings(settings UpdaterSettings, id tgtypes.UserID) (error) {
	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

	query := "UPDATE user_settings SET age_status = LEAST(age_status, 0), rating_mode = 0, blacklist_mode = 0 WHERE telegram_id = $1"
	_, err := tx.Exec(query, id)

	settings.Transaction.commit = mine && (err == nil)
	return err
}
