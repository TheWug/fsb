package api

import (
	"api/tags"

	"bytes"
	"strings"
)

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
