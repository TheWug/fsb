package tags

import (
	"strings"
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

// attempts to find a "rating:*" tag and interpret it, returning a rating string if it does and returning nothing if there isn't one.
func (this *TagSet) Rating() (string) {
	var s, q, e bool
	for t, _ := range this.Data {
		t = strings.ToLower(t)
		s = s || strings.HasPrefix(t, "rating:s")
		q = q || strings.HasPrefix(t, "rating:q")
		e = e || strings.HasPrefix(t, "rating:e")
	}
	if e { return "explicit" }
	if q { return "questionable" }
	if s { return "safe" }
	return ""
}

// apply a tag diff to the tag set.
func (this *TagSet) ApplyDiff(diff TagDiff) {
	for tag, _ := range diff.AddList {
		this.Set(tag)
	}
	for tag, _ := range diff.RemoveList {
		this.Clear(tag)
	}
}
