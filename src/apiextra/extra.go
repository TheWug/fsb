package apiextra

import (
	"regexp"
	"strconv"
)

var urlmatcher *regexp.Regexp

type settings interface {
	GetApiEndpoint() string
}

func Init(s settings) error {
	var err error
	urlmatcher, err = regexp.Compile(`(https?://)?(www\.)?` + regexp.QuoteMeta(s.GetApiEndpoint()) + `/(posts|post/show)/(\d+)`)
	return err
}

func GetPostIDFromURLInText(text string) *int {
	matches := urlmatcher.FindStringSubmatch(text)
	if len(matches) == 5 && matches[4] != "" {
		i, e := strconv.Atoi(matches[4])
		if e == nil { return &i }
	}
	return nil
}
