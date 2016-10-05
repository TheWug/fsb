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

	if r != nil {
		defer r.Body.Close()
		if r.Status != "200" {
			log.Printf("[telegram] API error (http): %s (%s)\n", url, r.Status)
		}
	}
	if e != nil {
		return
	}

	b, e := ioutil.ReadAll(r.Body)
	if e != nil {
		log.Printf("[telegram] API error (read): %s (%s)\n", url, r.Status)
		return
	}

	var out TGenericResponse
	e = json.Unmarshal(b, &out)

	if e != nil {
		log.Printf("[telegram] API error (unmarshal): %s (%s)\n", url, r.Status)
		return
	}

	e = HandleSoftError(&out)
	if e != nil {
		return
	}

	e = json.Unmarshal(*out.Result, &updates)

	if e != nil {
		log.Printf("[telegram] API error (unmarshal): %s (%s)\n", url, r.Status)
		return
	}

	// track the next update to request
	if len(updates) != 0 {
		mostRecentlyReceived = updates[len(updates) - 1].Update_id + 1
		log.Printf("[telegram] Got %d updates (latest update is now %d)\n", len(updates), mostRecentlyReceived)
	}

	return
}
