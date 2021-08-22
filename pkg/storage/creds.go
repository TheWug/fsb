package storage

import (
	"time"
	"database/sql"

	tgtypes "github.com/thewug/gogram/data"

	"github.com/thewug/dml"
)

var ErrNoLogin error = NewCommitAndYield("no stored credentials for telegram user")

type UserCreds struct {
	TelegramId           tgtypes.UserID `dml:"telegram_id"`
	User                 string         `dml:"api_user"`
	ApiKey               string         `dml:"api_apikey"`
	Janitor              bool           `dml:"privilege_janitorial"`
	Blacklist            string         `dml:"api_blacklist"`
	BlacklistFetched     time.Time      `dml:"api_blacklist_last_updated"`
}

func GetUserCreds(tx DBLike, id tgtypes.UserID) (UserCreds, error) {
	creds := UserCreds{TelegramId: id}

	f := func(tx DBLike) error {
		query := "SELECT telegram_id, api_user, api_key, privilege_janitorial, api_blacklist, api_blacklist_last_updated FROM remote_user_credentials WHERE telegram_id = $1"
		if err := dml.QuickScan(tx.QueryRow(query, id), &creds); err == sql.ErrNoRows {
			return ErrNoLogin
		} else {
			return err
		}
	}

	var err error
	if tx == nil {
		err = DefaultTransact(f)
	} else {
		err = f(tx)
	}

	return creds, err
}

func WriteUserCreds(tx DBLike, creds UserCreds) (error) {
	query := `
INSERT INTO remote_user_credentials (telegram_id, api_user, api_key, api_blacklist, api_blacklist_last_updated)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (telegram_id) DO UPDATE
SET	api_user = EXCLUDED.api_user,
	api_key = EXCLUDED.api_key,
	api_blacklist = EXCLUDED.api_blacklist,
	api_blacklist_last_updated = EXCLUDED.api_blacklist_last_updated
`
	_, err := tx.Exec(query, creds.TelegramId, creds.User, creds.ApiKey, creds.Blacklist, creds.BlacklistFetched)
	return err
}

func DeleteUserCreds(tx DBLike, id tgtypes.UserID) (error) {
	query := "DELETE FROM remote_user_credentials WHERE telegram_id = $1"
	_, err := tx.Exec(query, id)
	return err
}

func WriteUserTagRules(tx DBLike, id tgtypes.UserID, name, rules string) (error) {
	_, err := tx.Exec("DELETE FROM user_tagrules WHERE telegram_id = $1 AND name = $2", id, name)
	if (err != nil) { return err }
	_, err = tx.Exec("INSERT INTO user_tagrules (telegram_id, name, rules) VALUES ($1, $2, $3)", id, name, rules)
	return err
}

func GetUserTagRules(tx DBLike, id tgtypes.UserID, name string) (string, error) {
	row := tx.QueryRow("SELECT rules FROM user_tagrules WHERE telegram_id = $1 AND name = $2", id, name)
	var rules string
	err := row.Scan(&rules)
	return rules, err
}

func DeleteUserTagRules(tx DBLike, id tgtypes.UserID) (error) {
	_, err := tx.Exec("DELETE FROM user_tagrules WHERE telegram_id = $1", id)
	return err
}
