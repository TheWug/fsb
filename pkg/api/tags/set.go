package tags

import (
	"strings"
	"reflect"
)

type StringSet struct {
	Data map[string]bool
}

func (this StringSet) Equal(other StringSet) bool {
	return len(this.Data) == 0 && len(other.Data) == 0 || reflect.DeepEqual(this.Data, other.Data)
}

// apply a single string.
// accepts strings prefixed with -, which negates.
// when negating, accepts simple asterisk wildcards to match any number of characters (as a trivial example, '-*' unsets all matches)
func (this *StringSet) Apply(t string) {
	if strings.HasPrefix(t, "-") { // if it's a negation
		t = strings.ToLower(t[1:])
		if strings.Contains(t, "*") { // if it's a negative wildcard
			for k, _ := range this.Data { // for every tag we have
				if WildcardMatch(t, k) { this.Clear(k) } // delete it if it matches the wildcard
			}
		} else { this.Clear(t) } // if it's just a regular negative, delete it normally
	} else {
		this.Set(t)
	} // if it's a positive tag, add it in.
}

// set a string.
// does not accept prefixed strings.
func (this *StringSet) Set(t string) {
	if this.Data == nil {
		this.Data = make(map[string]bool)
	}

	this.Data[strings.ToLower(t)] = true
}

// unset a string.
// does not accept prefixed strings.
func (this *StringSet) Clear(t string) {
	delete(this.Data, strings.ToLower(t))
}

// checks to see if a specific tag is set.
// does not accept prefixed tags.
func (this *StringSet) Status(tag string) (DiffMembership) {
	switch this.Data[strings.ToLower(tag)] {
	case true:
		return AddsTag
	default:
		return NotPresent
	}
}

func (this StringSet) Clone() (StringSet) {
	newdata := make(map[string]bool)
	for k, v := range this.Data {
		newdata[k] = v
	}
	this.Data = newdata
	return this
}

// applies each tag in an array.
// accepts tags prefixed with -.
func (this *StringSet) ApplyArray(tags []string) {
	for _, t := range tags { this.Apply(t) }
}

// applies each token in a string with specified delimiter.
// accepts strings prefixed with -.
func (this *StringSet) ApplyStringWithDelimiter(tags, delim string) {
	for _, t := range strings.Split(tags, delim) { this.Apply(t) }
}

// toggles each tag in an array, deselecting them if they are currently selected and vice versa.
// accepts tags prefixed with either + or -, which overrides toggling behavior (the prefix always signals the end state)
func (this *StringSet) ToggleArray(tags []string) {
	for _, t := range tags {
		tag := t
		prefix := ""
		if strings.HasPrefix(t, "-") || strings.HasPrefix(t, "+") { prefix = t[0:1]; tag = t[1:] }

		if prefix == "-" {
			this.Clear(tag)
		} else if prefix == "+" || this.Status(tag) == NotPresent {
			this.Set(tag)
		} else {
			this.Clear(tag)
		}
	}
}

// toggles each tag in an array, deselecting them if they are currently selected and vice versa.
// accepts tags prefixed with either + or -, which overrides toggling behavior (the prefix always signals the end state)
func (this *StringSet) ToggleStringWithDelimiter(tags, delim string) {
	for _, t := range strings.Split(tags, delim) {
		tag := t
		prefix := ""
		if strings.HasPrefix(t, "-") || strings.HasPrefix(t, "+") { prefix = t[0:1]; tag = t[1:] }

		if prefix == "-" {
			this.Clear(tag)
		} else if prefix == "+" || this.Status(tag) == NotPresent {
			this.Set(tag)
		} else {
			this.Clear(tag)
		}
	}
}

// Emits the string set as a single string with the specified delimiter.
func (this StringSet) StringWithDelimiter(delim string) (string) {
	return strings.Join(sorted_keys(this.Data), delim)
}

// Counts the number of strings.
func (this StringSet) Len() (int) {
	return len(this.Data)
}

// Clears all strings.
func (this *StringSet) Reset() {
	*this = StringSet{}
}

// apply a diff to the tag set.
func (this *StringSet) ApplyDiff(diff StringDiff) {
	for tag, _ := range diff.AddList {
		this.Set(tag)
	}
	for tag, _ := range diff.RemoveList {
		this.Clear(tag)
	}
}

func (this *StringSet) Merge(other StringSet) {
	for tag, v := range other.Data {
		if v {
			this.Data[tag] = true
		}
	}
}
