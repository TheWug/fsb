package storage

import (
	tgtypes "github.com/thewug/gogram/data"

	"time"
)

type DialogPost struct {
	MsgId tgtypes.MsgID
	MsgTs time.Time
	ChatId tgtypes.ChatID
	DialogId tgtypes.DialogID
	DialogData []byte
}

func WriteDialogPost(tx DBLike, dialog_id tgtypes.DialogID, msg_id tgtypes.MsgID, chat_id tgtypes.ChatID, json string, msg_ts time.Time) error {
	sql := "INSERT INTO dialog_posts (dialog_id, msg_id, chat_id, dialog_data, msg_ts) VALUES ($1, $2, $3, $4, $5) ON CONFLICT ON CONSTRAINT dialog_posts_pkey DO UPDATE SET dialog_data = EXCLUDED.dialog_data"

	_, err := tx.Exec(sql, dialog_id, msg_id, chat_id, json, msg_ts)
	return err
}

func EraseDialogPost(tx DBLike, msg_id tgtypes.MsgID, chat_id tgtypes.ChatID) error {
	sql := "DELETE FROM dialog_posts WHERE msg_id = $1 AND chat_id = $2"

	_, err := tx.Exec(sql, msg_id, chat_id)
	return err
}

func FetchDialogPost(tx DBLike, msg_id tgtypes.MsgID, chat_id tgtypes.ChatID) (*DialogPost, error) {
	sql := "SELECT chat_id, msg_id, msg_ts, dialog_id, dialog_data FROM dialog_posts WHERE msg_id = $1 AND chat_id = $2"
	row := tx.QueryRow(sql, msg_id, chat_id)

	var out DialogPost
	err := row.Scan(&out.ChatId, &out.MsgId, &out.MsgTs, &out.DialogId, &out.DialogData)
	if err != nil { return nil, err }
	return &out, err
}
