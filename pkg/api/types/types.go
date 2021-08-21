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

type PostRating string
const (
	Explicit     PostRating = "e"
	Questionable PostRating = "q"
	Safe         PostRating = "s"
	Original     PostRating = ""
	Invalid      PostRating = "invalid"
)

func (r PostRating) String() string {
	if r == Safe { return "Safe" }
	if r == Questionable { return "Questionable" }
	if r == Explicit { return "Explicit" }
	return "Unknown"
}

// attempts to find a "rating:*" tag and interpret it, returning a rating string if it does and returning nothing if there isn't one.
func RatingFromTagSet(ts tags.TagSet) (PostRating) {
	var s, q, e bool
	for t, _ := range ts.Data {
		t = strings.ToLower(t)
		s = s || strings.HasPrefix(t, "rating:s")
		q = q || strings.HasPrefix(t, "rating:q")
		e = e || strings.HasPrefix(t, "rating:e")
	}
	if e { return Explicit }
	if q { return Questionable }
	if s { return Safe }
	return Original
}

type TTagData struct {
	Id        int         `json:"id"         dml:"tag_id"`
	Name      string      `json:"name"       dml:"tag_name"`
	Count     int         `json:"post_count" dml:"tag_count"`
	FullCount int                           `dml:"tag_count_full"`// this field is only present in the local DB
	Type      TagCategory `json:"category"   dml:"tag_type"`
	Locked   *bool        `json:"is_locked"  dml:"tag_type_locked"`

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

type TTagInfoArray []TTagData

type TTagListing struct {
	Tags TTagInfoArray
}

func (this *TTagListing) UnmarshalJSON(b []byte) (error) {
	var x struct {
		Tags *TTagInfoArray `json:"tags"`
	}

	err := multi_format_unmarshal(b, func() bool {
		return x.Tags != nil
	}, func() { x.Tags = nil }, &x, &x.Tags)
	if x.Tags != nil { this.Tags = *x.Tags }
	return err
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
	Aliases TAliasInfoArray
}

type alias_listing struct {
	Aliases *TAliasInfoArray `json:"tag_aliases"`
}

func (this *TAliasListing) UnmarshalJSON(b []byte) (error) {
	var x struct {
		Aliases *TAliasInfoArray `json:"tag_aliases"`
	}

	err := multi_format_unmarshal(b, func() bool {
		return x.Aliases != nil
	}, func() { x.Aliases = nil }, &x, &x.Aliases)
	if x.Aliases != nil { this.Aliases = *x.Aliases }
	return err
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

	Id            int        `json:"id"`
	Description   string     `json:"description"`
	Creator_id    int        `json:"uploader_id"`
	Change        int        `json:"change_seq"`
	Fav_count     int        `json:"fav_count"`
	Rating        PostRating `json:"rating"`
	Comment_count int        `json:"comment_count"`
	Sources     []string     `json:"sources,omitempty"`

	sources_internal string

//	Created_at    JSONTime `json:"created_at"`
//	Updated_at    JSONTime `json:"updated_at"`
//	Author        string `json:"author"`
//	Has_notes     bool `json:"has_notes"`
//	Artist      []string `json:"artist,omitempty"`
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
		matches = strings.HasPrefix(trimmed, "s") && post.Rating == Safe ||
		          strings.HasPrefix(trimmed, "q") && post.Rating == Questionable ||
		          strings.HasPrefix(trimmed, "e") && post.Rating == Explicit
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
	Posts TPostInfoArray
}

func (this *TPostListing) UnmarshalJSON(b []byte) (error) {
	var x struct {
		Posts *TPostInfoArray `json:"posts"`
	}

	err := multi_format_unmarshal(b, func() bool {
		return x.Posts != nil
	}, func() { x.Posts = nil }, &x, &x.Posts)
	if x.Posts != nil { this.Posts = *x.Posts }
	return err
}

type TSinglePostListing struct {
	Post TPostInfo
}

func (this *TSinglePostListing) UnmarshalJSON(b []byte) (error) {
	var x struct {
		Post *TPostInfo `json:"post"`
	}

	err := multi_format_unmarshal(b, func() bool {
		return x.Post != nil && x.Post.Id != 0
	}, func() { x.Post = nil }, &x, &x.Post)
	if x.Post != nil { this.Post = *x.Post }
	return err
}

// This function attempts to unmartial a single object in two different ways.
// - j should be the json byte array received from the json package.
// - good should be a function which returns true if the output object has been read successfully.
// - reset should be a function which returns the output to its unset state.
// - objects should contain addresses of objects suitable for passing to json.Unmarshal
//   (in other words, they must be pointers).
// It returns nil if any object successfully captures the provided json, or an error summarizing
// all of the sub-errors which occurred while trying to unmarshal the provided json.
//
// objects are considered in order, from first to last. If unmarshaling any of them succeeds,
// continued unmarshalling is halted.
//
// this function has no error protection, and will misbehave if misused,
// including in the following circumstances:
// - objects has length 0
// - an object has a non-pointer type
// - good() returns true in the state in which function is called
// - reset() returns the system to a state in which good() returns true
func multi_format_unmarshal(j []byte, good func() bool, reset func(), objects ...interface{}) (error) {
	var err error
	var errs []error
	for _, obj := range objects {
		err := json.Unmarshal(j, obj)
		if err == nil && good() { break }
		if err != nil {
			errs = append(errs, err)
			reset()
		}
	}

	if !good() {
		var errstrings []string
		for _, e := range errs { errstrings = append(errstrings, e.Error()) }
		err = errors.New(fmt.Sprintf("Couldn't figure out how to parse json response (%v)", strings.Join(errstrings, ", ")))
	}

	return err
}
