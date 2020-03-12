package api

import (
        "github.com/thewug/reqtify"
        "time"

	"strings"
	"net/http"
	"errors"
)

// common state for the entire api package.

var ApiName          string
var Endpoint         string
var FilteredEndpoint string
var StaticPrefix     string

var userAgent string = "KnottyBot (telegram, v1.0, operator: snergal)"
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
