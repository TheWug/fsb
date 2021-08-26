package botbehavior

import (
	"bot"
	"fmt"
	"fsb/proxify"
	"fsb/errorlog"
	"api"
	"telegram"
	"telegram/telebot"
	"strings"
	"log"
	"strconv"
)

func ShowHelp() {
	fmt.Println("CONFIGFILE options available:.")
	fmt.Println("  logfile     - controls the file to log to.")
	fmt.Println("  apikey      - sets the bot's telegram api token.")
	fmt.Println("  dburl       - sets the bot's telegram api token.")
	fmt.Println("  api_name              - the common, colloquial name of the api service.")
	fmt.Println("  api_endpoint          - the api endpoint hostname.")
	fmt.Println("  api_filtered_endpoint - the api SSF endpoint hostname.")
	fmt.Println("  api_static_prefix     - the api endpoint static resource hostname prefix/subdomain.")
}

type Behavior struct {
	Bot *telebot.TelegramBot
}

func (this *Behavior) ProcessCallback(callback telegram.TCallbackQuery) {
	bot.Handle(this.Bot, nil, &callback)
}

func (this *Behavior) ProcessMessage(message telegram.TMessage, edited bool) {
	bot.Handle(this.Bot, &message, nil)
}

// inline query, do tag search.
func (this *Behavior) ProcessInlineQuery(q telegram.TInlineQuery) {
	debugmode := strings.Contains(q.Query, "special:debugoutput")
	q.Query = strings.Replace(q.Query, "special:debugoutput", "", -1)
	var debugstr string
	if debugmode { debugstr = ", DEBUG" }
	log.Printf("[main    ] Received inline query (from %d %s%s): %s", q.From.Id, q.From.UsernameString(), debugstr, q.Query)
	offset := proxify.Offset(q.Offset)
	search_results, e := api.TagSearch(q.Query, offset, 50)
	errorlog.ErrorLog("api", "api.TagSearch", e)

	// take the suggestions we got from api and marshal them into inline query replies for telegram
	inline_suggestions := []interface{}{}

	if q.From.Id == 68060168 {
		for _, r := range search_results {
			new_result := proxify.ConvertApiResultToTelegramInline(r, proxify.ContainsSafeRatingTag(q.Query), q.Query, debugmode)

			if (new_result != nil) {
				inline_suggestions = append(inline_suggestions, new_result)
			}
		}
	} else {
		for _, r := range search_results {
			new_result := proxify.ConvertApiResultToTelegramInline(r, proxify.ContainsSafeRatingTag(q.Query), q.Query, false)

			if (new_result != nil) {
				inline_suggestions = append(inline_suggestions, new_result)
			}
		}
	}

	// send them out
	if len(inline_suggestions) != 0 {
		e = this.Bot.Remote.AnswerInlineQuery(q, inline_suggestions, strconv.FormatInt(int64(offset + 1), 10))
		errorlog.ErrorLog("telegram", "telegram.AnswerInlineQuery", e)
	}
}

func (this *Behavior) ProcessInlineQueryResult(r telegram.TChosenInlineResult) {
	log.Printf("[main    ] Inline selection: %s (by %d %s)\n", r.Result_id, r.From.Id, r.From.UsernameString())
}
