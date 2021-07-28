package tags

import (
	"reflect"
)

type TagSet struct {
	StringSet
}

const tagDelimiter string = " "

func (this TagSet) Equal(other TagSet) bool {
	return len(this.Data) == 0 && len(other.Data) == 0 || reflect.DeepEqual(this.Data, other.Data)
}

func (this *TagSet) ApplyString(tags string) {
	this.ApplyStringWithDelimiter(tags, tagDelimiter)
}

func (this *TagSet) ToggleString(tags string) {
	this.ToggleStringWithDelimiter(tags, tagDelimiter)
}

func (this TagSet) String() (string) {
	return this.StringWithDelimiter(tagDelimiter)
}

func (this TagSet) Clone() (TagSet) {
	return TagSet{this.StringSet.Clone()}
}

// apply a tag diff to the tag set.
func (this *TagSet) ApplyDiff(diff TagDiff) {
	this.StringSet.ApplyDiff(diff.StringDiff)
}

func (this *TagSet) Merge(other TagSet) {
	this.StringSet.Merge(other.StringSet)
}
