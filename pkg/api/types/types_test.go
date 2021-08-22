package types

import (
	"github.com/thewug/fsb/pkg/api/tags"

	"testing"
	"strings"
	"reflect"
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

func Test_TTagData_ApparentCount(t *testing.T) {
	testcases := map[string]struct{
		tag TTagData
		flag bool
		result int
	}{
		"full": {TTagData{Count: 75, FullCount: 100}, true, 100},
		"visible": {TTagData{Count: 75, FullCount: 100}, false, 75},
	}

	for k, v := range testcases {
		t.Run(k, func(t *testing.T) {
			result := v.tag.ApparentCount(v.flag)
			if result != v.result { t.Errorf("Unexpected output: got %d, expected %d", result, v.result) }
		})
	}
}

func Test_TTagListing_UnmarshalJSON(t *testing.T) {
	testcases := map[string]struct{
		jsondata string
		expected TTagInfoArray
		err string
	}{
		"empty-untagged": {`[]`, TTagInfoArray{}, ""},
		"empty": {`{"tags":[]}`, TTagInfoArray{}, ""},
		"full-untagged": {`[{"id": 1, "name": "cat"}, {"id": 2, "name": "dog"}]`, TTagInfoArray{TTagData{Id: 1, Name: "cat"}, TTagData{Id: 2, Name: "dog"}}, ""},
		"full": {`{"tags": [{"id": 1, "name": "cat"}, {"id": 2, "name": "dog"}]}`, TTagInfoArray{TTagData{Id: 1, Name: "cat"}, TTagData{Id: 2, Name: "dog"}}, ""},
		"missing": {`{"missing":true}`, TTagInfoArray{}, "figure out how to parse"},
	}

	for k, v := range testcases {
		t.Run(k, func(t *testing.T) {
			var start TTagListing
			result := start.UnmarshalJSON([]byte(v.jsondata))
			if result == nil && v.err != "" || result != nil && (v.err == "" || !strings.Contains(result.Error(), v.err)) {
				t.Errorf("Unexpected error: got %v, wanted matching %s", result, v.err)
			}

			if !(len(start.Tags) == 0 && len(v.expected) == 0 || reflect.DeepEqual(start.Tags, v.expected)) {
				t.Errorf("Unexpected result: got %v, expected %v", start.Tags, v.expected)
			}
		})
	}
}

func Test_TAliasListing_UnmarshalJSON(t *testing.T) {
	testcases := map[string]struct{
		jsondata string
		expected TAliasInfoArray
		err string
	}{
		"empty-untagged": {`[]`, TAliasInfoArray{}, ""},
		"empty": {`{"tag_aliases":[]}`, TAliasInfoArray{}, ""},
		"full-untagged": {`[{"id":1, "consequent_name": "cat"}, {"id":2, "consequent_name": "dog"}]`, TAliasInfoArray{TAliasData{Id: 1, Name: "cat"}, TAliasData{Id: 2, Name: "dog"}}, ""},
		"full": {`{"tag_aliases": [{"id":1, "consequent_name": "cat"}, {"id":2, "consequent_name": "dog"}]}`, TAliasInfoArray{TAliasData{Id: 1, Name: "cat"}, TAliasData{Id: 2, Name: "dog"}}, ""},
		"missing": {`{"missing":true}`, TAliasInfoArray{}, "figure out how to parse"},
	}

	for k, v := range testcases {
		t.Run(k, func(t *testing.T) {
			var start TAliasListing
			result := start.UnmarshalJSON([]byte(v.jsondata))
			if result == nil && v.err != "" || result != nil && (v.err == "" || !strings.Contains(result.Error(), v.err)) {
				t.Errorf("Unexpected error: got %v, wanted matching %s", result, v.err)
			}

			if !(len(start.Aliases) == 0 && len(v.expected) == 0 || reflect.DeepEqual(start.Aliases, v.expected)) {
				t.Errorf("Unexpected result: got %v, expected %v", start.Aliases, v.expected)
			}
		})
	}
}

func Test_TPostListing_UnmarshalJSON(t *testing.T) {
	testcases := map[string]struct{
		jsondata string
		expected TPostInfoArray
		err string
	}{
		"empty-untagged": {`[]`, TPostInfoArray{}, ""},
		"empty": {`{"posts":[]}`, TPostInfoArray{}, ""},
		"full-untagged": {`[{"id":1, "description": "hello"}, {"id":2, "description": "hello2"}]`, TPostInfoArray{TPostInfo{Id: 1, Description: "hello"}, TPostInfo{Id: 2, Description: "hello2"}}, ""},
		"full": {`{"posts": [{"id":1, "description": "hello"}, {"id":2, "description": "hello2"}]}`, TPostInfoArray{TPostInfo{Id: 1, Description: "hello"}, TPostInfo{Id: 2, Description: "hello2"}}, ""},
		"missing": {`{"missing":true}`, TPostInfoArray{}, "figure out how to parse"},
	}

	for k, v := range testcases {
		t.Run(k, func(t *testing.T) {
			var start TPostListing
			result := start.UnmarshalJSON([]byte(v.jsondata))
			if result == nil && v.err != "" || result != nil && (v.err == "" || !strings.Contains(result.Error(), v.err)) {
				t.Errorf("Unexpected error: got %v, wanted matching %s", result, v.err)
			}

			if !(len(start.Posts) == 0 && len(v.expected) == 0 || reflect.DeepEqual(start.Posts, v.expected)) {
				t.Errorf("Unexpected result: got %v, expected %v", start.Posts, v.expected)
			}
		})
	}
}

func Test_TSinglePostListing_UnmarshalJSON(t *testing.T) {
	testcases := map[string]struct{
		jsondata string
		expected TPostInfo
		err string
	}{
		"empty-untagged": {`null`, TPostInfo{}, "figure out how to parse"},
		"empty": {`{"post": null}`, TPostInfo{}, "figure out how to parse"},
		"full-untagged": {`{"id":1, "description": "hello"}`, TPostInfo{Id: 1, Description: "hello"}, ""},
		"full": {`{"post": {"id":1, "description": "hello"}}`, TPostInfo{Id: 1, Description: "hello"}, ""},
		"missing": {`{"missing":true}`, TPostInfo{}, "figure out how to parse"},
	}

	for k, v := range testcases {
		t.Run(k, func(t *testing.T) {
			var start TSinglePostListing
			result := start.UnmarshalJSON([]byte(v.jsondata))
			if result == nil && v.err != "" || result != nil && (v.err == "" || !strings.Contains(result.Error(), v.err)) {
				t.Errorf("Unexpected error: got %v, wanted matching %s", result, v.err)
			}

			if !(reflect.DeepEqual(start.Post, v.expected)) {
				t.Errorf("Unexpected result: got %v, expected %v", start.Post, v.expected)
			}
		})
	}
}

func Test_MatchesBlacklist(t *testing.T) {
	testcases := map[string]struct{
		post TPostInfo
		blacklist string
		matches bool
	}{
		"everything+": {TPostInfo{Id: 5, Rating: Questionable, TPostTags: TPostTags{General: []string{"cat", "dog", "dragon", "gryphon"}}}, "cat dog ~gryphon ~fox -walrus rating:q", true},
		"everything+spaces": {TPostInfo{Id: 5, Rating: Questionable, TPostTags: TPostTags{General: []string{"cat", "dog", "dragon", "gryphon"}}}, "cat   dog   ~gryphon   ~fox     -walrus       rating:q", true},
		"include+": {TPostInfo{Id: 5, Rating: Questionable, TPostTags: TPostTags{General: []string{"cat", "dog", "dragon", "gryphon"}}}, "cat", true},
		"exclude+": {TPostInfo{Id: 5, Rating: Questionable, TPostTags: TPostTags{General: []string{"cat", "dog", "dragon", "gryphon"}}}, "-fox", true},
		"either+": {TPostInfo{Id: 5, Rating: Questionable, TPostTags: TPostTags{General: []string{"cat", "dog", "dragon", "gryphon"}}}, "~dragon ~naga", true},
		"rating+": {TPostInfo{Id: 5, Rating: Questionable, TPostTags: TPostTags{General: []string{"cat", "dog", "dragon", "gryphon"}}}, "rating:q", true},
		"id+": {TPostInfo{Id: 5, Rating: Questionable, TPostTags: TPostTags{General: []string{"cat", "dog", "dragon", "gryphon"}}}, "id:5", true},
		"include-": {TPostInfo{Id: 5, Rating: Questionable, TPostTags: TPostTags{General: []string{"cat", "dog", "dragon", "gryphon"}}}, "walrus", false},
		"exclude-": {TPostInfo{Id: 5, Rating: Questionable, TPostTags: TPostTags{General: []string{"cat", "dog", "dragon", "gryphon"}}}, "-cat", false},
		"either-": {TPostInfo{Id: 5, Rating: Questionable, TPostTags: TPostTags{General: []string{"cat", "dog", "dragon", "gryphon"}}}, "~fox ~walrus", false},
		"empty-": {TPostInfo{Id: 5, Rating: Questionable, TPostTags: TPostTags{General: []string{"cat", "dog", "dragon", "gryphon"}}}, "", false},
		"rating-": {TPostInfo{Id: 5, Rating: Questionable, TPostTags: TPostTags{General: []string{"cat", "dog", "dragon", "gryphon"}}}, "rating:e", false},
		"id-": {TPostInfo{Id: 5, Rating: Questionable, TPostTags: TPostTags{General: []string{"cat", "dog", "dragon", "gryphon"}}}, "id:4", false},

		// support more blacklist tag options:
		// by upload user
		// by file type
	}

	for k, v := range testcases {
		t.Run(k, func(t *testing.T) {
			t.Logf("Blacklist: %s\nPost: %+v\n", v.blacklist, v.post)
			out := v.post.MatchesBlacklist(v.blacklist)
			if out != v.matches { t.Errorf("Unexpected result: got %t, expected %t", out, v.matches) }
		})
	}
}

// this is a dummy test. It's used to mark simple functions such as getters and setters which are too simple to test, so they
// show up as covered code. If it's more complicated than a getter or a setter, it shouldn't be here.
func Test_Others(t *testing.T) {
	(&TPostInfo{}).GetFields()
}

func Test_TPostInfo_PostScan(t *testing.T) {
	testcases := map[string]struct{
		raw_sources string
		expected []string
		err string
	}{
		"normal": {"a\nb\nc", []string{"a", "b", "c"}, ""},
		"empty": {"", []string{}, ""},
	}

	for k, v := range testcases {
		t.Run(k, func(t *testing.T) {
			post := TPostInfo{sources_internal: v.raw_sources}
			err := post.PostScan()
			if !(len(post.Sources) == 0 && len(v.expected) == 0 || reflect.DeepEqual(post.Sources, v.expected)) { t.Errorf("Unexpected result: got %v, expected %v", post.Sources, v.expected) }
			if err == nil && v.err != "" || err != nil && (v.err == "" || !strings.Contains(err.Error(), v.err)) { t.Errorf("Unexpected error: got %v, wanted matching %s", err, v.err) }
		})
	}
}
