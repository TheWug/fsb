package storage

import (
	apitypes "api/types"

	"database/sql"
	"log"
	"fmt"
	"strings"
	"errors"

	"github.com/lib/pq"
)

var Db_pool *sql.DB

var ErrNoLogin error = errors.New("No stored credentials for user")

type committer struct {
	commit bool
}

func handle_transaction(c *committer, tx *sql.Tx) {
	if c.commit {
		tx.Commit()
	} else {
		tx.Rollback()
	}
}

// initialize the DAL. Closing it might be important at some point, but who cares right now.
func DBInit(dburl string) (error) {
	var err error
	log.Println("[util    ] Connecting to postgres...")
	Db_pool, err = sql.Open("postgres", dburl)
	if err != nil {
		return err
	}
	log.Println("[util    ] OK!")
	return nil
}

func WriteUserCreds(id int, username, key string) (error) {
	tx, err := Db_pool.Begin()
	if err != nil { return err }

	var c committer
	defer handle_transaction(&c, tx)

	_, err = tx.Exec("INSERT INTO remote_user_credentials (telegram_id, api_user, api_apikey) VALUES ($1, $2, $3) " +
			"ON CONFLICT (telegram_id) DO UPDATE SET api_user = EXCLUDED.api_user, api_apikey = EXCLUDED.api_apikey", id, username, key)
	if (err != nil) { return err }

	c.commit = true
	return nil
}

func GetUserCreds(id int) (string, string, bool, error) {
	row := Db_pool.QueryRow("SELECT api_user, api_apikey, privilege_janitorial FROM remote_user_credentials WHERE telegram_id = $1", id)
	var user, key string
	var privilege bool
	err := row.Scan(&user, &key, &privilege)
	if err == sql.ErrNoRows || len(user) == 0 || len(key) == 0 { err = ErrNoLogin }
	return user, key, privilege, err
}

func WriteUserTagRules(id int, name, rules string) (error) {
	tx, err := Db_pool.Begin()
	if err != nil { return err }

	var c committer
	defer handle_transaction(&c, tx)

	_, err = tx.Exec("DELETE FROM user_tagrules WHERE telegram_id = $1 AND name = $2", id, name)
	if (err != nil) { return err }
	_, err = tx.Exec("INSERT INTO user_tagrules (telegram_id, name, rules) VALUES ($1, $2, $3)", id, name, rules)
	if (err != nil) { return err }

	c.commit = true
	return nil
}

func GetUserTagRules(id int, name string) (string, error) {
	row := Db_pool.QueryRow("SELECT (rules) FROM user_tagrules WHERE telegram_id = $1 AND name = $2", id, name)
	var rules string
	err := row.Scan(&rules)
	if err == sql.ErrNoRows { err = nil } // no data for user is not an error.
	return rules, err
}

func SetMigrationState(state, progress int) (error) {
	tx, err := Db_pool.Begin()
	if err != nil { return err }

	var c committer
	defer handle_transaction(&c, tx)

	sq := "DELETE FROM import_progress"
	_, err = tx.Exec(sq)
	if err != nil { return err }

	sq = "INSERT INTO import_progress VALUES ($1, $2)"
	_, err = tx.Exec(sq, state, progress)
	if err != nil { return err }

	c.commit = true
	return nil
}

func GetMigrationState() (int, int) {
	sq := "SELECT * FROM import_progress LIMIT 1"
	row := Db_pool.QueryRow(sq)

	var state, progress int
	err := row.Scan(&state, &progress)

	if err != nil { log.Println(err.Error()) }

	return state, progress
}

func PrefixedTagToTypedTag(name string) (string, int) {
	if trimmed := strings.TrimPrefix(name, "general:"); trimmed != name { return trimmed, apitypes.General }
	if trimmed := strings.TrimPrefix(name, "character:"); trimmed != name { return trimmed, apitypes.Character }
	if trimmed := strings.TrimPrefix(name, "artist:"); trimmed != name { return trimmed, apitypes.Artist }
	if trimmed := strings.TrimPrefix(name, "copyright:"); trimmed != name { return trimmed, apitypes.Copyright }
	if trimmed := strings.TrimPrefix(name, "species:"); trimmed != name { return trimmed, apitypes.Species }
	return name, apitypes.General
}

type EnumerateControl struct {
	OrderByCount bool
	CreatePhantom bool
}

func GetTag(name string, ctrl EnumerateControl) (*apitypes.TTagData, error) {
	sq := "SELECT tag_id, tag_name, tag_count, tag_type, tag_type_locked FROM tag_index WHERE LOWER(tag_name) = LOWER($1) LIMIT 1"
	name, typ := PrefixedTagToTypedTag(name)

	row := Db_pool.QueryRow(sq, name)

	var tag apitypes.TTagData
	err := row.Scan(&tag.Id, &tag.Name, &tag.Count, &tag.Type, &tag.Locked)

	if err == sql.ErrNoRows {
		if !ctrl.CreatePhantom { return nil, nil } // don't create phantom tag, so just return nil for "not found"
		// otherwise, insert a phantom tag
		row = Db_pool.QueryRow("INSERT INTO tag_index (tag_id, tag_name, tag_count, tag_type, tag_type_locked) VALUES (nextval('phantom_tag_seq'), $1, 0, $2, false) RETURNING *", name, typ)
		err = row.Scan(&tag.Id, &tag.Name, &tag.Count, &tag.Type, &tag.Locked)
		if err == sql.ErrNoRows { return nil, nil } // this really shouldn't happen, but just in case.
	}
	if err != nil {
		return nil, err
	}

	return &tag, err
}

func GetLastTag() (*apitypes.TTagData, error) {
	sq := "SELECT tag_id, tag_name, tag_count, tag_type, tag_type_locked FROM tag_index WHERE tag_id = (SELECT MAX(tag_id) FROM tag_index) LIMIT 1"
	row := Db_pool.QueryRow(sq)

	var tag apitypes.TTagData
	err := row.Scan(&tag.Id, &tag.Name, &tag.Count, &tag.Type, &tag.Locked)

	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	return &tag, nil
}

func ClearTagIndex() (error) {
	// don't delete phantom tags. phantom tags have an id less than zero, and that id is transient, so if the
	// tag database has phantom tags applied to any posts and they are deleted from here they will become dangling.
	// instead, keep them. they may conflict later with real tags, in which case they will be de-phantomified.
	_, err := Db_pool.Exec("DELETE FROM tag_index WHERE tag_id > 0")
	return err
}

func ClearAliasIndex() (error) {
	_, err := Db_pool.Exec("TRUNCATE alias_index")
	return err
}

func WriteTagEntries(list []interface{}) (error) {
	tx, err := Db_pool.Begin()
	if err != nil { return err }

	var c committer
	defer handle_transaction(&c, tx)

	stmt, err := tx.Prepare(pq.CopyIn("tag_index", "tag_id", "tag_name", "tag_count", "tag_type", "tag_type_locked"))
	
	for i := 0; i < len(list); i += 5 {
		_, err = stmt.Exec(list[i], list[i+1], list[i+2], list[i+3], list[i+4])
		if err != nil { return err }
	}

	_, err = stmt.Exec()
	if err != nil { return err }

	err = stmt.Close()
	if err != nil { return err }

	c.commit = true
	return nil
}

func GetTagsWithCountLess(count int) (apitypes.TTagInfoArray, error) { return getTagsWithCount(count, "<") }
func GetTagsWithCountGreater(count int) (apitypes.TTagInfoArray, error) { return getTagsWithCount(count, ">") }
func GetTagsWithCountEqual(count int) (apitypes.TTagInfoArray, error) { return getTagsWithCount(count, "=") }

func getTagsWithCount(count int, differentiator string) (apitypes.TTagInfoArray, error) {
	sql := fmt.Sprintf("SELECT tag_id, tag_name, tag_count, tag_type, tag_type_locked FROM tag_index WHERE tag_count %s $1", differentiator)
	rows, err := Db_pool.Query(sql, count)
	if err != nil {
		log.Printf("An error occurred when enumerating tags with negative counts: %s\n", err.Error())
		return nil, err
	}

	defer rows.Close()
	var d apitypes.TTagData
	var out apitypes.TTagInfoArray

	for rows.Next() {
		err = rows.Scan(&d.Id, &d.Name, &d.Count, &d.Type, &d.Locked)
		if err != nil {
			log.Printf("An error occurred when enumerating tags with negative counts: %s\n", err.Error())
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

func GetAliasesFor(tag string) (apitypes.TTagInfoArray, error) {
	sql :=	"SELECT a.tag_id, a.tag_name, a.tag_count, a.tag_type, a.tag_type_locked FROM " +
			"tag_index AS %s INNER JOIN " +
			"alias_index AS b ON (%s.tag_name = b.alias_name) INNER JOIN " +
			"tag_index AS %s ON (b.alias_target_id = %s.tag_id) " +
		"WHERE c.tag_name = $1"

	tx, err := Db_pool.Begin()
	if err != nil { return nil, err }
	defer tx.Rollback()

	var out apitypes.TTagInfoArray
	var t apitypes.TTagData

	rows, err := tx.Query(fmt.Sprintf(sql, "a", "a", "c", "c"), tag)
	if err != nil { return nil, err }

	for rows.Next() {
		err = rows.Scan(&t.Id, &t.Name, &t.Count, &t.Type, &t.Locked)
		if err != nil { return nil, err }
		out = append(out, t)
	}

	rows, err = tx.Query(fmt.Sprintf(sql, "c", "c", "a", "a"), tag)
	if err != nil { return nil, err }

	for rows.Next() {
		err = rows.Scan(&t.Id, &t.Name, &t.Count, &t.Type, &t.Locked)
		if err != nil { return nil, err }
		out = append(out, t)
	}

	return out, nil
}

func GetAliasedTags() (apitypes.TTagInfoArray, error) {
	sql := "SELECT tag_id, tag_name, tag_count, tag_type, tag_type_locked FROM tag_index INNER JOIN alias_index ON alias_name = tag_name WHERE tag_count != 0 AND tag_name != ''"
	rows, err := Db_pool.Query(sql)
	if err != nil {
		log.Printf("An error occurred when enumerating aliased tags: %s\n", err.Error())
		return nil, err
	}

	defer rows.Close()
	var d apitypes.TTagData
	var out apitypes.TTagInfoArray

	for rows.Next() {
		err = rows.Scan(&d.Id, &d.Name, &d.Count, &d.Type, &d.Locked)
		if err != nil {
			log.Printf("An error occurred when enumerating tags with negative counts: %s\n", err.Error())
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

func AliasUpdater(input chan apitypes.TAliasData) (error) {
	defer func(){ for _ = range input {} }()
	tx, err := Db_pool.Begin()
	if err != nil { return err }

	var c committer
	defer handle_transaction(&c, tx)

	for alias := range input {
		sql := "DELETE FROM alias_index WHERE alias_id = $1"
		_, err := tx.Exec(sql, alias.Id)
		if err != nil { 
			log.Printf("Error: %s", err.Error())
			return err
		}

		sql = "INSERT INTO alias_index (alias_id, alias_name, alias_target_id) VALUES ($1, $2, $3)"
		_, err = tx.Exec(sql, alias.Id, alias.Name, alias.Alias)
		if err != nil { 
			log.Printf("Error: %s", err.Error())
			return err
		}
	}

	c.commit = true
	return nil
}

func PostUpdater(input chan apitypes.TSearchResult, clear bool) (error) {
	tx, err := Db_pool.Begin()
	if err != nil { return err }

	var c committer
	defer handle_transaction(&c, tx)
	defer func(){ for _ = range input {} }()

	if clear {
		_, err := tx.Exec("TRUNCATE post_tags_by_name")
		if err != nil {
			log.Printf("Error 1: %s", err.Error())
			return err
		}
	}

	for post := range input {
		sql := "DELETE FROM post_tags WHERE post_id = $1"
		_, err = tx.Exec(sql, post.Id)
		if err != nil { return err }

		sql = "INSERT INTO post_tags_by_name (SELECT $1 as post_id, tag_name FROM UNNEST($2::varchar[]) as tag_name) ON CONFLICT DO NOTHING"
		_, err = tx.Exec(sql, post.Id, pq.Array(strings.Split(post.Tags, " ")))
		if err != nil { return err }
	}

	c.commit = true
	return nil
}

func TagUpdater(input chan apitypes.TTagData) (error) {
	tx, err := Db_pool.Begin()
	if err != nil { return err }

	var c committer
	defer handle_transaction(&c, tx)
	defer func(){ for _ = range input {} }()

	for tag := range input {
		sql := "DELETE FROM tag_index WHERE tag_id = $1"
		_, err := tx.Exec(sql, tag.Id)
		if err != nil { return err }

		f := false
		if tag.Locked == nil { tag.Locked = &f }

		sql = "INSERT INTO tag_index (tag_id, tag_name, tag_count, tag_type, tag_type_locked) VALUES ($1, $2, $3, $4, $5)"
		_, err = tx.Exec(sql, tag.Id, tag.Name, tag.Count, tag.Type, *tag.Locked)
		if err != nil { return err }
	}

	c.commit = true
	return nil
}

func ResolvePhantomTags() (error) {
	tx, err := Db_pool.Begin()
	if err != nil { return err }

	var c committer
	defer handle_transaction(&c, tx)

	// this one merits a tiny bit of explaining
	// fetches the phantom id and the real id using the name and the newest ID of all doubly duplicate tags (by name), then swaps all phantom ids with their associated real id in post_tags.
	_, err = tx.Exec("UPDATE post_tags SET tag_id = map.mapto FROM tag_index INNER JOIN (" +
				"WITH b AS (" +
					"WITH a AS (" +
						"SELECT COUNT(tag_name), tag_name FROM tag_index GROUP BY tag_name" +
					") SELECT tag_name, MAX(tag_id) FROM a INNER JOIN tag_index USING (tag_name) WHERE count = 2 GROUP BY tag_name" +
				") SELECT tag_id, max FROM b INNER JOIN tag_index USING (tag_name) WHERE max > 0 AND tag_id < 0" +
			") AS map(tag_id, mapto) USING (tag_id) WHERE tag_index.tag_id = post_tags.tag_id")
	if err != nil { return err }

	_, err = tx.Exec("DELETE FROM tag_index WHERE tag_id < 0")
	if err != nil { return err }
	_, err = tx.Exec("DELETE FROM post_tags WHERE tag_id < 0")
	if err != nil { return err }

	c.commit = true
	return nil
}

func FixTagsWhichTheAPIManglesByAccident() (error) {
	// what the fuck is this, you may ask?
	// well, it turns out, there are a small number of tags which include unicode characters past
	// codepoint 0xFFFF. apparently ruby on rails (api backend) serializes these badly to json, which
	// cannot escape codepoints higher than that. They just get munged. silently. :|
	// in this function, we manually UPDATE the rows in the tag database that will have these names.
	// thankfully there are only a small number of (known) ones.

	// these are all the ones i've found. there's probably more.
	fix_map := map[int]string{
		407005: "samochan" + "\U0001F49F" + "iluvml",
		628543: "\U0001F171",
		390821: "\U0001F60D",
		390822: "\U0001F61D",
		390824: "\U0001F635",
		390825: "\U0001F60B",
		390826: "\U0001F61B",
		483084: "\U0001F44C",
	}

	tx, err := Db_pool.Begin()
	if err != nil { return err }

	var c committer
	defer handle_transaction(&c, tx)

	for k, v := range fix_map {
		query := "UPDATE tag_index SET tag_name = $1 WHERE tag_id = $2"
		_, err := tx.Exec(query, v, k)
		if err != nil { return err }
	}

	c.commit = true
	return nil
}

//func EnumerateAllTagNames() ([]string, error) {
//	return []string{"tawny_otter_(character)", "tawny", "otter", "character"}, nil
//}

func EnumerateAllTags(ctrl EnumerateControl) (apitypes.TTagInfoArray, error) {
	var output apitypes.TTagInfoArray

	sql := "SELECT tag_id, tag_name, tag_count, tag_type, tag_type_locked FROM %s %s"
	order_by := "ORDER BY %s"
	table := "tag_index"
	
	if ctrl.OrderByCount {
		order_by = fmt.Sprintf(order_by, "-tag_count")
	} else {
		order_by = ""
	}

	sql = fmt.Sprintf(sql, table, order_by)
		
	rows, err := Db_pool.Query(sql)
	if err != nil { return nil, err }

	defer rows.Close()
	var d apitypes.TTagData

	for rows.Next() {
		err = rows.Scan(&d.Id, &d.Name, &d.Count, &d.Type, &d.Locked)
		if err != nil { return nil, err }

		output = append(output, d)
	}

	return output, nil
}

func RecalculateAliasedCounts() (error) {
	tx, err := Db_pool.Begin()
	if err != nil { return err }

	var c committer
	defer handle_transaction(&c, tx)

	sql := 	"UPDATE tag_index " +
			"SET tag_count = subquery.tag_count " +
		"FROM (SELECT a.tag_id, c.tag_count " +
			"FROM tag_index AS a INNER JOIN " +
				"alias_index AS b ON (a.tag_name = b.alias_name) INNER JOIN " +
				"tag_index AS c ON (b.alias_target_id = c.tag_id)) AS subquery " +
		"WHERE tag_index.tag_id = subquery.tag_id"
	_, err = tx.Exec(sql)
	if err != nil { return err }

	c.commit = true

	return nil
}

func SetTagHistoryCheckpoint(id int) (error) {
	tx, err := Db_pool.Begin()
	if err != nil { return err }

	var c committer
	defer handle_transaction(&c, tx)

	sq := "DELETE FROM tag_history_checkpoint"
	_, err = tx.Exec(sq)
	if err != nil { return err }

	sq = "INSERT INTO tag_history_checkpoint VALUES ($1)"
	_, err = tx.Exec(sq, id)
	if err != nil { return err }

	c.commit = true
	return nil
}

func GetTagHistoryCheckpoint() (int, error) {
	var out int
	err := Db_pool.QueryRow("SELECT * FROM tag_history_checkpoint LIMIT 1").Scan(&out)

	if err == sql.ErrNoRows {
		return 0, nil
	}

	return out, err
}

func ImportPostTagsFromNameToID(status chan string) (error) {
	tx, err := Db_pool.Begin()
	if err != nil { return err }

	var c committer
	defer handle_transaction(&c, tx)

	var new_count, existing_count int64
	if err = tx.QueryRow("SELECT COUNT(*) FROM post_tags_by_name").Scan(&new_count); err != nil { return err }
	if err = tx.QueryRow("SELECT COUNT(*) FROM post_tags").Scan(&existing_count); err != nil { return err }

	// check if the amount of new data is large relative to the size of the existing dataset (1% or more out of 10s of millions of rows usually)
	if new_count * 20 > existing_count {
		// for performance reasons, it is much better to drop the indexes, do the import, and then recreate them,
		// if we are importing a significant amount of data, compared to how much is already there, as individually
		// performing an enormous number of index insertions is much more expensive than building the index from scratch.
		// downside:	this insertion method will fail if any non-unique entries are present, including conflicts with
		//		existing data in the table, where a smarter but slower approach could work around them.

		// bump maintenance memory threshhold, the default value is low. this field's value is per transaction.
		query := "SET maintenance_work_mem TO '4 GB'"
		_, err = tx.Exec(query)
		if err != nil { return err }

		// drop the index and the primary key constraint
		status <- " (1/3 drop indices)"
		query = "DROP INDEX post_tag_order_tags"
		_, err = tx.Exec(query)
		if err != nil { return err }

		query = "ALTER TABLE post_tags DROP CONSTRAINT post_tags_pkey"
		_, err = tx.Exec(query)
		if err != nil { return err }

		// slurp all of the data into the table (very slow if indexes are present, which is why we killed them)
		status <- " (2/3 import data)"
		query = "INSERT INTO post_tags SELECT post_id, tag_id FROM post_tags_by_name INNER JOIN tag_index USING (tag_name)"
		_, err = tx.Exec(query)
		if err != nil { return err }

		// add the index and primary key constraint back to the table
		status <- " (3/3 re-index)"
		query = "ALTER TABLE post_tags ADD PRIMARY KEY (post_id, tag_id)"
		_, err = tx.Exec(query)
		if err != nil { return err }

		query = "CREATE INDEX post_tag_order_tags ON post_tags (tag_id)"
		_, err = tx.Exec(query)
		if err != nil { return err }
	} else {
		// if the amount of new data is not large compared to the amount of existing data, just one-by-one plunk them into the table.
		status <- " (1/1 tag cross-reference)"
		query := "INSERT INTO post_tags SELECT post_id, tag_id FROM post_tags_by_name INNER JOIN tag_index USING (tag_name)"
		_, err = tx.Exec(query)
		if err != nil { return err }
	}

	c.commit = true
	return nil
}

func FindPostGaps() ([]int, error) {
	query := "SELECT * from GENERATE_SERIES((SELECT MIN(post_id) FROM post_tags_by_name), (SELECT MAX(post_id) FROM post_tags_by_name)) AS post_id EXCEPT SELECT DISTINCT post_id FROM post_tags_by_name ORDER BY post_id"
	rows, err := Db_pool.Query(query)

	if err != nil { return nil, err }

	var i int
	var out []int
	for rows.Next() {
		err = rows.Scan(&i)
		if err != nil { return nil, err }
		out = append(out, i)
	}

	return out, nil
}

func ResetPostTags() (error) {
	tx, err := Db_pool.Begin()
	if err != nil { return err }

	var c committer
	defer handle_transaction(&c, tx)

	query := "TRUNCATE post_tags, post_tags_by_name"
	_, err = tx.Exec(query)
	if err != nil { return err }

	c.commit = true
	return nil
}

func CountTags(status chan string) (error) {
	tx, err := Db_pool.Begin()
	if err != nil { return err }

	var c committer
	defer handle_transaction(&c, tx)

	status <- " (1/2 reset cached counts)"
	query := "UPDATE tag_index SET tag_count = 0"
	_, err = tx.Exec(query)
	if err != nil { return err }

	status <- " (2/2 calculate actual tag counts)"
	query = "WITH subq AS (SELECT tag_id, COUNT(tag_id) AS real_count FROM post_tags GROUP BY tag_id) UPDATE tag_index SET tag_count = subq.real_count FROM subq WHERE subq.tag_id = tag_index.tag_id"
	_, err = tx.Exec(query)
	if err != nil { return err }

	c.commit = true
	return nil
}

func LocalTagSearch(tag apitypes.TTagData) (apitypes.TResultArray, error) {
	query := "SELECT post_id, (SELECT tag_name FROM tag_index WHERE tag_id = post_tags.tag_id) FROM (SELECT post_id FROM post_tags WHERE tag_id = $1) AS a INNER JOIN post_tags USING (post_id) ORDER BY post_id"
	rows, err := Db_pool.Query(query, tag.Id)
	if err != nil { return nil, err }

	var out apitypes.TResultArray
	var item apitypes.TSearchResult
	var intermed map[int][]string = make(map[int][]string)
	for rows.Next() {
		var id int
		var tag string
		err := rows.Scan(&id, &tag)
		if err != nil { return nil, err }
		intermed[id] = append(intermed[id], tag)
	}

	for k, v := range intermed {
		item.Id = k
		item.Tags = strings.Join(v, " ")
		out = append(out, item)
	}

	return out, nil
}

func UpdatePost(oldpost, newpost apitypes.TSearchResult) (error) {
	tx, err := Db_pool.Begin()
	if err != nil { return err }

	var c committer
	defer handle_transaction(&c, tx)

	count_deltas := make(map[string]int)
	for _, new_tag := range strings.Split(newpost.Tags, " ") {
		count_deltas[new_tag] += 1
	}
	for _, old_tag := range strings.Split(oldpost.Tags, " ") {
		count_deltas[old_tag] -= 1
	}

	for k, v := range count_deltas {
		if v == 0 { continue }
		query := "UPDATE tag_index SET tag_count = tag_count + $2 WHERE tag_name = $1"
		_, err = tx.Exec(query, k, v)
		if err != nil { return err }
	}

	query := "DELETE FROM post_tags WHERE post_id = $1"
	_, err = tx.Exec(query, oldpost.Id)
	if err != nil { return err }

	query = "INSERT INTO post_tags SELECT $1 as post_id, tag_id FROM UNNEST($2::varchar[]) AS tag_name INNER JOIN tag_index USING (tag_name)"
	_, err = tx.Exec(query, oldpost.Id, pq.Array(strings.Split(newpost.Tags, " ")))
	if err != nil { return err }

	c.commit = true
	return nil
}

func GetMarkedAndUnmarkedBlits() ([]int) {
	var id int
	var out []int
	rows, _ := Db_pool.Query("SELECT tag_id FROM blit_tag_registry")
	for rows.Next() {
		rows.Scan(&id)
		out = append(out, id)
	}
	return out
}

func MarkBlit(id int, mark bool) {
	Db_pool.Exec("INSERT INTO blit_tag_registry (tag_id, is_blit) VALUES ($1, $2) ON CONFLICT (tag_id) DO UPDATE SET is_blit = EXCLUDED.is_blit", id, mark)
}
