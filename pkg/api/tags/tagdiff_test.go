package tags

import (
	"testing"
)

func TestTagDiff(t *testing.T) {
	for _, x := range []struct {
		name string
		expected bool
		d1, d2 TagDiff
	}{
		{"nil", true,
			TagDiff{},
			TagDiff{}},
		{"nil to empty", true,
			TagDiff{},
			TagDiff{StringDiff: StringDiff{AddList: map[string]bool{}, RemoveList: map[string]bool{}}}},
		{"empty", true,
			TagDiff{StringDiff: StringDiff{AddList: map[string]bool{}, RemoveList: map[string]bool{}}},
			TagDiff{StringDiff: StringDiff{AddList: map[string]bool{}, RemoveList: map[string]bool{}}}},
		{"identity", true,
			TagDiff{StringDiff: StringDiff{AddList: map[string]bool{"foo":true}, RemoveList: map[string]bool{"bar":true}}},
			TagDiff{StringDiff: StringDiff{AddList: map[string]bool{"foo":true}, RemoveList: map[string]bool{"bar":true}}}},
		{"real to nil", false,
			TagDiff{StringDiff: StringDiff{AddList: map[string]bool{"foo":true}, RemoveList: map[string]bool{"bar":true}}},
			TagDiff{}},
		{"real to empty", false,
			TagDiff{StringDiff: StringDiff{AddList: map[string]bool{"foo":true}, RemoveList: map[string]bool{"bar":true}}},
			TagDiff{StringDiff: StringDiff{AddList: map[string]bool{}, RemoveList: map[string]bool{}}}},
		{"distinct", false,
			TagDiff{StringDiff: StringDiff{AddList: map[string]bool{"foo":true}, RemoveList: map[string]bool{"bar":true}}},
			TagDiff{StringDiff: StringDiff{AddList: map[string]bool{"foo":true}, RemoveList: map[string]bool{"derp":true}}}},
	}{
		t.Run("Equal/" + x.name, func(t *testing.T) {
			if x.d1.Equal(x.d2) != x.expected {
				t.Errorf("\nExpected: %+v\nActual:   %+v\n", x.expected, !x.expected)
			}
		})
	}

	t.Run("ApplyString", func(t *testing.T) {
		desired := TagDiff{StringDiff: StringDiff{AddList:map[string]bool{"string3":true, "string4":true}, RemoveList:map[string]bool{"string1":true}}}
		out := TagDiff{StringDiff: StringDiff{AddList:map[string]bool{"string1":true}, RemoveList:map[string]bool{"string2":true}}}
		out.ApplyString("string3 +string4 -string1 =string2")
		if !out.Equal(desired) {
			t.Errorf("\nExpected: %+v\nActual:   %+v\n", desired, out)
		}
	})

	t.Run("ApplyStrings", func(t *testing.T) {
		desired := TagDiff{StringDiff: StringDiff{AddList:map[string]bool{"string3":true, "string4":true}, RemoveList:map[string]bool{"string1":true}}}
		out := TagDiff{StringDiff: StringDiff{AddList:map[string]bool{"string1":true}, RemoveList:map[string]bool{"string2":true}}}
		out.ApplyStrings("string3 string4", "string1", "string2")
		if !out.Equal(desired) {
			t.Errorf("\nExpected: %+v\nActual:   %+v\n", desired, out)
		}
	})

	t.Run("String & APIString", func(t *testing.T) {
		desired := "string1 -string2"
		in := TagDiff{StringDiff: StringDiff{AddList:map[string]bool{"string1":true}, RemoveList:map[string]bool{"string2":true}}}
		str, apistr := in.String(), in.APIString()
		if str != desired {
			t.Errorf("\nExpected: %+v\nActual:   %+v\n", desired, str)
		}
		if apistr != desired {
			t.Errorf("\nExpected: %+v\nActual:   %+v\n", desired, apistr)
		}
	})

	t.Run("Difference", Difference)
	t.Run("Union", Union)
	t.Run("Invert", Invert)
	t.Run("Flatten", Flatten)
	t.Run("AddedSet", AddedRemovedSet)
}

func Difference(t *testing.T) {
	var pairs = []struct {
		name string
		first, second, answer TagDiff
	}{
		{"minus null",
			TagDiff{StringDiff: StringDiff{AddList:map[string]bool{"foo":true, "bar":true}, RemoveList:map[string]bool{"foo2":true, "bar2":true}}},
			TagDiff{},
			TagDiff{StringDiff: StringDiff{AddList:map[string]bool{"foo":true, "bar":true}, RemoveList:map[string]bool{"foo2":true, "bar2":true}}}},
		{"null minus",
			TagDiff{},
			TagDiff{StringDiff: StringDiff{AddList:map[string]bool{"foo":true, "bar":true}, RemoveList:map[string]bool{"foo2":true, "bar2":true}}},
			TagDiff{}},
		{"identity",
			TagDiff{StringDiff: StringDiff{AddList:map[string]bool{"foo":true}, RemoveList:map[string]bool{"bar":true}}},
			TagDiff{StringDiff: StringDiff{AddList:map[string]bool{"foo":true}, RemoveList:map[string]bool{"bar":true}}},
			TagDiff{}},
		{"normal",
			TagDiff{StringDiff: StringDiff{AddList:map[string]bool{"foo":true, "bar":true}, RemoveList:map[string]bool{"foo2":true, "bar2":true}}},
			TagDiff{StringDiff: StringDiff{AddList:map[string]bool{"foo":true, "bar2":true}, RemoveList:map[string]bool{"foo2":true, "bar":true}}},
			TagDiff{StringDiff: StringDiff{AddList:map[string]bool{"bar":true}, RemoveList:map[string]bool{"bar2":true}}}},
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			diff := x.first.Difference(x.second)
			if !diff.Equal(x.answer) {
				t.Errorf("\nExpected: %+v\nActual:   %+v\n", x.answer, diff)
			}
		})
	}
}

func Union(t *testing.T) {
	var pairs = []struct {
		name string
		first, second, answer TagDiff
	}{
		{"null",
			TagDiff{},
			TagDiff{},
			TagDiff{}},
		{"identity",
			TagDiff{StringDiff: StringDiff{AddList:map[string]bool{"foo":true}, RemoveList:map[string]bool{"bar":true}}},
			TagDiff{StringDiff: StringDiff{AddList:map[string]bool{"foo":true}, RemoveList:map[string]bool{"bar":true}}},
			TagDiff{StringDiff: StringDiff{AddList:map[string]bool{"foo":true}, RemoveList:map[string]bool{"bar":true}}}},
		{"normal",
			TagDiff{StringDiff: StringDiff{AddList:map[string]bool{"foo":true}, RemoveList:map[string]bool{"bar":true}}},
			TagDiff{StringDiff: StringDiff{AddList:map[string]bool{"foo2":true}, RemoveList:map[string]bool{"bar2":true}}},
			TagDiff{StringDiff: StringDiff{AddList:map[string]bool{"foo":true, "foo2":true}, RemoveList:map[string]bool{"bar":true, "bar2":true}}}},
		{"overlapping",
			TagDiff{StringDiff: StringDiff{AddList:map[string]bool{"foo":true, "foo2":true}, RemoveList:map[string]bool{"bar":true, "bar2":true}}},
			TagDiff{StringDiff: StringDiff{AddList:map[string]bool{"foo3":true, "bar2":true}, RemoveList:map[string]bool{"bar3":true, "foo2":true}}},
			TagDiff{StringDiff: StringDiff{AddList:map[string]bool{"foo":true, "foo3":true, "bar2":true}, RemoveList:map[string]bool{"bar":true, "bar3":true, "foo2":true}}}},
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			diff := x.first.Union(x.second)
			if !diff.Equal(x.answer) {
				t.Errorf("\nExpected: %+v\nActual:   %+v\n", x.answer, diff)
			}
		})
	}
}

func Invert(t *testing.T) {
	var pairs = []struct {
		name string
		first, answer TagDiff
	}{
		{"null",
			TagDiff{},
			TagDiff{}},
		{"normal",
			TagDiff{StringDiff: StringDiff{AddList:map[string]bool{"foo":true}, RemoveList:map[string]bool{"bar":true}}},
			TagDiff{StringDiff: StringDiff{AddList:map[string]bool{"bar":true}, RemoveList:map[string]bool{"foo":true}}}},
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			diff := x.first.Invert()
			if !diff.Equal(x.answer) {
				t.Errorf("\nExpected: %+v\nActual:   %+v\n", x.answer, diff)
			}
		})
	}
}

func Flatten(t *testing.T) {
	var pairs = []struct {
		name string
		stack TagDiffArray
		answer TagDiff
	}{
		{"empty", TagDiffArray{
			},
			TagDiff{}},
		{"identity", TagDiffArray{
				TagDiff{StringDiff: StringDiff{AddList:map[string]bool{"foo":true}, RemoveList:map[string]bool{"bar":true}}},
			},
			TagDiff{StringDiff: StringDiff{AddList:map[string]bool{"foo":true}, RemoveList:map[string]bool{"bar":true}}}},
		{"stack", TagDiffArray{
				TagDiff{StringDiff: StringDiff{AddList:map[string]bool{"foo":true}, RemoveList:map[string]bool{"bar":true}}},
				TagDiff{StringDiff: StringDiff{AddList:map[string]bool{"foo2":true}, RemoveList:map[string]bool{"bar2":true}}},
				TagDiff{StringDiff: StringDiff{AddList:map[string]bool{"bar":true}, RemoveList:map[string]bool{"foo":true}}},
			},
			TagDiff{StringDiff: StringDiff{AddList:map[string]bool{"foo2":true, "bar":true}, RemoveList:map[string]bool{"bar2":true, "foo":true}}}},
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			diff := x.stack.Flatten()
			if !diff.Equal(x.answer) {
				t.Errorf("\nExpected: %+v\nActual:   %+v\n", x.answer, diff)
			}
		})
	}
}

func AddedRemovedSet(t *testing.T) {
	testcases := map[string]struct {
		diff TagDiff
		added, removed TagSet
	}{
		"normal": {TagDiff{StringDiff: StringDiff{AddList:map[string]bool{"foo":true, "bar":true}, RemoveList:map[string]bool{"minus":true, "exclude":true}}},
				TagSet{StringSet: StringSet{Data:map[string]bool{"foo":true, "bar":true}}},
				TagSet{StringSet: StringSet{Data:map[string]bool{"minus":true, "exclude":true}}}},
	}

	for k, v := range testcases {
		t.Run(k, func(t *testing.T) {
			added := v.diff.AddedSet()
			removed := v.diff.RemovedSet()

			if !added.Equal(v.added) { t.Errorf("Expected: %+v\nActual: %+v\n", v.added, added) }
			if !removed.Equal(v.removed) { t.Errorf("Expected: %+v\nActual: %+v\n", v.removed, removed) }
		})
	}
}

func TestTagDiff_FromString(t *testing.T) {
	var pairs = []struct {
		name, test string
		expected TagDiff
	}{
		{"null", "",
			TagDiff{}},
		{"simple", "foo -bar derp -bork",
			TagDiff{StringDiff: StringDiff{AddList:map[string]bool{"foo":true, "derp":true}, RemoveList:map[string]bool{"bar":true, "bork":true}}}},
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			diff := TagDiffFromString(x.test)
			if !diff.Equal(x.expected) {
				t.Errorf("\nExpected: %+v\nActual:   %+v\n", x.expected, diff)
			}
		})
	}
}

func TestTagDiff_FromStrings(t *testing.T) {
	var pairs = []struct {
		name string
		add, remove string
		expected TagDiff
	}{
		{"null", "", "",
			TagDiff{}},
		{"simple", "foo derp", "bar bork",
			TagDiff{StringDiff: StringDiff{AddList:map[string]bool{"foo":true, "derp":true}, RemoveList:map[string]bool{"bar":true, "bork":true}}}},
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			diff := TagDiffFromStrings(x.add, x.remove)
			if !diff.Equal(x.expected) {
				t.Errorf("\nExpected: %+v\nActual:   %+v\n", x.expected, diff)
			}
		})
	}
}

func TestTagDiff_FromStringWithDelimiter(t *testing.T) {
	var pairs = []struct {
		name, test, delim string
		expected TagDiff
	}{
		{"null", "", " ",
			TagDiff{}},
		{"simple", "foo -bar derp -bork", " ",
			TagDiff{StringDiff: StringDiff{AddList:map[string]bool{"foo":true, "derp":true}, RemoveList:map[string]bool{"bar":true, "bork":true}}}},
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			diff := TagDiffFromStringWithDelimiter(x.test, x.delim)
			if !diff.Equal(x.expected) {
				t.Errorf("\nExpected: %+v\nActual:   %+v\n", x.expected, diff)
			}
		})
	}
}

func TestTagDiff_FromStringsWithDelimiter(t *testing.T) {
	var pairs = []struct {
		name string
		add, remove string
		delim string
		expected TagDiff
	}{
		{"null", "", "", " ",
			TagDiff{}},
		{"simple", "foo derp", "bar bork", " ",
			TagDiff{StringDiff: StringDiff{AddList:map[string]bool{"foo":true, "derp":true}, RemoveList:map[string]bool{"bar":true, "bork":true}}}},
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			diff := TagDiffFromStringsWithDelimiter(x.add, x.remove, x.delim)
			if !diff.Equal(x.expected) {
				t.Errorf("\nExpected: %+v\nActual:   %+v\n", x.expected, diff)
			}
		})
	}
}

func TestTagDiff_FromArray(t *testing.T) {
	var pairs = []struct {
		name string
		array []string
		expected TagDiff
	}{
		{"null", []string{},
			TagDiff{}},
		{"simple", []string{"foo", "-bar", "derp", "-bork"},
			TagDiff{StringDiff: StringDiff{AddList:map[string]bool{"foo":true, "derp":true}, RemoveList:map[string]bool{"bar":true, "bork":true}}}},
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			diff := TagDiffFromArray(x.array)
			if !diff.Equal(x.expected) {
				t.Errorf("\nExpected: %+v\nActual:   %+v\n", x.expected, diff)
			}
		})
	}
}

func TestTagDiff_FromArrays(t *testing.T) {
	var pairs = []struct {
		name string
		add, remove []string
		expected TagDiff
	}{
		{"null", []string{}, []string{},
			TagDiff{}},
		{"simple", []string{"foo", "derp"}, []string{"bar", "bork"},
			TagDiff{StringDiff: StringDiff{AddList:map[string]bool{"foo":true, "derp":true}, RemoveList:map[string]bool{"bar":true, "bork":true}}}},
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			diff := TagDiffFromArrays(x.add, x.remove)
			if !diff.Equal(x.expected) {
				t.Errorf("\nExpected: %+v\nActual:   %+v\n", x.expected, diff)
			}
		})
	}
}
