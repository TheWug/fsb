package types

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

type TagCategory int
func (this TagCategory) Value() int { return int(this) }
const TCGeneral     TagCategory = 0
const TCArtist      TagCategory = 1
const TCCopyright   TagCategory = 3
const TCCharacter   TagCategory = 4
const TCSpecies     TagCategory = 5
const TCInvalid     TagCategory = 6
const TCMeta        TagCategory = 7
const TCLore        TagCategory = 8

type TagSearchOrder string
func (this TagSearchOrder) String() string { return string(this) }
const TSONewest TagSearchOrder = "date" // default
const TSOCount  TagSearchOrder = "count"
const TSOName   TagSearchOrder = "name"

type AliasSearchOrder string
func (this AliasSearchOrder) String() string { return string(this) }
const ASOStatus  AliasSearchOrder = "status"
const ASOCreated AliasSearchOrder = "created_at"
const ASOUpdated AliasSearchOrder = "updated_at"
const ASOName    AliasSearchOrder = "name"
const ASOCount   AliasSearchOrder = "tag_count"

type AliasStatus string
func (this AliasStatus) String() string { return string(this) }
const ASApproved   AliasStatus = "Approved"
const ASActive     AliasStatus = "Active"
const ASPending    AliasStatus = "Pending"
const ASDeleted    AliasStatus = "Deleted"
const ASRetired    AliasStatus = "Retired"
const ASProcessing AliasStatus = "Processing"
const ASQueued     AliasStatus = "Queued"

type PostVote int
func (this PostVote) Value() int { return int(this) }
const Upvote   PostVote = 1
const Downvote PostVote = -1
const Neutral  PostVote = 0 // this can show up in API responses but you can't vote by specifying it, if you want to delete your vote, use the endpoint for that

type TagDiffMembership int
const AddsTag TagDiffMembership = 1
const ResetsTag TagDiffMembership = 0
const RemovesTag TagDiffMembership = -1

type PageSelector struct {
	Before *int
	After  *int
	Page   *int
}

func Before(i int) PageSelector {
	return PageSelector{Before: &i}
}

func After(i int) PageSelector {
	return PageSelector{After: &i}
}

func Page(i int) PageSelector {
	return PageSelector{Page: &i}
}

func PostsAfterChangeSeq(change int) (string) {
	return fmt.Sprintf("status:any order:change_asc change:>%d", change)
}

func DeletedPostsAfterId(id int) (string) {
	return fmt.Sprintf("status:deleted order:id_asc id:>%d", id)
}

func (this PageSelector) String() string {
	if this.After != nil {
		return fmt.Sprintf("a%d", *this.After)
	} else if this.Before != nil {
		return fmt.Sprintf("b%d", *this.Before)
	} else if this.Page != nil {
		return fmt.Sprintf("%d", *this.Page)
	}
	return ""
}

type ListTagsOptions struct {
	Page       PageSelector
	Limit      int
	MatchTags  string
	Category  *TagCategory
	Order      TagSearchOrder
	HideEmpty  bool
	HasWiki   *bool
	HasArtist *bool
}

type ListTagAliasOptions struct {
	Page                PageSelector
	Limit               int
	MatchAliases        string
//	AntecedentCategory *TagCategory
//	ConsequentCategory *TagCategory // disabled right now due to unexpected behavior
	Status              AliasStatus
	Order               AliasSearchOrder
}

type ListPostOptions struct {
	Page        PageSelector
	Limit       int
	SearchQuery string
}

type TagDiff struct {
	Add    map[string]bool `json:"add"`
	Remove map[string]bool `json:"remove"`
}

func (this *TagDiff) TagStatus(tag string) TagDiffMembership {
	if _, ok := this.Add[tag]; ok {
		return AddsTag
	} else if _, ok := this.Remove[tag]; ok {
		return RemovesTag
	}

	return ResetsTag
}

func (this *TagDiff) AddTag(tag string) {
	if tag == "" { return }

	if this.Add == nil {
		this.Add = make(map[string]bool)
	}

	this.Add[tag] = true
	delete(this.Remove, tag)
}

func (this *TagDiff) RemoveTag(tag string) {
	if tag == "" { return }

	if this.Remove == nil {
		this.Remove = make(map[string]bool)
	}

	this.Remove[tag] = true
	delete(this.Add, tag)
}

func (this *TagDiff) ResetTag(tag string) {
	delete(this.Add, tag)
	delete(this.Remove, tag)
}

func (this *TagDiff) ApplyString(tag_diff string) {
	this.ApplyArray(strings.Split(tag_diff, " "))
}

func (this *TagDiff) ApplyArray(tag_diff []string) {
	for _, tag := range tag_diff {
		if strings.HasPrefix(tag, "-") {
			this.RemoveTag(strings.TrimPrefix(tag, "-"))
		} else if strings.HasPrefix(tag, "+") {
			this.AddTag(strings.TrimPrefix(tag, "+"))
		} else if strings.HasPrefix(tag, "=") {
			this.ResetTag(strings.TrimPrefix(tag, "="))
		} else {
			this.AddTag(tag)
		}
	}
}

func (this *TagDiff) ApplyStrings(add_tags, remove_tags, reset_tags string) {
	this.ApplyArrays(strings.Split(add_tags, " "), strings.Split(remove_tags, " "), strings.Split(reset_tags, " "))
}

func (this *TagDiff) ApplyArrays(add_tags, remove_tags, reset_tags []string) {
	for _, tag := range add_tags {
		this.AddTag(strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(tag, "="), "+"), "-"))
	}

	for _, tag := range remove_tags {
		this.RemoveTag(strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(tag, "="), "+"), "-"))
	}

	for _, tag := range reset_tags {
		this.ResetTag(strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(tag, "="), "+"), "-"))
	}
}

func (this TagDiff) IsZero() bool {
	return len(this.Add) == 0 && len(this.Remove) == 0
}

func TagDiffFromString(tag_diff string) (TagDiff) {
	return TagDiffFromArray(strings.Split(tag_diff, " "))
}

func TagDiffFromStrings(add_tags, remove_tags, reset_tags string) (TagDiff) {
	return TagDiffFromArrays(strings.Split(add_tags, " "), strings.Split(remove_tags, " "), strings.Split(reset_tags, " "))
}

func TagDiffFromArray(tag_diff []string) (TagDiff) {
	var diff TagDiff
	diff.ApplyArray(tag_diff)
	return diff
}

func TagDiffFromArrays(add_tags, remove_tags, reset_tags []string) (TagDiff) {
	var diff TagDiff
	diff.ApplyArrays(add_tags, remove_tags, reset_tags)
	return diff
}

// the output of this function is stable and can be used to compare TagDiff objects
func (this TagDiff) APIString() string {
	var buf bytes.Buffer
	keys := make([]string, 0, len(this.Add))
	for k, v := range this.Add { if v { keys = append(keys, k) } }
	sort.Slice(keys, func(i, j int) bool {return keys[i] < keys[j]})
	for _, k := range keys {
		if buf.Len() != 0 {
			buf.WriteString(" ")
		}
		buf.WriteString(k)
	}
	keys = make([]string, 0, len(this.Add))
	for k, v := range this.Remove { if v { keys = append(keys, k) } }
	sort.Slice(keys, func(i, j int) bool {return keys[i] < keys[j]})
	for _, k := range keys {
		if buf.Len() != 0 {
			buf.WriteString(" ")
		}
		buf.WriteRune('-')
		buf.WriteString(k)
	}
	return buf.String()
}

func (this TagDiff) String() string {
	return this.APIString()
}

func (this TagDiff) Difference(other TagDiff) TagDiff {
	var n TagDiff
	for a, v := range this.Add { if v { n.AddTag(a) } }
	for a, v := range other.Add { if v { delete(n.Add, a) } }
	for r, v := range this.Remove { if v { n.RemoveTag(r) } }
	for r, v := range other.Remove { if v { delete(n.Remove, r) } }
	return n
}

func (this TagDiff) Invert() TagDiff {
	var n TagDiff
	for a, v := range this.Remove { if v { n.AddTag(a) } }
	for r, v := range this.Add { if v { n.RemoveTag(r) } }
	return n
}

func (this TagDiff) Union(other TagDiff) TagDiff {
	var n TagDiff
	for a, v := range this.Add { if v { n.AddTag(a) } }
	for a, v := range other.Add { if v { n.AddTag(a) } }
	for r, v := range this.Remove { if v { n.RemoveTag(r) } }
	for r, v := range other.Remove { if v { n.RemoveTag(r) } }
	return n
}

type TagDiffArray []TagDiff

func (this TagDiffArray) Flatten() TagDiff {
	var n TagDiff
	for _, other := range this {
		for a, v := range other.Add { if v { n.AddTag(a) } }
		for r, v := range other.Remove { if v { n.RemoveTag(r) } }
	}
	return n
}
