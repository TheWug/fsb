package main

import (
	"telegram"
	"api"
	"fsb/proxify"
	"log"
	"fsb/errorlog"
	"fmt"
	"os"
	"io/ioutil"
	"encoding/json"
)

// This bot runs by polling telegram for updates, and then making synchronous calls to api. Because of the latency
// involved, it won't handle high query volumes very well.

// Config is read from a config file, which is passed as argument 1 (or ~/.fsb.json if none is specified)
// It should contain a json object and supports the following keys: "logfile", "apikey"

type FsbSettings struct {
	Logfile string `json:"logfile"`
	ApiKey  string `json:"apikey"`
	ApiName             string `json:"api_name"`
	ApiEndpoint         string `json:"api_endpoint"`
	ApiFilteredEndpoint string `json:"api_filtered_endpoint"`
	ApiStaticPrefix     string `json:"api_static_prefix"`
}

func (s FsbSettings) GetApiEndpoint() string {
	return s.ApiEndpoint
}

func (s FsbSettings) GetApiFilteredEndpoint() string {
	return s.ApiFilteredEndpoint
}

func (s FsbSettings) GetApiStaticPrefix() string {
	return s.ApiStaticPrefix
}

func main() {
	// handle command line arguments
	settingsFile := "~/.fsb.json"
	if len(os.Args) > 1 && os.Args[1] != "" {
		if os.Args[1] == "--help" || os.Args[1] == "-h" {
			fmt.Printf("Usage: %s [CONFIGFILE]\n", os.Args[0])
			fmt.Println("  CONFIGFILE  - Read this file for settings. (if omitted, use ~/.fsb.json)")
			fmt.Println("CONFIGFILE options available:.")
			fmt.Println("  logfile     - controls the file to log to.")
			fmt.Println("  apikey      - sets the bot's telegram api token.")
			fmt.Println("  api_name              - the common, colloquial name of the api service.")
			fmt.Println("  api_endpoint          - the api endpoint hostname.")
			fmt.Println("  api_filtered_endpoint - the api SSF endpoint hostname.")
			fmt.Println("  api_static_endpoint   - the api endpoint static resource hostname.")
			os.Exit(0)
		} else {
			settingsFile = os.Args[1]
		}
	}

	bytes, e := ioutil.ReadFile(settingsFile)
	errorlog.ErrorLogFatal("main", "main:main (read settings file)", e, 3)
	var settings FsbSettings
	e = json.Unmarshal(bytes, &settings)
	errorlog.ErrorLogFatal("main", "json.Unmarshal (settings file)", e, 4)
	bytes = []byte{}

	e = errorlog.RedirectLog(settings.Logfile)
	errorlog.ErrorLogFatal("main", "main:main (log redirection)", e, 1)

	e = api.Init(settings)
	errorlog.ErrorLogFatal("main", "main:main (api.Init)", e, 100)

	telegram.SetAPIKey(settings.ApiKey)

	e = telegram.Test()
	errorlog.ErrorLogFatal("main", "main:main (set api key): " + settings.ApiKey, e, 2)

	log.Println("Running main loop...")

	for {
		// grab the next batch of updates from telegram.
		updates, e := telegram.GetUpdates()
		errorlog.ErrorLog("telegram", "telegram.GetUpdates", e)
		for _, u := range updates {
			q := u.Inline_query
			r := u.Chosen_inline_result
			if q != nil {
				// if we got an inline query, do an api tag search.
				log.Printf("[main    ] Received inline query (from %s): %s", q.From.Username, q.Query)
				offset := proxify.Offset(q.Offset)

				search_results, e := api.TagSearch(q.Query, offset, 50)
				errorlog.ErrorLog("api", "api.TagSearch", e)

				// take the suggestions we got from api and marshal them into inline query replies for telegram
				inline_suggestions := []interface{}{}
				for _, r := range search_results {
					new_result := proxify.ConvertApiResultToTelegramInline(r, proxify.ContainsSafeRatingTag(q.Query), q.Query)

					if (new_result != nil) {
						inline_suggestions = append(inline_suggestions, new_result)
					}
				}

				// send them out
				e = telegram.AnswerInlineQuery(*q, inline_suggestions, offset + 1)
				errorlog.ErrorLog("telegram", "telegram.AnswerInlineQuery", e)
			}
			if r != nil {
				// snoop on users who actually pick results
				log.Printf("[main    ] Inline selection: %s (by %s)\n", r.Result_id, r.From.Username)
			}
		}
	}
}
