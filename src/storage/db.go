package storage

import (
	"api/tags"
	apitypes "api/types"

	_ "github.com/lib/pq"
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

func ClearAliasIndex(settings UpdaterSettings) (error) {
	mine, tx := settings.Transaction.PopulateIfEmpty(Db_pool)
	defer settings.Transaction.Finalize(mine)
	if settings.Transaction.err != nil { return settings.Transaction.err }

	_, err := tx.Exec("TRUNCATE alias_index")

	settings.Transaction.commit = mine
	return err
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

type PostSuggestedEdit struct {
	Prompt        tags.TagDiffArray      `json:"prompt"`
	AutoFix       tags.TagDiffArray      `json:"autofix"`
	SelectedEdits map[string]bool        `json:"selected_edits"`
	AppliedEdits  map[string]bool        `json:"applied_edits"`
	Represents  []int64                  `json:"represents"`
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

