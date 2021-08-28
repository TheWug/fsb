package tags

import (
	"testing"
)

func TestTagSetEqual(t *testing.T) {
	var pairs = []struct {
		name string
		expected bool
		first, second TagSet
	}{
		{"empty to empty", true,
			TagSet{},
			TagSet{}},
		{"nil to empty", true,
			TagSet{Tags: map[string]bool{}},
			TagSet{}},
		{"empty to nonempty", false,
			TagSet{},
			TagSet{Tags: map[string]bool{"new string":true}}},
		{"same", true,
			TagSet{Tags: map[string]bool{"new string":true, "old string":true}},
			TagSet{Tags: map[string]bool{"old string":true, "new string":true}}},
		{"different", false,
			TagSet{Tags: map[string]bool{"string 1":true}},
			TagSet{Tags: map[string]bool{"string 2":true}}},
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			if x.first.Equal(x.second) != x.expected {
				t.Errorf("\nExpected: %t\nActual:   %t\n", x.expected, !x.expected)
			}
		})
	}
}

func TestSetTag(t *testing.T) {
	var pairs = []struct {
		name string
		add string
		before, after TagSet
	}{
		{"empty with space", "new tag",
			TagSet{},
			TagSet{Tags: map[string]bool{"new tag":true}}},
		{"empty", "newtag",
			TagSet{},
			TagSet{Tags: map[string]bool{"newtag":true}}},
		{"prefixed", "-newtag",
			TagSet{},
			TagSet{Tags: map[string]bool{"-newtag":true}}},
		{"nonempty", "newtag",
			TagSet{Tags: map[string]bool{"existing":true}},
			TagSet{Tags: map[string]bool{"existing":true, "newtag":true}}},
		{"duplicate", "duplicate",
			TagSet{Tags: map[string]bool{"duplicate":true}},
			TagSet{Tags: map[string]bool{"duplicate":true}}},
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			x.before.SetTag(x.add)
			if !x.before.Equal(x.after) {
				t.Errorf("\nExpected: %+v\nActual:   %+v\n", x.after, x.before)
			}
		})
	}
}

func TestClearTag(t *testing.T) {
	var pairs = []struct {
		name string
		remove string
		before, after TagSet
	}{
		{"empty", "tag",
			TagSet{},
			TagSet{}},
		{"applicable", "tag",
			TagSet{Tags: map[string]bool{"tag":true}},
			TagSet{}},
		{"not applicable", "tag",
			TagSet{Tags: map[string]bool{"othertag":true}},
			TagSet{Tags: map[string]bool{"othertag":true}}},
		{"prefixed", "-tag",
			TagSet{Tags: map[string]bool{"-tag":true}},
			TagSet{}},
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			x.before.ClearTag(x.remove)
			if !x.before.Equal(x.after) {
				t.Errorf("\nExpected: %+v\nActual:   %+v\n", x.after, x.before)
			}
		})
	}
}

func TestApplyTag(t *testing.T) {
	var pairs = []struct {
		name string
		tag string
		before, after TagSet
	}{
		{"empty add", "tag",
			TagSet{},
			TagSet{Tags: map[string]bool{"tag":true}}},
		{"empty remove", "-tag",
			TagSet{},
			TagSet{}},
		{"extra add", "tag",
			TagSet{Tags: map[string]bool{"extra":true}},
			TagSet{Tags: map[string]bool{"extra":true, "tag":true}}},
		{"extra remove", "-tag",
			TagSet{Tags: map[string]bool{"extra":true, "tag":true}},
			TagSet{Tags: map[string]bool{"extra":true}}},
		{"duplicate add", "tag",
			TagSet{Tags: map[string]bool{"tag":true}},
			TagSet{Tags: map[string]bool{"tag":true}}},
		{"applicable remove", "-tag",
			TagSet{Tags: map[string]bool{"tag":true}},
			TagSet{}},
		{"wildcard remove", "-tag_*",
			TagSet{Tags: map[string]bool{"tag_a":true, "tag_b":true}},
			TagSet{}},
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			x.before.ApplyTag(x.tag)
			if !x.before.Equal(x.after) {
				t.Errorf("\nExpected: %+v\nActual:   %+v\n", x.after, x.before)
			}
		})
	}
}

func TestIsSet(t *testing.T) {
	var pairs = []struct {
		name string
		tag string
		expected bool
		set TagSet
	}{
		{"empty", "tag", false,
			TagSet{}},
		{"nonmember", "tag", false,
			TagSet{Tags: map[string]bool{"other":true}}},
		{"member", "tag", true,
			TagSet{Tags: map[string]bool{"tag":true}}},
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			has := x.set.IsSet(x.tag)
			if has != x.expected {
				t.Errorf("\nExpected: %t\nActual:   %t\n", x.expected, has)
			}
		})
	}
}

func TestMergeTags(t *testing.T) {
	var pairs = []struct {
		name string
		merge []string
		start, end TagSet
	}{
		{"empty", []string{},
			TagSet{},
			TagSet{}},
		{"identity", []string{"member"},
			TagSet{Tags: map[string]bool{"member":true}},
			TagSet{Tags: map[string]bool{"member":true}}},
		{"normal", []string{"bar", "-foo", "derp"},
			TagSet{Tags: map[string]bool{"foo":true}},
			TagSet{Tags: map[string]bool{"derp":true, "bar":true}}},
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			x.start.MergeTags(x.merge)
			if !x.start.Equal(x.end) {
				t.Errorf("\nExpected: %+v\nActual:   %+v\n", x.end, x.start)
			}
		})
	}
}

func TestToggleTags(t *testing.T) {
	var pairs = []struct {
		name string
		toggle []string
		start, end TagSet
	}{
		{"empty", []string{},
			TagSet{},
			TagSet{}},
		{"identity", []string{},
			TagSet{Tags: map[string]bool{"foo":true}},
			TagSet{Tags: map[string]bool{"foo":true}}},
		{"on", []string{"new"},
			TagSet{Tags: map[string]bool{"member":true}},
			TagSet{Tags: map[string]bool{"member":true, "new":true}}},
		{"off", []string{"old"},
			TagSet{Tags: map[string]bool{"member":true, "old":true}},
			TagSet{Tags: map[string]bool{"member":true}}},
		{"plus present", []string{"+member"},
			TagSet{Tags: map[string]bool{"member":true, "old":true}},
			TagSet{Tags: map[string]bool{"member":true, "old":true}}},
		{"plus missing", []string{"+member"},
			TagSet{Tags: map[string]bool{"old":true}},
			TagSet{Tags: map[string]bool{"old":true, "member":true}}},
		{"minus present", []string{"-member"},
			TagSet{Tags: map[string]bool{"member":true, "old":true}},
			TagSet{Tags: map[string]bool{"old":true}}},
		{"minus missing", []string{"-member"},
			TagSet{Tags: map[string]bool{"old":true}},
			TagSet{Tags: map[string]bool{"old":true}}},
		
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			x.start.ToggleTags(x.toggle)
			if !x.start.Equal(x.end) {
				t.Errorf("\nExpected: %+v\nActual:   %+v\n", x.end, x.start)
			}
		})
	}
}

func TestTagSetString(t *testing.T) {
	var pairs = []struct {
		name, expected string
		start TagSet
	}{
		{"empty", "",
			TagSet{}},
		{"weird", "+foo -bar",
			TagSet{Tags: map[string]bool{"+foo":true, "-bar":true}}},
		{"normal", "bar foo",
			TagSet{Tags: map[string]bool{"foo":true, "bar":true}}},
		
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			out := x.start.String()
			if out != x.expected {
				t.Errorf("\nExpected: %s\nActual:   %s\n", x.expected, out)
			}
		})
	}
}

func TestTagSetLen(t *testing.T) {
	var pairs = []struct {
		name string
		count int
		value TagSet
	}{
		{"empty", 0,
			TagSet{}},
		{"4", 4,
			TagSet{Tags: map[string]bool{"a":true, "aa":true, "aaa":true, "aaaa":true}}},
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			out := x.value.Len()
			if out != x.count {
				t.Errorf("\nExpected: %d\nActual:   %d\n", x.count, out)
			}
		})
	}
}

func TestTagSetReset(t *testing.T) {
	set := TagSet{Tags: map[string]bool{"foobar":true}}
	set.Reset()
	if !set.Equal(TagSet{}) {
		t.Errorf("\nExpected: %+v\nActual:   %+v\n", TagSet{}, set)
	}
}

func TestRating(t *testing.T) {
	var pairs = []struct {
		name, rating string
		value TagSet
	}{
		{"empty", "",
			TagSet{}},
		{"no rating", "",
			TagSet{Tags: map[string]bool{"a":true, "foo":true, "bar":true}}},
		{"s", "safe",
			TagSet{Tags: map[string]bool{"rating:safe":true, "foo":true}}},
		{"q", "questionable",
			TagSet{Tags: map[string]bool{"bar":true, "rating:q":true}}},
		{"e", "explicit",
			TagSet{Tags: map[string]bool{"rating:enormouspenis":true}}},
		{"overload", "explicit",
			TagSet{Tags: map[string]bool{"rating:E":true, "rating:quonk":true, "rating:silly":true}}},
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			out := x.value.Rating()
			if out != x.rating {
				t.Errorf("\nExpected: %s\nActual:   %s\n", x.rating, out)
			}
		})
	}
}

func TestApplyDiff(t *testing.T) {
	var pairs = []struct {
		name string
		in TagSet
		diff TagDiff
		out TagSet
	}{
		{"empty",
			TagSet{},
			TagDiff{},
			TagSet{}},
		{"identity",
			TagSet{Tags: map[string]bool{"foo":true, "bar":true}},
			TagDiff{},
			TagSet{Tags: map[string]bool{"foo":true, "bar":true}}},
		{"mixed remove",
			TagSet{Tags: map[string]bool{"foo":true, "bar":true}},
			TagDiff{StringDiff: StringDiff{RemoveList: map[string]bool{"bar":true, "baz":true}}},
			TagSet{Tags: map[string]bool{"foo":true}}},
		{"mixed add",
			TagSet{Tags: map[string]bool{"foo":true, "bar":true}},
			TagDiff{StringDiff: StringDiff{AddList: map[string]bool{"bar":true, "baz":true}}},
			TagSet{Tags: map[string]bool{"foo":true, "bar":true, "baz":true}}},
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			x.in.ApplyDiff(x.diff)
			if !x.in.Equal(x.out) {
				t.Errorf("\nExpected: %+v\nActual:   %+v\n", x.out, x.in)
			}
		})
	}
}
