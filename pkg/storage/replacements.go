package storage

import (
	"time"

	"github.com/thewug/fsb/pkg/api/tags"

	"github.com/lib/pq"
	tgdata "github.com/thewug/gogram/data"
)

type Replacer struct {
	Id          int64
	MatchSpec   string
	ReplaceSpec string
	Autofix     bool
}

type Scannable interface {
	Scan(...interface{}) error
}

func (this *Replacer) ScanFrom(rows Scannable) error {
	return rows.Scan(&this.Id, &this.MatchSpec, &this.ReplaceSpec, &this.Autofix)
}

func (this *Replacer) Matcher() ReplacerMatcher {
	return ReplacerMatcher{
		MatchSpec: tags.TagDiffFromString(this.MatchSpec),
		ReplaceSpec: tags.TagDiffFromString(this.ReplaceSpec),
	}
}

type ReplacerMatcher struct {
	MatchSpec, ReplaceSpec tags.TagDiff
}

func (rm *ReplacerMatcher) Matches(postTags tags.TagSet) bool {
	for k, _ := range rm.MatchSpec.AddList {
		if postTags.Status(k) == tags.NotPresent { return false }
	}
	for k, _ := range rm.MatchSpec.RemoveList {
		if postTags.Status(k) == tags.AddsTag { return false }
	}
	return true
}

type ReplacementHistory struct {
	ReplacementHistoryKey

	Id int64
	TelegramUserId tgdata.UserID
	Timestamp time.Time
}

type ReplacementHistoryKey struct {
	ReplacerId int64
	PostId int
}

func (this *ReplacementHistory) ScanFrom(rows Scannable) error {
	return rows.Scan(&this.Id, &this.TelegramUserId, &this.ReplacerId, &this.PostId, &this.Timestamp)
}

func AddReplacement(tx DBLike, repl Replacer) (*Replacer, error) {
	query := "INSERT INTO replacements (match_spec, replace_spec, autofix) VALUES ($1, $2, $3) RETURNING replace_id"

	row := tx.QueryRow(query, repl.MatchSpec, repl.ReplaceSpec, repl.Autofix)
	err := row.Scan(&repl.Id)
	return &repl, err
}

func UpdateReplacement(tx DBLike, repl Replacer) error {
	query := "UPDATE replacements SET match_spec = $2, replace_spec = $3, autofix = $4 WHERE replace_id = $1"
	_, err := tx.Exec(query, repl.Id, repl.MatchSpec, repl.ReplaceSpec, repl.Autofix)
	return err
}

func DeleteReplacement(tx DBLike, id int64) (error) {
	query := "DELETE FROM replacements WHERE replace_id = $1"
	_, err := tx.Exec(query, id)
	return err
}

func GetReplacements(tx DBLike, after_id int64) ([]Replacer, error) {
	query := "SELECT replace_id, match_spec, replace_spec, autofix FROM replacements WHERE replace_id > $1 ORDER BY replace_id LIMIT 500"
	rows, err := tx.Query(query, after_id)
	defer rows.Close()
	if err != nil { return nil, err }

	var out []Replacer

	for rows.Next() {
		var r Replacer
		err = r.ScanFrom(rows)
		if err != nil { return nil, err }

		out = append(out, r)
	}

	return out, nil
}

func GetReplacementHistorySince(tx DBLike, post_ids []int, since time.Time) (map[ReplacementHistoryKey]ReplacementHistory, error) {
	query := "SELECT action_id, telegram_user_id, replace_id, post_id, action_ts FROM replacement_actions WHERE post_id = ANY($1::int[]) AND action_ts > $2"
	rows, err := tx.Query(query, pq.Array(post_ids), since)
	if err != nil { return nil, err }
	defer rows.Close()

	out := make(map[ReplacementHistoryKey]ReplacementHistory)

	for rows.Next() {
		var r ReplacementHistory
		err = r.ScanFrom(rows)
		if err != nil { return nil, err }

		out[r.ReplacementHistoryKey] = r
	}

	return out, nil
}

func AddReplacementHistory(tx DBLike, event *ReplacementHistory) error {
	query := "INSERT INTO replacement_actions (action_id, telegram_user_id, replace_id, post_id, action_ts) VALUES (default, $1, $2, $3, $4) RETURNING action_id"
	row := tx.QueryRow(query, event.TelegramUserId, event.ReplacerId, event.PostId, event.Timestamp)
	err := row.Scan(event.Id)
	return err // ErrNoRows will be passed through here, and we do want to propagate that because it should never happen
}
