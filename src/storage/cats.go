package storage

import (
	"fmt"
	"database/sql"

	apitypes "api/types"
)

type CatData struct {
	Id int64
	Merged apitypes.TTagData
	First, Second *apitypes.TTagData
	Marked bool
	ReplaceId *int64
}

func (c CatData) String() string {
	if c.First == nil || c.Second == nil {
		return c.Merged.Name
	} else {
		return fmt.Sprintf("%-32s %s + %s", c.Merged.Name, c.First.Name, c.Second.Name)
	}
}

func GetCats(tx *sql.Tx, yes, no bool) ([]CatData, []CatData, error) {
	query :=
`SELECT cat_id, marked,
    a.tag_id, a.tag_name, a.tag_type, a.tag_count,
    b.tag_id, b.tag_name, b.tag_type, b.tag_count,
    c.tag_id, c.tag_name, c.tag_type, c.tag_count
FROM cats_registered
     LEFT JOIN tag_index AS a ON tag_id_merged = a.tag_id
     LEFT JOIN tag_index AS b ON tag_id_1 = b.tag_id
     LEFT JOIN tag_index AS c ON tag_id_2 = c.tag_id
WHERE ($1 AND marked)
   OR ($2 AND NOT marked)`

	rows, err := tx.Query(query, yes, no)
	if err != nil { return nil, nil, err }

	var out_yes, out_no []CatData
	for rows.Next() {
		var cat CatData
		var id_1, id_2, id_merged *int
		var name_1, name_2, name_merged *string
		var type_1, type_2, type_merged *apitypes.TagCategory
		var count_1, count_2, count_merged *int

		err = rows.Scan(&cat.Id, &cat.Marked,
		                &id_merged, &name_merged, &type_merged, &count_merged,
		                &id_1, &name_1, &type_1, &count_1,
		                &id_2, &name_2, &type_2, &count_2)

		if err != nil { return nil, nil, err }
		cat.Merged = apitypes.TTagData{Id: *id_merged, Name: *name_merged, Type: *type_merged, Count: *count_merged}
		if id_1 != nil &&id_2 != nil {
			cat.First  = &apitypes.TTagData{Id: *id_1, Name: *name_1, Type: *type_1, Count: *count_1}
			cat.Second = &apitypes.TTagData{Id: *id_2, Name: *name_2, Type: *type_2, Count: *count_2}
		}

		if cat.Marked {
			out_yes = append(out_yes, cat)
		} else {
			out_no  = append(out_no,  cat)
		}
	}

	return out_yes, out_no, err
}

func SetCatByTagNames(tx *sql.Tx, cat CatData, marked, autofix bool) error {
	merged, err := GetTagByName(tx, cat.Merged.Name, true)
	if err != nil { return err }
	cat.Merged = *merged

	if marked {
		cat.First, err = GetTagByName(tx, cat.First.Name, true)
		if err != nil { return err }
		cat.Second, err = GetTagByName(tx, cat.Second.Name, true)
		if err != nil { return err }
	} else {
		cat.First = nil
		cat.Second = nil
	}
	cat.Marked = marked

	query :=
`INSERT
    INTO cats_registered (marked, tag_id_merged, tag_id_1, tag_id_2)
        VALUES ($1, $2, $3, $4)
    ON CONFLICT (tag_id_merged)
DO UPDATE SET
    marked = EXCLUDED.marked,
    tag_id_1 = EXCLUDED.tag_id_1,
    tag_id_2 = EXCLUDED.tag_id_2
RETURNING cat_id, replace_id`

	var row *sql.Row
	if cat.First == nil || cat.Second == nil{
		row = tx.QueryRow(query, cat.Marked, cat.Merged.Id, nil, nil)
	} else {
		row = tx.QueryRow(query, cat.Marked, cat.Merged.Id, cat.First.Id, cat.Second.Id)
	}
	err = row.Scan(&cat.Id, &cat.ReplaceId)
	if err != nil { return err } // you should get a row back, if that fails something is wrong

	if cat.Marked {
		if cat.ReplaceId == nil {
			replacement := &Replacer{MatchSpec: cat.Merged.Name, ReplaceSpec: fmt.Sprintf("-%s %s %s", cat.Merged.Name, cat.First.Name, cat.Second.Name), Autofix: autofix}
			replacement, err = AddReplacement(EnumerateControl{Transaction: TransactionBox{tx: tx}}, *replacement)
			if err != nil { return err }

			query := `UPDATE cats_registered SET replace_id = $2 WHERE cat_id = $1`
			_, err = tx.Exec(query, cat.Id, replacement.Id)

		} else {
			err = UpdateReplacement(EnumerateControl{Transaction: TransactionBox{tx: tx}}, Replacer{Id: *cat.ReplaceId, MatchSpec: cat.Merged.Name, ReplaceSpec: fmt.Sprintf("-%s %s %s", cat.Merged.Name, cat.First.Name, cat.Second.Name), Autofix: autofix})

		}
	} else if cat.ReplaceId != nil {
		err = DeleteReplacement(EnumerateControl{Transaction: TransactionBox{tx: tx}}, *cat.ReplaceId)
	}
	return err
}

func DeleteCatByTagNames(tx *sql.Tx, cat CatData) error {
	merged, err := GetTagByName(tx, cat.Merged.Name, false)
	if err != nil { return err }
	if merged == nil { return nil }
	cat.Merged = *merged

        _, err = tx.Exec(`DELETE FROM replacements WHERE replace_id = (SELECT replace_id FROM cats_registered WHERE tag_id_merged = $1)`, cat.Merged.Id)
	if err != nil { return err }
        _, err = tx.Exec(`DELETE FROM cats_registered WHERE tag_id_merged = $1`, cat.Merged.Id)

	return err
}
