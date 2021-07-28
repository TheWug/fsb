package types

import (
	"github.com/thewug/fsb/pkg/api/tags"

	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/lib/pq"
	"github.com/thewug/dml"
)

type TTagData struct {
	Id int `json:"id" dml:"tag_id"`
	Name string `json:"name" dml:"tag_name"`
	Count int `json:"post_count" dml:"tag_count"`
	FullCount int `dml:"tag_count_full"`// this field is only present in the local DB
	Type TagCategory `json:"category" dml:"tag_type"`
	Locked *bool `json:"is_locked" dml:"tag_type_locked"`

	// created_at
	// updated_at
	// related_tags
	// related_tags_updated_at
}

func (this TTagData) ApparentCount(include_deleted bool) int {
	if include_deleted {
		return this.FullCount
	} else {
		return this.Count
	}
}

func (tag *TTagData) ScanFrom(row Scannable) error {
	return row.Scan(&tag.Id, &tag.Name, &tag.Count, &tag.FullCount, &tag.Type, &tag.Locked)
}

type TTagInfoArray []TTagData

type TTagListing struct {
	Tags TTagInfoArray `json:"tags"`
}

func (this *TTagListing) UnmarshalJSON(b []byte) (error) {

	type TTagListingAlt TTagListing
	var temp TTagListingAlt
	err1 := json.Unmarshal(b, &temp)
	if err1 == nil {
		*this = TTagListing(temp)
		return nil
	}
	err2 := json.Unmarshal(b, &this.Tags)
	if err2 == nil {
		return nil
	}
	return errors.New(fmt.Sprintf("Couldn't figure out how to parse json response (%s) (%s)", err1.Error(), err2.Error()))
}

type TAliasData struct {
	Id int       `json:"id"`
	Name string  `json:"consequent_name"`
	Alias string `json:"antecedent_name"`

	// reason
	// creator_id
	// created_at
	// updated_at
	// forum_post_id
	// forum_topic_id
}

type TAliasInfoArray []TAliasData

type TAliasListing struct {
	Aliases TAliasInfoArray `json:"tag_aliases"`
}

func (this *TAliasListing) UnmarshalJSON(b []byte) (error) {

	type TAliasListingAlt TAliasListing
	var temp TAliasListingAlt
	err1 := json.Unmarshal(b, &temp)
	if err1 == nil {
		*this = TAliasListing(temp)
		return nil
	}
	err2 := json.Unmarshal(b, &this.Aliases)
	if err2 == nil {
		return nil
	}
	return errors.New(fmt.Sprintf("Couldn't figure out how to parse json response (%s) (%s)", err1.Error(), err2.Error()))
}

type TTagHistory struct {
	Id int `json:"id"`
	Post_id int `json:"post_id"`
	Tags string `json:"tags"`
	// there's other fields, but I don't care about them and all they'll do is waste memory.
}

type THistoryArray []TTagHistory

type TPostScore struct {
	Upvotes   int      `json:"up"`
	Downvotes int      `json:"down"`
	Score     int      `json:"total"`
	OurScore  PostVote `json:"our_score"` // only shown when actually voting
}

type TPostFile struct {
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	File_ext  string `json:"ext"`
	File_size int    `json:"size"`
	File_url  string `json:"url"`
	Md5       string `json:"md5"`
}

type TPostPreview struct {
	Preview_width  int    `json:"width"`
	Preview_height int    `json:"height"`
	Preview_url    string `json:"url"`
}

type TPostSample struct {
	Sample_width  int    `json:"width"`
	Sample_height int    `json:"height"`
	Sample_url    string `json:"url"`
	Has_sample    bool   `json:"has"`
}

type TPostFlags struct {
	Pending       bool `json:"pending"`
	Flagged       bool `json:"flagged"`
	Locked_notes  bool `json:"note_locked"`
	Locked_status bool `json:"status_locked"`
	Locked_rating bool `json:"rating_locked"`
	Deleted       bool `json:"deleted"`
}

type TPostRelationships struct {
	Parent_id           int  `json:"parent_id,omitempty"`
	Has_children        bool `json:"has_children"`
	Has_active_children bool `json:"has_active_children"`
	Children          []int  `json:"children,omitempty"`
}

type TPostTags struct {
	General   []string `json:"general"`
	Species   []string `json:"species"`
	Character []string `json:"character"`
	Copyright []string `json:"copyright"`
	Artist    []string `json:"artist"`
	Invalid   []string `json:"invalid"`
	Lore      []string `json:"lore"`
	Meta      []string `json:"meta"`
}

type TPostInfo struct {
	// anonymous nested subcomponents
	TPostScore         `json:"score"`
	TPostFile          `json:"file"`
	TPostPreview       `json:"preview"`
	TPostSample        `json:"sample"`
	TPostFlags         `json:"flags"`
	TPostRelationships `json:"relationships"`
	TPostTags          `json:"tags"`

	Id            int    `json:"id"`
	Description   string `json:"description"`
	Creator_id    int    `json:"uploader_id"`
	Change        int    `json:"change_seq"`
	Fav_count     int    `json:"fav_count"`
	Rating        string `json:"rating"`
	Comment_count int    `json:"comment_count"`
	Sources     []string `json:"sources,omitempty"`

	sources_internal string

//	Created_at    JSONTime `json:"created_at"`
//	Updated_at    JSONTime `json:"updated_at"`
//	Author        string `json:"author"`
//	Has_notes     bool `json:"has_notes"`
//	Artist      []string `json:"artist,omitempty"`
}

// the edit post endpoint returns this piece of shit instead of a real TPostInfo object. bodge it in and add a converter.
type TPostEditInfo struct {
	Id          int    `json:"id"`
	Change      int    `json:"change_seq"`
	Rating      string `json:"rating"`
	Description string `json:"description"`
	Source      string `json:"source"`
	Md5         string `json:"md5"`
	Deleted     bool   `json:"is_deleted"`

	TagStringGeneral   string `json:"tag_string_general"`
	TagStringSpecies   string `json:"tag_string_species"`
	TagStringCharacter string `json:"tag_string_character"`
	TagStringCopyright string `json:"tag_string_copyright"`
	TagStringArtist    string `json:"tag_string_artist"`
	TagStringInvalid   string `json:"tag_string_invalid"`
	TagStringLore      string `json:"tag_string_lore"`
	TagStringMeta      string `json:"tag_string_meta"`
}

func (this TPostEditInfo) TPostInfo() (*TPostInfo) {
	return &TPostInfo{
		Id: this.Id,
		Change: this.Change,
		Rating: this.Rating,
		Description: this.Description,
		Sources: strings.Split(this.Source, "\n"),
		TPostFile: TPostFile{
			Md5: this.Md5,
		},
		TPostFlags: TPostFlags{
			Deleted: this.Deleted,
		},
		TPostTags: TPostTags{
			General: strings.Split(this.TagStringGeneral, " "),
			Species: strings.Split(this.TagStringSpecies, " "),
			Character: strings.Split(this.TagStringCharacter, " "),
			Copyright: strings.Split(this.TagStringCopyright, " "),
			Artist: strings.Split(this.TagStringArtist, " "),
			Invalid: strings.Split(this.TagStringInvalid, " "),
			Lore: strings.Split(this.TagStringLore, " "),
			Meta: strings.Split(this.TagStringMeta, " "),
		},
	}
}

type TUserInfo struct {
	Id              int    `json:"id"`
	CreatedAt       string `json:"created_at"`
	Name            string `json:"name"`
	Level           int    `json:"level"`
	BaseUploadLimit int    `json:"base_upload_limit"`
	PostUploadCount int    `json:"post_upload_count"`
	PostUpdateCount int    `json:"post_update_count"`
	NoteUpdateCount int    `json:"note_update_count"`
	IsBanned        bool   `json:"is_banned"`
	CanApprovePosts bool   `json:"can_approve_posts"`
	CanUploadFree   bool   `json:"can_upload_free"`
	LevelString     string `json:"level_string"`
	Email           string `json:"email"` // only present when logged in, and only for your own account
	Blacklist       string `json:"blacklisted_tags"`
}

func (this *TPostInfo) GetFields() (dml.NamedFields, error) {
	return dml.NamedFields{
		Names: []string{
			"post_id", "post_change_seq", "post_rating", "post_description", "post_sources", "post_hash", "post_deleted", "post_tags",
		},
		Fields: []interface{}{
			&this.Id, &this.Change, &this.Rating, &this.Description, &this.sources_internal, &this.Md5, &this.Deleted, pq.Array(&this.General),
		},
	}, nil
}

func (this *TPostInfo) PostScan() error {
	this.Sources = strings.Split(this.sources_internal, "\n")
	this.sources_internal = ""
	return nil
}

type Scannable interface {
	Scan(...interface{}) error
}

func (this *TPostInfo) ScanFrom(rows Scannable) error {
	var sources string
	err := rows.Scan(&this.Id, &this.Change, &this.Rating, &this.Description, &sources, &this.Md5, &this.Deleted, pq.Array(&this.General))
	if err != nil { return err }
	this.Sources = strings.Split(sources, "\n")
	return nil
}

func (this *TPostInfo) Tags() ([]string) {
	var tags []string
	tags = append(tags, this.General...)
	tags = append(tags, this.Species...)
	tags = append(tags, this.Character...)
	tags = append(tags, this.Copyright...)
	tags = append(tags, this.Artist...)
	tags = append(tags, this.Invalid...)
	tags = append(tags, this.Lore...)
	tags = append(tags, this.Meta...)
	return tags
}

func (this *TPostInfo) TagSet() (tags.TagSet) {
	var t tags.TagSet
	t.ApplyArray(this.General)
	t.ApplyArray(this.Species)
	t.ApplyArray(this.Character)
	t.ApplyArray(this.Copyright)
	t.ApplyArray(this.Artist)
	t.ApplyArray(this.Invalid)
	t.ApplyArray(this.Lore)
	t.ApplyArray(this.Meta)
	return t
}

func (this *TPostInfo) ExtendedTagSet() (tags.TagSet) {
	t := this.TagSet()
	t.Set(fmt.Sprintf("rating:%s", this.Rating))
	t.Set(fmt.Sprintf("type:%s", this.File_ext))
	if this.Parent_id != 0 {
		t.Set(fmt.Sprintf("parent:%d", this.Parent_id))
	}
	return t
}

func matchIntRange(tag string, against int) bool {
	if strings.HasPrefix(tag, ">=") {
		i, err := strconv.Atoi(tag[2:])
		return err == nil && i >= against
	}
	if strings.HasPrefix(tag, "<=") {
		i, err := strconv.Atoi(tag[2:])
		return err == nil && i <= against
	}
	if strings.HasPrefix(tag, ">") {
		i, err := strconv.Atoi(tag[1:])
		return err == nil && i > against
	}
	if strings.HasPrefix(tag, "<") {
		i, err := strconv.Atoi(tag[1:])
		return err == nil && i < against
	}
	if strings.Contains(tag, "..") {
		ids := strings.Split(tag, "..")
		bottom, err1 := strconv.Atoi(ids[0])
		top, err2 := strconv.Atoi(ids[1])
		return err1 == nil && err2 == nil && against >= bottom && against <= top
	} else {
		i, err := strconv.Atoi(tag)
		return err == nil && i == against
	}
	return false
}

func matchesTag(post *TPostInfo, t *tags.TagSet, tag string) bool {
	tag_noprefix := strings.TrimPrefix(tag, "-")
	tag, positive_match := tag_noprefix, tag == tag_noprefix
	matches := false

	if trimmed := strings.TrimPrefix(tag, "rating:"); trimmed != tag {
		if strings.HasPrefix(trimmed, "s") {
			matches = strings.HasPrefix(post.Rating, "s")
		} else if strings.HasPrefix(trimmed, "q") {
			matches = strings.HasPrefix(post.Rating, "q")
		} else if strings.HasPrefix(trimmed, "e") {
			matches = strings.HasPrefix(post.Rating, "e")
		} else {
			matches = false
		}
	} else if trimmed := strings.TrimPrefix(tag, "id:"); trimmed != tag {
		matches = matchIntRange(trimmed, post.Id)
	} else if trimmed := strings.TrimPrefix(tag, "score:"); trimmed != tag {
		matches = matchIntRange(trimmed, post.Score)
	} else if trimmed := strings.TrimPrefix(tag, "favcount:"); trimmed != tag {
		matches = matchIntRange(trimmed, post.Fav_count)
	} else {
		matches = t.Status(tag) == tags.AddsTag
	}

	return matches == positive_match
}

func matchesBlacklistLine(post *TPostInfo, tags *tags.TagSet, line string) bool {
	no_or_tags := true
	cumulative_or_tags := false
	cumulative_and_tags := true

	for _, tag := range strings.Split(line, " ") {
		if tag == "" {
			continue
		} else if strings.HasPrefix(tag, "~") {
			no_or_tags = false
			cumulative_or_tags = cumulative_or_tags || matchesTag(post, tags, tag[1:])
		} else {
			cumulative_and_tags = cumulative_and_tags && matchesTag(post, tags, tag)
		}
	}

	return (no_or_tags || cumulative_or_tags) && cumulative_and_tags
}

func (this *TPostInfo) MatchesBlacklist(blacklist string) (bool) {
	tags := this.TagSet()
	var lines []string
	if len(blacklist) != 0 { lines = strings.Split(blacklist, "\n") }
	for _, line := range lines {
		if matchesBlacklistLine(this, &tags, line) { return true }
	}
	return false
}

type TUserInfoArray []TUserInfo

type TPostInfoArray []TPostInfo

type TApiStatus struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Reason  string `json:"reason"`
	Code   *string `json:"code"`
}

type TPostListing struct {
	Posts TPostInfoArray `json:"posts"`
}

func (this *TPostListing) UnmarshalJSON(b []byte) (error) {

	type TPostListingAlt TPostListing
	var temp TPostListingAlt
	err1 := json.Unmarshal(b, &temp)
	if err1 == nil && len(temp.Posts) != 0 {
		*this = TPostListing(temp)
		return nil
	}
	err2 := json.Unmarshal(b, &this.Posts)
	if err2 == nil {
		return nil
	}
	return errors.New(fmt.Sprintf("Couldn't figure out how to parse json response (%s) (%s)", err1.Error(), err2.Error()))
}
