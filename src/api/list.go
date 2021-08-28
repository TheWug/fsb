package api

import (
	"net/http"
	"net/url"
	"strconv"
	"io/ioutil"
	"encoding/json"
	"log"
)

func TagSearch(tags string, page int, limit int) (results []TSearchResult, e error) {
	url := apiEndpoint + "/index.json?" +
	       "tags=" + url.QueryEscape(tags) +
	       "&page=" + strconv.Itoa(page) +
	       "&limit=" + strconv.Itoa(limit)

	r, e := http.Get(url)
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
