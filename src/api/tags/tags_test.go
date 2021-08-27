package tags

import (
	"testing"
)

func TestWildcardMatch(t *testing.T) {
	var pairs = []struct{
		wildcard, test string
		shouldMatch bool
	}{
		{"tag_e*", "tag_everything", true},
		{"tag_e*", "tag_anything", false},
		{"middle_*_match", "middle_foo_match", true},
		{"middle_*_match", "middle_match", false},
		{"middle_*_match", "middle_asdf_badend", false},
		{"middle_*_match", "middle_asdf_match_but_theres_more", false},
		{"exact_match", "exact_match", true},
	}

	for _, x := range pairs {
		if WildcardMatch(x.wildcard, x.test) != x.shouldMatch {
			t.Errorf("\nExpected: %t\nActual:   %t\n", x.shouldMatch, !x.shouldMatch)
		}
	}
}
