package telegram

import (
	"net/http"
	"log"
	"encoding/json"
	"io/ioutil"
	"errors"
)

// common state for the entire telegram package.

var apiEndpoint string = "https://api.telegram.org/bot"
var apiKey string = "CHANGEME"

var mostRecentlyReceived int = -25

func SetAPIKey(newKey string) () {
	apiKey = newKey
}

func Test() (error) {
	url := apiEndpoint + apiKey + "/getMe"
	r, e := http.Get(url)

	if r != nil {
		defer r.Body.Close()
		log.Printf("[telegram] API call: %s (%s)\n", url, r.Status)
	}
	if e != nil {
		return e
	}

	b, e := ioutil.ReadAll(r.Body)
	if e != nil {
		return e
	}

	var resp TGenericResponse
	e = json.Unmarshal(b, &resp)

	if e != nil {
		return e
	}

	e = HandleSoftError(&resp)
	if e != nil {
		return e
	}

	if resp.Result == nil {
		return errors.New("Missing required field (result)!")
	}

	var me TUser
	e = json.Unmarshal(*resp.Result, &me)

	if e != nil {
		return e
	}

	log.Printf("Validated API key (%s)\n", me.Username)
	return nil
}
