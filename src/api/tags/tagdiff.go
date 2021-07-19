package tags

import (
	"strings"
)

type TagDiff struct {
	StringDiff
}

type TagDiffArray []TagDiff

func (this TagDiff) Equal(other TagDiff) bool {
	return this.StringDiff.Equal(other.StringDiff)
}

func (this *TagDiff) ApplyString(tag_diff string) {
	this.ApplyStringWithDelimiter(tag_diff, " ")
}

func (this *TagDiff) ApplyStrings(add_tags, remove_tags, reset_tags string) {
	this.ApplyStringsWithDelimiter(add_tags, remove_tags, reset_tags, " ")
}

func (this TagDiff) APIString() string {
	return this.StringWithDelimiter(" ")
}

func (this TagDiff) String() string {
	return this.StringWithDelimiter(" ")
}

func (this TagDiff) Difference(other TagDiff) TagDiff {
	return TagDiff{StringDiff: this.StringDiff.Difference(other.StringDiff)}
}

func (this TagDiff) Invert() TagDiff {
	return TagDiff{StringDiff: this.StringDiff.Invert()}
}

func (this TagDiff) Union(other TagDiff) TagDiff {
	return TagDiff{StringDiff: this.StringDiff.Union(other.StringDiff)}
}

func (this TagDiff) AddedSet() TagSet {
	return TagSet{StringSet{Data: this.AddList}}
}

func (this TagDiff) RemovedSet() TagSet {
	return TagSet{StringSet{Data: this.RemoveList}}
}

func (this TagDiffArray) Flatten() TagDiff {
	var n TagDiff
	for _, other := range this {
		for a, v := range other.AddList { if v { n.Add(a) } }
		for r, v := range other.RemoveList { if v { n.Remove(r) } }
	}
	return n
}

func TagDiffFromString(tag_diff string) (TagDiff) {
	return TagDiffFromStringWithDelimiter(tag_diff, " ")
}

func TagDiffFromStringWithDelimiter(tag_diff, delimiter string) (TagDiff) {
	return TagDiffFromArray(strings.Split(tag_diff, delimiter))
}

func TagDiffFromStrings(add_tags, remove_tags string) (TagDiff) {
	return TagDiffFromStringsWithDelimiter(add_tags, remove_tags, " ")
}

func TagDiffFromStringsWithDelimiter(add_tags, remove_tags, delimiter string) (TagDiff) {
	return TagDiffFromArrays(strings.Split(add_tags, delimiter), strings.Split(remove_tags, delimiter))
}

func TagDiffFromArray(tag_diff []string) (TagDiff) {
	return TagDiff{StringDiff: StringDiffFromArray(tag_diff)}
}

func TagDiffFromArrays(add_tags, remove_tags []string) (TagDiff) {
	return TagDiff{StringDiff: StringDiffFromArrays(add_tags, remove_tags)}
}
