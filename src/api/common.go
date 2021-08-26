package api

import (
	"strings"
	"net/http"
	"errors"
)

// common state for the entire api package.

var Endpoint         string
var FilteredEndpoint string
var StaticPrefix     string

var userAgent string = "KnottyBot (telegram, v1.0, operator: snergal)"

type settings interface {
	GetApiEndpoint() string
	GetApiFilteredEndpoint() string
	GetApiStaticPrefix() string
}

func Init(s settings) error {
	Endpoint = s.GetApiEndpoint()
	FilteredEndpoint = s.GetApiFilteredEndpoint()
	StaticPrefix = s.GetApiStaticPrefix()

	if Endpoint == "" || FilteredEndpoint == "" || StaticPrefix == "" {
		return errors.New("missing required parameter")
	}

	return nil
}

func SanitizeRating(input string) (string) {
	input = strings.Replace(strings.ToLower(input), "rating:", "", -1)
	if input == "explicit" || input == "e" { return "explicit" }
	if input == "questionable" || input == "q" { return "questionable" }
	if input == "safe" || input == "s" { return "safe" }
	return ""
}

var apiClient *http.Client = &http.Client{Transport: &http.Transport{} }

func apiGet(url string) (*http.Response, error) {
        req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

        req.Header.Set("User-Agent", userAgent)

	return apiClient.Do(req)
}

func apiDo(req *http.Request) (resp *http.Response, err error) {
        req.Header.Set("User-Agent", userAgent)
	return apiClient.Do(req)
}
