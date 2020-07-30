package tags

import (
	"bytes"
	"sort"
	"strings"
)

type TagDiffMembership int
const AddsTag TagDiffMembership = 1
const ResetsTag TagDiffMembership = 0
const RemovesTag TagDiffMembership = -1

type TagDiff struct {
	Add    map[string]bool `json:"add"`
	Remove map[string]bool `json:"remove"`
}

type TagDiffArray []TagDiff

func (this *TagDiff) Reset() {
	*this = TagDiff{}
}

func (this *TagDiff) TagStatus(tag string) TagDiffMembership {
	if _, ok := this.Add[tag]; ok {
		return AddsTag
	} else if _, ok := this.Remove[tag]; ok {
		return RemovesTag
	}

	return ResetsTag
}

func baseAddRemove(x, y map[string]bool, tag string) {
	if tag == "" { return }

	if x == nil {
		x = make(map[string]bool)
	}

	x[tag] = true
	delete(y, tag)
}

func (this *TagDiff) AddTag(tag string) {
	baseAddRemove(this.Add, this.Remove, tag)
}

func (this *TagDiff) RemoveTag(tag string) {
	baseAddRemove(this.Remove, this.Add, tag)
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

func (this TagDiff) Len() int {
	return len(this.Add) + len(this.Remove)
}

// the output of this function is stable and can be used to compare TagDiff objects
func (this TagDiff) APIString() string {
	return this.APIStringWithDelimiter(" ")
}

func (this TagDiff) APIStringWithDelimiter(delim string) string {
	var buf bytes.Buffer
	keys := make([]string, 0, len(this.Add))
	for k, v := range this.Add { if v { keys = append(keys, k) } }
	sort.Slice(keys, func(i, j int) bool {return keys[i] < keys[j]})
	for _, k := range keys {
		if buf.Len() != 0 {
			buf.WriteString(delim)
		}
		buf.WriteString(k)
	}
	keys = make([]string, 0, len(this.Add))
	for k, v := range this.Remove { if v { keys = append(keys, k) } }
	sort.Slice(keys, func(i, j int) bool {return keys[i] < keys[j]})
	for _, k := range keys {
		if buf.Len() != 0 {
			buf.WriteString(delim)
		}
		buf.WriteRune('-')
		buf.WriteString(k)
	}
	return buf.String()
}

func (this TagDiff) APIArray() []string {
	var out []string
	keys := make([]string, 0, len(this.Add))
	for k, v := range this.Add { if v { keys = append(keys, k) } }
	sort.Slice(keys, func(i, j int) bool {return keys[i] < keys[j]})
	for _, k := range keys {
		out = append(out, k)
	}
	keys = make([]string, 0, len(this.Add))
	for k, v := range this.Remove { if v { keys = append(keys, k) } }
	sort.Slice(keys, func(i, j int) bool {return keys[i] < keys[j]})
	for _, k := range keys {
		out = append(out, "-" + k)
	}
	return out
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

func (this TagDiffArray) Flatten() TagDiff {
	var n TagDiff
	for _, other := range this {
		for a, v := range other.Add { if v { n.AddTag(a) } }
		for r, v := range other.Remove { if v { n.RemoveTag(r) } }
	}
	return n
}

func TagDiffFromString(tag_diff string) (TagDiff) {
	return TagDiffFromStringWithDelimiter(tag_diff, " ")
}

func TagDiffFromStringWithDelimiter(tag_diff, delimiter string) (TagDiff) {
	return TagDiffFromArray(strings.Split(tag_diff, delimiter))
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
