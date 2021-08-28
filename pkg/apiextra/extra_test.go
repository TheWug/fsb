package apiextra

import (
	"github.com/thewug/fsb/pkg/api"

	"testing"
)

type S struct {
}

func (s S) GetApiName() string { return "website name" }
func (s S) GetApiEndpoint() string { return "website" }
func (s S) GetApiFilteredEndpoint() string { return "filteredwebsite" }
func (s S) GetApiStaticPrefix() string { return "static." }

func TestMain(m *testing.M) {
	api.Init(S{})
	Init(S{})

	os.Exit(m.Run())
}

func Test_regexes(t *testing.T) {
	id_testcases := map[string]struct{
		test string
		match matcher
		expected int
	}{
		"url1": {"http://" + api.Endpoint + "/post/show/111", apiurlmatch, 111},
		"url2": {"https://www." + api.Endpoint + "/posts/111", apiurlmatch, 111},
		"url3": {api.FilteredEndpoint + "/post/show/111", apiurlmatch, 111},
		"url4": {"link among a string " + api.FilteredEndpoint + "/post/show/111 yadda yadda", apiurlmatch, 111},
		"url5": {"(" + api.FilteredEndpoint + "/post/show/111)", apiurlmatch, 111},
		"url6": {api.FilteredEndpoint + "/post/show/111", apiurlmatch, 111},
		"url7": {"htp:/" + api.FilteredEndpoint + ".nope/post/show/111", apiurlmatch, NONEXISTENT_POST},
		"id1": {"222", numericmatch, 222},
		"id2": {" x y z 222 yadda", numericmatch, 222},
		"id3": {"(222)", numericmatch, 222},
		"id4": {"#222", numericmatch, 222},
		"id5": {"-222", numericmatch, NONEXISTENT_POST},
	}

	string_testcases := map[string]struct{
		test string
		match matcher
		expected string
	}{
		"md5_1": {"md5:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", md5hashmatch, "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},
		"md5_2": {" md5:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", md5hashmatch, "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},
		"md5_3": {"(md5:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA)", md5hashmatch, "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},
		"md5_4": {"md5:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", md5hashmatch, ""},
		"md5_5": {"https://" + api.StaticPrefix + api.Endpoint + "/data/1F/F1/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA.png", md5hashmatch, "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},
		"md5_6": {"http://" + api.StaticPrefix + api.Endpoint + "/data/1F/F1/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA.png", md5hashmatch, "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},
		"md5_7": {"(" + api.StaticPrefix + api.Endpoint + "/data/1F/F1/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA.png)", md5hashmatch, "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},
		"md5_8": {api.StaticPrefix + api.Endpoint + "/data/1F/F1/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA.png", md5hashmatch, ""},
	}

	for k, v := range id_testcases {
		t.Run(k, func(t *testing.T) {
			out := v.match.Match(v.test)
			if out != v.expected { t.Errorf("Unexpected result: got %d, expected %d (%s)", out, v.expected, v.test) }
		})
	}

	for k, v := range string_testcases {
		t.Run(k, func(t *testing.T) {
			out := v.match.MatchString(v.test)
			if out != v.expected { t.Errorf("Unexpected result: got %s, expected %s (%s)", out, v.expected, v.test) }
		})
	}
}
