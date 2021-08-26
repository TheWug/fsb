package botbehavior

import (
	"github.com/thewug/gogram"
	"github.com/thewug/gogram/data"

	"fmt"
	"fsb/proxify"
	"fsb/errorlog"
	"api"
	apitypes "api/types"
	"strings"
	"log"
	"strconv"
	"storage"
	"errors"
	"api/tagindex"
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
	fmt.Println("  search_user      - api user with which unathenticated searches are performed.")
	fmt.Println("  search_apikey    - api key with which unathenticated searches are performed.")
	fmt.Println("  results_per_page - max number of telegram inline results to return at a time in searches.")
	fmt.Println("  owner               - numeric telegram user ID of bot operator.")
	fmt.Println("  no_results_photo_id - base64 telegram photo ID of 'no results' placeholder photo.")
	fmt.Println("  error_photo_id      - base64 telegram photo ID of 'error' placeholder photo.")
}

type Behavior struct {
	ForwardTo *gogram.MessageStateMachine
	MySettings Settings

	maintain chan bool
}

func (this *Behavior) GetInterval() int64 {
	return 600
}

func (this *Behavior) DoMaintenance(bot *gogram.TelegramBot) {
	if this.maintain == nil {
		this.maintain = this.StartMaintenanceAsync(bot)
		this.maintain <- true
	} else {
		select {
		case this.maintain <- true:
			// do nothing, the maintenance routine is now running async
			return
		default:
			bot.Log.Println("Skipping maintenance (backlogged?)")
		}
	}
}

func (this *Behavior) StartMaintenanceAsync(bot *gogram.TelegramBot) (chan bool) {
	channel := make(chan bool)
	go func() {
		for maintenances := 0; true; maintenances++ {
			_ = <- channel
			bot.Log.Println("Maintenance sync.")

			var err error
			extra_expensive := (maintenances % 144 == 143)
			settings := storage.UpdaterSettings{Full: false}
			settings.Transaction, err = storage.NewTxBox()
			if err != nil {
				bot.Log.Println("Error in maintenance loop:", err.Error())
			}

			tagindex.SyncPostsInternal(this.MySettings.SearchUser, this.MySettings.SearchAPIKey, settings, extra_expensive, extra_expensive, nil, nil)

			settings.Transaction.MarkForCommit()
			settings.Transaction.Finalize(true)
			bot.Log.Println("Maintenance sync complete.")
		}
	}()
	return channel
}

func (this *Behavior) ProcessCallback(ctx *gogram.CallbackCtx) {
	if ctx.Cb.Message == nil {
		return // message is too old, just do nothing for now.
	}

	m := *ctx.Cb.Message
	m.Text = ctx.Cb.Data
	this.ForwardTo.ProcessMessage(gogram.NewMessageCtx(&m, false, ctx.Bot))
	ctx.Bot.Remote.AnswerCallbackQuery(data.OCallback{Id: ctx.Cb.Id})
}

// inline query, do tag search.
func (this *Behavior) ProcessInlineQuery(ctx *gogram.InlineCtx) {
	debugmode := strings.Contains(ctx.Query.Query, "special:debugoutput") && (ctx.Query.From.Id == this.MySettings.Owner)
	ctx.Query.Query = strings.Replace(ctx.Query.Query, "special:debugoutput", "", -1)

	var debugstr string
	if debugmode { debugstr = ", DEBUG" }
	log.Printf("[behavior] Received inline query (from %d %s%s): %s", ctx.Query.From.Id, ctx.Query.From.UsernameString(), debugstr, ctx.Query.Query)

	user, apikey, _, err := storage.GetUserCreds(storage.UpdaterSettings{}, ctx.Query.From.Id)
	if err == storage.ErrNoLogin {
		user, apikey = this.MySettings.DefaultSearchCredentials()
	}

	var iqa data.OInlineQueryAnswer

	offset, err := proxify.Offset(ctx.Query.Offset)
	if err == nil {
		search_results, err := api.TagSearch(user, apikey, ctx.Query.Query, offset + 1, this.MySettings.ResultsPerPage)
		errorlog.ErrorLog(ctx.Bot.ErrorLog, "api", "api.TagSearch", err)
		iqa = this.ApiResultsToInlineResponse(ctx.Query.Query, search_results, offset, err, debugmode)
	} else {
		errorlog.ErrorLog(ctx.Bot.ErrorLog, "proxify", "proxify.Offset", errors.New(fmt.Sprintf("Bad Offset: %s (%s)", ctx.Query.Offset, err.Error())))
		iqa = this.ApiResultsToInlineResponse(ctx.Query.Query, nil, 0, err, debugmode)
	}

	ctx.AnswerAsync(iqa, nil)
}

func (this *Behavior) ApiResultsToInlineResponse(query string, search_results apitypes.TPostInfoArray, current_offset int, err error, debugmode bool) data.OInlineQueryAnswer {
	iqa := data.OInlineQueryAnswer{CacheTime: 30}
	if err != nil {
		if placeholder := this.GetErrorPlaceholder(); placeholder != nil {
			iqa.Results = append(iqa.Results, placeholder)
		}
	} else if len(search_results) == 0 && current_offset == 0 {
		if placeholder := this.GetNoResultsPlaceholder(query); placeholder != nil {
			iqa.Results = append(iqa.Results, placeholder)
		}
	} else if len(search_results) == this.MySettings.ResultsPerPage {
		iqa.NextOffset = strconv.FormatInt(int64(current_offset + 1), 10)
	}

	for _, r := range search_results {
		new_result := proxify.ConvertApiResultToTelegramInline(r, proxify.ContainsSafeRatingTag(query), query, debugmode)

		if (new_result != nil) {
			iqa.Results = append(iqa.Results, new_result)
		}
	}

	return iqa
}

func (this *Behavior) GetErrorPlaceholder() *data.TInlineQueryResultCachedPhoto {
	if this.MySettings.ErrorPhotoID == "" { return nil }
	return &data.TInlineQueryResultCachedPhoto{
		Type: "photo",
		Id: "no-results",
		PhotoId: this.MySettings.ErrorPhotoID,
		InputMessageContent: &data.TInputMessageTextContent{
			MessageText: "Oopsie woopsie, somebody did a fucky wucky!",
		},
	}
}

func (this *Behavior) GetNoResultsPlaceholder(query string) *data.TInlineQueryResultCachedPhoto {
	h := data.ParseHTML
	if this.MySettings.NoResultsPhotoID == "" { return nil }
	return &data.TInlineQueryResultCachedPhoto{
		Type: "photo",
		Id: "no-results",
		PhotoId: this.MySettings.NoResultsPhotoID,
		InputMessageContent: &data.TInputMessageTextContent{
			MessageText: fmt.Sprintf("There are no results on " + api.ApiName + " for <code>%s</code> :(", query),
			ParseMode: &h,
		},
	}
}

func (this *Behavior) ProcessInlineQueryResult(ctx *gogram.InlineResultCtx) {
	log.Printf("[behavior] Inline selection: %s (by %d %s)\n", ctx.Result.ResultId, ctx.Result.From.Id, ctx.Result.From.UsernameString())
}
