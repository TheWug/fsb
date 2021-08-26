package types

import (
	"encoding/json"
	"errors"
	"fmt"
)

type TTagData struct {
	Id int `json:"id"`
	Name string `json:"name"`
	Count int `json:"post_count"`
	Type TagCategory `json:"category"`
	Locked *bool `json:"is_locked"`

	// created_at
	// updated_at
	// related_tags
	// related_tags_updated_at
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
	Upvotes   int `json:"up"`
	Downvotes int `json:"down"`
	Score     int `json:"total"`
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

type TPostInfoArray []TPostInfo


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
