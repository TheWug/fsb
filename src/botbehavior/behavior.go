package botbehavior

import (
	"github.com/thewug/gogram"
	"github.com/thewug/gogram/data"

	"fmt"
	"fsb/proxify"
	"fsb/errorlog"
	"api"
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
	ForwardTo *gogram.MessageStateMachine
}

func (this *Behavior) ProcessCallback(ctx *gogram.CallbackCtx) {
	if ctx.Cb.Message == nil {
		return // message is too old, just do nothing for now.
	}

	m := *ctx.Cb.Message
	m.Text = ctx.Cb.Data
	this.ForwardTo.ProcessMessage(gogram.NewMessageCtx(&m, false, ctx.Bot))
	ctx.Bot.Remote.AnswerCallbackQuery(data.OCallback{QueryID: ctx.Cb.Id})
}

// inline query, do tag search.
func (this *Behavior) ProcessInlineQuery(ctx *gogram.InlineCtx) {
	debugmode := strings.Contains(ctx.Query.Query, "special:debugoutput")
	ctx.Query.Query = strings.Replace(ctx.Query.Query, "special:debugoutput", "", -1)
	var debugstr string
	if debugmode { debugstr = ", DEBUG" }
	log.Printf("[main    ] Received inline query (from %d %s%s): %s", ctx.Query.From.Id, ctx.Query.From.UsernameString(), debugstr, ctx.Query.Query)
	offset := proxify.Offset(ctx.Query.Offset)
	search_results, e := api.TagSearch(ctx.Query.Query, offset, 50)
	errorlog.ErrorLog("api", "api.TagSearch", e)

	// take the suggestions we got from api and marshal them into inline query replies for telegram
	inline_suggestions := []interface{}{}

	if ctx.Query.From.Id == 68060168 {
		for _, r := range search_results {
			new_result := proxify.ConvertApiResultToTelegramInline(r, proxify.ContainsSafeRatingTag(ctx.Query.Query), ctx.Query.Query, debugmode)

			if (new_result != nil) {
				inline_suggestions = append(inline_suggestions, new_result)
			}
		}
	} else {
		for _, r := range search_results {
			new_result := proxify.ConvertApiResultToTelegramInline(r, proxify.ContainsSafeRatingTag(ctx.Query.Query), ctx.Query.Query, false)

			if (new_result != nil) {
				inline_suggestions = append(inline_suggestions, new_result)
			}
		}
	}

	// send them out
	if len(inline_suggestions) != 0 {
		ctx.AnswerAsync(data.OInlineQueryAnswer{QueryID: ctx.Query.Id, Results: inline_suggestions, NextOffset: strconv.FormatInt(int64(offset + 1), 10), CacheTime: 30}, nil)
	}
}

func (this *Behavior) ProcessInlineQueryResult(ctx *gogram.InlineResultCtx) {
	log.Printf("[main    ] Inline selection: %s (by %d %s)\n", ctx.Result.Result_id, ctx.Result.From.Id, ctx.Result.From.UsernameString())
}
