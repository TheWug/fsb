package storage

import (
	"fmt"
	"database/sql"

	apitypes "api/types"
)

type TypoData2 struct {
	Id int64
	Tag apitypes.TTagData
	Fix *apitypes.TTagData
	Marked bool
	ReplaceId *int64
}

func DelTagTypoByTag(tx *sql.Tx, typo TypoData2) error {
        _, err := tx.Exec(`DELETE FROM replacements WHERE replace_id = (SELECT replace_id FROM typos_registered WHERE tag_typo_id = $1)`, typo.Tag.Id)
	if err != nil { return err }
        _, err = tx.Exec(`DELETE FROM typos_registered WHERE tag_typo_id = $1`, typo.Tag.Id)
	return err
}

func SetTagTypoByTag(tx *sql.Tx, typo TypoData2, marked, autofix bool) error {
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

	var err error
	if typo.Fix == nil {
		err = tx.QueryRow(query, typo.Marked, typo.Tag.Id, nil).Scan(&typo.Id, &typo.ReplaceId)
	} else {
		err = tx.QueryRow(query, typo.Marked, typo.Tag.Id, typo.Fix.Id).Scan(&typo.Id, &typo.ReplaceId)
	}
	if err != nil { return err } // you should get a row back, if that fails something is wrong

	if typo.Marked {
		if typo.ReplaceId == nil {
			var replacement *Replacer
			replacement, err := AddReplacement(tx, Replacer{MatchSpec: typo.Tag.Name, ReplaceSpec: fmt.Sprintf("-%s %s", typo.Tag.Name, typo.Fix.Name), Autofix: autofix})
			if err != nil { return err }

			query := `UPDATE typos_registered SET replace_id = $2 WHERE typo_id = $1`
			_, err = tx.Exec(query, typo.Id, replacement.Id)

		} else {
			err = UpdateReplacement(tx, Replacer{Id: *typo.ReplaceId, MatchSpec: typo.Tag.Name, ReplaceSpec: fmt.Sprintf("-%s %s", typo.Tag.Name, typo.Fix.Name), Autofix: autofix})

		}
	} else if typo.ReplaceId != nil {
		err = DeleteReplacement(tx, *typo.ReplaceId)
	}
	return err
}
