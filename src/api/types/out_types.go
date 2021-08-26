package types

import (
	"fmt"
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
