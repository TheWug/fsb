package storage

import (
	"time"

	"github.com/thewug/fsb/pkg/api/tags"

	"github.com/lib/pq"
	tgdata "github.com/thewug/gogram/data"
	"github.com/thewug/dml"
)

type Replacer struct {
	Id          int64  `dml:"replace_id"`
	MatchSpec   string `dml:"match_spec"`
	ReplaceSpec string `dml:"replace_spec"`
	Autofix     bool   `dml:"autofix"`
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

	Id             int64         `dml:"action_id"`
	TelegramUserId tgdata.UserID `dml:"telegram_user_id"`
	Timestamp      time.Time     `dml:"action_ts"`
}

type ReplacementHistoryKey struct {
	ReplacerId int64 `dml:"replace_id"`
	PostId     int   `dml:"post_id"`
}

func AddReplacement(d DBLike, repl Replacer) (*Replacer, error) {
	query := "INSERT INTO replacements (match_spec, replace_spec, autofix) VALUES ($1, $2, $3) RETURNING replace_id"

	err := d.Enter(func(tx Queryable) error { return tx.QueryRow(query, repl.MatchSpec, repl.ReplaceSpec, repl.Autofix).Scan(&repl.Id) })
	return &repl, err
}

func UpdateReplacement(d DBLike, repl Replacer) error {
	query := "UPDATE replacements SET match_spec = $2, replace_spec = $3, autofix = $4 WHERE replace_id = $1"
	return d.Enter(func(tx Queryable) error { return WrapExec(tx.Exec(query, repl.Id, repl.MatchSpec, repl.ReplaceSpec, repl.Autofix)) })
}

func DeleteReplacement(d DBLike, id int64) (error) {
	query := "DELETE FROM replacements WHERE replace_id = $1"
	return d.Enter(func(tx Queryable) error { return WrapExec(tx.Exec(query, id)) })
}

func GetReplacements(d DBLike, after_id int64, page_size int) ([]Replacer, error) {
	query := "SELECT replace_id, match_spec, replace_spec, autofix FROM replacements WHERE replace_id > $1 ORDER BY replace_id LIMIT $2"
	var out []Replacer

	err := d.Enter(func(tx Queryable) error {
		rows, err := dml.X(tx.Query(query, after_id, page_size))
		if err != nil { return err }
		defer rows.Close()

		return dml.ScanArray(rows, &out)
	})

	if err != nil {
		out = nil
	}
	return out, err
}

type ReplacersPage struct {
	Replacers []Replacer
	Err         error
}

func PaginatedGetAllReplacements(d DBLike, page_size int) chan ReplacersPage {
	out := make(chan ReplacersPage)

	go func() {
		current_id := int64(-1)
		for {
			replacers, err := GetReplacements(d, current_id, page_size)

			if len(replacers) != 0 {
				current_id = replacers[len(replacers) - 1].Id
				out <- ReplacersPage{Replacers: replacers}
			} else if err != nil {
				out <- ReplacersPage{Err: err}
				break
			} else {
				break
			}
		}

		close(out)
	}()

	return out
}

func GetReplacementHistorySince(d DBLike, post_ids []int, since time.Time) (map[ReplacementHistoryKey]ReplacementHistory, error) {
	query := "SELECT action_id, telegram_user_id, replace_id, post_id, action_ts FROM replacement_actions WHERE post_id = ANY($1::int[]) AND action_ts > $2"
	out := make(map[ReplacementHistoryKey]ReplacementHistory)

	d.Enter(func(tx Queryable) error {
		rows, err := dml.X(tx.Query(query, pq.Array(post_ids), since))
		if err != nil { return err }
		defer rows.Close()

		for rows.Next() {
			var r ReplacementHistory
			err = dml.Scan(rows, &r)
			if err != nil { return err }

			out[r.ReplacementHistoryKey] = r
		}

		return nil
	})

	return out, nil
}

func AddReplacementHistory(d DBLike, event *ReplacementHistory) error {
	query := "INSERT INTO replacement_actions (action_id, telegram_user_id, replace_id, post_id, action_ts) VALUES (default, $1, $2, $3, $4) RETURNING action_id"

	return d.Enter(func(tx Queryable) error {
		return tx.QueryRow(query, event.TelegramUserId, event.ReplacerId, event.PostId, event.Timestamp).Scan(event.Id)
	}) // ErrNoRows will be passed through here, and we do want to propagate that because it should never happen
}
