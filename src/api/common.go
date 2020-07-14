package api

import (
        "github.com/thewug/reqtify"

	"errors"
        "fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
        "time"
)

// common state for the entire api package.

var ApiName          string
var Endpoint         string
var FilteredEndpoint string
var StaticPrefix     string

var userAgent string = "KnottyBot (telegram, v1.1, operator: snergal)"
var api reqtify.Reqtifier

type settings interface {
	GetApiName() string
	GetApiEndpoint() string
	GetApiFilteredEndpoint() string
	GetApiStaticPrefix() string
}

func Init(s settings) error {
	ApiName = s.GetApiName()
	Endpoint = s.GetApiEndpoint()
	FilteredEndpoint = s.GetApiFilteredEndpoint()
	StaticPrefix = s.GetApiStaticPrefix()

	if ApiName == "" || Endpoint == "" || FilteredEndpoint == "" || StaticPrefix == "" {
		return errors.New("missing required parameter")
	}

	api = reqtify.New(fmt.Sprintf("https://%s", Endpoint), time.NewTicker(750 * time.Millisecond), nil, nil, userAgent)

	apiurlmatch, err = regexp.Compile(fmt.Sprintf(`https?://(www\.)?(%s|%s)/(posts|post/show)/(\d+)`,
	            regexp.QuoteMeta(s.GetApiEndpoint()),
	            regexp.QuoteMeta(s.GetApiFilteredEndpoint())))

	return err
}

var apiurlmatch *regexp.Regexp
var ErrMalformedURL error = errors.New("Not a valid site URL")

// filters rating tags into valid rating letters.
// "clean" is not a valid rating, but for convenience, it is treated as identical to "safe".
func SanitizeRating(input string) (string, error) {
	input = strings.Replace(strings.ToLower(input), "rating:", "", -1)
	if input == "explicit" || input == "e" { return "e", nil }
	if input == "questionable" || input == "q" { return "q", nil }
	if input == "clean" || input == "c" || input == "safe" || input == "s" { return "s", nil }
	return "", errors.New("Invalid rating")
}

// filters ratings into valid rating letters, and the zero value to revert a change.
func SanitizeRatingForEdit(input string) (string, error) {
	input = strings.Replace(strings.ToLower(input), "rating:", "", -1)
	if input == "explicit" || input == "e" { return "e", nil }
	if input == "questionable" || input == "q" { return "q", nil }
	if input == "clean" || input == "c" || input == "safe" || input == "s" { return "s", nil }
	if input == "original" || input == "o" { return "", nil }
	return "", errors.New("Invalid rating")
}

func ApiURLToPostID(url string) (int, error) {
	submatches := apiurlmatch.FindStringSubmatch(url)
	if submatches == nil { return 0, ErrMalformedURL }
	return strconv.Atoi(submatches[len(submatches) - 1]) // return last subgroup
}

func APILog(url, user string, length int, response *http.Response, err error) {
	caller := "unauthenticated"
	if user != "" {
		caller = fmt.Sprintf("as %s", user)
	}

	httpstatus := "Request Failure"
	if response != nil {
		httpstatus = response.Status
	}

	lengthstr := ""
	if length >= 0 {
		lengthstr = fmt.Sprintf(", %d results", length)
	}

	if err == nil {
		log.Printf("[api     ] API call: %s [%s] (%s%s)\n", url, caller, httpstatus, lengthstr)
	} else {
		log.Printf("[api     ] API call: %s [%s] (%s: %s%s)", url, caller, httpstatus, err.Error(), lengthstr)
	}
}
