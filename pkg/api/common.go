package api

import (
        "github.com/thewug/reqtify"

	"github.com/thewug/fsb/pkg/api/types"

	"errors"
        "fmt"
	"log"
	"net/http"
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
	return nil
}

const DefaultBlacklist string = "gore\nscat\nwatersports\nyoung -rating:s\nloli\nshota"

// filters rating tags into valid rating letters.
// "clean" is not a valid rating, but for convenience, it is treated as identical to "safe".
func SanitizeRating(input string) (types.PostRating, error) {
	input = strings.Replace(strings.ToLower(input), "rating:", "", -1)
	if input == "explicit" || input == "e" { return types.Explicit, nil }
	if input == "questionable" || input == "q" { return types.Questionable, nil }
	if input == "clean" || input == "c" || input == "safe" || input == "s" { return types.Safe, nil }
	return types.Invalid, errors.New("Invalid rating")
}

// filters ratings into valid rating letters, and the zero value to revert a change.
func SanitizeRatingForEdit(input string) (types.PostRating, error) {
	input = strings.Replace(strings.ToLower(input), "rating:", "", -1)
	if input == "explicit" || input == "e" { return types.Explicit, nil }
	if input == "questionable" || input == "q" { return types.Questionable, nil }
	if input == "clean" || input == "c" || input == "safe" || input == "s" { return types.Safe, nil }
	if input == "original" || input == "o" { return types.Original, nil }
	return types.Invalid, errors.New("Invalid rating")
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

func LocationToURLWithRating(location string, rating types.PostRating) string {
	if rating == types.Safe {
		return fmt.Sprintf("https://%s%s", FilteredEndpoint, location)
	} else {
		return fmt.Sprintf("https://%s%s", Endpoint, location)
	}
}

func LocationToURL(location string) string {
	return LocationToURLWithRating(location, "e")
}
