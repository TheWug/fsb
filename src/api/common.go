package api

import (
        "github.com/thewug/reqtify"

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

func SanitizeRating(input string) (string) {
	input = strings.Replace(strings.ToLower(input), "rating:", "", -1)
	if input == "explicit" || input == "e" { return "e" }
	if input == "questionable" || input == "q" { return "q" }
	if input == "safe" || input == "s" { return "s" }
	return ""
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
