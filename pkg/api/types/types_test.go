package types

import (
	"github.com/thewug/fsb/pkg/api/tags"
	
	"testing"
)

func Test_Simple(t *testing.T) {
	Explicit.String()
	Questionable.String()
	Safe.String()
	Original.String()
}

func Test_RatingFromTagSet(t *testing.T) {
	var testcases = map[string]struct {
		rating PostRating
		value tags.TagSet
	}{
		"empty": {Original, tags.TagSet{}},
		"no rating": {Original, tags.TagSet{StringSet: tags.StringSet{Data: map[string]bool{"a":true, "foo":true, "bar":true}}}},
		"s": {Safe, tags.TagSet{StringSet: tags.StringSet{Data: map[string]bool{"rating:safe":true, "foo":true}}}},
		"q": {Questionable, tags.TagSet{StringSet: tags.StringSet{Data: map[string]bool{"bar":true, "rating:q":true}}}},
		"e": {Explicit, tags.TagSet{StringSet: tags.StringSet{Data: map[string]bool{"rating:enormouspenis":true}}}},
		"overload": {Explicit, tags.TagSet{StringSet: tags.StringSet{Data: map[string]bool{"rating:E":true, "rating:quonk":true, "rating:silly":true}}}},
	}

	for k, v := range testcases {
		t.Run(k, func(t *testing.T) {
			out := RatingFromTagSet(v.value)
			if out != v.rating {
				t.Errorf("\nExpected: %s\nActual:   %s\n", v.rating, out)
			}
		})
	}
}
