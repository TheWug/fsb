package storage

import (
	"api/tags"

	"github.com/lib/pq"
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

type ReplacementHistoryShim struct {
	ReplacerId int64
	PostId int
}

func (this *ReplacementHistoryShim) ScanFrom(rows Scannable) error {
	return rows.Scan(&this.ReplacerId, &this.PostId)
}

func AddReplacement(ctrl EnumerateControl, repl Replacer) (*Replacer, error) {
	mine, tx := ctrl.Transaction.PopulateIfEmpty(Db_pool)
	defer ctrl.Transaction.Finalize(mine)
	if ctrl.Transaction.err != nil { return nil, ctrl.Transaction.err }

	query :=
`INSERT
    INTO replacements (match_spec, replace_spec, autofix)
        VALUES ($1, $2, $3)
RETURNING replace_id`

	row := tx.QueryRow(query, repl.MatchSpec, repl.ReplaceSpec, repl.Autofix)
	err := row.Scan(&repl.Id)
	if err != nil { return nil, err }

	ctrl.Transaction.commit = mine
	return &repl, err
}

func UpdateReplacement(ctrl EnumerateControl, repl Replacer) error {
	mine, tx := ctrl.Transaction.PopulateIfEmpty(Db_pool)
	defer ctrl.Transaction.Finalize(mine)
	if ctrl.Transaction.err != nil { return ctrl.Transaction.err }

	query :=
`UPDATE replacements
    SET match_spec = $2,
        replace_spec = $3,
        autofix = $4
WHERE replace_id = $1`

	_, err := tx.Exec(query, repl.Id, repl.MatchSpec, repl.ReplaceSpec, repl.Autofix)
	if err != nil { return err }

	ctrl.Transaction.commit = mine
	return err
}

func DeleteReplacement(ctrl EnumerateControl, id int64) (error) {
	mine, tx := ctrl.Transaction.PopulateIfEmpty(Db_pool)
	defer ctrl.Transaction.Finalize(mine)
	if ctrl.Transaction.err != nil { return ctrl.Transaction.err }

	query :=
`DELETE
    FROM replacements
WHERE replace_id = $1`

	_, err := tx.Exec(query, id)
	if err != nil { return err }

	ctrl.Transaction.commit = mine
	return err
}

func GetReplacements(settings UpdaterSettings, after_id int64) ([]Replacer, error) {
	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return nil, settings.Transaction.err }

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

	settings.Transaction.commit = mine
	return out, nil
}

func GetReplacementHistory(settings UpdaterSettings, post_ids []int) (map[ReplacementHistoryShim]bool, error) {
	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return nil, settings.Transaction.err }

	query := "SELECT replace_id, post_id FROM replacement_history WHERE post_id = ANY($1::int[])"
	rows, err := tx.Query(query, pq.Array(post_ids))
	if err != nil { return nil, err }
	defer rows.Close()

	out := make(map[ReplacementHistoryShim]bool)

	for rows.Next() {
		var r ReplacementHistoryShim
		err = r.ScanFrom(rows)
		if err != nil { return nil, err }

		out[r] = true
	}

	settings.Transaction.commit = mine
	return out, nil
}
