package proxify

import (
	"testing"
	"net/url"
	"encoding/json"
	"reflect"
)

func UrlMust(u *url.URL, err error) *url.URL {
	if err != nil { panic(err) }
	return u
}

func Test_tokenCount(t *testing.T) {
	testcases_matches := map[string]struct{
		link string
		subject tokenCount
		matches bool
	}{
		"simple1": {"http://example.com/path/with/four/directories", tokenCount{Token: "/", Count: 3}, true},
		"simple2": {"http://example.com/path/with/four/directories", tokenCount{Token: "/", Count: 4}, true},
		"simple3": {"http://example.com/path/with/four/directories", tokenCount{Token: "/", Count: 5}, false},
		"simple4": {"http://example.com/path/with/four/directories", tokenCount{Token: "th", Count: 2}, true},
		"simple5": {"http://example.com/path/with/four/directories", tokenCount{Token: "e", Count: 2}, true},
		"simple6": {"http://example.com/path/with/four/directories", tokenCount{Token: "e", Count: 3}, false},
		"simple7": {"http://example.com/path/with/four/directories", tokenCount{Token: "t", Count: 3}, true},
		"simple8": {"http://example.com/path/with/four/directories", tokenCount{Token: "t", Count: 4}, false},
		"simple9": {"http://example.com/path/with/four/directories", tokenCount{Token: "nothing", Count: 0}, true},
		"simple0": {"http://example.com/path/with/four/directories", tokenCount{Token: "nothing", Count: 1}, false},
	}

	for k, v := range testcases_matches {
		t.Run(k, func(t *testing.T) {
			out := v.subject.Matches(UrlMust(url.Parse(v.link)))
			if out != v.matches { t.Errorf("Match failure: got %t, expected %t (%s, %+v)", out, v.matches, v.link, v.subject) }
		})
	}

	testcases_unmarshal := map[string]struct{
		js string
		expected tokenCount
		err bool
	}{
		"empty": {"[]", tokenCount{}, true},
		"empty-object": {"{}", tokenCount{}, true},
		"null": {"null", tokenCount{}, true},
		"normal": {`["token", 5]`, tokenCount{Token: "token", Count: 5}, false},
		"double-numeric": {`[4, 5]`, tokenCount{}, true},
		"too-long": {`["foo", 5, 6]`, tokenCount{Token: "foo", Count: 5}, false},
		"non-numeric": {`["foo", "bar"]`, tokenCount{}, true},
	}

	for k, v := range testcases_unmarshal {
		t.Run(k, func(t *testing.T) {
			var tc tokenCount
			err := json.Unmarshal([]byte(v.js), &tc)
			if (err != nil) != v.err {
				t.Errorf("Unexpected error: got %v, did-expect-error: %t (%s)", err, v.err, v.js)
			} else if !v.err && tc != v.expected {
				t.Errorf("Unexpected result: got %+v, expected %+v (%s)", tc, v.expected, v.js)
			}
		})
	}
}

func Test_simplematchers(t *testing.T) {
	testcases_hostnameEqualityMatcher := map[string]struct{
		test hostnameEqualityMatcher
		url string
		expected bool
	}{
		"match": {"example.com", "https://example.com/foobar?123", true},
		"nomatch": {"example.no", "https://example.com/foobar?123", false},
		"subdomain": {"example.com", "https://foo.example.com/foobar?123", false},
	}

	t.Run("hostnameEqualityMatcher", func(t *testing.T) {
		for k, v := range testcases_hostnameEqualityMatcher {
			t.Run(k, func(t *testing.T) {
				out := hostnameEqualityMatcher(v.test).Matches(UrlMust(url.Parse(v.url)))
				if out != v.expected { t.Errorf("Unexpected result: got %t, expected %t (%s, %s)", out, v.expected, v.url, v.test) }
			})
		}
	})
	
	testcases_subdomainMatcher := map[string]struct{
		test subdomainMatcher
		url string
		expected bool
	}{
		"normal": {"example.com", "https://example.com/foobar?123", false},
		"subdomain": {"example.com", "https://example.example.com/foobar?123", true},
		"middle": {"example.com", "https://foo.example.com.com/foobar?123", false},
	}

	t.Run("subdomainMatcher", func(t *testing.T) {
		for k, v := range testcases_subdomainMatcher {
			t.Run(k, func(t *testing.T) {
				out := subdomainMatcher(v.test).Matches(UrlMust(url.Parse(v.url)))
				if out != v.expected { t.Errorf("Unexpected result: got %t, expected %t (%s, %s)", out, v.expected, v.url, v.test) }
			})
		}
	})
	
	testcases_pathPrefixMatcher := map[string]struct{
		test pathPrefixMatcher
		url string
		expected bool
	}{
		"exactmatch": {"/foobar", "https://example.com/foobar?123", true},
		"match": {"/foobar", "https://example.com/foobar/something?123", true},
		"midway": {"/foobar", "https://example.example.com/something/foobar/something?123", false},
		"noquery": {"/foobar?", "https://foo.example.com.com/foobar?123", false},
		"nodomain": {"com/foobar", "https://foo.example.com.com/foobar?123", false},
	}

	t.Run("pathPrefixMatcher", func(t *testing.T) {
		for k, v := range testcases_pathPrefixMatcher {
			t.Run(k, func(t *testing.T) {
				out := pathPrefixMatcher(v.test).Matches(UrlMust(url.Parse(v.url)))
				if out != v.expected { t.Errorf("Unexpected result: got %t, expected %t (%s, %s)", out, v.expected, v.url, v.test) }
			})
		}
	})
}

func Test_anySucceeds(t *testing.T) {
	testcases := map[string]struct{
		matchers []matcher
		link string
		expected bool
	}{
		"empty": {nil, "https://www.example.com/foo/bar/test.html?param=true", true},
		"first": {[]matcher{hostnameEqualityMatcher("www.example.com"), hostnameEqualityMatcher("other.site")}, "https://www.example.com/foo/bar/test.html?param=true", true},
		"last": {[]matcher{hostnameEqualityMatcher("other.site"), hostnameEqualityMatcher("www.example.com")}, "https://www.example.com/foo/bar/test.html?param=true", true},
		"none": {[]matcher{hostnameEqualityMatcher("other.site"), hostnameEqualityMatcher("dummy.com")}, "https://www.example.com/foo/bar/test.html?param=true", false},
	}
	
	for k, v := range testcases {
		t.Run(k, func(t *testing.T) {
			success := anySucceeds(v.matchers, UrlMust(url.Parse(v.link)))
			if success != v.expected { t.Errorf("Unexpected result: got %t, expected %t", success, v.expected) }
		})
	}
}

func Test_sourceDisplayDecider(t *testing.T) {
	testcases_unmarshal := map[string]struct{
		js string
		expected sourceDisplayDecider
		err bool
	}{
		"simple": {`{}`, sourceDisplayDecider{Valid: true}, false},
		"catchall": {`{"next": "end"}`, sourceDisplayDecider{Result: "end", Valid: true}, false},
		"match-single": {`{"hostname": "host.name", "subdomain_of": "name", "path_prefix": "/foo/bar", "token_count": ["tok", 3], "stickers": true, "next": "yes"}`,
			sourceDisplayDecider{
				Hostname: []matcher{hostnameEqualityMatcher("host.name")},
				SubdomainOf: []matcher{subdomainMatcher("name")},
				PathPrefix: []matcher{pathPrefixMatcher("/foo/bar")},
				TokenCount: []matcher{tokenCount{Token: "tok", Count: 3}},
				Result: "yes", Stickers: true, Valid: true},
			false,
		},
		"match-multiple": {`{"hostname": ["h1", "h2"], "subdomain_of": ["s1", "s2"], "path_prefix": ["p1", "p2"], "token_count": [["tok", 3], ["tok2", 4]], "stickers": true, "next": "yes"}`,
			sourceDisplayDecider{Hostname: []matcher{hostnameEqualityMatcher("h1"), hostnameEqualityMatcher("h2")},
				SubdomainOf: []matcher{subdomainMatcher("s1"), subdomainMatcher("s2")},
				PathPrefix: []matcher{pathPrefixMatcher("p1"), pathPrefixMatcher("p2")},
				TokenCount: []matcher{tokenCount{Token: "tok", Count: 3}, tokenCount{Token: "tok2", Count: 4}},
				Result: "yes", Stickers: true, Valid: true},
			false,
		},
		"single-next": {`{"next": {"next": "yes", "stickers": true}}`,
			sourceDisplayDecider{Next: []sourceDisplayDecider{
				sourceDisplayDecider{Result: "yes", Stickers: true, Valid: true},
			}, Valid: true},
			false,
		},
		"multi-next": {`{"next": [{"next": "yes", "stickers": true}, {"next": "no", "stickers": true}]}`,
			sourceDisplayDecider{Next: []sourceDisplayDecider{
				sourceDisplayDecider{Result: "yes", Stickers: true, Valid: true},
				sourceDisplayDecider{Result: "no", Stickers: true, Valid: true},
			}, Valid: true},
			false,
		},
	}
	
	t.Run("Unmarshal", func(t *testing.T) {
		for k, v := range testcases_unmarshal {
			t.Run(k, func(t *testing.T) {
				var x sourceDisplayDecider
				err := json.Unmarshal([]byte(v.js), &x)
				if (err != nil) != v.err {
					t.Errorf("Unexpected error: got %v, did-expect-error: %t (%s)", err, v.err, v.js)
				} else if !v.err && !reflect.DeepEqual(x, v.expected) {
					t.Errorf("Unexpected result: got\n%+v\n, expected\n%+v\n (%s)", x, v.expected, v.js)
				}
			})
		}
	})
	
	testcases_matches := map[string]struct{
		in_obj sourceDisplayDecider
		in_url string
		out_title string
		out_match bool
		out_sticker bool
	}{
		"empty-catchall": {
			sourceDisplayDecider{Result: "Website!"},
			"http://test.website.com/webpage/foofoofoo/", "Website!", true, false,
		},
		"stickers": {
			sourceDisplayDecider{Result: "Website!", Stickers: true},
			"http://test.website.com/webpage/foofoofoo/", "Website!", true, true,
		},
		"nested": {
			sourceDisplayDecider{SubdomainOf: []matcher{subdomainMatcher("website.com")}, Next: []sourceDisplayDecider{
				sourceDisplayDecider{Hostname: []matcher{hostnameEqualityMatcher("test.website.com")}, Next: []sourceDisplayDecider{
					sourceDisplayDecider{PathPrefix: []matcher{pathPrefixMatcher("/webpage/")}, Next: []sourceDisplayDecider{
						sourceDisplayDecider{TokenCount: []matcher{tokenCount{Token: "foo", Count: 3}}, Result: "TestWebsite"},
					}},
				}},
			}},
			"http://test.website.com/webpage/foofoofoo/", "TestWebsite", true, false,
		},
		"compound": {
			sourceDisplayDecider{
				Hostname: []matcher{hostnameEqualityMatcher("test.website.com")},
				SubdomainOf: []matcher{subdomainMatcher("website.com")},
				PathPrefix: []matcher{pathPrefixMatcher("/webpage/")},
				TokenCount: []matcher{tokenCount{Token: "foo", Count: 3}},
				Result: "TestWebsite",
			},
			"http://test.website.com/webpage/foofoofoo/", "TestWebsite", true, false,
		},
		"fail1": {
			sourceDisplayDecider{
				Hostname: []matcher{hostnameEqualityMatcher("testx.website.com")},
				Result: "TestWebsite",
			},
			"http://test.website.com/webpage/foofoofoo/", "", false, false,
		},
		"fail2": {
			sourceDisplayDecider{
				SubdomainOf: []matcher{subdomainMatcher("otherwebsite.com")},
				Result: "TestWebsite",
			},
			"http://test.website.com/webpage/foofoofoo/", "", false, false,
		},
		"fail3": {
			sourceDisplayDecider{
				PathPrefix: []matcher{pathPrefixMatcher("/nonexistent/path")},
				Result: "TestWebsite",
			},
			"http://test.website.com/webpage/foofoofoo/", "", false, false,
		},
		"fail4": {
			sourceDisplayDecider{
				TokenCount: []matcher{tokenCount{Token: "missing", Count: 1000}},
				Result: "TestWebsite",
			},
			"http://test.website.com/webpage/foofoofoo/", "", false, false,
		},
		"fallback": {
			sourceDisplayDecider{
				Hostname: []matcher{hostnameEqualityMatcher("test.website.com")},
				Next: []sourceDisplayDecider{
					sourceDisplayDecider{PathPrefix: []matcher{pathPrefixMatcher("/nonexistent/path/")}, Result: "TestWebsite"},
					sourceDisplayDecider{Result: "TestWebsiteAlternate"},
				},
			},
			"http://test.website.com/webpage/foofoofoo/", "TestWebsiteAlternate", true, false,
		},
	}
	
	t.Run("Matches", func(t *testing.T) {
		for k, v := range testcases_matches {
			t.Run(k, func(t *testing.T) {
				title, match, sticker := v.in_obj.Matches(UrlMust(url.Parse(v.in_url)))
				if title != v.out_title || match != v.out_match || sticker != v.out_sticker {
					t.Errorf("Unexpected result: got %s %t %t, expected %s %t %t (%+v)", title, match, sticker, v.out_title, v.out_match, v.out_sticker, v.in_obj)
				}
			})
		}
	})
}
