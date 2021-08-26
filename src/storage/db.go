package storage

import (
	"database/sql"
	"log"

	_ "github.com/lib/pq"
)

var Db_pool *sql.DB

type committer struct {
	commit bool
}

func handle_transaction(c *committer, tx *sql.Tx) {
	if c.commit {
		tx.Commit()
	} else {
		tx.Rollback()
	}
}

// initialize the DAL. Closing it might be important at some point, but who cares right now.
func DBInit(dburl string) (error) {
	var err error
	log.Println("[util    ] Connecting to postgres...")
	Db_pool, err = sql.Open("postgres", dburl)
	if err != nil {
		return err
	}
	log.Println("[util    ] OK!")
	return nil
}

func WriteUserCreds(id int, username, key string) (error) {
	tx, err := Db_pool.Begin()
	if err != nil { return err }

	var c committer
	defer handle_transaction(&c, tx)

	_, err = tx.Exec("DELETE FROM remote_user_credentials WHERE telegram_id = $1", id)
	if (err != nil) { return err }
	_, err = tx.Exec("INSERT INTO remote_user_credentials (telegram_id, api_user, api_apikey) VALUES ($1, $2, $3)", id, username, key)
	if (err != nil) { return err }

	c.commit = true
	return nil
}

func GetUserCreds(id int) (string, string, error) {
	row := Db_pool.QueryRow("SELECT api_user, api_apikey FROM remote_user_credentials WHERE telegram_id = $1", id)
	var user, key string
	err := row.Scan(&user, &key)
	return user, key, err
}

func WriteUserTagRules(id int, name, rules string) (error) {
	tx, err := Db_pool.Begin()
	if err != nil { return err }

	var c committer
	defer handle_transaction(&c, tx)

	_, err = tx.Exec("DELETE FROM user_tagrules WHERE telegram_id = $1 AND name = $2", id, name)
	if (err != nil) { return err }
	_, err = tx.Exec("INSERT INTO user_tagrules (telegram_id, name, rules) VALUES ($1, $2, $3)", id, name, rules)
	if (err != nil) { return err }

	c.commit = true
	return nil
}

func GetUserTagRules(id int, name string) (string, error) {
	row := Db_pool.QueryRow("SELECT (rules) FROM user_tagrules WHERE telegram_id = $1 AND name = $2", id, name)
	var rules string
	err := row.Scan(&rules)
	if err == sql.ErrNoRows { err = nil } // no data for user is not an error.
	return rules, err
}
