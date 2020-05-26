package storage

import (
	apitypes "api/types"
	tgtypes "github.com/thewug/gogram/data"

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

type TransactionBox struct {
	tx    *sql.Tx
	err    error
	commit bool
}

func (this *TransactionBox) Commit() error {
	log.Printf("Committing transaction.\n")
	return this.tx.Commit()
}

func (this *TransactionBox) Rollback() error {
	log.Printf("Aborting transaction.\n")
	return this.tx.Rollback()
}

// This function is intended to be deferred when starting a transaction block.
// its argument is, ideally, the return value of PopulateIfEmpty. If it's true,
// the transaction will be closed out, otherwise this function is a no-op.
// in this way, you can safely handle both situations where a function creates its
// own transaction and is expected to close it, and situations where a function is
// given a pre-existing transaction and is expected to leave it open.

// is idempotent. The first call will commit the transaction, subsequent
// ones are no-ops.

// use it like this:
// mine, tx := box.PopulateIfEmpty(db)
// defer box.Finalize(mine)
func (this *TransactionBox) Finalize(is_transaction_mine bool) {
	if is_transaction_mine && this.tx != nil {
		if this.commit {
			this.Commit()
		} else {
			this.Rollback()
		}
		*this = TransactionBox{}
	} else {
	// if the transaction isn't ours, or there is no transaction (maybe because
	// this isn't the first call), do nothing.
	}
}

func (this *TransactionBox) MarkForCommit() {
	this.commit = true
}

// populates the transaction box with a new transaction. If it succeeds, it returns
// true to indicate that this context "owns" the transaction. if the box was already
// populated, this function is a no-op, and returns false to indicate that the
// transaction is owned by some other context.
func (this *TransactionBox) PopulateIfEmpty(db *sql.DB) (bool, *sql.Tx) {
	if this.tx == nil {
		this.tx, this.err = db.Begin()
		return true, this.tx
	}
	return false, this.tx
}

// creates a new transaction box, containing a fresh transaction.  If you want to use
// Finalize on a transaction created in this way in the context in which it was
// created, pass true for its argument.
func NewTxBox() (TransactionBox, error) {
	newtx, err := Db_pool.Begin()
	return TransactionBox{
		tx: newtx,
	}, err
}

func handle_transaction(c *committer, tx *sql.Tx) {
	if c.commit {
		tx.Commit()
	} else {
		log.Println("wtf?")
		tx.Rollback()
	}
}

func suffix(table, suffix string) string {
	if suffix == "" {
		return table
	} else {
		return table + "__" + suffix
	}
}

func prototype(table string) string {
	return table + "_prototype"
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

func WriteUserCreds(settings UpdaterSettings, id tgtypes.UserID, username, key string) (error) {
	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

	_, err := tx.Exec("INSERT INTO remote_user_credentials (telegram_id, api_user, api_apikey) VALUES ($1, $2, $3) " +
			"ON CONFLICT (telegram_id) DO UPDATE SET api_user = EXCLUDED.api_user, api_apikey = EXCLUDED.api_apikey", id, username, key)
	if (err != nil) { return err }

	settings.Transaction.commit = mine
	return nil
}

func GetUserCreds(settings UpdaterSettings, id tgtypes.UserID) (string, string, bool, error) {
	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return "", "", false, settings.Transaction.err }

	row := tx.QueryRow("SELECT api_user, api_apikey, privilege_janitorial FROM remote_user_credentials WHERE telegram_id = $1", id)
	var user, key string
	var privilege bool
	err := row.Scan(&user, &key, &privilege)
	if err == sql.ErrNoRows || len(user) == 0 || len(key) == 0 { err = ErrNoLogin }

	if err == nil {
		log.Println("user creds no error")
	} else {
		log.Println("user creds", err.Error())
	}

	settings.Transaction.commit = mine
	return user, key, privilege, err
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

func SetMigrationState(settings UpdaterSettings, state, progress int, is_full bool) (error) {
	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

	sq := "DELETE FROM import_progress"
	_, err := tx.Exec(sq)
	if err != nil { return err }

	sq = "INSERT INTO import_progress VALUES ($1, $2, $3)"
	_, err = tx.Exec(sq, state, progress, is_full)
	if err != nil { return err }

	settings.Transaction.commit = mine
	return nil
}

func GetMigrationState(settings UpdaterSettings) (int, int, bool) {
	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return 0, 0, false }

	sq := "SELECT state, progress, is_full FROM import_progress LIMIT 1"
	row := tx.QueryRow(sq)

	var state, progress int
	var is_full bool
	err := row.Scan(&state, &progress, &is_full)

	if err != nil { log.Println(err.Error()) }

	settings.Transaction.commit = mine
	return state, progress, is_full
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
	Transaction TransactionBox
}

func GetTag(name string, ctrl EnumerateControl) (*apitypes.TTagData, error) {
	mine, tx := ctrl.Transaction.PopulateIfEmpty(Db_pool)
	defer ctrl.Transaction.Finalize(mine)
	if ctrl.Transaction.err != nil { return nil, ctrl.Transaction.err }

	sq := "SELECT tag_id, tag_name, tag_count, tag_type, tag_type_locked FROM tag_index WHERE LOWER(tag_name) = LOWER($1) LIMIT 1"
	name, typ := PrefixedTagToTypedTag(name)

	row := tx.QueryRow(sq, name)

	var tag apitypes.TTagData
	err := row.Scan(&tag.Id, &tag.Name, &tag.Count, &tag.Type, &tag.Locked)

	if err == sql.ErrNoRows {
		if !ctrl.CreatePhantom { return nil, nil } // don't create phantom tag, so just return nil for "not found"
		// otherwise, insert a phantom tag
		row = tx.QueryRow("INSERT INTO tag_index (tag_id, tag_name, tag_count, tag_type, tag_type_locked) VALUES (nextval('phantom_tag_seq'), $1, 0, $2, false) RETURNING *", name, typ)
		err = row.Scan(&tag.Id, &tag.Name, &tag.Count, &tag.Type, &tag.Locked)
		if err == sql.ErrNoRows { return nil, nil } // this really shouldn't happen, but just in case.
	}
	if err != nil {
		return nil, err
	}

	ctrl.Transaction.commit = mine
	return &tag, err
}

func GetLastTag(settings UpdaterSettings) (*apitypes.TTagData, error) {
	suf := func(table string) string { return suffix(table, settings.TableSuffix) }

	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return nil, settings.Transaction.err }

	table := "tag_index"

	sq := "SELECT tag_id, tag_name, tag_count, tag_type, tag_type_locked FROM " + suf(table) + " WHERE tag_id = (SELECT MAX(tag_id) FROM tag_index) LIMIT 1"
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
	suf := func(table string) string { return suffix(table, settings.TableSuffix) }

	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

	table := "tag_index"

	_, err := tx.Exec("DELETE FROM " + suf(table) + " WHERE tag_id > 0")

	settings.Transaction.commit = mine
	return err
}

func ClearAliasIndex(settings UpdaterSettings) (error) {
	suf := func(table string) string { return suffix(table, settings.TableSuffix) }

	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

	table := "alias_index"

	_, err := tx.Exec("TRUNCATE " + suf(table))

	settings.Transaction.commit = mine
	return err
}

func ClearPosts(settings UpdaterSettings) (error) {
	suf := func(table string) string { return suffix(table, settings.TableSuffix) }

	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

	table1 := "post_tags"
	table2 := "post_tags_by_name"
	table3 := "post_index"

	log.Println("doing truncate")
	_, err := tx.Exec("TRUNCATE " + suf(table1) + ", " + suf(table2) + ", " + suf(table3))

	log.Println("clear posts")

	settings.Transaction.commit = mine && (err != nil)
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

func AliasUpdater(input chan apitypes.TAliasData, settings UpdaterSettings) (error) {
	defer func(){ for _ = range input {} }()
	suf := func(table string) string { return suffix(table, settings.TableSuffix) }

	table := "alias_index"

	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

	for alias := range input {
		sql := "DELETE FROM " + suf(table) + " WHERE alias_id = $1"
		_, err := tx.Exec(sql, alias.Id)
		if err != nil { return err }

		sql = "INSERT INTO " + suf(table) + " (alias_id, alias_name, alias_target_id) SELECT $1, $2, tag_id FROM tag_index WHERE tag_name = $3"
		_, err = tx.Exec(sql, alias.Id, alias.Name, alias.Alias)
		if err != nil { return err }
	}

	settings.Transaction.commit = mine
	return nil
}

type UpdaterSettings struct {
	Full bool
	Transaction TransactionBox
	TableSuffix string
}

func PostUpdater(input chan apitypes.TPostInfo, settings UpdaterSettings) (error) {
	defer func(){ for _ = range input {} }()
	suf := func(table string) string { return table }

	table1 := "post_tags_by_name"
	table2 := "post_index"

	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

	i := 0
	for post := range input {
		i++
		_, err := tx.Exec("DELETE FROM " + suf(table1) + " WHERE post_id = $1", post.Id)
		if err != nil { return err }
		_, err = tx.Exec("DELETE FROM " + suf(table2) + " WHERE post_id = $1", post.Id)
		if err != nil { return err }

		_, err = tx.Exec("INSERT INTO " + suf(table1) + " (SELECT $1 as post_id, tag_name FROM UNNEST($2::varchar[]) as tag_name) ON CONFLICT DO NOTHING",
				 post.Id, pq.Array(post.Tags()))
		if err != nil { return err }
		_, err = tx.Exec("INSERT INTO " + suf(table2) + " (post_id, post_change_seq, post_rating, post_description, post_sources, post_hash) VALUES ($1, $2, $3, $4, $5, $6)",
				 post.Id, post.Change, post.Rating, post.Description, strings.Join(post.Sources, " "), post.Md5)
		if err != nil { return err }
	}

	settings.Transaction.commit = mine
	log.Println("postupdater normal exit")
	return nil
}

func GetHighestStagedPostID(settings UpdaterSettings) (int) {
	suf := func(table string) string { return suffix(table, settings.TableSuffix) }

	table := "post_tags_by_name"

	row := Db_pool.QueryRow("SELECT MAX(post_id) FROM " + suf(table))
	var result int
	_ = row.Scan(&result)
	return result
}

func TagUpdater(input chan apitypes.TTagData, settings UpdaterSettings) (error) {
	suf := func(table string) string { return suffix(table, settings.TableSuffix) }

	table := "tag_index"

	mine, _ := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

	defer func(){ for _ = range input {} }()

	for tag := range input {
		_, err := settings.Transaction.tx.Exec("DELETE FROM " + suf(table) + " WHERE tag_id = $1", tag.Id)
		if err != nil { return err }

		f := false
		if tag.Locked == nil { tag.Locked = &f }

		_, err = settings.Transaction.tx.Exec("INSERT INTO " + suf(table) + " (tag_id, tag_name, tag_count, tag_type, tag_type_locked) VALUES ($1, $2, $3, $4, $5)", tag.Id, tag.Name, tag.Count, tag.Type, *tag.Locked)
		if err != nil { return err }
	}

	settings.Transaction.commit = mine
	return nil
}

func ResolvePhantomTags(settings UpdaterSettings) (error) {
	suf := func(table string) string { return suffix(table, settings.TableSuffix) }

	table1 := "post_tags"
	table2 := "tag_index"

	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

	// this one merits a tiny bit of explaining
	// fetches the phantom id and the real id using the name and the newest ID of all doubly duplicate tags (by name), then swaps all phantom ids with their associated real id in post_tags.
	_, err := tx.Exec("UPDATE " + suf(table1) + " SET tag_id = map.mapto FROM " + suf(table2) + " INNER JOIN (" +
				"WITH b AS (" +
					"WITH a AS (" +
						"SELECT COUNT(tag_name), tag_name FROM " + suf(table2) + " GROUP BY tag_name" +
					") SELECT tag_name, MAX(tag_id) FROM a INNER JOIN " + suf(table2) + " USING (tag_name) WHERE count = 2 GROUP BY tag_name" +
				") SELECT tag_id, max FROM b INNER JOIN " + suf(table2) + " USING (tag_name) WHERE max > 0 AND tag_id < 0" +
			") AS map(tag_id, mapto) USING (tag_id) WHERE " + suf(table2) + ".tag_id = " + suf(table1) + ".tag_id")
	if err != nil { return err }

	_, err = tx.Exec("DELETE FROM " + suf(table2) + " WHERE tag_id < 0")
	if err != nil { return err }
	_, err = tx.Exec("DELETE FROM " + suf(table1) + " WHERE tag_id < 0")
	if err != nil { return err }

	settings.Transaction.commit = mine
	return nil
}

func FixTagsWhichTheAPIManglesByAccident(settings UpdaterSettings) (error) {
	// what the fuck is this, you may ask?
	// well, it turns out, there are a small number of tags which include unicode characters past
	// codepoint 0xFFFF. apparently ruby on rails (api backend) serializes these badly to json, which
	// cannot escape codepoints higher than that. They just get munged. silently. :|
	// in this function, we manually UPDATE the rows in the tag database that will have these names.
	// thankfully there are only a small number of (known) ones.

	// these are all the ones i've found. there's probably more.
	suf := func(table string) string { return suffix(table, settings.TableSuffix) }

	table := "tag_index"

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

	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

	for k, v := range fix_map {
		query := "UPDATE " + suf(table) + " SET tag_name = $1 WHERE tag_id = $2"
		_, err := tx.Exec(query, v, k)
		if err != nil { return err }
	}

	settings.Transaction.commit = mine
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

func EnumerateCatsExceptions(ctrl EnumerateControl) ([]string, error) {
	mine, tx := ctrl.Transaction.PopulateIfEmpty(Db_pool)
	defer ctrl.Transaction.Finalize(mine)
	if ctrl.Transaction.err != nil { return nil, ctrl.Transaction.err }

	sql := "SELECT tag FROM cats_ignored"
	rows, err := tx.Query(sql)

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

	log.Println("get most recently updated post")

	if err == sql.ErrNoRows {
		log.Println("no rows")
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	log.Println("yes rows")

	settings.Transaction.commit = mine
	return &p, err
}

func ImportPostTagsFromNameToID(settings UpdaterSettings, status chan string) (error) {
	suf := func(table string) string { return suffix(table, settings.TableSuffix) }
	suf2 := func(table, extrasuffix string) string { return suffix(table, settings.TableSuffix) + extrasuffix }

	table1 := "post_tags"
	table2 := "post_tags_by_name"
	table3 := "tag_index"

	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

	var new_count, existing_count int64
	var err error
	if err = tx.QueryRow("SELECT COUNT(*) FROM " + suf(table2)).Scan(&new_count); err != nil { return err }
	if err = tx.QueryRow("SELECT COUNT(*) FROM " + suf(table1)).Scan(&existing_count); err != nil { return err }

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
		status <- " (1/4 tag clear overrides)"
		query = "DELETE FROM " + suf(table1) + " WHERE post_id IN (SELECT DISTINCT post_id FROM " + suf(table2) + ")"
		_, err = tx.Exec(query)
		if err != nil { return err }

		// drop the index and the primary key constraint
		status <- " (2/4 drop indices)"
		query = "DROP INDEX " + suf2(table1, "_tag_id_idx")
		_, err = tx.Exec(query)
		if err != nil { return err }

		query = "ALTER TABLE " + suf(table1) + " DROP CONSTRAINT " + suf2(table1, "_pkey")
		_, err = tx.Exec(query)
		if err != nil { return err }

		// slurp all of the data into the table (very slow if indexes are present, which is why we killed them)
		status <- " (3/4 import data)"
		query = "INSERT INTO " + suf(table1) + " SELECT post_id, tag_id FROM " + suf(table2) + " INNER JOIN " + suf(table3) + " USING (tag_name)"
		_, err = tx.Exec(query)
		if err != nil { return err }

		// add the index and primary key constraint back to the table
		status <- " (4/4 re-index)"
		query = "ALTER TABLE " + suf(table1) + " ADD PRIMARY KEY (post_id, tag_id)"
		_, err = tx.Exec(query)
		if err != nil { return err }

		query = "CREATE INDEX ON " + suf(table1) + " (tag_id)"
		_, err = tx.Exec(query)
		if err != nil { return err }
	} else {
		// if the amount of new data is not large compared to the amount of existing data, just one-by-one plunk them into the table.
		status <- " (1/2 tag clear overrides)"
		query := "DELETE FROM " + suf(table1) + " WHERE post_id IN (SELECT DISTINCT post_id FROM " + suf(table2) + ")"
		_, err = tx.Exec(query)
		if err != nil { return err }

		status <- " (2/2 tag gross-reference)"
		query = "INSERT INTO " + suf(table1) + " SELECT post_id, tag_id FROM " + suf(table2) + " INNER JOIN " + suf(table3) + " USING (tag_name)"
		_, err = tx.Exec(query)
		if err != nil { return err }
	}

	_, err = tx.Exec("TRUNCATE " + suf(table2))
	if err != nil { return err }

	settings.Transaction.commit = mine
	return nil
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

func SetupSyncStagingEnvironment(settings UpdaterSettings) (error) {
	suf := func(table string) string { return suffix(table, settings.TableSuffix) }
	proto := func(table string) string { return prototype(table) }

	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

	tables := []string{
		"post_tags",
		"post_tags_by_name",
		"tag_history_checkpoint",
		"tag_index",
	}

	for _, table := range tables {
		_, err := tx.Exec("CREATE TABLE IF NOT EXISTS " + suf(table) + " (LIKE " + proto(table) + " INCLUDING ALL)")
		if err != nil { return err }
		_, err = tx.Exec("TRUNCATE " + suf(table))
		if err != nil { return err }
	}

	settings.Transaction.commit = mine
	return nil
}

func PromoteStagingEnvironment(settings UpdaterSettings) (error) {
	// if we aren't using a staging environment, then we're already done, do nothing further.
	if settings.TableSuffix == "" {
		return nil
	}

	suf := func(table string) string { return suffix(table, settings.TableSuffix) }
	suf2 := func(table, extrasuffix string) string { return suffix(table, settings.TableSuffix) + extrasuffix }

	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

	tables := []string{
		"post_tags",
		"post_tags_by_name",
		"tag_history_checkpoint",
		"tag_index",
	}

	for _, table := range tables {
		_, err := tx.Exec("DROP TABLE IF EXISTS " + table)
		if err != nil { return err }
		_, err = tx.Exec("ALTER TABLE " + suf(table) + " RENAME TO " + table)
		if err != nil { return err }
	}

	_, err := tx.Exec("ALTER INDEX " + suf2("post_tags", "_pkey") + " RENAME TO post_tags_pkey")
	if err != nil { return err }
	_, err = tx.Exec("ALTER INDEX " + suf2("post_tags", "_tag_id_idx") + " RENAME TO post_tags_tag_id_idx")
	if err != nil { return err }

	settings.Transaction.commit = mine
	return nil
}

func ResetIntermediateEnvironment(settings UpdaterSettings) (error) {
	suf := func(table string) string { return suffix(table, settings.TableSuffix) }
	proto := func(table string) string { return prototype(table) }

	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

	tables := []string{
		"post_tags_by_name",
	}

	for _, table := range tables {
		_, err := tx.Exec("CREATE TABLE IF NOT EXISTS " + suf(table) + " (LIKE " + proto(table) + " INCLUDING ALL)")
		if err != nil { return err }
		_, err = tx.Exec("TRUNCATE " + suf(table))
		if err != nil { return err }
	}

	settings.Transaction.commit = mine
	return nil
}

func CountTags(settings UpdaterSettings, status chan string) (error) {
	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

	status <- " (1/2 reset cached counts)"
	query := "UPDATE tag_index SET tag_count = 0"
	_, err := tx.Exec(query)
	if err != nil { return err }

	status <- " (2/2 calculate actual tag counts)"
	query = "WITH subq AS (SELECT tag_id, COUNT(tag_id) AS real_count FROM post_tags GROUP BY tag_id) UPDATE tag_index SET tag_count = subq.real_count FROM subq WHERE subq.tag_id = tag_index.tag_id"
	_, err = tx.Exec(query)
	if err != nil { return err }

	settings.Transaction.commit = mine
	return nil
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

func UpdatePost(oldpost, newpost apitypes.TPostInfo, settings UpdaterSettings) (error) {
	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

	count_deltas := make(map[string]int)
	for _, new_tag := range newpost.Tags() {
		count_deltas[new_tag] += 1
	}
	for _, old_tag := range oldpost.Tags() {
		count_deltas[old_tag] -= 1
	}

	for k, v := range count_deltas {
		if v == 0 { continue }
		query := "UPDATE tag_index SET tag_count = tag_count + $2 WHERE tag_name = $1"
		_, err := tx.Exec(query, k, v)
		if err != nil { return err }
	}

	query := "DELETE FROM post_tags WHERE post_id = $1"
	_, err := tx.Exec(query, oldpost.Id)
	if err != nil { return err }

	query = "INSERT INTO post_tags SELECT $1 as post_id, tag_id FROM UNNEST($2::varchar[]) AS tag_name INNER JOIN tag_index USING (tag_name)"
	_, err = tx.Exec(query, oldpost.Id, pq.Array(newpost.Tags()))
	if err != nil { return err }

	settings.Transaction.commit = mine
	return nil
}

type BlitData struct {
	apitypes.TTagData

	Valid bool
}

func GetMarkedAndUnmarkedBlits(ctrl EnumerateControl) ([]BlitData, error) {
	mine, tx := ctrl.Transaction.PopulateIfEmpty(Db_pool)
	defer ctrl.Transaction.Finalize(mine)
	if ctrl.Transaction.err != nil { return nil, ctrl.Transaction.err }

	var blit BlitData
	var out []BlitData
	rows, _ := tx.Query("SELECT is_blit, tag_id, tag_name, tag_count, tag_type, tag_type_locked FROM blit_tag_registry INNER JOIN tag_index USING (tag_id) ORDER BY NOT is_blit, tag_name")
	for rows.Next() {
		err := rows.Scan(&blit.Valid, &blit.Id, &blit.Name, &blit.Count, &blit.Type, &blit.Locked)
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

/*
func BigSearchPaginated(tag_expression string, last_post_id int64) {
    expression_tokens := tagindex.Tokenize(tag_expression)
    search_expression := tagindex.Parse(expression_tokens)
    sql_format_substring, sql_tokens := tagindex.Serialize(search_expression)
    for k, v := range sql_tokens {
        sql_tokens[k] = pq.QuoteLiteral(v)
    }
    replace["tag_id"] = "tag_id"
    replace["tag_index"] = "tag_index"
    replace["tag_name"] = "tag_name"
    replace["temp"] = "x"
    var buf bytes.Buffer
    template.Must(template.New("decoder").Parse(sql_format_substring)).Execute(&buf, sql_tokens)
    sql_substring := buf.String()
    sql_format_string := `
        SELECT 
          post_id
        FROM (
            SELECT 
              post_id,
              (
                  SELECT (
                      WITH x AS (
                          SELECT 
                            tag_id
                          FROM post_tags
                          WHERE post_id = x.post_id
                      ) (
                          SELECT %s
                      )
                  )
              ) AS present
            FROM post_tags AS x
            GROUP BY post_id
            ORDER BY post_id
        ) AS _
        WHERE
          present AND
          post_id > $1 limit 320`
    sql_string := fmt.Sprintf(sql_format_string, sql_substring)

	tx, err := Db_pool.Begin()
	if err != nil { return err }

	var c committer
	defer handle_transaction(&c, tx)

    err = tx.Exec("SET LOCAL statement_timeout to 20000")
    // TODO finish this
    rows, err := tx.Query(sql_string, last_post_id)
}

*/
