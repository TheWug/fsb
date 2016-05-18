package main

import (
	"telegram"
	"api"
	"fsb/proxify"
	"log"
	"fsb/errorlog"
	"fmt"
	"os"
)

// This bot runs by polling telegram for updates, and then making synchronous calls to api. Because of the latency
// involved, it won't handle high query volumes very well.

// sensible defaults.
// This is the api key for WugTestBot, so that's the bot it will attach to if you run without passing an api key.
// Also, if that api key is ever revoked, this will need to be updated or the default configuration will not work.

func main() {
	// handle command line arguments
	if len(os.Args) > 1 && os.Args[1] != "" {
		if os.Args[1] == "--help" || os.Args[1] == "-h" {
			fmt.Printf("Usage: %s [LOGFILE] [APIKEY]\n", os.Args[0])
			fmt.Println("  LOGFILE - Name of a file to write logs to. If it already exists, it will be appended to.")
			fmt.Println("  APIKEY  - Telegram api key for this bot.  This determines which user the bot will act as (default is @WugTestBot)")
			os.Exit(0)
		}
		e := errorlog.RedirectLog(os.Args[1])
		errorlog.ErrorLogFatal("main", "main:main (log redirection)", e, 1)
	}
	if len(os.Args) > 2 {
		telegram.SetAPIKey(os.Args[2])
	}

	e := telegram.Test()
	errorlog.ErrorLogFatal("main", "main:main (set api key)", e, 2)

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
				search_results, e := api.TagSearch(q.Query, offset, 10)
				errorlog.ErrorLog("api", "api.TagSearch", e)

				// take the suggestions we got from api and marshal them into inline query replies for telegram
				inline_suggestions := []interface{}{}
				for _, r := range search_results {
					new_result := proxify.ConvertApiResultToTelegramInline(r, proxify.ContainsSafeRatingTag(q.Query))

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
