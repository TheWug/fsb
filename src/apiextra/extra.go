package apiextra

import (
	"storage"

	"github.com/thewug/gogram/data"
	"api/types"

	"regexp"
	"strconv"
	"strings"
	"fmt"
)

const NONEXISTENT_POST = -1

const NONEXISTENT_PARENT = -2
const BLANK_PARENT = -1
const UNCHANGED_PARENT = 0

type settings interface {
	GetApiEndpoint() string
	GetApiFilteredEndpoint() string
	GetApiStaticPrefix() string
}

func Init(s settings) error {
	var err error
	apiurlmatch.fields = []int{5}
	apiurlmatch.regexp, err = regexp.Compile(fmt.Sprintf(`(https?://)?(www\.)?(%s|%s)/(posts|post/show)/(\d+)`,
	                                                     regexp.QuoteMeta(s.GetApiEndpoint()),
	                                                     regexp.QuoteMeta(s.GetApiFilteredEndpoint())))
	md5hashmatch.fields = []int{3,8}
	md5hashmatch.regexp, err = regexp.Compile(fmt.Sprintf(`((^|[^\w-])md5:([0-9A-Fa-f]{32})(\W|$)|(https?://)%s(%s|%s)/data/(\w+/)+([0-9A-Za-z]{32})\.\w+)`,
	                                                      regexp.QuoteMeta(s.GetApiStaticPrefix()),
	                                                      regexp.QuoteMeta(s.GetApiEndpoint()),
	                                                      regexp.QuoteMeta(s.GetApiFilteredEndpoint())))
	return err
}

type matcher struct {
	regexp *regexp.Regexp
	fields []int
}

// matches one string by one matcher.
// returns the first provided matching field that is non-empty, converted to an int.
// returns NONEXISTENT_POST if no match is found.
func (this matcher) Match(text string) int {
	x, err := strconv.Atoi(this.MatchString(text))
	if err == nil {
		return x
	}

	return NONEXISTENT_POST
}

// matches one string by one matcher.
// returns the first provided matching field that is non-empty
// returns the empty string if no match is found.
func (this matcher) MatchString(text string) string {
	matching := this.regexp.FindStringSubmatch(text)

	for _, field := range this.fields {
		if len(matching) >= field + 1 && matching[field] != "" {
			return matching[field]
		}
	}

	return ""
}

var apiurlmatch matcher

var numericmatch = matcher{
	regexp.MustCompile(`(^|[^\w-])(\d+)(\W|$)`),
	[]int{2},
}

var md5hashmatch matcher

// attempts to recover a post id from the specified text string.
// first, searches for a matching post url and returns its post number if present.
// second, searches for and returns a non-negative number not part of another word.
// returns NONEXISTENT_POST if no matches were found.
func GetPostIDFromText(text string) int {
	found := apiurlmatch.Match(text)

	if found == NONEXISTENT_POST {
		found = numericmatch.Match(text)
	}

	if found == NONEXISTENT_POST {
		md5 := md5hashmatch.MatchString(text)
		if md5 != "" {
			var post *types.TPostInfo
			var err error
			err = storage.DefaultTransact(func(tx storage.DBLike) error {
				post, err = storage.PostByMD5(tx, md5)
				return err
			})
			if post != nil && err == nil {
				found = post.Id
			}
		}
	}

	return found
}

// attempts to recover a post id from a telegram message.
// first, tries to match any URL in a url text entity.
// second, tries GetPostIDFromText on the full message plaintext.
// returns NONEXISTENT_POST if no matches were found.
func GetPostIDFromMessage(msg *data.TMessage) (int) {
	for _, entity := range msg.GetEntities() {
		if entity.Url != nil {
			found := apiurlmatch.Match(*entity.Url)
			if found != NONEXISTENT_POST { return found }
		}
	}

	return GetPostIDFromText(msg.PlainText())
}

// attempts to recover a post id from a string, for use as a parent post to another post.
// relies on GetPostIDFromText for this.
// also accepts the special value "none" to reset the parent's post to nothing, or "original" to leave the parent unchanged.
// returns a positive integer post id, BLANK_PARENT to indicate "none", or UNCHANGED_PARENT to indicate "original".
// if no post id could be discovered, returns NONEXISTENT_PARENT.
func GetParentPostFromText(text string) int {
	if text == "none" {
		return BLANK_PARENT
	} else if text == "original" {
		return UNCHANGED_PARENT
	} else {
		found := GetPostIDFromText(text)
		if found > 0 {
			return found
		}
	}

	return NONEXISTENT_PARENT
}

type Ratings struct {
	Safe, Questionable, Explicit bool
}

func (this Ratings) And(other Ratings) Ratings {
	this.Safe = this.Safe && other.Safe
	this.Questionable = this.Questionable && other.Questionable
	this.Explicit = this.Explicit && other.Explicit
	return this
}

func (this Ratings) RatingTag() string {
	if this.Safe {
		if this.Questionable {
			if this.Explicit {
				return ""		// SQE
			} else {
				return "-rating:e"	// SQ
			}
		} else {
			if this.Explicit {
				return "-rating:q"	// SE
			} else {
				return "rating:s"	// S
			}
		}
	} else {
		if this.Questionable {
			if this.Explicit {
				return "-rating:s"	// QE
			} else {
				return "rating:q"	// Q
			}
		} else {
			if this.Explicit {
				return "rating:e"	// E
			} else {
				return "id:<0"		// none
			}
		}
	}
}

var ws *regexp.Regexp = regexp.MustCompile(`\s+`)

func RatingsFromString(tags string) Ratings {
	var r Ratings = Ratings{true, true, true}
	for _, tag := range ws.Split(tags, -1) {
		if strings.HasPrefix(tag, "rating:") {
			switch {
			case strings.HasPrefix(tag[7:], "s"):
				r.Safe, r.Questionable, r.Explicit = true, false, false
			case strings.HasPrefix(tag[7:], "q"):
				r.Safe, r.Questionable, r.Explicit = false, true, false
			case strings.HasPrefix(tag[7:], "e"):
				r.Safe, r.Questionable, r.Explicit = false, false, true
			}
		} else if strings.HasPrefix(tag, "-rating:") {
			switch {
			case strings.HasPrefix(tag[8:], "s"):
				r.Safe, r.Questionable, r.Explicit = false, true, true
			case strings.HasPrefix(tag[8:], "q"):
				r.Safe, r.Questionable, r.Explicit = true, false, true
			case strings.HasPrefix(tag[8:], "e"):
				r.Safe, r.Questionable, r.Explicit = true, true, false
			}
		}
	}
	return r
}
