package types

import (
	"github.com/thewug/fsb/pkg/api/tags"

	"testing"
	"strings"
)

func Test_Simple(t *testing.T) {
	Explicit.String()
	Questionable.String()
	Safe.String()
	Original.String()

	TCGeneral.Value()
	TSONewest.String()
	ASOStatus.String()
	ASApproved.String()
	Upvote.Value()
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

func Test_PageSelector(t *testing.T) {
	testcases := map[string]struct{
		value int
		after, before, page string
	}{
		"zero": {0, "a0", "b0", "0"},
		"normal": {999, "a999", "b999", "999"},
	}

	for k, v := range testcases {
		t.Run(k, func(t *testing.T) {
			ps := After(v.value)
			if ps.String() != v.after { t.Errorf("Unexpected output (After): got %s, expected %s", ps.String(), v.after) }
			ps = Before(v.value)
			if ps.String() != v.before { t.Errorf("Unexpected output (Before): got %s, expected %s", ps.String(), v.before) }
			ps = Page(v.value)
			if ps.String() != v.page { t.Errorf("Unexpected output (Page): got %s, expected %s", ps.String(), v.page) }
		})
	}

	t.Run("blank", func(t *testing.T) {
		ps := PageSelector{}
		if ps.String() != "" { t.Errorf("Unexpected output (Blank PageSelector): got %s, expected %s", ps.String(), "") }
	})
}

func Test_PostsAfterChangeSeq(t *testing.T) {
	testcases := map[string]struct{
		change int
		containsEach []string
	}{
		"simple": {1000, []string{"status:any", "order:change_asc", "change:>1000"}},
	}

	for k, v := range testcases {
		t.Run(k, func(t *testing.T) {
			result := PostsAfterChangeSeq(v.change)
			for _, x := range v.containsEach {
				if !strings.Contains(result, x) { t.Errorf("Unexpected output: result %s does not contain %s", result, x) }
			}
		})
	}
}

func Test_PostsAfterId(t *testing.T) {
	testcases := map[string]struct{
		id int
		containsEach []string
	}{
		"simple": {1000, []string{"status:any", "order:id_asc", "id:>1000"}},
	}

	for k, v := range testcases {
		t.Run(k, func(t *testing.T) {
			result := PostsAfterId(v.id)
			for _, x := range v.containsEach {
				if !strings.Contains(result, x) { t.Errorf("Unexpected output: result %s does not contain %s", result, x) }
			}
		})
	}
}

func Test_DeletedPostsAfterId(t *testing.T) {
	testcases := map[string]struct{
		id int
		containsEach []string
	}{
		"simple": {1000, []string{"status:deleted", "order:id_asc", "id:>1000"}},
	}

	for k, v := range testcases {
		t.Run(k, func(t *testing.T) {
			result := DeletedPostsAfterId(v.id)
			for _, x := range v.containsEach {
				if !strings.Contains(result, x) { t.Errorf("Unexpected output: result %s does not contain %s", result, x) }
			}
		})
	}
}

func Test_SinglePostByMd5(t *testing.T) {
	testcases := map[string]struct{
		md5 string
		containsEach []string
	}{
		"simple": {"DEADBEEF", []string{"md5:DEADBEEF"}},
	}

	for k, v := range testcases {
		t.Run(k, func(t *testing.T) {
			result := SinglePostByMd5(v.md5)
			for _, x := range v.containsEach {
				if !strings.Contains(result, x) { t.Errorf("Unexpected output: result %s does not contain %s", result, x) }
			}
		})
	}
}
