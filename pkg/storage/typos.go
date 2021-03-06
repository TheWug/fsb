package storage

import (
	"fmt"

	apitypes "github.com/thewug/fsb/pkg/api/types"

	"github.com/thewug/dml"
)

type TypoData struct {
	Id         int64              `dml:"typo_id"`
	Tag        apitypes.TTagData
	Fix       *apitypes.TTagData
	Marked     bool               `dml:"marked"`
	ReplaceId *int64              `dml:"replace_id"`
}

func DelTagTypoByTag(d DBLike, typo TypoData) error {
	query1 := `DELETE FROM replacements WHERE replace_id = (SELECT replace_id FROM typos_registered WHERE tag_typo_id = $1)`
	query2 := `DELETE FROM typos_registered WHERE tag_typo_id = $1`

	return d.Enter(func(tx Queryable) error {
		if err := WrapExec(tx.Exec(query1, typo.Tag.Id)); err != nil { return err }
		return WrapExec(tx.Exec(query2, typo.Tag.Id))
	})
}

func SetTagTypoByTag(d DBLike, typo TypoData, marked, autofix bool) error {
	if !marked {
		typo.Fix = nil
	}
	typo.Marked = marked

	query :=
`INSERT
    INTO typos_registered (marked, tag_typo_id, tag_fix_id)
        VALUES ($1, $2, $3)
    ON CONFLICT (tag_typo_id)
DO UPDATE SET
    marked = EXCLUDED.marked,
    tag_fix_id = EXCLUDED.tag_fix_id
RETURNING typo_id, replace_id`

	err := d.Enter(func(tx Queryable) error {
		if typo.Fix == nil {
			return tx.QueryRow(query, typo.Marked, typo.Tag.Id, nil).Scan(&typo.Id, &typo.ReplaceId)
		} else {
			return tx.QueryRow(query, typo.Marked, typo.Tag.Id, typo.Fix.Id).Scan(&typo.Id, &typo.ReplaceId)
		}
	})
	if err != nil { return err } 

	if typo.Marked {
		if typo.ReplaceId == nil {
			var replacement *Replacer
			replacement, err := AddReplacement(d, Replacer{MatchSpec: typo.Tag.Name, ReplaceSpec: fmt.Sprintf("-%s %s", typo.Tag.Name, typo.Fix.Name), Autofix: autofix})
			if err != nil { return err }

			query := `UPDATE typos_registered SET replace_id = $2 WHERE typo_id = $1`
			err = d.Enter(func(tx Queryable) error { return WrapExec(tx.Exec(query, typo.Id, replacement.Id)) })

		} else {
			err = UpdateReplacement(d, Replacer{Id: *typo.ReplaceId, MatchSpec: typo.Tag.Name, ReplaceSpec: fmt.Sprintf("-%s %s", typo.Tag.Name, typo.Fix.Name), Autofix: autofix})

		}
	} else if typo.ReplaceId != nil {
		err = DeleteReplacement(d, *typo.ReplaceId)
	}
	return err
}

func GetTagTypos(d DBLike, tag string) (map[string]TypoData, error) {
	query := `
		SELECT	typo_id, marked, replace_id,
			a.tag_id, a.tag_name, a.tag_count, a.tag_count_full, a.tag_type, a.tag_type_locked,
			b.tag_id, b.tag_name, b.tag_count, b.tag_count_full, b.tag_type, b.tag_type_locked
		FROM	typos_registered
			INNER JOIN tag_index as a ON a.tag_id = tag_typo_id
			LEFT JOIN tag_index as b ON b.tag_id = tag_fix_id
		WHERE	a.tag_name = $1 OR b.tag_name = $1
		`
	results := make(map[string]TypoData)

	err := d.Enter(func(tx Queryable) error {
		rows, err := dml.X(tx.Query(query, tag))
		if err != nil { return err }
		defer rows.Close()

		// XXX needs more dml
		for rows.Next() {
			var data TypoData
			var fix_tag apitypes.TTagData
			err = dml.Scan(rows, &data, &data.Tag, &fix_tag)
			if fix_tag.Id != 0 {
				data.Fix = &fix_tag
			}
			if err != nil { return err }
			results[data.Tag.Name] = data
		}

		return nil
	})

	if err != nil {
		results = nil
	}
	return results, nil
}
