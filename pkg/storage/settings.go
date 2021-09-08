package storage

import (
	"github.com/thewug/fsb/pkg/bot/types"

	tgtypes "github.com/thewug/gogram/data"

	"database/sql"
)

type UserSettings struct {
	TelegramId tgtypes.UserID
	AgeStatus types.AgeStatus
	RatingMode types.RatingMode
	BlacklistMode types.BlacklistMode
}

func GetUserSettings(d DBLike, telegram_id tgtypes.UserID) (*UserSettings, error) {
	query := "SELECT telegram_id, age_status, rating_mode, blacklist_mode FROM user_settings WHERE telegram_id = $1"
	u := &UserSettings{}

	err := d.Enter(func(tx Queryable) error { return tx.QueryRow(query, telegram_id).Scan(&u.TelegramId, &u.AgeStatus, &u.RatingMode, &u.BlacklistMode) })

	if err == sql.ErrNoRows {
		u.TelegramId = telegram_id
		err = nil
	}

	if err != nil {
		u = nil
	}
	return u, err
}

func WriteUserSettings(d DBLike, s *UserSettings) (error) {
	query := "INSERT INTO user_settings (telegram_id, age_status, rating_mode, blacklist_mode) VALUES ($1, $2, $3, $4) ON CONFLICT (telegram_id) DO UPDATE SET age_status = EXCLUDED.age_status, rating_mode = EXCLUDED.rating_mode, blacklist_mode = EXCLUDED.blacklist_mode"
	return d.Enter(func(tx Queryable) error { return WrapExec(tx.Exec(query, s.TelegramId, s.AgeStatus, s.RatingMode, s.BlacklistMode)) })
}

func DeleteUserSettings(d DBLike, id tgtypes.UserID) (error) {
	query := "UPDATE user_settings SET age_status = LEAST(age_status, 0), rating_mode = 0, blacklist_mode = 0 WHERE telegram_id = $1"
	return d.Enter(func(tx Queryable) error { return WrapExec(tx.Exec(query, id)) })
}
