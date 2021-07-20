package storage

import (
	apitypes "api/types"
	
	"database/sql"
	"errors"
	"fmt"
	
	"github.com/lib/pq"
)

// Looks up a tag by its name.
// Has a special option `createPhantom`, which when set, will create a "phantom tag" in the database.
// the type will be inferred from the tag name (a prefix can be used to specify its type, see 
// PrefixedTagToTypedTag. It is created with a negative tag_id (all such tags with negative IDs are
// phantoms).
// Phantom tags have all of the hallmarks of real tags but don't exist server-side. When the bot
// fetches an update that includes the phantom tag, its `tag_id` will be updated to reflect the true
// id (thus promoting it from a phantom tag to a real one) and will chain this update to all tables
// that use foreign keys to refer to tag_ids.
func GetTagByName(tx *sql.Tx, name string, createPhantom bool) (*apitypes.TTagData, error) {
	query := "SELECT tag_id, tag_name, tag_count, tag_count_full, tag_type, tag_type_locked FROM tag_index WHERE LOWER(tag_name) = LOWER($1)"
	name, typ := PrefixedTagToTypedTag(name)

	var tag apitypes.TTagData
	err := tag.ScanFrom(tx.QueryRow(query, name))

	if err == sql.ErrNoRows {
		if !createPhantom { return nil, nil } // don't create phantom tag, so just return nil for "not found"
		// otherwise, insert a phantom tag
		query = "INSERT INTO tag_index (tag_id, tag_name, tag_count, tag_type, tag_type_locked) VALUES (nextval('phantom_tag_seq'), $1, 0, $2, false) RETURNING tag_id, tag_name, tag_count, tag_count_full, tag_type, tag_type_locked"
		err = tag.ScanFrom(tx.QueryRow(query, name, typ))
		if err == sql.ErrNoRows { return nil, errors.New("failed to add phantom tag") }
	}
	
	if err != nil {
		return nil, err
	}
	
	return &tag, err
}

func GetTag(tx *sql.Tx, id int) (*apitypes.TTagData, error) {
	out, err := GetTags(tx, []int{id})
	if len(out) == 0 || err != nil {
		return nil, err
	}
	return &out[0], nil
}

func GetTags(tx *sql.Tx, ids []int) (apitypes.TTagInfoArray, error) {
	query := "SELECT tag_id, tag_name, tag_count, tag_count_full, tag_type, tag_type_locked FROM tag_index WHERE tag_id = ANY($1)"
	rows, err := tx.Query(query, pq.Array(ids))
	if err != nil { return nil, err }
	
	var tag apitypes.TTagData
	var out apitypes.TTagInfoArray
	for rows.Next() {
		err := tag.ScanFrom(rows)
		if err != nil {	return nil, err }
		out = append(out, tag)
	}
	
	return out, nil
}

func GetLastTag(tx *sql.Tx) (*apitypes.TTagData, error) {
	sq := "SELECT tag_id, tag_name, tag_count, tag_count_full, tag_type, tag_type_locked FROM tag_index WHERE tag_id = (SELECT MAX(tag_id) FROM tag_index) LIMIT 1"
	row := tx.QueryRow(sq)
	var tag apitypes.TTagData
	err := tag.ScanFrom(row)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return &tag, nil
}

func GetTagsWithCountLess(tx *sql.Tx, count int) (apitypes.TTagInfoArray, error) { return getTagsWithCount(tx, count, "<") }
func GetTagsWithCountGreater(tx *sql.Tx, count int) (apitypes.TTagInfoArray, error) { return getTagsWithCount(tx, count, ">") }
func GetTagsWithCountEqual(tx *sql.Tx, count int) (apitypes.TTagInfoArray, error) { return getTagsWithCount(tx, count, "=") }

func getTagsWithCount(tx *sql.Tx, count int, differentiator string) (apitypes.TTagInfoArray, error) {
	query := fmt.Sprintf("SELECT tag_id, tag_name, tag_count, tag_count_full, tag_type, tag_type_locked FROM tag_index WHERE tag_count %s $1", differentiator)
	rows, err := tx.Query(query, count)
	if err != nil { return nil, err }

	var tag apitypes.TTagData
	var out apitypes.TTagInfoArray
	for rows.Next() {
		err = tag.ScanFrom(rows)
		if err != nil {	return nil, err }
		out = append(out, tag)
	}
	
	return out, nil
}

func TagUpdater(tx *sql.Tx, input chan apitypes.TTagData) (error) {
	defer func(){ for _ = range input {} }()

	for tag := range input {
		f := false
		if tag.Locked == nil { tag.Locked = &f }

		_, err := tx.Exec("INSERT INTO tag_index (tag_id, tag_name, tag_count, tag_type, tag_type_locked) VALUES ($1, $2, $3, $4, $5) ON CONFLICT (tag_name) DO UPDATE SET tag_id = EXCLUDED.tag_id, tag_count = EXCLUDED.tag_count, tag_type = EXCLUDED.tag_type, tag_type_locked = EXCLUDED.tag_type_locked", tag.Id, tag.Name, tag.Count, tag.Type, *tag.Locked)
		if err != nil { return err }
	}

	return nil
}

func EnumerateAllTags(tx *sql.Tx, orderByCount bool) (apitypes.TTagInfoArray, error) {
	query := "SELECT tag_id, tag_name, tag_count, tag_count_full, tag_type, tag_type_locked FROM tag_index %s"
	order_by := "ORDER BY %s"

	if orderByCount {
		order_by = fmt.Sprintf(order_by, "-tag_count")
	} else {
		order_by = ""
	}

	rows, err := tx.Query(fmt.Sprintf(query, order_by))
	if err != nil { return nil, err }

	var output apitypes.TTagInfoArray
	var tag apitypes.TTagData
	for rows.Next() {
		err = tag.ScanFrom(rows)
		if err != nil { return nil, err }
		output = append(output, tag)
	}

	return output, nil
}
