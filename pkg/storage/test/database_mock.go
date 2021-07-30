package test

import (
	"github.com/thewug/fsb/pkg/storage"
	
	"database/sql"
	"os"
	"os/exec"
	"io/ioutil"
	"encoding/json"
	"fmt"
	"errors"
)

var db_test *sql.DB

var settings struct {
	User string   `json:"user"`
	Dbname string `json:"dbname"`
	Host string   `json:"host"`
	Port string   `json:"port"`
}

func dburl() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s", settings.User, settings.User, settings.Host, settings.Port, settings.Dbname)
}

func ensureSettings() error {
	if settings.User != "" { return nil }
	
	file, err := os.Open("../../database/test_db.json")
	if err != nil { return err }
	
	data, err := ioutil.ReadAll(file)
	if err != nil { return err }
	
	err = json.Unmarshal(data, &settings)
	if err != nil { return err }
	
	if settings.User == "" { return errors.New("settings file did not produce valid configuration") }
	
	return nil
}

func TestDatabase() (*sql.DB, error) {
	if db_test != nil { return db_test, nil }
	
	err := ensureSettings()
	if err != nil { return nil, err }
	
	db, err := sql.Open("postgres", dburl())
	if err != nil { return nil, err }
	
	db_test = db
	return db, nil
}

func ResetTestDatabase() error {
	err := ensureSettings()
	if err != nil { return err }
	
	cmd := exec.Command("../../database/reset_test_database.sh", settings.User, settings.Dbname, settings.Host, settings.Port)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", settings.User))
	return cmd.Run()
}

func Rollbacker(fn func(storage.DBLike) error) (func(storage.DBLike) error) {
	return func(tx storage.DBLike) error {
		err := fn(tx)
		if err == nil { err = storage.NewRollbackAndMask("success") }
		return err
	}
}
