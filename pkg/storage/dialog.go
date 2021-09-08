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

func WriteDialogPost(d DBLike, dialog_id tgtypes.DialogID, msg_id tgtypes.MsgID, chat_id tgtypes.ChatID, json string, msg_ts time.Time) error {
	query := "INSERT INTO dialog_posts (dialog_id, msg_id, chat_id, dialog_data, msg_ts) VALUES ($1, $2, $3, $4, $5) ON CONFLICT ON CONSTRAINT dialog_posts_pkey DO UPDATE SET dialog_data = EXCLUDED.dialog_data"

	return d.Enter(func(tx Queryable) error { return WrapExec(tx.Exec(query, dialog_id, msg_id, chat_id, json, msg_ts)) })
}

func EraseDialogPost(d DBLike, msg_id tgtypes.MsgID, chat_id tgtypes.ChatID) error {
	query := "DELETE FROM dialog_posts WHERE msg_id = $1 AND chat_id = $2"

	return d.Enter(func(tx Queryable) error { return WrapExec(tx.Exec(query, msg_id, chat_id)) })
}

func FetchDialogPost(d DBLike, msg_id tgtypes.MsgID, chat_id tgtypes.ChatID) (*DialogPost, error) {
	query := "SELECT chat_id, msg_id, msg_ts, dialog_id, dialog_data FROM dialog_posts WHERE msg_id = $1 AND chat_id = $2"
	out := &DialogPost{}

	err := d.Enter(func(tx Queryable) error {
		return tx.QueryRow(query, msg_id, chat_id).Scan(&out.ChatId, &out.MsgId, &out.MsgTs, &out.DialogId, &out.DialogData)
	})

	if err != nil {
		out = nil
	}
	return out, err
}
