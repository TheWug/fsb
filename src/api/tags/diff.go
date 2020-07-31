package tags

import (
	"bytes"
	"sort"
	"strings"
	"reflect"
)

type DiffMembership int
const AddsTag DiffMembership = 1
const ResetsTag DiffMembership = 0
const RemovesTag DiffMembership = -1

type StringDiff struct {
	AddList    map[string]bool `json:"add"`
	RemoveList map[string]bool `json:"remove"`
}

type StringDiffArray []StringDiff

func (this *StringDiff) Clear() {
	*this = StringDiff{}
}

func (this *StringDiff) TagStatus(tag string) DiffMembership {
	if _, ok := this.AddList[tag]; ok {
		return AddsTag
	} else if _, ok := this.RemoveList[tag]; ok {
		return RemovesTag
	}

	return ResetsTag
}

func base_add_remove(x, y *map[string]bool, tag string) {
	if tag == "" { return }

	if (*x) == nil {
		(*x) = make(map[string]bool)
	}

	(*x)[tag] = true
	delete((*y), tag)
}

func (this *StringDiff) Add(tag string) {
	base_add_remove(&this.AddList, &this.RemoveList, tag)
}

func (this *StringDiff) Remove(tag string) {
	base_add_remove(&this.RemoveList, &this.AddList, tag)
}

func (this *StringDiff) Reset(tag string) {
	delete(this.AddList, tag)
	delete(this.RemoveList, tag)
}

func (this *StringDiff) Apply(tag string) {
	if strings.HasPrefix(tag, "-") {
		this.Remove(strings.TrimPrefix(tag, "-"))
	} else if strings.HasPrefix(tag, "+") {
		this.Add(strings.TrimPrefix(tag, "+"))
	} else if strings.HasPrefix(tag, "=") {
		this.Reset(strings.TrimPrefix(tag, "="))
	} else {
		this.Add(tag)
	}
}

func (this *StringDiff) ApplyStringWithDelimiter(tag_diff, delimiter string) {
	this.ApplyArray(strings.Split(tag_diff, delimiter))
}

func (this *StringDiff) ApplyArray(tag_diff []string) {
	for _, tag := range tag_diff {
		this.Apply(tag)
	}
}

func (this *StringDiff) ApplyStringsWithDelimiter(add_tags, remove_tags, reset_tags, delimiter string) {
	this.ApplyArrays(
		strings.Split(add_tags, delimiter),
		strings.Split(remove_tags, delimiter),
		strings.Split(reset_tags, delimiter),
	)
}

func (this *StringDiff) ApplyArrays(add_tags, remove_tags, reset_tags []string) {
	for _, tag := range add_tags {
		this.Add(tag)
	}

	for _, tag := range remove_tags {
		this.Remove(tag)
	}

	for _, tag := range reset_tags {
		this.Reset(tag)
	}
}

func (this StringDiff) IsZero() bool {
	return len(this.AddList) == 0 && len(this.RemoveList) == 0
}

func (this StringDiff) Len() int {
	return len(this.AddList) + len(this.RemoveList)
}

func sorted_keys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k, v := range m { if v { keys = append(keys, k) } }
	sort.Strings(keys)
	return keys
}

func (this StringDiff) StringWithDelimiter(delimiter string) string {
	var buf bytes.Buffer
	for _, k := range sorted_keys(this.AddList) {
		if buf.Len() != 0 {
			buf.WriteString(delimiter)
		}
		buf.WriteString(k)
	}
	for _, k := range sorted_keys(this.RemoveList) {
		if buf.Len() != 0 {
			buf.WriteString(delimiter)
		}
		buf.WriteRune('-')
		buf.WriteString(k)
	}
	return buf.String()
}

func (this StringDiff) Array() []string {
	out := make([]string, 0, this.Len())
	for _, k := range sorted_keys(this.AddList) {
		out = append(out, k)
	}
	for _, k := range sorted_keys(this.RemoveList) {
		out = append(out, "-" + k)
	}
	return out
}

func (this StringDiff) Difference(other StringDiff) StringDiff {
	var n StringDiff
	for a, v := range this.AddList { if v { n.Add(a) } }
	for a, v := range other.AddList { if v { delete(n.AddList, a) } }
	for r, v := range this.RemoveList { if v { n.Remove(r) } }
	for r, v := range other.RemoveList { if v { delete(n.RemoveList, r) } }
	return n
}

func (this StringDiff) Invert() StringDiff {
	var n StringDiff
	for a, v := range this.RemoveList { if v { n.Add(a) } }
	for r, v := range this.AddList { if v { n.Remove(r) } }
	return n
}

func (this StringDiff) Union(other StringDiff) StringDiff {
	var n StringDiff
	for a, v := range this.AddList { if v { n.Add(a) } }
	for r, v := range this.RemoveList { if v { n.Remove(r) } }
	for a, v := range other.AddList { if v { n.Add(a) } }
	for r, v := range other.RemoveList { if v { n.Remove(r) } }
	return n
}

func (this StringDiff) Equal(other StringDiff) bool {
	// the complexity here is necessary because DeepEqual returns false when comparing
	// nil to an empty map, and we want to treat this comparison as true.
	return	(len(this.AddList)    == 0 && len(other.AddList)    == 0 || reflect.DeepEqual(this.AddList,    other.AddList))    &&
		(len(this.RemoveList) == 0 && len(other.RemoveList) == 0 || reflect.DeepEqual(this.RemoveList, other.RemoveList))
}

func (this StringDiffArray) Flatten() StringDiff {
	var n StringDiff
	for _, other := range this {
		for a, v := range other.AddList { if v { n.Add(a) } }
		for r, v := range other.RemoveList { if v { n.Remove(r) } }
	}
	return n
}

func StringDiffFromStringWithDelimiter(tag_diff, delimiter string) (StringDiff) {
	return StringDiffFromArray(strings.Split(tag_diff, delimiter))
}

func StringDiffFromStringsWithDelimiter(add_tags, remove_tags, reset_tags, delimiter string) (StringDiff) {
	return StringDiffFromArrays(strings.Split(add_tags,delimiter), strings.Split(remove_tags, delimiter), strings.Split(reset_tags, delimiter))
}

func StringDiffFromArray(tag_diff []string) (StringDiff) {
	var diff StringDiff
	diff.ApplyArray(tag_diff)
	return diff
}

func StringDiffFromArrays(add_tags, remove_tags, reset_tags []string) (StringDiff) {
	var diff StringDiff
	diff.ApplyArrays(add_tags, remove_tags, reset_tags)
	return diff
}
