package botbehavior

import (
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
	fmt.Println("  ownerid     - sets the bot's owner's account ID.")
	fmt.Println("  api_name              - the common, colloquial name of the api service.")
	fmt.Println("  api_endpoint          - the api endpoint hostname.")
	fmt.Println("  api_filtered_endpoint - the api SSF endpoint hostname.")
	fmt.Println("  api_static_prefix     - the api endpoint static resource hostname prefix/subdomain.")
}

type Behavior struct {
	ForwardTo *telebot.MessageStateMachine
}

func (this *Behavior) ProcessCallback(bot *telebot.TelegramBot, callback *telegram.TCallbackQuery) {
	var ctx telebot.MsgContext
	ctx.Bot = bot
	if callback.Data != nil {
		ctx.Cmd, ctx.CmdError = telebot.ParseCommandFromString(*callback.Data)
	}
	if callback.Message != nil {
		ctx.Msg.Chat = callback.Message.Chat
	}
	ctx.Msg.From = &callback.From
	ctx.Machine = this.ForwardTo
	this.ForwardTo.FeedContext(&ctx)
	bot.Remote.AnswerCallbackQuery(callback.Id, "", true)
}

// inline query, do tag search.
func (this *Behavior) ProcessInlineQuery(b *telebot.TelegramBot, q *telegram.TInlineQuery) {
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
		e = b.Remote.AnswerInlineQuery(*q, inline_suggestions, strconv.FormatInt(int64(offset + 1), 10))
		errorlog.ErrorLog("telegram", "telegram.AnswerInlineQuery", e)
	}
}

func (this *Behavior) ProcessInlineQueryResult(b *telebot.TelegramBot, r *telegram.TChosenInlineResult) {
	log.Printf("[main    ] Inline selection: %s (by %d %s)\n", r.Result_id, r.From.Id, r.From.UsernameString())
}
