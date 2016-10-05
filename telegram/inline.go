package telegram

import (
	"strconv"
	"net/http"
	"net/url"
	"io/ioutil"
	"encoding/json"
	"log"
)

func AnswerInlineQuery(q TInlineQuery, out []interface{}, last_offset int) (e error) {
	b, e := json.Marshal(out)
	if e != nil {
		return
	}

	// next_offset should get stuck at -1 forever if pagination breaks somehow, to prevent infinite loops.
	next_offset := ""
	if last_offset == -1 {
		next_offset = "-1"
	} else {
		next_offset = strconv.Itoa(last_offset + 1)
	}

	surl := apiEndpoint + apiKey + "/answerInlineQuery?" +
	                 "inline_query_id=" + url.QueryEscape(q.Id) +
	                 "&next_offset=" + next_offset +
	                 "&cache_time=30" +
	                 "&results="

	r, e := http.Get(surl + url.QueryEscape(string(b)))

	if r != nil {
		defer r.Body.Close()
		log.Printf("[telegram] API call: %s (%s)\n", surl + "[snip]", r.Status)
	}
	if e != nil {
		return
	}

	b, e = ioutil.ReadAll(r.Body)
	if e != nil {
		return
	}

	var resp TGenericResponse
	e = json.Unmarshal(b, &resp)

	if e != nil {
		return
	}

	e = HandleSoftError(&resp)
	if e != nil {
		return
	}

	log.Printf("[telegram] Pushed %d inline query results (id: %s)", len(out), q.Id)

	return
}

