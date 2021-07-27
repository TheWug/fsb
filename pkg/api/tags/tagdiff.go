package tags

import (
	"strings"
)

// This represents a diff to be applied to a tag set. It is specialized
// for string-delimited tags. Tags can, using some functions, be prefixed
// with +, -, or = to override default behaviors, check function docs to
// see where this is allowed.
//
// TagDiff embeds StringDiff, and thus provides many of the same functions
// which are integral to its usage, and are documented by StringDiff.
type TagDiff struct {
	StringDiff
}

// This represents a list of diffs, applied sequentially.
type TagDiffArray []TagDiff

// Compares a TagDiff to another. The two are only equal if they both
// add the same set of new tags, and remove the same set of existing tags.
func (this TagDiff) Equal(other TagDiff) bool {
	return this.StringDiff.Equal(other.StringDiff)
}

// ApplyString delimits the provided string by spaces and applies each tag in it.
// Tags can be prefixed:
// + forces addition (this is the default if a prefix isn't specified)
// - forces removal
// = forces reset (undoing a previous add or remove)
func (this *TagDiff) ApplyString(tag_diff string) {
	this.ApplyStringWithDelimiter(tag_diff, " ")
}

// ApplyStrings takes three strings, representing tags to add, remove, and reset.
// Each string is delimited by spaces and each tag in it is applied.
// Prefixes are not allowed in this function, if they are present they will be
// included literally (this is not usually the desired result).
func (this *TagDiff) ApplyStrings(add_tags, remove_tags, reset_tags string) {
	this.ApplyStringsWithDelimiter(add_tags, remove_tags, reset_tags, " ")
}

// Builds an API compatible representation of the tag difference.
// Substrings are ordered first (adds, removes), then alphabetically.
// Removes are prefixed by a minus sign. Adds are unprefixed, and no
// prefix is permitted.
func (this TagDiff) APIString() string {
	return this.StringWithDelimiter(" ")
}
// Produces a human readable representation of this TagDiff object.
func (this TagDiff) String() string {
	return this.StringWithDelimiter(" ")
}

// Difference yields a TagDiff such that every tag added or removed by `this` will
// be processed UNLESS that tag is to be added or removed by `other`, in which
// case the opposite will occur. Tags are added and removed by `this` first, and
// then by `other` second (in case the two inputs try to both add or remove the
// same tag).
func (this TagDiff) Difference(other TagDiff) TagDiff {
	return TagDiff{StringDiff: this.StringDiff.Difference(other.StringDiff)}
}

// Invert yields a TagDiff with its add and remove sets swapped. In the case
// where a TagDiff applies perfectly to a TagSet (i.e. every tag in its add list
// was absent and added, and in its remove list, present and removed), Applying
// that tag diff's Invert() value will undo the change.  (If that precondition is
// not true, it's a little more complicated, be careful)
func (this TagDiff) Invert() TagDiff {
	return TagDiff{StringDiff: this.StringDiff.Invert()}
}

// Union yields a TagDiff which adds every tag added by either of its two
// inputs, and likewise with tags removed. Tags are added and removed by
// `this` first, and by `other` second (in case the two inputs try to both
// add remove the same tag).
func (this TagDiff) Union(other TagDiff) TagDiff {
	return TagDiff{StringDiff: this.StringDiff.Union(other.StringDiff)}
}

// AddedSet produces an ALIAS to the tags added by this TagDiff.  Changes
// made to one will reflect upon the other.
func (this TagDiff) AddedSet() TagSet {
	return TagSet{StringSet{Data: this.AddList}}
}

// RemovedSet produces an ALIAS to the tags removed by this TagDiff.  Changes
// made to one will reflect upon the other.
func (this TagDiff) RemovedSet() TagSet {
	return TagSet{StringSet{Data: this.RemoveList}}
}

// Flatten sequentially applies every operation from a TagDiffArray as though
// calling Union on them over and over, starting with the first.
func (this TagDiffArray) Flatten() TagDiff {
	var n TagDiff
	for _, other := range this {
		for a, v := range other.AddList { if v { n.Add(a) } }
		for r, v := range other.RemoveList { if v { n.Remove(r) } }
	}
	return n
}

// TagDiffFromString is the reverse operation of APIString. It is unambiguous for any
// API-legal string. Its argument is a string consisting of tags delimited by spaces,
// with the following prefixes:
// - to denote removal
// no prefix to denote addition (this is the default)
func TagDiffFromString(tag_diff string) (TagDiff) {
	return TagDiffFromStringWithDelimiter(tag_diff, " ")
}

// This is the same as TagDiffFromString, except that you may specify the delimiter.
func TagDiffFromStringWithDelimiter(tag_diff, delimiter string) (TagDiff) {
	return TagDiffFromArray(strings.Split(tag_diff, delimiter))
}

// TagDiffFromStrings takes a list of tags to add and a list of tags to remove,
// both delimited by spaces, and builds a TagDiff as though each tag in the add list
// was passed to Add, and each tag in the remove list passed to Remove. No prefixes
// are permitted, tags will be passed verbatim (and if they are present, it may be
// possible to build a TagDiff which is not API-legal)
func TagDiffFromStrings(add_tags, remove_tags string) (TagDiff) {
	return TagDiffFromStringsWithDelimiter(add_tags, remove_tags, " ")
}

// This is the same as TagDiffFromStrings, except that you may specify the delimiter.
func TagDiffFromStringsWithDelimiter(add_tags, remove_tags, delimiter string) (TagDiff) {
	return TagDiffFromArrays(strings.Split(add_tags, delimiter), strings.Split(remove_tags, delimiter))
}

// This is the same as TagDiffFromString, except that you specify an array of pre-split tags.
func TagDiffFromArray(tag_diff []string) (TagDiff) {
	return TagDiff{StringDiff: StringDiffFromArray(tag_diff)}
}

// This is the same as TagDiffFromStrings, except that you specify arrays of pre-split tags.
func TagDiffFromArrays(add_tags, remove_tags []string) (TagDiff) {
	return TagDiff{StringDiff: StringDiffFromArrays(add_tags, remove_tags)}
}
