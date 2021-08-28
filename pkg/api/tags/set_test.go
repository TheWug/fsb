package tags

import (
	"testing"
)

func TestStringSet_Equal(t *testing.T) {
	var pairs = []struct {
		name string
		expected bool
		first, second StringSet
	}{
		{"empty to empty", true,
			StringSet{},
			StringSet{}},
		{"nil to empty", true,
			StringSet{Data: map[string]bool{}},
			StringSet{}},
		{"empty to nonempty", false,
			StringSet{},
			StringSet{Data: map[string]bool{"new string":true}}},
		{"same", true,
			StringSet{Data: map[string]bool{"new string":true, "old string":true}},
			StringSet{Data: map[string]bool{"old string":true, "new string":true}}},
		{"different", false,
			StringSet{Data: map[string]bool{"string 1":true}},
			StringSet{Data: map[string]bool{"string 2":true}}},
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			if x.first.Equal(x.second) != x.expected {
				t.Errorf("\nExpected: %t\nActual:   %t\n", x.expected, !x.expected)
			}
		})
	}
}

func TestStringSet_Set(t *testing.T) {
	var pairs = []struct {
		name string
		add string
		before, after StringSet
	}{
		{"empty with space", "new tag",
			StringSet{},
			StringSet{Data: map[string]bool{"new tag":true}}},
		{"empty", "newtag",
			StringSet{},
			StringSet{Data: map[string]bool{"newtag":true}}},
		{"prefixed", "-newtag",
			StringSet{},
			StringSet{Data: map[string]bool{"-newtag":true}}},
		{"nonempty", "newtag",
			StringSet{Data: map[string]bool{"existing":true}},
			StringSet{Data: map[string]bool{"existing":true, "newtag":true}}},
		{"duplicate", "duplicate",
			StringSet{Data: map[string]bool{"duplicate":true}},
			StringSet{Data: map[string]bool{"duplicate":true}}},
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			x.before.Set(x.add)
			if !x.before.Equal(x.after) {
				t.Errorf("\nExpected: %+v\nActual:   %+v\n", x.after, x.before)
			}
		})
	}
}

func TestStringSet_Clear(t *testing.T) {
	var pairs = []struct {
		name string
		remove string
		before, after StringSet
	}{
		{"empty", "tag",
			StringSet{},
			StringSet{}},
		{"applicable", "tag",
			StringSet{Data: map[string]bool{"tag":true}},
			StringSet{}},
		{"not applicable", "tag",
			StringSet{Data: map[string]bool{"othertag":true}},
			StringSet{Data: map[string]bool{"othertag":true}}},
		{"prefixed", "-tag",
			StringSet{Data: map[string]bool{"-tag":true}},
			StringSet{}},
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			x.before.Clear(x.remove)
			if !x.before.Equal(x.after) {
				t.Errorf("\nExpected: %+v\nActual:   %+v\n", x.after, x.before)
			}
		})
	}
}

func TestStringSet_Apply(t *testing.T) {
	var pairs = []struct {
		name string
		tag string
		before, after StringSet
	}{
		{"empty add", "tag",
			StringSet{},
			StringSet{Data: map[string]bool{"tag":true}}},
		{"empty remove", "-tag",
			StringSet{},
			StringSet{}},
		{"extra add", "tag",
			StringSet{Data: map[string]bool{"extra":true}},
			StringSet{Data: map[string]bool{"extra":true, "tag":true}}},
		{"extra remove", "-tag",
			StringSet{Data: map[string]bool{"extra":true, "tag":true}},
			StringSet{Data: map[string]bool{"extra":true}}},
		{"duplicate add", "tag",
			StringSet{Data: map[string]bool{"tag":true}},
			StringSet{Data: map[string]bool{"tag":true}}},
		{"applicable remove", "-tag",
			StringSet{Data: map[string]bool{"tag":true}},
			StringSet{}},
		{"wildcard remove", "-tag_*",
			StringSet{Data: map[string]bool{"tag_a":true, "tag_b":true}},
			StringSet{}},
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			x.before.Apply(x.tag)
			if !x.before.Equal(x.after) {
				t.Errorf("\nExpected: %+v\nActual:   %+v\n", x.after, x.before)
			}
		})
	}
}

func TestStringSet_Status(t *testing.T) {
	var pairs = []struct {
		name string
		tag string
		expected DiffMembership
		set StringSet
	}{
		{"empty", "tag", NotPresent,
			StringSet{}},
		{"nonmember", "tag", NotPresent,
			StringSet{Data: map[string]bool{"other":true}}},
		{"member", "tag", AddsTag,
			StringSet{Data: map[string]bool{"tag":true}}},
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			has := x.set.Status(x.tag)
			if has != x.expected {
				t.Errorf("\nExpected: %d\nActual:   %d\n", x.expected, has)
			}
		})
	}
}

func TestStringSet_Merge(t *testing.T) {
	var pairs = []struct {
		name string
		merge StringSet
		start, end StringSet
	}{
		{"empty", StringSet{},
			StringSet{},
			StringSet{}},
		{"identity", StringSet{Data: map[string]bool{"member":true}},
			StringSet{Data: map[string]bool{"member":true}},
			StringSet{Data: map[string]bool{"member":true}}},
		{"normal", StringSet{Data: map[string]bool{"bar":true, "no":false}},
			StringSet{Data: map[string]bool{"foo":true}},
			StringSet{Data: map[string]bool{"foo":true, "bar":true}}},
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			x.start.Merge(x.merge)
			if !x.start.Equal(x.end) {
				t.Errorf("\nExpected: %+v\nActual:   %+v\n", x.end, x.start)
			}
		})
	}
}

func TestStringSet_ToggleStringWithDelimiter(t *testing.T) {
	var pairs = []struct {
		name, toggle, delim string
		start, end StringSet
	}{
		{"empty", "", " ",
			StringSet{},
			StringSet{}},
		{"identity", "", " ",
			StringSet{Data: map[string]bool{"foo":true}},
			StringSet{Data: map[string]bool{"foo":true}}},
		{"on", "new", " ",
			StringSet{Data: map[string]bool{"member":true}},
			StringSet{Data: map[string]bool{"member":true, "new":true}}},
		{"off", "old", " ",
			StringSet{Data: map[string]bool{"member":true, "old":true}},
			StringSet{Data: map[string]bool{"member":true}}},
		{"plus present", "+member", " ",
			StringSet{Data: map[string]bool{"member":true, "old":true}},
			StringSet{Data: map[string]bool{"member":true, "old":true}}},
		{"plus missing", "+member", " ",
			StringSet{Data: map[string]bool{"old":true}},
			StringSet{Data: map[string]bool{"old":true, "member":true}}},
		{"minus present", "-member", " ",
			StringSet{Data: map[string]bool{"member":true, "old":true}},
			StringSet{Data: map[string]bool{"old":true}}},
		{"minus missing", "-member", " ",
			StringSet{Data: map[string]bool{"old":true}},
			StringSet{Data: map[string]bool{"old":true}}},
		{"combo", "old\n+yes\n-no", "\n",
			StringSet{Data: map[string]bool{"old":true, "no":true}},
			StringSet{Data: map[string]bool{"yes":true}}},
		
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			x.start.ToggleStringWithDelimiter(x.toggle, x.delim)
			if !x.start.Equal(x.end) {
				t.Errorf("\nExpected: %+v\nActual:   %+v\n", x.end, x.start)
			}
		})
	}
}

func TestStringSet_ApplyStringWithDelimiter(t *testing.T) {
	var pairs = []struct {
		name, toggle, delim string
		start, end StringSet
	}{
		{"empty", "", " ",
			StringSet{},
			StringSet{}},
		{"identity", "", " ",
			StringSet{Data: map[string]bool{"foo":true}},
			StringSet{Data: map[string]bool{"foo":true}}},
		{"on", "new", " ",
			StringSet{Data: map[string]bool{"member":true}},
			StringSet{Data: map[string]bool{"member":true, "new":true}}},
		{"off", "old", " ",
			StringSet{Data: map[string]bool{"member":true, "old":true}},
			StringSet{Data: map[string]bool{"member":true, "old":true}}},
		{"minus present", "-member", " ",
			StringSet{Data: map[string]bool{"member":true, "old":true}},
			StringSet{Data: map[string]bool{"old":true}}},
		{"minus missing", "-member", " ",
			StringSet{Data: map[string]bool{"old":true}},
			StringSet{Data: map[string]bool{"old":true}}},
		{"combo", "old\nyes\n-no", "\n",
			StringSet{Data: map[string]bool{"old":true, "no":true}},
			StringSet{Data: map[string]bool{"old":true, "yes":true}}},
		
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			x.start.ApplyStringWithDelimiter(x.toggle, x.delim)
			if !x.start.Equal(x.end) {
				t.Errorf("\nExpected: %+v\nActual:   %+v\n", x.end, x.start)
			}
		})
	}
}

func TestStringSet_StringWithDelimiter(t *testing.T) {
	var pairs = []struct {
		name, expected, delimiter string
		start StringSet
	}{
		{"empty", "", " ",
			StringSet{}},
		{"weird", "+foo -bar", " ",
			StringSet{Data: map[string]bool{"+foo":true, "-bar":true}}},
		{"normal", "bar foo", " ",
			StringSet{Data: map[string]bool{"foo":true, "bar":true}}},
		{"special_delim", "bar&foo", "&",
			StringSet{Data: map[string]bool{"foo":true, "bar":true}}},
		
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			out := x.start.StringWithDelimiter(x.delimiter)
			if out != x.expected {
				t.Errorf("\nExpected: %s\nActual:   %s\n", x.expected, out)
			}
		})
	}
}

func TestStringSet_Len(t *testing.T) {
	var pairs = []struct {
		name string
		count int
		value StringSet
	}{
		{"empty", 0,
			StringSet{}},
		{"4", 4,
			StringSet{Data: map[string]bool{"a":true, "aa":true, "aaa":true, "aaaa":true}}},
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

func TestStringSet_Reset(t *testing.T) {
	set := StringSet{Data: map[string]bool{"foobar":true}}
	set.Reset()
	if !set.Equal(StringSet{}) {
		t.Errorf("\nExpected: %+v\nActual:   %+v\n", StringSet{}, set)
	}
}

func TestStringSet_ApplyDiff(t *testing.T) {
	var pairs = []struct {
		name string
		in StringSet
		diff StringDiff
		out StringSet
	}{
		{"empty",
			StringSet{},
			StringDiff{},
			StringSet{}},
		{"identity",
			StringSet{Data: map[string]bool{"foo":true, "bar":true}},
			StringDiff{},
			StringSet{Data: map[string]bool{"foo":true, "bar":true}}},
		{"mixed remove",
			StringSet{Data: map[string]bool{"foo":true, "bar":true}},
			StringDiff{RemoveList: map[string]bool{"bar":true, "baz":true}},
			StringSet{Data: map[string]bool{"foo":true}}},
		{"mixed add",
			StringSet{Data: map[string]bool{"foo":true, "bar":true}},
			StringDiff{AddList: map[string]bool{"bar":true, "baz":true}},
			StringSet{Data: map[string]bool{"foo":true, "bar":true, "baz":true}}},
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

func TestStringSet_Clone(t *testing.T) {
	testcases := map[string]struct{
		in StringSet
	}{
		"empty":  {},
		"normal": {StringSet{Data: map[string]bool{"foo":true, "bar":true}}},
	}

	for k, v := range testcases {
		t.Run(k, func(t *testing.T) {
			out := v.in.Clone()
			if !v.in.Equal(out) {
				t.Errorf("\nExpected: %+v\nActual:   %+v\n", out, v.in)
			}
			out.Set("previously-unset")
			if v.in.Equal(out) {
				t.Errorf("Clone performed a shallow copy!")
			}
		})
	}
}
