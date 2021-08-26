package api

import (
	"strings"
	"bytes"
	"api/tags"
)

type TagDiff struct {
	add    map[string]bool
	remove map[string]bool
}

func (this *TagDiff) AddTag(tag string) {
	if tag == "" { return }

	if this.add == nil {
		this.add = make(map[string]bool)
	}

	this.add[tag] = true
	delete(this.remove, tag)
}

func (this *TagDiff) RemoveTag(tag string) {
	if tag == "" { return }

	if this.remove == nil {
		this.remove = make(map[string]bool)
	}

	this.remove[tag] = true
	delete(this.add, tag)
}

func (this *TagDiff) ResetTag(tag string) {
	delete(this.add, tag)
	delete(this.remove, tag)
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
		} else {
			this.AddTag(tag)
		}
	}
}

func (this *TagDiff) ApplyStrings(add_tags, remove_tags string) {
	this.ApplyArrays(strings.Split(add_tags, " "), strings.Split(remove_tags, " "))
}

func (this *TagDiff) ApplyArrays(add_tags, remove_tags []string) {
	for _, tag := range add_tags {
		this.AddTag(strings.TrimPrefix(strings.TrimPrefix(tag, "+"), "-"))
	}

	for _, tag := range remove_tags {
		this.RemoveTag(strings.TrimPrefix(strings.TrimPrefix(tag, "+"), "-"))
	}
}

func (this *TagDiff) IsZero() bool {
	return len(this.add) == 0 && len(this.remove) == 0
}

func TagDiffFromString(tag_diff string) (TagDiff) {
	return TagDiffFromArray(strings.Split(tag_diff, " "))
}

func TagDiffFromStrings(add_tags, remove_tags string) (TagDiff) {
	return TagDiffFromArrays(strings.Split(add_tags, " "), strings.Split(remove_tags, " "))
}

func TagDiffFromArray(tag_diff []string) (TagDiff) {
	var diff TagDiff
	diff.ApplyArray(tag_diff)
	return diff
}

func TagDiffFromArrays(add_tags, remove_tags []string) (TagDiff) {
	var diff TagDiff
	diff.ApplyArrays(add_tags, remove_tags)
	return diff
}

func (this TagDiff) APIString() string {
	var buf bytes.Buffer
	for k, v := range this.add {
		if v {
			if buf.Len() != 0 {
				buf.WriteString(" ")
			}
			buf.WriteString(k)
		}
	}
	for k, v := range this.remove {
		if v {
			if buf.Len() != 0 {
				buf.WriteString(" ")
			}
			buf.WriteRune('-')
			buf.WriteString(k)
		}
	}
	return buf.String()
}

func (this TagDiff) String() string {
	return this.APIString()
}

type TagSet struct {
	Tags map[string]int
}

func NewTagSet() (*TagSet) {
	t := TagSet{Tags: make(map[string]int)}
	return &t
}

func (this *TagSet) ApplyTag(t string) {
	if strings.HasPrefix(t, "-") { // if it's a negation
		t = strings.ToLower(t[1:])
		if strings.Contains(t, "*") { // if it's a negative wildcard
			for k, _ := range this.Tags { // for every tag we have
				if tags.WildcardMatch(t, k) { this.ClearTag(k) } // delete it if it matches the wildcard
			}
		} else { this.ClearTag(t) } // if it's just a regular negative, delete it normally
	} else {
		this.SetTag(t)
	} // if it's a positive tag, add it in.
}

func (this *TagSet) SetTag(t string) {
	this.Tags[strings.ToLower(t)] = 1
}

func (this *TagSet) ClearTag(t string) {
	t = strings.ToLower(t)
	if _, ok := this.Tags[t]; ok {
		this.Tags[t] = 0
	}
}

func (this *TagSet) IsSet(tag string) (bool) {
	val, ok := this.Tags[strings.ToLower(tag)]
	return ok && val != 0
}

func (this *TagSet) MergeTags(tags []string) {
	for _, t := range tags { this.ApplyTag(t) }
}

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

func (this *TagSet) Len() (int) {
	return len(this.Tags)
}

func (this *TagSet) Reset() {
	*this = *NewTagSet()
}

func (this *TagSet) Rating() (string) {
	var s, q, e bool
	for t, v := range this.Tags {
		if v != 0 {
			t = strings.ToLower(t)
			s = s || strings.HasPrefix(t, "rating:s")
			q = q || strings.HasPrefix(t, "rating:q")
			e = e || strings.HasPrefix(t, "rating:e")
		}
	}
	if e { return "explicit" }
	if q { return "questionable" }
	if s { return "safe" }
	return ""
}
