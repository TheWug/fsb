package api

import (
	"net/url"
	"strconv"
	"io/ioutil"
	"encoding/json"
	"log"
)

type FailedCall struct {
	Success bool `json:"success"`
}

func TagSearch(tags string, page int, limit int) (results []TSearchResult, e error) {
	url := apiEndpoint + "post/index.json?" +
	       "tags=" + url.QueryEscape(tags) +
	       "&page=" + strconv.Itoa(page) +
	       "&limit=" + strconv.Itoa(limit)

	r, e := apiGet(url)
	log.Printf("[api     ] API call: %s (%s)\n", url, r.Status)

	if r != nil {
		defer r.Body.Close()
	}
	if e != nil {
		return
	}

	b, e := ioutil.ReadAll(r.Body)

	if e != nil {
		return
	}

	e = json.Unmarshal(b, &results)

	if e != nil {
		return
	}

	log.Printf("[api     ] Returned %d results.\n", len(results))

	return
}

func TestLogin(user, apitoken string) (bool, error) {
	URL := apiEndpoint + "dmail/inbox.json"

	realurl := URL +
		"?login=" + url.QueryEscape(user) +
		"&password_hash=" + url.QueryEscape(apitoken)

	var canary interface{}
	r, e := apiGet(realurl)
	log.Printf("[api     ] API call: %s [as %s] (%s)\n", URL, user, r.Status)

	if r != nil {
		defer r.Body.Close()
	}
	if e != nil {
		return false, e
	}

	b, e := ioutil.ReadAll(r.Body)

	if e != nil {
		return false, e
	}

	e = json.Unmarshal(b, &canary)

	if e != nil {
		return false, e
	}

	switch v := canary.(type) {
	case map[string]interface{}:
		switch w := v["success"].(type) {
		case bool:
			return w, nil
		default:
			return true, nil
		} 
	default:
		return true, nil
	}
}
