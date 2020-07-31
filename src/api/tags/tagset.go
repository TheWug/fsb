package tags

import (
	"strings"
	"reflect"
)

type TagSet struct {
	Tags map[string]bool
}

func (this TagSet) Equal(other TagSet) bool {
	return len(this.Tags) == 0 && len(other.Tags) == 0 || reflect.DeepEqual(this.Tags, other.Tags)
}

// apply a single tag.
// accepts tags prefixed with -, which negates the tag.
// when negating tags, accepts simple asterisk wildcards to match any number of characters (as a trivial example, '-*' unsets all tags)
func (this *TagSet) ApplyTag(t string) {
	if strings.HasPrefix(t, "-") { // if it's a negation
		t = strings.ToLower(t[1:])
		if strings.Contains(t, "*") { // if it's a negative wildcard
			for k, _ := range this.Tags { // for every tag we have
				if WildcardMatch(t, k) { this.ClearTag(k) } // delete it if it matches the wildcard
			}
		} else { this.ClearTag(t) } // if it's just a regular negative, delete it normally
	} else {
		this.SetTag(t)
	} // if it's a positive tag, add it in.
}

// set a tag.
// does not accept prefixed tags.
func (this *TagSet) SetTag(t string) {
	if this.Tags == nil {
		this.Tags = make(map[string]bool)
	}

	this.Tags[strings.ToLower(t)] = true
}

// unset a tag.
// does not accept prefixed tags.
func (this *TagSet) ClearTag(t string) {
	delete(this.Tags, strings.ToLower(t))
}

// checks to see if a specific tag is set.
// does not accept prefixed tags.
func (this *TagSet) IsSet(tag string) (bool) {
	return this.Tags[strings.ToLower(tag)]
}

// applies each tag in an array.
// accepts tags prefixed with -.
func (this *TagSet) MergeTags(tags []string) {
	for _, t := range tags { this.ApplyTag(t) }
}

// toggles each tag in an array, deselecting them if they are currently selected and vice versa.
// accepts tags prefixed with either + or -, which overrides toggling behavior (the prefix always signals the end state)
func (this *TagSet) ToggleTags(tags []string) {
	for _, t := range tags {
		tag := t
		prefix := ""
		if strings.HasPrefix(t, "-") || strings.HasPrefix(t, "+") { prefix = t[0:1]; tag = t[1:] }

		if prefix == "-" {
			this.ClearTag(tag)
		} else if prefix == "+" || !this.IsSet(tag) {
			this.SetTag(tag)
		} else {
			this.ClearTag(tag)
		}
	}
}

// Emits the tag set as a space delimited string.
func (this *TagSet) ToString() (string) {
	builder := bytes.NewBuffer(nil)
	for k, v := range this.Tags {
		if v != 0 {
			if builder.Len() != 0 { builder.WriteRune(' ') }
			builder.WriteString(k)
		}
	}
	return builder.String()
}

// Counts the number of tags set.
func (this *TagSet) Len() (int) {
	return len(this.Tags)
}

// Clears all tags.
func (this *TagSet) Reset() {
	*this = TagSet{}
}

// attempts to find a "rating:*" tag and interpret it, returning a rating string if it does and returning nothing if there isn't one.
func (this *TagSet) Rating() (string) {
	var s, q, e bool
	for t, _ := range this.Tags {
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
		this.SetTag(tag)
	}
	for tag, _ := range diff.RemoveList {
		this.ClearTag(tag)
	}
}
