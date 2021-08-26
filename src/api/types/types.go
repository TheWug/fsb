package types

import (
	"encoding/json"
	"errors"
	"strings"
	"fmt"
)

type TTagData struct {
	Id int `json:"id"`
	Name string `json:"name"`
	Count int `json:"post_count"`
	FullCount int // this field is only present in the local DB
	Type TagCategory `json:"category"`
	Locked *bool `json:"is_locked"`

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

	Id            int `json:"id"`
	Description   string `json:"description"`
	Creator_id    int `json:"uploader_id"`
	Change        int `json:"change_seq"`
	Fav_count     int `json:"fav_count"`
	Rating        string `json:"rating"`
	Comment_count int `json:"comment_count"`
	Sources     []string `json:"sources,omitempty"`

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
	if err1 == nil {
		*this = TPostListing(temp)
		return nil
	}
	err2 := json.Unmarshal(b, &this.Posts)
	if err2 == nil {
		return nil
	}
	return errors.New(fmt.Sprintf("Couldn't figure out how to parse json response (%s) (%s)", err1.Error(), err2.Error()))
}
