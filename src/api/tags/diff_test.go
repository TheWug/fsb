package tags

import (
	"testing"
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
