package tags

import (
	"testing"
	"reflect"
)

func TestEqual(t *testing.T) {
	var pairs = []struct {
		name string
		compares bool
		first, second StringDiff
	}{
		{"nil", true,
			StringDiff{},
			StringDiff{}},
		{"nil to empty", true,
			StringDiff{AddList:map[string]bool{}},
			StringDiff{RemoveList:map[string]bool{}}},
		{"nil to added", false,
			StringDiff{},
			StringDiff{AddList:map[string]bool{"new string":true}}},
		{"empty to added", false,
			StringDiff{AddList:map[string]bool{}},
			StringDiff{AddList:map[string]bool{"new string":true}}},
		{"nil to removed", false,
			StringDiff{},
			StringDiff{RemoveList:map[string]bool{"new string":true}}},
		{"empty to removed", false,
			StringDiff{RemoveList:map[string]bool{}},
			StringDiff{RemoveList:map[string]bool{"new string":true}}},
		{"different added", false,
			StringDiff{AddList:map[string]bool{"string 1":true}},
			StringDiff{AddList:map[string]bool{"string 2":true}}},
		{"different removed", false,
			StringDiff{RemoveList:map[string]bool{"string 1":true}},
			StringDiff{RemoveList:map[string]bool{"string 2":true}}},
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			if x.first.Equal(x.second) != x.compares {
				t.Errorf("\nExpected: %t\nActual:   %t\n", x.compares, !x.compares)
			}
		})
	}
}

func TestAdd(t *testing.T) {
	var pairs = []struct {
		name, add string
		before, after StringDiff
	}{
		{"empty",     "new string",
			StringDiff{},
			StringDiff{AddList:map[string]bool{"new string": true}}},
		{"nonempty",  "new string",
			StringDiff{AddList:map[string]bool{"string 1":true}, RemoveList:map[string]bool{"string 2":true}},
			StringDiff{AddList:map[string]bool{"string 1":true, "new string":true}, RemoveList:map[string]bool{"string 2":true}}},
		{"duplicate", "new string",
			StringDiff{AddList:map[string]bool{"new string":true}},
			StringDiff{AddList:map[string]bool{"new string":true}}},
		{"override",  "new string",
			StringDiff{RemoveList:map[string]bool{"new string":true}},
			StringDiff{AddList:map[string]bool{"new string":true}}},
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			x.before.Add(x.add)
			if !x.before.Equal(x.after) {
				t.Errorf("\nExpected: %+v\nActual:   %+v\n", x.after, x.before)
			}
		})
	}
}

func TestRemove(t *testing.T) {
	var pairs = []struct {
		name, remove string
		before, after StringDiff
	}{
		{"empty",     "old string",
			StringDiff{},
			StringDiff{RemoveList:map[string]bool{"old string":true}}},
		{"nonempty",  "old string",
			StringDiff{AddList:map[string]bool{"string 1":true}, RemoveList:map[string]bool{"string 2":true}},
			StringDiff{AddList:map[string]bool{"string 1":true}, RemoveList:map[string]bool{"string 2":true, "old string":true}}},
		{"duplicate", "old string",
			StringDiff{RemoveList:map[string]bool{"old string":true}},
			StringDiff{RemoveList:map[string]bool{"old string":true}}},
		{"override",  "old string",
			StringDiff{AddList:map[string]bool{"old string":true}},
			StringDiff{RemoveList:map[string]bool{"old string":true}}},
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			x.before.Remove(x.remove)
			if !x.before.Equal(x.after) {
				t.Errorf("\nExpected: %+v\nActual:   %+v\n", x.after, x.before)
			}
		})
	}
}

func TestReset(t *testing.T) {
	var pairs = []struct {
		name, reset string
		before, after StringDiff
	}{
		{"empty",     "old string",
			StringDiff{},
			StringDiff{}},
		{"matching add",  "old string",
			StringDiff{AddList:map[string]bool{"old string":true, "string 2":true}, RemoveList:map[string]bool{"string 1":true}},
			StringDiff{AddList:map[string]bool{"string 2":true}, RemoveList:map[string]bool{"string 1":true}}},
		{"matching remove", "old string",
			StringDiff{AddList:map[string]bool{"string 1":true}, RemoveList:map[string]bool{"old string":true, "string 2":true}},
			StringDiff{AddList:map[string]bool{"string 1":true}, RemoveList:map[string]bool{"string 2":true}}},
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			x.before.Reset(x.reset)
			if !x.before.Equal(x.after) {
				t.Errorf("\nExpected: %+v\nActual:   %+v\n", x.after, x.before)
			}
		})
	}
}

func TestApply(t *testing.T) {
	var pairs = []struct {
		name, apply string
		before, after StringDiff
	}{
		{"default",  "new",
			StringDiff{AddList:map[string]bool{"added":true}, RemoveList:map[string]bool{"removed":true}},
			StringDiff{AddList:map[string]bool{"new":true, "added":true}, RemoveList:map[string]bool{"removed":true}}},
		{"+ prefix", "+new",
			StringDiff{AddList:map[string]bool{"added":true}, RemoveList:map[string]bool{"removed":true}},
			StringDiff{AddList:map[string]bool{"new":true, "added":true}, RemoveList:map[string]bool{"removed":true}}},
		{"- prefix", "-new",
			StringDiff{AddList:map[string]bool{"added":true}, RemoveList:map[string]bool{"removed":true}},
			StringDiff{AddList:map[string]bool{"added":true}, RemoveList:map[string]bool{"new":true, "removed":true}}},
		{"= prefix", "=new",
			StringDiff{AddList:map[string]bool{"added":true, "new":true}, RemoveList:map[string]bool{"removed":true}},
			StringDiff{AddList:map[string]bool{"added":true}, RemoveList:map[string]bool{"removed":true}}},
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			x.before.Apply(x.apply)
			if !x.before.Equal(x.after) {
				t.Errorf("\nExpected: %+v\nActual:   %+v\n", x.after, x.before)
			}
		})
	}
}

func TestApplyStringWithDelimiter(t *testing.T) {
	var pairs = []struct {
		name, apply, delim string
		before, after StringDiff
	}{
		{"null",  "", " ",
			StringDiff{AddList:map[string]bool{"added":true}, RemoveList:map[string]bool{"removed":true}},
			StringDiff{AddList:map[string]bool{"added":true}, RemoveList:map[string]bool{"removed":true}}},
		{"comprehensive space", "+plus -minus =equals default -added +removed", " ",
			StringDiff{AddList:map[string]bool{"added":true, "equals":true}, RemoveList:map[string]bool{"removed":true}},
			StringDiff{AddList:map[string]bool{"default":true, "removed":true, "plus":true}, RemoveList:map[string]bool{"added":true, "minus":true}}},
		{"comprehensive CR", "+plus\r-minus\r=equals\rdefault\r-added\r+removed", "\r",
			StringDiff{AddList:map[string]bool{"added":true, "equals":true}, RemoveList:map[string]bool{"removed":true}},
			StringDiff{AddList:map[string]bool{"default":true, "removed":true, "plus":true}, RemoveList:map[string]bool{"added":true, "minus":true}}},
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			x.before.ApplyStringWithDelimiter(x.apply, x.delim)
			if !x.before.Equal(x.after) {
				t.Errorf("\nExpected: %+v\nActual:   %+v\n", x.after, x.before)
			}
		})
	}
}

func TestApplyArray(t *testing.T) {
	var pairs = []struct {
		name string
		apply []string
		before, after StringDiff
	}{
		{"null", []string{},
			StringDiff{AddList:map[string]bool{"added":true}, RemoveList:map[string]bool{"removed":true}},
			StringDiff{AddList:map[string]bool{"added":true}, RemoveList:map[string]bool{"removed":true}}},
		{"comprehensive", []string{"+plus","-minus","=equals","default","-added","+removed"},
			StringDiff{AddList:map[string]bool{"added":true, "equals":true}, RemoveList:map[string]bool{"removed":true}},
			StringDiff{AddList:map[string]bool{"default":true, "removed":true, "plus":true}, RemoveList:map[string]bool{"added":true, "minus":true}}},
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			x.before.ApplyArray(x.apply)
			if !x.before.Equal(x.after) {
				t.Errorf("\nExpected: %+v\nActual:   %+v\n", x.after, x.before)
			}
		})
	}
}

func TestApplyStringsWithDelimiter(t *testing.T) {
	var pairs = []struct {
		name, add, remove, reset, delim string
		before, after StringDiff
	}{
		{"null", "", "", "", " ",
			StringDiff{AddList:map[string]bool{"added":true, "resetfoo":true}, RemoveList:map[string]bool{"removed":true, "resetbar":true}},
			StringDiff{AddList:map[string]bool{"added":true, "resetfoo":true}, RemoveList:map[string]bool{"removed":true, "resetbar":true}}},
		{"comprehensive space", "addfoo addbar", "removefoo removebar", "resetfoo resetbar", " ",
			StringDiff{AddList:map[string]bool{"added":true, "resetfoo":true}, RemoveList:map[string]bool{"removed":true, "resetbar":true}},
			StringDiff{AddList:map[string]bool{"added":true, "addfoo":true, "addbar":true}, RemoveList:map[string]bool{"removed":true, "removefoo":true, "removebar":true}}},
		{"comprehensive CR", "addfoo\raddbar", "removefoo\rremovebar", "resetfoo\rresetbar", "\r",
			StringDiff{AddList:map[string]bool{"added":true, "resetfoo":true}, RemoveList:map[string]bool{"removed":true, "resetbar":true}},
			StringDiff{AddList:map[string]bool{"added":true, "addfoo":true, "addbar":true}, RemoveList:map[string]bool{"removed":true, "removefoo":true, "removebar":true}}},
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			x.before.ApplyStringsWithDelimiter(x.add, x.remove, x.reset, x.delim)
			if !x.before.Equal(x.after) {
				t.Errorf("\nExpected: %+v\nActual:   %+v\n", x.after, x.before)
			}
		})
	}
}

func TestApplyArrays(t *testing.T) {
	var pairs = []struct {
		name string
		add, remove, reset []string
		before, after StringDiff
	}{
		{"null", []string{}, []string{}, []string{},
			StringDiff{AddList:map[string]bool{"added":true, "resetfoo":true}, RemoveList:map[string]bool{"removed":true, "resetbar":true}},
			StringDiff{AddList:map[string]bool{"added":true, "resetfoo":true}, RemoveList:map[string]bool{"removed":true, "resetbar":true}}},
		{"comprehensive", []string{"addfoo", "addbar"}, []string{"removefoo", "removebar"}, []string{"resetfoo", "resetbar"},
			StringDiff{AddList:map[string]bool{"added":true, "resetfoo":true}, RemoveList:map[string]bool{"removed":true, "resetbar":true}},
			StringDiff{AddList:map[string]bool{"added":true, "addfoo":true, "addbar":true}, RemoveList:map[string]bool{"removed":true, "removefoo":true, "removebar":true}}},
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			x.before.ApplyArrays(x.add, x.remove, x.reset)
			if !x.before.Equal(x.after) {
				t.Errorf("\nExpected: %+v\nActual:   %+v\n", x.after, x.before)
			}
		})
	}
}

func TestIsZero(t *testing.T) {
	var pairs = []struct {
		name string
		zero bool
		test StringDiff
	}{
		{"null", true,
			StringDiff{}},
		{"empty", true,
			StringDiff{AddList:map[string]bool{}, RemoveList:map[string]bool{}}},
		{"added", false,
			StringDiff{AddList:map[string]bool{"tag":true}}},
		{"removed", false,
			StringDiff{RemoveList:map[string]bool{"tag":true}}},
		{"both", false,
			StringDiff{AddList:map[string]bool{"tag2":true}, RemoveList:map[string]bool{"tag":true}}},
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			if x.test.IsZero() != x.zero {
				t.Errorf("\nExpected: %+v\nActual:   %+v\n", x.zero, !x.zero)
			}
		})
	}
}

func TestLen(t *testing.T) {
	var pairs = []struct {
		name string
		length int
		test StringDiff
	}{
		{"null", 0,
			StringDiff{}},
		{"empty", 0,
			StringDiff{AddList:map[string]bool{}, RemoveList:map[string]bool{}}},
		{"4", 4,
			StringDiff{AddList:map[string]bool{"tag":true, "tag2":true}, RemoveList:map[string]bool{"tag3":true, "tag4":true}}},
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			l := x.test.Len()
			if l != x.length {
				t.Errorf("\nExpected: %+v\nActual:   %+v\n", x.length, l)
			}
		})
	}
}

func TestStringWithDelimiter(t *testing.T) {
	var pairs = []struct {
		name, delim, expected string
		test StringDiff
	}{
		{"null", " ", "",
			StringDiff{}},
		{"empty", " ", "",
			StringDiff{AddList:map[string]bool{}, RemoveList:map[string]bool{}}},
		{"mixture space", " ", "bar foo -bar2 -foo2",
			StringDiff{AddList:map[string]bool{"foo":true, "bar":true}, RemoveList:map[string]bool{"foo2":true, "bar2":true}}},
		{"mixture CR", "\r", "bar\rfoo\r-bar2\r-foo2",
			StringDiff{AddList:map[string]bool{"foo":true, "bar":true}, RemoveList:map[string]bool{"foo2":true, "bar2":true}}},
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			str := x.test.StringWithDelimiter(x.delim)
			if str != x.expected {
				t.Errorf("\nExpected: %+v\nActual:   %+v\n", x.expected, str)
			}
		})
	}
}

func TestArray(t *testing.T) {
	var pairs = []struct {
		name string
		expected []string
		test StringDiff
	}{
		{"null", []string{},
			StringDiff{}},
		{"empty", []string{},
			StringDiff{AddList:map[string]bool{}, RemoveList:map[string]bool{}}},
		{"mixture", []string{"bar", "foo", "-bar2", "-foo2"},
			StringDiff{AddList:map[string]bool{"foo":true, "bar":true}, RemoveList:map[string]bool{"foo2":true, "bar2":true}}},
	}

	for _, x := range pairs {
		t.Run(x.name, func(t *testing.T) {
			array := x.test.Array()
			if !(reflect.DeepEqual(array, x.expected) || (len(array) == 0 && len(x.expected) == 0)) {
				t.Errorf("\nExpected: %+v\nActual:   %+v\n", x.expected, array)
			}
		})
	}
}

func TestDifference(t *testing.T) {
	var pairs = []struct {
		name string
		first, second, answer StringDiff
	}{
		{"minus null",
			StringDiff{AddList:map[string]bool{"foo":true, "bar":true}, RemoveList:map[string]bool{"foo2":true, "bar2":true}},
			StringDiff{},
			StringDiff{AddList:map[string]bool{"foo":true, "bar":true}, RemoveList:map[string]bool{"foo2":true, "bar2":true}}},
		{"null minus",
			StringDiff{},
			StringDiff{AddList:map[string]bool{"foo":true, "bar":true}, RemoveList:map[string]bool{"foo2":true, "bar2":true}},
			StringDiff{}},
		{"identity",
			StringDiff{AddList:map[string]bool{"foo":true}, RemoveList:map[string]bool{"bar":true}},
			StringDiff{AddList:map[string]bool{"foo":true}, RemoveList:map[string]bool{"bar":true}},
			StringDiff{}},
		{"normal",
			StringDiff{AddList:map[string]bool{"foo":true, "bar":true}, RemoveList:map[string]bool{"foo2":true, "bar2":true}},
			StringDiff{AddList:map[string]bool{"foo":true, "bar2":true}, RemoveList:map[string]bool{"foo2":true, "bar":true}},
			StringDiff{AddList:map[string]bool{"bar":true}, RemoveList:map[string]bool{"bar2":true}}},
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

func TestUnion(t *testing.T) {
	var pairs = []struct {
		name string
		first, second, answer StringDiff
	}{
		{"null",
			StringDiff{},
			StringDiff{},
			StringDiff{}},
		{"identity",
			StringDiff{AddList:map[string]bool{"foo":true}, RemoveList:map[string]bool{"bar":true}},
			StringDiff{AddList:map[string]bool{"foo":true}, RemoveList:map[string]bool{"bar":true}},
			StringDiff{AddList:map[string]bool{"foo":true}, RemoveList:map[string]bool{"bar":true}}},
		{"normal",
			StringDiff{AddList:map[string]bool{"foo":true}, RemoveList:map[string]bool{"bar":true}},
			StringDiff{AddList:map[string]bool{"foo2":true}, RemoveList:map[string]bool{"bar2":true}},
			StringDiff{AddList:map[string]bool{"foo":true, "foo2":true}, RemoveList:map[string]bool{"bar":true, "bar2":true}}},
		{"overlapping",
			StringDiff{AddList:map[string]bool{"foo":true, "foo2":true}, RemoveList:map[string]bool{"bar":true, "bar2":true}},
			StringDiff{AddList:map[string]bool{"foo3":true, "bar2":true}, RemoveList:map[string]bool{"bar3":true, "foo2":true}},
			StringDiff{AddList:map[string]bool{"foo":true, "foo3":true, "bar2":true}, RemoveList:map[string]bool{"bar":true, "bar3":true, "foo2":true}}},
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

func TestInvert(t *testing.T) {
	var pairs = []struct {
		name string
		first, answer StringDiff
	}{
		{"null",
			StringDiff{},
			StringDiff{}},
		{"normal",
			StringDiff{AddList:map[string]bool{"foo":true}, RemoveList:map[string]bool{"bar":true}},
			StringDiff{AddList:map[string]bool{"bar":true}, RemoveList:map[string]bool{"foo":true}}},
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
