package storage

import (
	"database/sql"
	"strings"

	apitypes "github.com/thewug/fsb/pkg/api/types"
	"github.com/lib/pq"

	"github.com/thewug/dml"
)

func PostUpdater(tx DBLike, input chan apitypes.TPostInfo) (error) {
	defer func(){ for _ = range input {} }()

	for post := range input {
		_, err := tx.Exec("DELETE FROM post_tags_by_name WHERE post_id = $1", post.Id)
		if err != nil { return err }
		_, err = tx.Exec("DELETE FROM post_index WHERE post_id = $1", post.Id)
		if err != nil { return err }

		_, err = tx.Exec("INSERT INTO post_tags_by_name (SELECT $1 as post_id, tag_name FROM UNNEST($2::varchar[]) as tag_name) ON CONFLICT DO NOTHING",
				 post.Id, pq.Array(post.Tags()))
		if err != nil { return err }
		_, err = tx.Exec("INSERT INTO post_index (post_id, post_change_seq, post_rating, post_description, post_sources, post_hash, post_deleted) VALUES ($1, $2, $3, $4, $5, $6, $7)",
				 post.Id, post.Change, post.Rating, post.Description, strings.Join(post.Sources, "\n"), strings.ToLower(post.Md5), post.Deleted)
		if err != nil { return err }
	}

	return nil
}

func PostDeleter(tx DBLike, input chan []int) (error) {
	defer func(){ for _ = range input {} }()

	for list := range input {
		_, err := tx.Exec("UPDATE post_index SET post_deleted = true WHERE post_id = ANY($1::int[])", pq.Array(list))
		if err != nil { return err }
	}

	return nil
}

func MarkPostDeleted(tx DBLike, post_id int) error {
	query := "UPDATE post_index SET post_deleted = TRUE WHERE post_id = $1"
	_, err := tx.Exec(query, post_id)
	return err
}

func GetHighestPostID(tx DBLike) (int, error) {
	query := "SELECT MAX(post_id) FROM post_index"

	row := tx.QueryRow(query)
	var result int
	err := row.Scan(&result)
	if err == sql.ErrNoRows { err = nil }
	return result, err
}

func GetMostRecentlyUpdatedPost(tx DBLike) (*apitypes.TPostInfo, error) {
	var p apitypes.TPostInfo
	row := tx.QueryRow("SELECT post_id, post_change_seq, post_rating, post_description, post_hash FROM post_index ORDER BY post_change_seq DESC LIMIT 1")
	err := row.Scan(&p.Id, &p.Change, &p.Rating, &p.Description, &p.Md5)

	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	return &p, err
}

func PostsWithTag(tx DBLike, tag apitypes.TTagData, includeDeleted bool) (apitypes.TPostInfoArray, error) {
	query := "SELECT post_id, post_change_seq, post_rating, post_description, post_sources, post_hash, post_deleted, ARRAY(SELECT tag_name FROM tag_index INNER JOIN post_tags USING (tag_id) WHERE post_id = post_index.post_id) AS post_tags FROM post_index WHERE post_id IN (SELECT post_id FROM post_tags WHERE tag_id = $1) AND NOT post_deleted"
	if includeDeleted {
		query = "SELECT post_id, post_change_seq, post_rating, post_description, post_sources, post_hash, post_deleted, ARRAY(SELECT tag_name FROM tag_index INNER JOIN post_tags USING (tag_id) WHERE post_id = post_index.post_id) AS post_tags FROM post_index WHERE post_id IN (SELECT post_id FROM post_tags WHERE tag_id = $1)"
	}

	rows, err := dml.X(tx.Query(query, tag.Id))
	if err != nil { return nil, err }

	defer rows.Close()

	var out apitypes.TPostInfoArray
	var item apitypes.TPostInfo
	for rows.Next() {
		err := dml.Scan(rows, &item)
		if err != nil { return nil, err }
		out = append(out, item)
	}

	return out, nil
}

func PostByID(tx DBLike, id int) (*apitypes.TPostInfo, error) {
	out, err := PostsById(tx, []int{id})
	if len(out) == 0 {
		return nil, err
	} else {
		return &out[0], err
	}
}

func PostsById(tx DBLike, ids []int) ([]apitypes.TPostInfo, error) {
	query := "SELECT post_id, post_change_seq, post_rating, post_description, post_sources, post_hash, post_deleted, ARRAY(SELECT tag_name FROM tag_index INNER JOIN post_tags USING (tag_id) WHERE post_id = post_index.post_id) AS post_tags FROM post_index WHERE post_id = ANY($1::int[])"
	rows, err := dml.X(tx.Query(query, pq.Array(ids)))
	if err != nil { return nil, err }
	defer rows.Close()

	var out []apitypes.TPostInfo
	err = dml.ScanArray(rows, &out)

	return out, err
}

func PostByMD5(tx DBLike, md5 string) (*apitypes.TPostInfo, error) {
	var item apitypes.TPostInfo
	var sources string
	query := "SELECT post_id, post_change_seq, post_rating, post_description, post_sources, post_hash, post_deleted, ARRAY(SELECT tag_name FROM tag_index INNER JOIN post_tags USING (tag_id) WHERE post_id = post_index.post_id) AS post_tags FROM post_index WHERE post_hash = $1;"
	err := tx.QueryRow(query, md5).Scan(&item.Id, &item.Change, &item.Rating, &item.Description, &sources, &item.Md5, &item.Deleted, pq.Array(&item.General))
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != sql.ErrNoRows && err != nil {
		return nil, err
	}

	item.Sources = strings.Split(sources, "\n")

	return &item, nil
}

func UpdatePost(tx DBLike, post apitypes.TPostInfo) (error) {
	count_deltas := make(map[string]int)
	// up-count all of the tags in the modified post
	for _, new_tag := range post.Tags() {
		count_deltas[new_tag] += 1
	}

	// down-count all of the tags that were there before.
	rows, err := tx.Query("SELECT tag_name FROM post_tags INNER JOIN tag_index USING (tag_id) WHERE post_id = $1", post.Id)
	if err != nil { return err }

	for rows.Next() {
		var old_tag string
		err := rows.Scan(&old_tag)
		if err != nil { return err }
		count_deltas[old_tag] -= 1
	}

	for k, v := range count_deltas {
		if v == 0 { continue }
		query := "UPDATE tag_index SET tag_count = tag_count + $2 WHERE tag_name = $1"
		_, err := tx.Exec(query, k, v)
		if err != nil { return err }
	}

	query := "DELETE FROM post_tags WHERE post_id = $1"
	_, err = tx.Exec(query, post.Id)
	if err != nil { return err }

	query = "DELETE FROM post_index WHERE post_id = $1"
	_, err = tx.Exec(query, post.Id)
	if err != nil { return err }

	query = "INSERT INTO post_index (post_id, post_change_seq, post_rating, post_description, post_sources, post_hash, post_deleted) VALUES ($1, $2, $3, $4, $5, $6, $7)"
	_, err = tx.Exec(query, post.Id, post.Change, post.Rating, post.Description, strings.Join(post.Sources, "\n"), strings.ToLower(post.Md5), post.Deleted)
	if err != nil { return err }

	query = "INSERT INTO post_tags SELECT $1 as post_id, tag_id FROM UNNEST($2::varchar[]) AS tag_name INNER JOIN tag_index USING (tag_name)"
	_, err = tx.Exec(query, post.Id, pq.Array(post.Tags()))
	return err
}

func ImportPostTagsFromNameToID(tx DBLike, sfx chan string) (error) {
	status := func(s string) {
		if sfx != nil {
			sfx <- s
		}
	}

	var new_count, existing_count int64
	var err error
	if err = tx.QueryRow("SELECT COUNT(*) FROM post_tags_by_name").Scan(&new_count); err != nil {
		return err
	}
	var unique_count int64
	if err = tx.QueryRow("SELECT COUNT(*) FROM (SELECT DISTINCT post_id FROM post_tags_by_name) as q").Scan(&unique_count); err != nil {
		return err
	}
	if err = tx.QueryRow("SELECT n_live_tup FROM pg_stat_all_tables WHERE relname = 'post_tags'").Scan(&existing_count); err != nil { return err } // estimate, but super fast

	// check if the amount of new data is large relative to the size of the existing dataset (1% or more out of 10s of millions of rows usually)
	if new_count * 20 > existing_count {
		// for performance reasons, it is much better to drop the indexes, do the import, and then recreate them,
		// if we are importing a significant amount of data, compared to how much is already there, as individually
		// performing an enormous number of index insertions is much more expensive than building the index from scratch.
		// downside:	this insertion method will fail if any non-unique entries are present, including conflicts with
		//		existing data in the table, where a smarter but slower approach could work around them.

		// delete existing tag records before removing indices because it will be a lot slower without them
		status(" (1/4 tag clear overrides)")
		query := "DELETE FROM post_tags WHERE post_id IN (SELECT DISTINCT post_id FROM post_tags_by_name)"
		_, err = tx.Exec(query)
		if err != nil { return err }

		// drop the index and the primary key constraint
		status(" (2/4 drop indices)")
		query = "DROP INDEX post_tags_tag_id_idx"
		_, err = tx.Exec(query)
		if err != nil { return err }

		query = "ALTER TABLE post_tags DROP CONSTRAINT post_tags_pkey"
		_, err = tx.Exec(query)
		if err != nil { return err }

		// slurp all of the data into the table (very slow if indexes are present, which is why we killed them)
		status(" (3/4 import data)")
		query = "INSERT INTO post_tags SELECT post_id, tag_id FROM post_tags_by_name INNER JOIN tag_index USING (tag_name)"
		_, err = tx.Exec(query)
		if err != nil { return err }

		// add the index and primary key constraint back to the table
		status(" (4/4 re-index)")
		query = "ALTER TABLE post_tags ADD CONSTRAINT post_tags_pkey PRIMARY KEY (post_id, tag_id)"
		_, err = tx.Exec(query)
		if err != nil { return err }

		query = "CREATE INDEX post_tags_tag_id_idx ON post_tags (tag_id)"
		_, err = tx.Exec(query)
		if err != nil { return err }
	} else {
		// if the amount of new data is not large compared to the amount of existing data, just one-by-one plunk them into the table.
		status(" (1/2 tag clear overrides)")
		query := "DELETE FROM post_tags WHERE post_id IN (SELECT DISTINCT post_id FROM post_tags_by_name)"
		_, err = tx.Exec(query)
		if err != nil { return err }

		status(" (2/2 tag gross-reference)")
		query = "INSERT INTO post_tags SELECT post_id, tag_id FROM post_tags_by_name INNER JOIN tag_index USING (tag_name)"
		_, err = tx.Exec(query)
		if err != nil { return err }
	}

	_, err = tx.Exec("TRUNCATE post_tags_by_name")
	return err
}
