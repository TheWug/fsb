package proxify

import (
	"encoding/json"
	"errors"
	"html"
	"net/url"
	"strings"
)

var allDisplayDeciders []sourceDisplayDecider

type sett interface {
	GetSourceMap() json.RawMessage
}

func Init(s sett) error {
	return json.Unmarshal(s.GetSourceMap(), &allDisplayDeciders)
}

// expects json in format `["token", n]`
type tokenCount struct {
	Token string
	Count int
}

func (t *tokenCount) UnmarshalJSON(b []byte) error {
	var array [2]interface{}
	err := json.Unmarshal(b, &array)
	if err != nil { return err }

	var ok bool
	t.Token, ok = array[0].(string)
	if !ok { return errors.New("token was not a string") }

	f, ok := array[1].(float64)
	if !ok { return errors.New("token count was not int") }
	t.Count = int(f)
	return nil
}

func (t tokenCount) Matches(u *url.URL) bool {
	return strings.Count(u.EscapedPath(), t.Token) >= t.Count
}

type hostnameEqualityMatcher string
func (h hostnameEqualityMatcher) Matches(u *url.URL) bool { return string(h) == u.Hostname() }

type subdomainMatcher string
func (s subdomainMatcher) Matches(u *url.URL) bool { return u.Hostname() == string(s) || strings.HasSuffix(u.Hostname(), "." + string(s)) }

type pathPrefixMatcher string
func (p pathPrefixMatcher) Matches(u *url.URL) bool { return strings.HasPrefix(u.EscapedPath(), string(p)) }

type matcher interface {
	Matches(*url.URL) bool
}

type sourceDisplayDecider struct {
	Hostname    []matcher
	SubdomainOf []matcher
	PathPrefix  []matcher
	TokenCount  []matcher

	Result        string
	Next        []sourceDisplayDecider

	Stickers      bool
	Valid         bool
}

func anySucceeds(array []matcher, u *url.URL) bool {
	if len(array) == 0 { return true }
	for _, m := range array {
		if m.Matches(u) { return true }
	}
	return false
}

func (s *sourceDisplayDecider) Matches(u *url.URL) (string, bool, bool) {
	matches := anySucceeds(s.Hostname, u) && anySucceeds(s.SubdomainOf, u) && anySucceeds(s.PathPrefix, u) && anySucceeds(s.TokenCount, u)

	// if this node matches, and is a terminating node
	if matches {
		if len(s.Next) == 0 {
			return s.Result, matches, s.Stickers
		} else {
			for _, ns := range s.Next {
				if label, ok, sticker := ns.Matches(u); ok {
					return label, ok, sticker
				}
			}
		}
	}

	return "", false, false
}

func readStringArrayFromSingleOrMultiple(m json.RawMessage, single interface{}, multiple interface{}) error {
	if len(m) == 0 {
		return nil
	}

	err1 := json.Unmarshal(m, single)
	err2 := json.Unmarshal(m, multiple)

	if err1 != nil && err2 != nil {
		return err1
	}
	return nil
}

func (t *sourceDisplayDecider) UnmarshalJSON(b []byte) error {
	type X struct {
		Hostname    json.RawMessage `json:"hostname"`
		SubdomainOf json.RawMessage `json:"subdomain_of"`
		PathPrefix  json.RawMessage `json:"path_prefix"`
		TokenCount  json.RawMessage `json:"token_count"`

		Next        json.RawMessage `json:"next"`
		Stickers    bool            `json:"stickers"`
	}

	t.Valid = false

	var temp sourceDisplayDecider
	var x X

	err := json.Unmarshal(b, &x)
	if err != nil { return err }

	h, ha := hostnameEqualityMatcher(""), []hostnameEqualityMatcher(nil)
	err = readStringArrayFromSingleOrMultiple(x.Hostname, &h, &ha)
	if err != nil { return err }
	if len(ha) == 0 && len(h) != 0 { ha = append(ha, h) }
	for _, h := range ha { temp.Hostname = append(temp.Hostname, h) }

	s, sa := subdomainMatcher(""), []subdomainMatcher(nil)
	err = readStringArrayFromSingleOrMultiple(x.SubdomainOf, &s, &sa)
	if err != nil { return err }
	if len(sa) == 0 && len(s) != 0 { sa = append(sa, s) }
	for _, s := range sa { temp.SubdomainOf = append(temp.SubdomainOf, s) }

	p, pa := pathPrefixMatcher(""), []pathPrefixMatcher(nil)
	err = readStringArrayFromSingleOrMultiple(x.PathPrefix, &p, &pa)
	if err != nil { return err }
	if len(pa) == 0 && len(p) != 0 { pa = append(pa, p) }
	for _, p := range pa { temp.PathPrefix = append(temp.PathPrefix, p) }

	tc, tca := tokenCount{}, []tokenCount(nil)
	err = readStringArrayFromSingleOrMultiple(x.TokenCount, &tc, &tca)
	if err != nil { return err }
	if len(tc.Token) != 0 { tca = append([]tokenCount(nil), tc) }
	for _, tc := range tca { temp.TokenCount = append(temp.TokenCount, tc) }

	var result string
	if x.Next == nil {
	} else if json.Unmarshal(x.Next, &result) != nil {
		var dd    sourceDisplayDecider
		var dda []sourceDisplayDecider
		err = readStringArrayFromSingleOrMultiple(x.Next, &dd, &dda)
		if err != nil { return err }
		if len(dda) == 0 && dd.Valid { dda = append(dda, dd) }
		temp.Next = dda
	} else {
		temp.Result = html.EscapeString(result)
	}

	temp.Stickers = x.Stickers
	temp.Valid = true
	*t = temp
	return nil
}
