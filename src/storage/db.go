package storage

import (
	"api/tags"
	apitypes "api/types"

	"github.com/lib/pq"
	tgtypes "github.com/thewug/gogram/data"

	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
)

var Db_pool *sql.DB

var ErrNoLogin error = errors.New("No stored credentials for user")

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

type UserCreds struct {
	TelegramId tgtypes.UserID
	User, ApiKey string
	Janitor bool
	Blacklist string
	BlacklistFetched time.Time
}

func WriteUserCreds(settings UpdaterSettings, creds UserCreds) (error) {
	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

	query := `
INSERT INTO remote_user_credentials (telegram_id, api_user, api_key, api_blacklist, api_blacklist_last_updated)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (telegram_id) DO UPDATE
SET	api_user = EXCLUDED.api_user,
	api_key = EXCLUDED.api_key,
	api_blacklist = EXCLUDED.api_blacklist,
	api_blacklist_last_updated = EXCLUDED.api_blacklist_last_updated
`
	_, err := tx.Exec(query, creds.TelegramId, creds.User, creds.ApiKey, creds.Blacklist, creds.BlacklistFetched)
	if (err != nil) { return err }

	settings.Transaction.commit = mine
	return nil
}

func GetUserCreds(settings UpdaterSettings, id tgtypes.UserID) (UserCreds, error) {
	creds := UserCreds{TelegramId: id}

	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return creds, settings.Transaction.err }

	row := tx.QueryRow("SELECT api_user, api_key, privilege_janitorial, api_blacklist, api_blacklist_last_updated FROM remote_user_credentials WHERE telegram_id = $1", id)

	err := row.Scan(&creds.User, &creds.ApiKey, &creds.Janitor, &creds.Blacklist, &creds.BlacklistFetched)
	if err == sql.ErrNoRows || len(creds.User) == 0 || len(creds.ApiKey) == 0 { err = ErrNoLogin }

	settings.Transaction.commit = mine
	return creds, err
}

func DeleteUserCreds(settings UpdaterSettings, id tgtypes.UserID) (error) {
	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

	query := "DELETE FROM remote_user_credentials WHERE telegram_id = $1"
	_, err := tx.Exec(query, id)

	settings.Transaction.commit = mine && (err == nil)
	return err
}

func WriteUserTagRules(settings UpdaterSettings, id tgtypes.UserID, name, rules string) (error) {
	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

	_, err := tx.Exec("DELETE FROM user_tagrules WHERE telegram_id = $1 AND name = $2", id, name)
	if (err != nil) { return err }
	_, err = tx.Exec("INSERT INTO user_tagrules (telegram_id, name, rules) VALUES ($1, $2, $3)", id, name, rules)
	if (err != nil) { return err }

	settings.Transaction.commit = mine
	return nil
}

func GetUserTagRules(settings UpdaterSettings, id tgtypes.UserID, name string) (string, error) {
	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return "", settings.Transaction.err }

	row := tx.QueryRow("SELECT rules FROM user_tagrules WHERE telegram_id = $1 AND name = $2", id, name)
	var rules string
	err := row.Scan(&rules)
	if err == sql.ErrNoRows { err = nil } // no data for user is not an error.

	settings.Transaction.commit = mine
	return rules, err
}

func DeleteUserTagRules(settings UpdaterSettings, id tgtypes.UserID) (error) {
	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

	_, err := tx.Exec("DELETE FROM user_tagrules WHERE telegram_id = $1", id)

	settings.Transaction.commit = mine && (err == nil)
	return err
}

func PrefixedTagToTypedTag(name string) (string, int) {
	if trimmed := strings.TrimPrefix(name, "general:"); trimmed != name { return trimmed, apitypes.TCGeneral.Value() }
	if trimmed := strings.TrimPrefix(name, "character:"); trimmed != name { return trimmed, apitypes.TCCharacter.Value() }
	if trimmed := strings.TrimPrefix(name, "artist:"); trimmed != name { return trimmed, apitypes.TCArtist.Value() }
	if trimmed := strings.TrimPrefix(name, "copyright:"); trimmed != name { return trimmed, apitypes.TCCopyright.Value() }
	if trimmed := strings.TrimPrefix(name, "species:"); trimmed != name { return trimmed, apitypes.TCSpecies.Value() }
	if trimmed := strings.TrimPrefix(name, "invalid:"); trimmed != name { return trimmed, apitypes.TCInvalid.Value() }
	if trimmed := strings.TrimPrefix(name, "meta:"); trimmed != name { return trimmed, apitypes.TCMeta.Value() }
	if trimmed := strings.TrimPrefix(name, "lore:"); trimmed != name { return trimmed, apitypes.TCLore.Value() }
	return name, apitypes.TCGeneral.Value()
}

type EnumerateControl struct {
	OrderByCount bool
	CreatePhantom bool
	IncludeDeleted bool
	Transaction TransactionBox
}

func GetTag(name string, ctrl EnumerateControl) (*apitypes.TTagData, error) {
	mine, tx := ctrl.Transaction.PopulateIfEmpty(Db_pool)
	defer ctrl.Transaction.Finalize(mine)
	if ctrl.Transaction.err != nil { return nil, ctrl.Transaction.err }

	sq := "SELECT tag_id, tag_name, tag_count, tag_count_full, tag_type, tag_type_locked FROM tag_index WHERE LOWER(tag_name) = LOWER($1) LIMIT 1"
	name, typ := PrefixedTagToTypedTag(name)

	row := tx.QueryRow(sq, name)

	var tag apitypes.TTagData
	err := row.Scan(&tag.Id, &tag.Name, &tag.Count, &tag.FullCount, &tag.Type, &tag.Locked)

	if err == sql.ErrNoRows {
		if !ctrl.CreatePhantom { return nil, nil } // don't create phantom tag, so just return nil for "not found"
		// otherwise, insert a phantom tag
		row = tx.QueryRow("INSERT INTO tag_index (tag_id, tag_name, tag_count, tag_type, tag_type_locked) VALUES (nextval('phantom_tag_seq'), $1, 0, $2, false) RETURNING tag_id, tag_name, tag_count, tag_count_full, tag_type, tag_type_locked", name, typ)
		err = row.Scan(&tag.Id, &tag.Name, &tag.Count, &tag.FullCount, &tag.Type, &tag.Locked)
		if err == sql.ErrNoRows { return nil, nil } // this really shouldn't happen, but just in case.
	}
	if err != nil {
		return nil, err
	}

	ctrl.Transaction.commit = mine
	return &tag, err
}

func GetLastTag(settings UpdaterSettings) (*apitypes.TTagData, error) {
	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return nil, settings.Transaction.err }

	sq := "SELECT tag_id, tag_name, tag_count, tag_type, tag_type_locked FROM tag_index WHERE tag_id = (SELECT MAX(tag_id) FROM tag_index) LIMIT 1"
	row := tx.QueryRow(sq)

	var tag apitypes.TTagData
	err := row.Scan(&tag.Id, &tag.Name, &tag.Count, &tag.Type, &tag.Locked)

	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	settings.Transaction.commit = mine
	return &tag, nil
}

func ClearTagIndex(settings UpdaterSettings) (error) {
	// don't delete phantom tags. phantom tags have an id less than zero, and that id is transient, so if the
	// tag database has phantom tags applied to any posts and they are deleted from here they will become dangling.
	// instead, keep them. they may conflict later with real tags, in which case they will be de-phantomified.

	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

	_, err := tx.Exec("DELETE FROM tag_index WHERE tag_id > 0")

	settings.Transaction.commit = mine
	return err
}

func ClearAliasIndex(settings UpdaterSettings) (error) {
	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

	_, err := tx.Exec("TRUNCATE alias_index")

	settings.Transaction.commit = mine
	return err
}

func ClearPosts(settings UpdaterSettings) (error) {
	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

	_, err := tx.Exec("TRUNCATE post_tags, post_tags_by_name, post_index")

	settings.Transaction.commit = mine && (err != nil)
	return err
}

func WriteTagEntries(list []interface{}, settings UpdaterSettings) (error) {
	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

	stmt, err := tx.Prepare(pq.CopyIn("tag_index", "tag_id", "tag_name", "tag_count", "tag_type", "tag_type_locked"))

	for i := 0; i < len(list); i += 5 {
		_, err = stmt.Exec(list[i], list[i+1], list[i+2], list[i+3], list[i+4])
		if err != nil { return err }
	}

	_, err = stmt.Exec()
	if err != nil { return err }

	err = stmt.Close()

	settings.Transaction.commit = mine && (err != nil)
	return err
}

func GetTagsWithCountLess(count int) (apitypes.TTagInfoArray, error) { return getTagsWithCount(count, "<") }
func GetTagsWithCountGreater(count int) (apitypes.TTagInfoArray, error) { return getTagsWithCount(count, ">") }
func GetTagsWithCountEqual(count int) (apitypes.TTagInfoArray, error) { return getTagsWithCount(count, "=") }

func getTagsWithCount(count int, differentiator string) (apitypes.TTagInfoArray, error) {
	sql := fmt.Sprintf("SELECT tag_id, tag_name, tag_count, tag_type, tag_type_locked FROM tag_index WHERE tag_count %s $1", differentiator)
	rows, err := Db_pool.Query(sql, count)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	var d apitypes.TTagData
	var out apitypes.TTagInfoArray

	for rows.Next() {
		err = rows.Scan(&d.Id, &d.Name, &d.Count, &d.Type, &d.Locked)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

func GetAliasesFor(tag string, ctrl EnumerateControl) (apitypes.TTagInfoArray, error) {
	sql :=	"SELECT a.tag_id, a.tag_name, a.tag_count, a.tag_type, a.tag_type_locked FROM " +
			"tag_index AS %s INNER JOIN " +
			"alias_index AS b ON (%s.tag_name = b.alias_name) INNER JOIN " +
			"tag_index AS %s ON (b.alias_target_id = %s.tag_id) " +
		"WHERE c.tag_name = $1"

	mine, tx := ctrl.Transaction.PopulateIfEmpty(Db_pool)
	defer ctrl.Transaction.Finalize(mine)
	if ctrl.Transaction.err != nil { return nil, ctrl.Transaction.err }

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

	ctrl.Transaction.commit = mine
	return out, nil
}

func GetAliasedTags() (apitypes.TTagInfoArray, error) {
	sql := "SELECT tag_id, tag_name, tag_count, tag_type, tag_type_locked FROM tag_index INNER JOIN alias_index ON alias_name = tag_name WHERE tag_count != 0 AND tag_name != ''"
	rows, err := Db_pool.Query(sql)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	var d apitypes.TTagData
	var out apitypes.TTagInfoArray

	for rows.Next() {
		err = rows.Scan(&d.Id, &d.Name, &d.Count, &d.Type, &d.Locked)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

func AliasUpdater(input chan apitypes.TAliasData, settings UpdaterSettings) (error) {
	defer func(){ for _ = range input {} }()

	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

	for alias := range input {
		sql := "DELETE FROM alias_index WHERE alias_id = $1"
		_, err := tx.Exec(sql, alias.Id)
		if err != nil { return err }

		sql = "INSERT INTO alias_index (alias_id, alias_name, alias_target_id) SELECT $1, $2, tag_id FROM tag_index WHERE tag_name = $3"
		_, err = tx.Exec(sql, alias.Id, alias.Name, alias.Alias)
		if err != nil { return err }
	}

	settings.Transaction.commit = mine
	return nil
}

func PostUpdater(input chan apitypes.TPostInfo, settings UpdaterSettings) (error) {
	defer func(){ for _ = range input {} }()

	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

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

	settings.Transaction.commit = mine
	return nil
}

func PostDeleter(input chan []int, settings UpdaterSettings) (error) {
	defer func(){ for _ = range input {} }()

	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

	for list := range input {
		_, err := tx.Exec("UPDATE post_index SET post_deleted = true WHERE post_id = ANY($1::int[])", pq.Array(list))
		if err != nil { return err }
	}

	settings.Transaction.commit = mine
	return nil
}

func MarkPostDeleted(post_id int, settings UpdaterSettings) error {
	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

	query := "UPDATE post_index SET post_deleted = TRUE WHERE post_id = $1"
	_, err := tx.Exec(query, post_id)

	settings.Transaction.commit = mine && err == nil
	return err
}

func GetHighestStagedPostID(settings UpdaterSettings) (int) {
	row := Db_pool.QueryRow("SELECT MAX(post_id) FROM post_tags_by_name")
	var result int
	_ = row.Scan(&result)
	return result
}

func GetHighestPostID(settings UpdaterSettings) (int) {
	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return 0 }

	query := "SELECT MAX(post_id) FROM post_index"

	row := tx.QueryRow(query)
	var result int
	err := row.Scan(&result)

	settings.Transaction.commit = mine && (err == nil || err == sql.ErrNoRows)
	return result
}

func TagUpdater(input chan apitypes.TTagData, settings UpdaterSettings) (error) {
	mine, _ := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

	defer func(){ for _ = range input {} }()

	for tag := range input {
		f := false
		if tag.Locked == nil { tag.Locked = &f }

		_, err := settings.Transaction.tx.Exec("INSERT INTO tag_index (tag_id, tag_name, tag_count, tag_type, tag_type_locked) VALUES ($1, $2, $3, $4, $5) ON CONFLICT (tag_name) DO UPDATE SET tag_id = EXCLUDED.tag_id, tag_count = EXCLUDED.tag_count, tag_type = EXCLUDED.tag_type, tag_type_locked = EXCLUDED.tag_type_locked", tag.Id, tag.Name, tag.Count, tag.Type, *tag.Locked)
		if err != nil { return err }
	}

	settings.Transaction.commit = mine
	return nil
}

//func EnumerateAllTagNames() ([]string, error) {
//	return []string{"tawny_otter_(character)", "tawny", "otter", "character"}, nil
//}

func EnumerateAllTags(ctrl EnumerateControl) (apitypes.TTagInfoArray, error) {
	mine, tx := ctrl.Transaction.PopulateIfEmpty(Db_pool)
	defer ctrl.Transaction.Finalize(mine)
	if ctrl.Transaction.err != nil { return nil, ctrl.Transaction.err }

	var output apitypes.TTagInfoArray

	sql := "SELECT tag_id, tag_name, tag_count, tag_count_full, tag_type, tag_type_locked FROM tag_index %s"
	order_by := "ORDER BY %s"

	if ctrl.OrderByCount {
		order_by = fmt.Sprintf(order_by, "-tag_count")
	} else {
		order_by = ""
	}

	sql = fmt.Sprintf(sql, order_by)

	rows, err := tx.Query(sql)
	if err != nil { return nil, err }

	defer rows.Close()
	var d apitypes.TTagData

	for rows.Next() {
		err = rows.Scan(&d.Id, &d.Name, &d.Count, &d.FullCount, &d.Type, &d.Locked)
		if err != nil { return nil, err }

		output = append(output, d)
	}

	ctrl.Transaction.commit = mine
	return output, nil
}

func EnumerateAllBlits(ctrl EnumerateControl) (map[string]bool, error) {
	mine, tx := ctrl.Transaction.PopulateIfEmpty(Db_pool)
	defer ctrl.Transaction.Finalize(mine)
	if ctrl.Transaction.err != nil { return nil, ctrl.Transaction.err }

	result := make(map[string]bool)
	sql := "SELECT tag_name, is_blit FROM blit_tag_registry INNER JOIN tag_index USING (tag_id)"
	rows, err := tx.Query(sql)
	if err != nil { return nil, err }

	defer rows.Close()
	for rows.Next() {
		var is_blit bool
		var tag_name string
		err := rows.Scan(&tag_name, &is_blit)
		if err != nil { return nil, err }
		result[tag_name] = is_blit
	}

	ctrl.Transaction.commit = mine
	return result, nil
}

func EnumerateCatsExceptions(ctrl EnumerateControl) ([]string, error) {
	mine, tx := ctrl.Transaction.PopulateIfEmpty(Db_pool)
	defer ctrl.Transaction.Finalize(mine)
	if ctrl.Transaction.err != nil { return nil, ctrl.Transaction.err }

	sql := "SELECT tag FROM cats_ignored"
	rows, err := tx.Query(sql)
	if err != nil { return nil, err }

	var output []string

	for rows.Next() {
		var tag string
		err = rows.Scan(&tag)
		if err != nil { return nil, err }

		output = append(output, tag)
	}

	ctrl.Transaction.commit = mine
	return output, nil
}

func SetCatsException(tag string, ctrl EnumerateControl) (error) {
	mine, tx := ctrl.Transaction.PopulateIfEmpty(Db_pool)
	defer ctrl.Transaction.Finalize(mine)
	if ctrl.Transaction.err != nil { return ctrl.Transaction.err }

	sql := "INSERT INTO cats_ignored (tag) VALUES ($1)"
	_, err := tx.Exec(sql, tag)

	ctrl.Transaction.commit = mine
	return err
}

func ClearCatsException(tag string, ctrl EnumerateControl) (error) {
	mine, tx := ctrl.Transaction.PopulateIfEmpty(Db_pool)
	defer ctrl.Transaction.Finalize(mine)
	if ctrl.Transaction.err != nil { return ctrl.Transaction.err }

	sql := "DELETE FROM cats_ignored WHERE tag = $1"
	_, err := tx.Exec(sql, tag)

	ctrl.Transaction.commit = mine
	return err
}

func RecalculateAliasedCounts(settings UpdaterSettings) (error) {
	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

	sql := 	"UPDATE tag_index " +
			"SET tag_count = subquery.tag_count " +
		"FROM (SELECT a.tag_id, c.tag_count " +
			"FROM tag_index AS a INNER JOIN " +
				"alias_index AS b ON (a.tag_name = b.alias_name) INNER JOIN " +
				"tag_index AS c ON (b.alias_target_id = c.tag_id)) AS subquery " +
		"WHERE tag_index.tag_id = subquery.tag_id"
	_, err := tx.Exec(sql)
	if err != nil { return err }

	sql = 	"UPDATE tag_index " +
			"SET tag_count_full = subquery.tag_count_full " +
		"FROM (SELECT a.tag_id, c.tag_count_full " +
			"FROM tag_index AS a INNER JOIN " +
				"alias_index AS b ON (a.tag_name = b.alias_name) INNER JOIN " +
				"tag_index AS c ON (b.alias_target_id = c.tag_id)) AS subquery " +
		"WHERE tag_index.tag_id = subquery.tag_id"
	_, err = tx.Exec(sql)
	if err != nil { return err }

	settings.Transaction.commit = mine
	return nil
}

func GetMostRecentlyUpdatedPost(settings UpdaterSettings) (*apitypes.TPostInfo, error) {
	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return nil, settings.Transaction.err }

	var p apitypes.TPostInfo
	row := tx.QueryRow("SELECT post_id, post_change_seq, post_rating, post_description, post_hash FROM post_index ORDER BY post_change_seq DESC LIMIT 1")
	err := row.Scan(&p.Id, &p.Change, &p.Rating, &p.Description, &p.Md5)

	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	settings.Transaction.commit = mine
	return &p, err
}

func ImportPostTagsFromNameToID(settings UpdaterSettings, sfx chan string) (error) {
	status := func(s string) {
		if sfx != nil {
			sfx <- s
		}
	}

	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

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

		// bump maintenance memory threshhold, the default value is low. this field's value is per transaction.
		query := "SET maintenance_work_mem TO '4 GB'"
		_, err = tx.Exec(query)
		if err != nil { return err }

		// delete existing tag records before removing indices because it will be a lot slower without them
		status(" (1/4 tag clear overrides)")
		query = "DELETE FROM post_tags WHERE post_id IN (SELECT DISTINCT post_id FROM post_tags_by_name)"
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
	if err != nil { return err }

	settings.Transaction.commit = mine
	return nil
}

func ResetPostTags(settings UpdaterSettings) (error) {
	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

	query := "TRUNCATE post_tags, post_tags_by_name"
	_, err := tx.Exec(query)

	settings.Transaction.commit = mine && (err != nil)
	return err
}

func CountTags(settings UpdaterSettings, sfx chan string) (error) {
	status := func(s string) {
		if sfx != nil {
			sfx <- s
		}
	}

	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

	status(" (1/3 reset cached counts)")
	query := "UPDATE tag_index SET tag_count = 0"
	_, err := tx.Exec(query)
	if err != nil { return err }

	status(" (2/3 calculate full tag counts)")
	query = "WITH subq AS (SELECT tag_id, COUNT(tag_id) AS real_count FROM post_tags GROUP BY tag_id) UPDATE tag_index SET tag_count_full = subq.real_count FROM subq WHERE subq.tag_id = tag_index.tag_id"
	_, err = tx.Exec(query)
	if err != nil { return err }

	status(" (3/3 calculate visible tag counts)")
	query = "WITH subq AS (SELECT tag_id, COUNT(tag_id) AS real_count FROM post_tags INNER JOIN post_index USING (post_id) WHERE NOT post_deleted GROUP BY tag_id) UPDATE tag_index SET tag_count = subq.real_count FROM subq WHERE subq.tag_id = tag_index.tag_id"
	_, err = tx.Exec(query)
	if err != nil { return err }

	settings.Transaction.commit = mine
	return nil
}

func PostsWithTag(tag apitypes.TTagData, ctrl EnumerateControl) (apitypes.TPostInfoArray, error) {
	mine, tx := ctrl.Transaction.PopulateIfEmpty(Db_pool)
	defer ctrl.Transaction.Finalize(mine)
	if ctrl.Transaction.err != nil { return nil, ctrl.Transaction.err }

	query := ""
	if ctrl.IncludeDeleted {
		query = "SELECT post_id FROM post_tags WHERE tag_id = $1 ORDER BY post_id"
	} else {
		query = "SELECT post_id FROM post_tags INNER JOIN post_index USING (post_id) WHERE tag_id = $1 AND NOT post_deleted ORDER BY post_id"
	}
	rows, err := tx.Query(query, tag.Id)
	if err != nil { return nil, err }

	var out apitypes.TPostInfoArray
	var item apitypes.TPostInfo
	for rows.Next() {
		err := rows.Scan(&item.Id)
		if err != nil { return nil, err }
		out = append(out, item)
	}

	ctrl.Transaction.commit = mine
	return out, nil
}

func PostByID(id int, ctrl UpdaterSettings) (*apitypes.TPostInfo, error) {
	out, err := PostsById([]int{id}, ctrl)
	if len(out) == 0 {
		return nil, err
	} else {
		return &out[0], err
	}
}

func PostsById(ids []int, ctrl UpdaterSettings) ([]apitypes.TPostInfo, error) {
	mine, tx := ctrl.Transaction.PopulateIfEmpty(Db_pool)
	defer ctrl.Transaction.Finalize(mine)
	if ctrl.Transaction.err != nil { return nil, ctrl.Transaction.err }

	var item apitypes.TPostInfo
	query := "SELECT post_id, post_change_seq, post_rating, post_description, post_sources, post_hash, post_deleted, ARRAY(SELECT tag_name FROM tag_index INNER JOIN post_tags USING (tag_id) WHERE post_id = post_index.post_id) AS post_tags FROM post_index WHERE post_id = ANY($1::int[])"
	rows, err := tx.Query(query, pq.Array(ids))
	if err != nil { return nil, err }
	defer rows.Close()

	var out []apitypes.TPostInfo
	for rows.Next() {
		err = item.ScanFrom(rows)
		if err != nil { return nil, err }
		out = append(out, item)
	}

	ctrl.Transaction.commit = mine
	return out, nil
}

func PostByMD5(md5 string, ctrl UpdaterSettings) (*apitypes.TPostInfo, error) {
	mine, tx := ctrl.Transaction.PopulateIfEmpty(Db_pool)
	defer ctrl.Transaction.Finalize(mine)
	if ctrl.Transaction.err != nil { return nil, ctrl.Transaction.err }

	var item apitypes.TPostInfo
	var sources string
	query := "SELECT post_id, post_change_seq, post_rating, post_description, post_sources, post_hash, post_deleted, ARRAY(SELECT tag_name FROM tag_index INNER JOIN post_tags USING (tag_id) WHERE post_id = post_index.post_id) AS post_tags FROM post_index WHERE post_hash = $1;"
	err := tx.QueryRow(query, md5).Scan(&item.Id, &item.Change, &item.Rating, &item.Description, &sources, &item.Md5, &item.Deleted, pq.Array(&item.General))
	if err != sql.ErrNoRows && err != nil  { return nil, err }
	item.Sources = strings.Split(sources, "\n")

	ctrl.Transaction.commit = mine
	return &item, nil
}

func LocalTagSearch(tag apitypes.TTagData, ctrl EnumerateControl) (apitypes.TPostInfoArray, error) {
	mine, tx := ctrl.Transaction.PopulateIfEmpty(Db_pool)
	defer ctrl.Transaction.Finalize(mine)
	if ctrl.Transaction.err != nil { return nil, ctrl.Transaction.err }

	query := "SELECT post_id, (SELECT tag_name FROM tag_index WHERE tag_id = post_tags.tag_id) FROM (SELECT post_id FROM post_tags WHERE tag_id = $1) AS a INNER JOIN post_tags USING (post_id) ORDER BY post_id"
	rows, err := tx.Query(query, tag.Id)
	if err != nil { return nil, err }

	var out apitypes.TPostInfoArray
//	var item apitypes.TPostInfo
	var intermed map[int][]string = make(map[int][]string)
	for rows.Next() {
		var id int
		var tag string
		err := rows.Scan(&id, &tag)
		if err != nil { return nil, err }
		intermed[id] = append(intermed[id], tag)
	}

	panic("needs lots of updates!")

	//for k, v := range intermed {
	//	item.Id = k
	//	item.Tags = strings.Join(v, " ")
	//	out = append(out, item)
	//}

	//ctrl.Transaction.commit = mine
	return out, nil
}

func UpdatePost(post apitypes.TPostInfo, settings UpdaterSettings) (error) {
	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

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
	if err != nil { return err }

	settings.Transaction.commit = mine
	return nil
}

type BlitData struct {
	apitypes.TTagData

	Valid bool
}

func name_of(tag_type apitypes.TagCategory) string {
	switch tag_type {
	case apitypes.TCGeneral:
		return "[GENERAL]"
	case apitypes.TCSpecies:
		return "[SPECIES]"
	case apitypes.TCArtist:
		return "[ARTIST]"
	case apitypes.TCCopyright:
		return "[CPYRIGT]"
	case apitypes.TCCharacter:
		return "[CHRACTR]"
	case apitypes.TCLore:
		return "[LORE]"
	case apitypes.TCMeta:
		return "[META]"
	case apitypes.TCInvalid:
		return "[INVALID]"
	default:
		return "[UNKNOWN]"
	}
}

func (b BlitData) String() string {
	return fmt.Sprintf("%8d %9s %s", b.Count, name_of(b.Type), b.Name)
}

func GetBlits(yes, no, wild bool, ctrl EnumerateControl) ([]BlitData, []BlitData, []BlitData, error) {
	mine, tx := ctrl.Transaction.PopulateIfEmpty(Db_pool)
	defer ctrl.Transaction.Finalize(mine)
	if ctrl.Transaction.err != nil { return nil, nil, nil, ctrl.Transaction.err }

	var blit BlitData
	var out_yes, out_no, out_wild []BlitData

	query := "SELECT is_blit, tag_id, tag_name, tag_count, tag_type, tag_type_locked FROM tag_index LEFT JOIN blit_tag_registry USING (tag_id) WHERE (LENGTH(tag_name) <= 2 OR is_blit IS NOT NULL) AND (($1 AND is_blit IS NULL) OR ($2 AND is_blit IS TRUE) OR ($3 AND is_blit IS FALSE)) ORDER BY is_blit, tag_count DESC"
	rows, err := tx.Query(query, wild, yes, no)
	if err != nil { return nil, nil, nil, err }
	defer rows.Close()

	for rows.Next() {
		var status *bool
		err = rows.Scan(&status, &blit.Id, &blit.Name, &blit.Count, &blit.Type, &blit.Locked)

		if err != nil { return nil, nil, nil, err }

		if status == nil {
			blit.Valid = false
			out_wild = append(out_wild, blit)
		} else if *status == true {
			blit.Valid = true
			out_yes = append(out_yes, blit)
		} else {
			blit.Valid = false
			out_no = append(out_no, blit)
		}
	}

	ctrl.Transaction.commit = mine
	return out_yes, out_no, out_wild, nil
}

func GetMarkedAndUnmarkedBlits(ctrl EnumerateControl) ([]BlitData, error) {
	mine, tx := ctrl.Transaction.PopulateIfEmpty(Db_pool)
	defer ctrl.Transaction.Finalize(mine)
	if ctrl.Transaction.err != nil { return nil, ctrl.Transaction.err }

	var blit BlitData
	var out []BlitData

	query := "SELECT is_blit, tag_id, tag_name, tag_count, tag_type, tag_type_locked FROM blit_tag_registry INNER JOIN tag_index USING (tag_id) ORDER BY NOT is_blit, tag_name"
	rows, err := tx.Query(query)
	if err != nil { return nil, err }
	defer rows.Close()

	for rows.Next() {
		err = rows.Scan(&blit.Valid, &blit.Id, &blit.Name, &blit.Count, &blit.Type, &blit.Locked)
		if err != nil { return nil, err }
		out = append(out, blit)
	}

	ctrl.Transaction.commit = mine
	return out, nil
}

func MarkBlit(id int, mark bool, ctrl EnumerateControl) (error) {
	mine, tx := ctrl.Transaction.PopulateIfEmpty(Db_pool)
	defer ctrl.Transaction.Finalize(mine)
	if ctrl.Transaction.err != nil { return ctrl.Transaction.err }

	_, err := tx.Exec("INSERT INTO blit_tag_registry (tag_id, is_blit) VALUES ($1, $2) ON CONFLICT (tag_id) DO UPDATE SET is_blit = EXCLUDED.is_blit", id, mark)

	ctrl.Transaction.commit = mine
	return err
}

var ErrNoTag = errors.New("no corresponding tag exists")

func MarkBlitByName(name string, mark bool, ctrl EnumerateControl) (error) {
	mine, tx := ctrl.Transaction.PopulateIfEmpty(Db_pool)
	defer ctrl.Transaction.Finalize(mine)
	if ctrl.Transaction.err != nil { return ctrl.Transaction.err }

	out, err := tx.Exec("INSERT INTO blit_tag_registry SELECT tag_id, $2 as is_blit FROM tag_index WHERE tag_name = $1 ON CONFLICT (tag_id) DO UPDATE SET is_blit = EXCLUDED.is_blit", name, mark)

	rows, err := out.RowsAffected()
	if err != nil { return err }

	ctrl.Transaction.commit = mine

	if rows == 0 {
		return ErrNoTag
	}

	return err
}

func DeleteBlitByName(name string, ctrl EnumerateControl) (error) {
	mine, tx := ctrl.Transaction.PopulateIfEmpty(Db_pool)
	defer ctrl.Transaction.Finalize(mine)
	if ctrl.Transaction.err != nil { return ctrl.Transaction.err }

	out, err := tx.Exec("DELETE FROM blit_tag_registry WHERE tag_id = (SELECT tag_id FROM tag_index WHERE tag_name = $1)", name)

	rows, err := out.RowsAffected()
	if err != nil { return err }

	ctrl.Transaction.commit = mine

	if rows == 0 {
		return ErrNoTag
	}

	return err
}

type CorrectionMode int
const Untracked CorrectionMode = 0
const Ignore CorrectionMode = 1
const Prompt CorrectionMode = 2
const AutoFix CorrectionMode = 3
func (this CorrectionMode) Display() string {
	switch this {
	case Untracked:
		return " "
	case Ignore:
		return "i"
	case Prompt:
		return "P"
	case AutoFix:
		return "X"
	default:
		return "?"
	}
}

func AddTagTypo(real_name, typo_name string, mode CorrectionMode, settings UpdaterSettings) (error) {
	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

	query := "INSERT INTO typos_identified (tag_implied_name, tag_typo_name, mode) VALUES ($1, $2, $3)"
	_, err := tx.Exec(query, real_name, typo_name, mode)
	if err != nil { return err }

	settings.Transaction.commit = mine
	return nil
}

type TypoData struct {
	Tag  apitypes.TTagData
	Mode CorrectionMode
}

func GetTagTypos(tag string, ctrl EnumerateControl) (map[string]TypoData, error) {
	mine, tx := ctrl.Transaction.PopulateIfEmpty(Db_pool)
	defer ctrl.Transaction.Finalize(mine)
	if ctrl.Transaction.err != nil { return nil, ctrl.Transaction.err }

	query := "SELECT mode, tag_id, tag_name, tag_count, tag_count_full, tag_type, tag_type_locked FROM typos_identified INNER JOIN tag_index ON tag_name = tag_typo_name WHERE tag_implied_name = $1"
	rows, err := tx.Query(query, tag)
	if err != nil { return nil, err }

	defer rows.Close()

	results := make(map[string]TypoData)
	for rows.Next() {
		var data TypoData
		err = rows.Scan(&data.Mode, &data.Tag.Id, &data.Tag.Name, &data.Tag.Count, &data.Tag.FullCount, &data.Tag.Type, &data.Tag.Locked)
		if err != nil { return nil, err }
		results[data.Tag.Name] = data
	}

	ctrl.Transaction.commit = mine
	return results, nil
}

type PostSuggestedEdit struct {
	Prompt        tags.TagDiffArray      `json:"prompt"`
	AutoFix       tags.TagDiffArray      `json:"autofix"`
	SelectedEdits map[string]bool        `json:"selected_edits"`
	AppliedEdits  map[string]bool        `json:"applied_edits"`
}

func (this *PostSuggestedEdit) SelectAutofix() {
	for _, diff := range this.AutoFix { this.SelectDirect(diff.APIString()) }
}

func (this *PostSuggestedEdit) SelectDirect(api_string string) {
	if this.SelectedEdits == nil {
		this.SelectedEdits = make(map[string]bool)
	}
	this.SelectedEdits[api_string] = true
}

func (this *PostSuggestedEdit) DeselectDirect(api_string string) {
	delete(this.SelectedEdits, api_string)
}

// do nothing if an invalid selector is specified
func (this *PostSuggestedEdit) Select(from string, index int) {
	if this.SelectedEdits == nil {
		this.SelectedEdits = make(map[string]bool)
	}

	array := &this.Prompt
	if from == "prompt" {
	} else if from == "autofix" {
		array = &this.AutoFix
	} else { return }

	if len(*array) > index {
		this.SelectedEdits[(*array)[index].APIString()] = true
	}
}

func (this *PostSuggestedEdit) SelectAll() {
	if this.SelectedEdits == nil {
		this.SelectedEdits = make(map[string]bool)
	}

	for _, diff := range this.Prompt {
		this.SelectedEdits[diff.APIString()] = true
	}
	for _, diff := range this.AutoFix {
		this.SelectedEdits[diff.APIString()] = true
	}
}

// do nothing if an invalid selector is specified
func (this *PostSuggestedEdit) Deselect(from string, index int) {
	array := &this.Prompt
	if from == "prompt" {
	} else if from == "autofix" {
		array = &this.AutoFix
	} else { return }

	if len(*array) > index {
		delete(this.SelectedEdits, (*array)[index].APIString())
	}
}

func (this *PostSuggestedEdit) DeselectAll() {
	this.SelectedEdits = nil
}

func (this *PostSuggestedEdit) Apply() {
	this.AppliedEdits = make(map[string]bool)
	for k, _ := range this.SelectedEdits {
		this.AppliedEdits[k] = true
	}
}

func (this PostSuggestedEdit) Value() (driver.Value, error) {
	return json.Marshal(this)
}

func (this *PostSuggestedEdit) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(b, &this)
}

func (this PostSuggestedEdit) GetChangeToApply() tags.TagDiff {
	var apply tags.TagDiff
	for selected, _ := range this.SelectedEdits {
		if !this.AppliedEdits[selected] {
			apply.ApplyString(selected)
		}
	}

	var revert tags.TagDiff
	for applied, _ := range this.AppliedEdits {
		if !this.SelectedEdits[applied] {
			revert.ApplyString(applied)
		}
	}
	return revert.Invert().Union(apply)
}

// merge two edit lists, with certain requirements.
// the edit list of `this` must remain index-stable, that is to say, items in
// its prompt and autofix lists should keep the same indices in the output.
// this allows us to merge multiple edit lists onto one, while not breaking
// the user experience by making buttons move around if we edit a post while
// they're pushing them.
func (this PostSuggestedEdit) Append(other PostSuggestedEdit) PostSuggestedEdit {
	var new_pse PostSuggestedEdit

	// build a membership map, and add everything from `this.Prompt`
	membership := make(map[string]bool)
	for _, diff := range this.Prompt {
		membership[diff.APIString()] = true
		new_pse.Prompt = append(new_pse.Prompt, diff)
	}

	// add everything from `other.Prompt` that isn't already a member
	for _, diff := range other.Prompt {
		api_string := diff.APIString()
		if membership[api_string] { continue }
		new_pse.Prompt = append(new_pse.Prompt, diff)
		membership[api_string] = true
	}

	// build a membership map, and add everything from `this.AutoFix`
	membership = make(map[string]bool)
	for _, diff := range this.AutoFix {
		membership[diff.APIString()] = true
		new_pse.AutoFix = append(new_pse.AutoFix, diff)
	}

	// add everything from `other.AutoFix` that isn't already a member
	for _, diff := range other.AutoFix {
		api_string := diff.APIString()
		if membership[api_string] { continue }
		new_pse.AutoFix = append(new_pse.AutoFix, diff)
		membership[api_string] = true
	}

	// merge the selected/applied edit lists.
	for k := range this.SelectedEdits { new_pse.SelectDirect(k) }
	for k := range other.SelectedEdits { new_pse.SelectDirect(k) }
	for k := range this.AppliedEdits { new_pse.SelectDirect(k) }
	for k := range other.AppliedEdits { new_pse.SelectDirect(k) }
	return new_pse
}

func (this PostSuggestedEdit) Flatten() tags.TagDiff {
	return this.Prompt.Flatten().Union(this.AutoFix.Flatten())
}

func GetSuggestedPostEdits(posts []int, settings UpdaterSettings) (map[int]PostSuggestedEdit, error) {
	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return nil, settings.Transaction.err }

	results := make(map[int]PostSuggestedEdit)
	populate := func(id int, mode CorrectionMode) *tags.TagDiff {
		value := results[id]

		array := &value.Prompt
		if mode == Prompt {
		} else if mode == AutoFix {
			array = &value.AutoFix
		} else {
			panic("bad CorrectionMode? should be either Prompt or Autofix")
		}

		*array = append(*array, tags.TagDiff{})
		results[id] = value
		return &(*array)[len(*array) - 1]
	}

	query := "SELECT post_id, tag_typo_name, tag_implied_name, mode FROM typos_identified INNER JOIN tag_index ON tag_typo_name = tag_name INNER JOIN post_tags USING (tag_id) WHERE mode IN ($1, $2) AND post_id = ANY($3::int[])"
	rows, err := tx.Query(query, Prompt, AutoFix, pq.Array(posts))
	if err != nil { return nil, err }
	defer rows.Close()

	for rows.Next() {
		var id int
		var typo, fixed string
		var mode CorrectionMode
		err = rows.Scan(&id, &typo, &fixed, &mode)
		if err != nil { return nil, err }

		diff := populate(id, mode)

		diff.Remove(typo)
		diff.Add(fixed)
	}

	query = "SELECT post_id, tag_cat_name, tag_1_name, tag_2_name, mode FROM cats_identified INNER JOIN tag_index ON tag_cat_name = tag_name INNER JOIN post_tags USING (tag_id) WHERE mode IN ($1, $2) AND post_id = ANY($3::int[])"
	rows, err = tx.Query(query, Prompt, AutoFix, pq.Array(posts))
	if err != nil { return nil, err }
	defer rows.Close()

	for rows.Next() {
		var id int
		var cat, first, second string
		var mode CorrectionMode
		err = rows.Scan(&id, &cat, &first, &second, &mode)
		if err != nil { return nil, err }

		diff := populate(id, mode)

		diff.Remove(cat)
		diff.Add(first)
		diff.Add(second)
	}

	settings.Transaction.commit = mine
	return results, nil
}

type PromptPostInfo struct {
	PostId     int
	PostType   string
	PostURL    string
	SampleURL  string
	PostMd5    string
	PostWidth  int
	PostHeight int
	MsgId      tgtypes.MsgID
	ChatId     tgtypes.ChatID
	Timestamp  time.Time
	Captioned  bool
	Edit      *PostSuggestedEdit
}

func GetAutoFixHistoryForPosts(posts []int, settings UpdaterSettings) (map[int][]tags.TagDiff, error) {
	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return nil, settings.Transaction.err }

	query := "SELECT post_id, fix_change FROM autofix_history WHERE post_id = ANY($1::int[])"
	rows, err := tx.Query(query, pq.Array(posts))
	if err != nil { return nil, err }
	defer rows.Close()

	results := make(map[int][]tags.TagDiff)

	for rows.Next()	{
		var id int
		var diff_string string

		err = rows.Scan(&id, &diff_string)
		if err != nil { return nil, err }

		results[id] = append(results[id], tags.TagDiffFromString(diff_string))
	}

	settings.Transaction.commit = mine
	return results, nil
}

func AddAutoFixHistoryForPost(post_id int, changes []string, settings UpdaterSettings) (error) {
	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

	query := "INSERT INTO autofix_history (post_id, fix_ts, fix_change) SELECT $1, now(), change FROM UNNEST($2::varchar[]) AS change ON CONFLICT DO NOTHING"
	_, err := tx.Exec(query, post_id, pq.Array(changes))
	if err != nil { return err }

	settings.Transaction.commit = mine
	return nil
}

func FindPromptPost(id int, settings UpdaterSettings) (*PromptPostInfo, error) {
	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return nil, settings.Transaction.err }

	var x PromptPostInfo
	query := "SELECT post_id, post_type, post_url, sample_url, post_hash, post_width, post_height, msg_id, chat_id, msg_ts, msg_captioned, edit_list_json FROM prompt_posts WHERE post_id = $1"
	err := tx.QueryRow(query, id).Scan(&x.PostId, &x.PostType, &x.PostURL, &x.SampleURL, &x.PostMd5, &x.PostWidth, &x.PostHeight, &x.MsgId, &x.ChatId, &x.Timestamp, &x.Captioned, &x.Edit)

	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	} else {
		return &x, nil
	}
}

func FindPromptPostByMessage(chat_id tgtypes.ChatID, msg_id tgtypes.MsgID, settings UpdaterSettings) (*PromptPostInfo, error) {
	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return nil, settings.Transaction.err }

	var x PromptPostInfo
	query := "SELECT post_id, post_type, post_url, sample_url, post_hash, post_width, post_height, msg_id, chat_id, msg_ts, msg_captioned, edit_list_json FROM prompt_posts WHERE chat_id = $1 AND msg_id = $2"
	err := tx.QueryRow(query, chat_id, msg_id).Scan(&x.PostId, &x.PostType, &x.PostURL, &x.SampleURL, &x.PostMd5, &x.PostWidth, &x.PostHeight, &x.MsgId, &x.ChatId, &x.Timestamp, &x.Captioned, &x.Edit)

	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	} else {
		return &x, nil
	}
}

func FindPromptPostsOlderThan(time_ago time.Duration, settings UpdaterSettings) ([]PromptPostInfo, error) {
	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return nil, settings.Transaction.err }

	query := "SELECT post_id, post_type, post_url, sample_url, post_hash, post_width, post_height, msg_id, chat_id, msg_ts, msg_captioned, edit_list_json FROM prompt_posts WHERE msg_ts <= NOW() - ($1 * '1 second'::interval)"
	rows, err := tx.Query(query, time_ago.Seconds())
	if err != nil { return nil, err }

	var out []PromptPostInfo
	for rows.Next() {
		var x PromptPostInfo
		err = rows.Scan(&x.PostId, &x.PostType, &x.PostURL, &x.SampleURL, &x.PostMd5, &x.PostWidth, &x.PostHeight, &x.MsgId, &x.ChatId, &x.Timestamp, &x.Captioned, &x.Edit)
		if err != nil { return nil, err }
		out = append(out, x)
	}

	settings.Transaction.commit = mine
	return out, nil
}

func SavePromptPost(id int, x *PromptPostInfo, settings UpdaterSettings) (error) {
	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

	query := "DELETE FROM prompt_posts WHERE post_id = $1"
	_, err := tx.Exec(query, id)
	if err != nil { return err }

	if x != nil {
		query = "INSERT INTO prompt_posts (post_id, post_type, post_url, sample_url, post_hash, post_width, post_height, msg_id, chat_id, msg_ts, msg_captioned, edit_list_json) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)"
		_, err = tx.Exec(query, id, x.PostType, x.PostURL, x.SampleURL, strings.ToLower(x.PostMd5), x.PostWidth, x.PostHeight, x.MsgId, x.ChatId, x.Timestamp, x.Captioned, x.Edit)
		if err != nil { return err }
	}

	settings.Transaction.commit = mine
	return nil
}

