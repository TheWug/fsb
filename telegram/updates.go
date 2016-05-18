package telegram

import (
	"strconv"
	"net/http"
	"io/ioutil"
	"encoding/json"
	"log"
)

func GetUpdates() (updates []TUpdate, e error) {
	url := apiEndpoint + apiKey + "/getUpdates?" + 
	       "offset=" + strconv.Itoa(mostRecentlyReceived) +
	       "&timeout=3600"
	r, e := http.Get(url)
	log.Printf("[telegram] API call: %s (%s)\n", url, r.Status)

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

	var out TGenericResponse
	e = json.Unmarshal(b, &out)

	if e != nil {
		return
	}

	e = HandleSoftError(&out)
	if e != nil {
		return
	}

	e = json.Unmarshal(*out.Result, &updates)

	if e != nil {
		return
	}

	// track the next update to request
	if len(updates) != 0 {
		mostRecentlyReceived = updates[len(updates) - 1].Update_id + 1
		log.Printf("[telegram] Got %d updates (latest update is now %d)\n", len(updates), mostRecentlyReceived)
	}

	return
}
