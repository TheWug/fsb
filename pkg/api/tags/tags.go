package tags

import (
	"strings"
)

func WildcardMatch(wildcard, tag string) (bool) {
	tokens := strings.Split(wildcard, "*")
	lastfound := 0
	for i, t := range tokens {
		if t == "" { continue }
		current := strings.Index(tag[lastfound:], t)
		if current == -1 { return false }
		if i == 0 && current != 0 { return false }
		if i == len(tokens) - 1 && current + len(t) + lastfound != len(tag) { return false}
		lastfound = lastfound + current + len(t)
	}
	return true
}
