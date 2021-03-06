package botbehavior

import (
	"github.com/thewug/fsb/pkg/botbehavior/settings"
	bottypes "github.com/thewug/fsb/pkg/bot/types"
	"github.com/thewug/fsb/pkg/api"
	"github.com/thewug/fsb/pkg/api/tags"
	"github.com/thewug/fsb/pkg/api/tagindex"
	apitypes "github.com/thewug/fsb/pkg/api/types"
	"github.com/thewug/fsb/pkg/apiextra"
	"github.com/thewug/fsb/pkg/fsb/errorlog"
	"github.com/thewug/fsb/pkg/fsb/proxify"
	"github.com/thewug/fsb/pkg/storage"

	"github.com/thewug/gogram"
	"github.com/thewug/gogram/data"

	"bytes"
	"errors"
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"
)

func ShowHelp() {
	fmt.Println("CONFIGFILE options available:.")
	fmt.Println("  logfile     - controls the file to log to.")
	fmt.Println("  pidfile     - controls the daemon pid file name.")
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
	fmt.Println("  max_artists      - max number of artists an inline result can include.")
	fmt.Println("  max_chars        - max number of characters an inline result can include.")
	fmt.Println("  max_sources      - max number of sources an inline result can include.")
	fmt.Println("  owner - numeric telegram user ID of bot operator.")
	fmt.Println("  home  - numeric telegram chat ID of bot's service chat.")
	fmt.Println("  no_results_photo_id  - base64 telegram photo ID of 'no results' placeholder photo.")
	fmt.Println("  blacklisted_photo_id - base64 telegram photo ID of 'all results blacklisted' placeholder photo.")
	fmt.Println("  error_photo_id       - base64 telegram photo ID of 'error' placeholder photo.")
	fmt.Println("  media_convert_directory   - conversion directory for webm -> mp4 conversions.")
	fmt.Println("  webm2mp4_convert_script   - script to convert webms into mp4s.")
	fmt.Println("  media_store_channel       - numeric telegram chat ID of channel to use for converted media storage.")
	fmt.Println("  maintenance_sync_interval - number of seconds between automatic api post syncs.")
	fmt.Println("  debug_media_received      - helper flag, show media ids of incoming photos (useful for setting *_photo_id settings).")
	fmt.Println("  source_map   - a json array of match rules which control how to format sources.")
	fmt.Println("                 rules have the following keys:")
	fmt.Println("                   hostname, subdomain_of, path_prefix - strings, or arrays of strings")
	fmt.Println("                   token_count - arrays of the form [\"token\", N], or arrays of those")
	fmt.Println("                   stickers - true to use this as a sticker pack link, which is displayed specially")
	fmt.Println("                   next - either a string (to use this label), or one or more match rules to be evaluated recursively")
}

type Behavior struct {
	ForwardTo *gogram.MessageStateMachine
	MySettings settings.Settings

	maintain chan bool
}

func (this *Behavior) GetInterval() int64 {
	return int64(this.MySettings.MaintenanceSyncInterval)
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

			err := storage.DefaultTransact(func(tx storage.DBLike) error { return this.maintenanceInternal(tx, bot, maintenances % 144 == 143) })
			if err != nil {
				bot.ErrorLog.Println("Error during maintenance routine:", err)
			}
		}
	}()
	return channel
}

func (this *Behavior) maintenanceInternal(tx storage.DBLike, bot *gogram.TelegramBot, extra_expensive bool) error {
	var err error

	update_chan := make(chan []apitypes.TPostInfo)
	var updated_post_ids []int
	updated_posts := make(map[int]apitypes.TPostInfo)

	go func() {
		for posts := range update_chan {
			for _, p := range posts {
				if !p.Deleted { // skip deleted posts since they can't be edited.
					updated_post_ids = append(updated_post_ids, p.Id)
					updated_posts[p.Id] = p
				}
			}
		}
		close(update_chan)
	}()

	err = tagindex.SyncPostsInternal(tx, this.MySettings.SearchUser, this.MySettings.SearchAPIKey, extra_expensive, extra_expensive, nil, update_chan)
	if err != nil { return err }

	edits := make(map[int]*storage.PostSuggestedEdit)

	page_channel := storage.PaginatedPostsById(tx, updated_post_ids, 10000)
	for page := range page_channel {
		if page.Err != nil { return page.Err }
		type shim struct {
			post *apitypes.TPostInfo
			metadata tags.TagSet
		}

		posts_and_stuff := make(map[int]shim)

		for i, _ := range page.Posts {
			posts_and_stuff[page.Posts[i].Id] = shim{post: &page.Posts[i], metadata: page.Posts[i].ExtendedTagSet()}
		}

		replacement_history, err := storage.GetReplacementHistorySince(tx, page.Page, time.Now().Add(-1 * 7 * 24 * time.Hour))
		if err != nil {	return err }

		replacement_chan := storage.PaginatedGetAllReplacements(tx, 1000)
		for replacements := range replacement_chan {
			if replacements.Err != nil { return replacements.Err }
			for _, r := range replacements.Replacers {
				m := r.Matcher()
				for id, sh := range posts_and_stuff {
					if _, ok := replacement_history[storage.ReplacementHistoryKey{ReplacerId: r.Id, PostId: id}]; ok { continue }
					if m.Matches(sh.metadata) {
						if edits[id] == nil {
							edits[id] = &storage.PostSuggestedEdit{}
						}

						edits[id].Represents = append(edits[id].Represents, r.Id)
						to := &edits[id].Prompt
						if r.Autofix {
							to = &edits[id].AutoFix
						}
						*to = append(*to, m.ReplaceSpec)
					}
				}
			}
		}
	}

	default_creds := this.MySettings.DefaultSearchCredentials()

	for id, edit := range edits {
		edit.SelectAutofix()
		auto_diff := edit.GetChangeToApply()
		if !auto_diff.IsZero() {
			post, err := api.UpdatePost(default_creds.User, default_creds.ApiKey, id, auto_diff, apitypes.Original, nil, nil, nil, sptr("Automatic tag cleanup: typos and concatenations (via KnottyBot)"))
			if err != nil {
				bot.ErrorLog.Println("Error updating post:", err.Error())
			} else {
				edit.Apply()
				var applied_api []string
				for k, _ := range edit.AppliedEdits { applied_api = append(applied_api, k) }

				for _, replacerId := range edit.Represents {
					storage.AddReplacementHistory(
						tx,
						&storage.ReplacementHistory{
							ReplacementHistoryKey: storage.ReplacementHistoryKey{ReplacerId: replacerId, PostId: id},
							TelegramUserId: -1,
							Timestamp: time.Now(),
						},
					)
				}

				if post != nil {
					err = storage.UpdatePost(tx, *post)
					if err != nil { return err }
				}
			}
		}

		// generate a prompt post, or find an existing one and edit it
		post_info, err := storage.FindPromptPost(tx, id)
		if err != nil { return err }
		post := updated_posts[id]
		post_info = this.PromptPost(bot, post_info, id, &post, edit)
		time.Sleep(4 * time.Second) // avoid rate limiting in telegram message sending
		err = storage.SavePromptPost(tx, id, post_info)
		if err != nil { return err }
	}

	return nil
}

func ternary(b bool, x, y string) string {
	if b { return x }
	return y
}

func sptr(s string) *string {
	return &s
}

func GetInlineKeyboardForEdit(edit *storage.PostSuggestedEdit) (*data.TInlineKeyboard) {
	const DIAMOND string = "\U0001F539"
	const RED_DOT string = "\U0001F534"
	const GREEN_DOT string = "\U0001F7E2"

	var keyboard data.TInlineKeyboard
	keyboard.Buttons = append(keyboard.Buttons, []data.TInlineKeyboardButton{data.TInlineKeyboardButton{
		Text: DIAMOND + " Commit",
		Data: sptr("/af-commit"),
	},data.TInlineKeyboardButton{
		Text: DIAMOND + " Dismiss",
		Data: sptr("/af-dismiss"),
	}})
	set := len(edit.AutoFix) + len(edit.Prompt) == len(edit.SelectedEdits)
	keyboard.Buttons = append(keyboard.Buttons, []data.TInlineKeyboardButton{data.TInlineKeyboardButton{
		Text: fmt.Sprintf("%s all", ternary(set, strings.Repeat(RED_DOT, 3) + " Clear", strings.Repeat(GREEN_DOT, 3) + " Apply")),
		Data: sptr(fmt.Sprintf("/af-toggle everything - %s", ternary(len(edit.AutoFix) + len(edit.Prompt) == len(edit.SelectedEdits), "0", "1"))),
	}})
	for i, diff := range edit.AutoFix {
		api_string := diff.APIString()
		_, set := edit.SelectedEdits[api_string]
		keyboard.Buttons = append(keyboard.Buttons, []data.TInlineKeyboardButton{data.TInlineKeyboardButton{
			Text: fmt.Sprintf("%s %s", ternary(set, GREEN_DOT, RED_DOT), api_string),
			Data: sptr(fmt.Sprintf("/af-toggle autofix %d %s", i, ternary(set, "0", "1"))),
		}})
	}
	for i, diff := range edit.Prompt {
		api_string := diff.APIString()
		_, set := edit.SelectedEdits[api_string]
		keyboard.Buttons = append(keyboard.Buttons, []data.TInlineKeyboardButton{data.TInlineKeyboardButton{
			Text: fmt.Sprintf("%s %s", ternary(set, GREEN_DOT, RED_DOT), api_string),
			Data: sptr(fmt.Sprintf("/af-toggle prompt %d %s", i, ternary(set, "0", "1"))),
		}})
	}

	return &keyboard
}

func (this *Behavior) PromptPost(bot *gogram.TelegramBot, post_info *storage.PromptPostInfo, id int, post *apitypes.TPostInfo, edit *storage.PostSuggestedEdit) (*storage.PromptPostInfo) {
	create_mode := (post_info == nil)

	if create_mode {
		// do nothing if we are trying to create from scratch an empty prompt post, as there are no changes to make.
		if edit == nil { return nil }

		post_info = &storage.PromptPostInfo{
			PostId: post.Id,
			PostType: post.File_ext,
			PostURL: post.File_url,
			SampleURL: post.Sample_url,
			PostMd5: post.Md5,
			PostWidth: post.Width,
			PostHeight: post.Height,
			Edit: edit,
		}
	} else if edit != nil {
		*post_info.Edit = post_info.Edit.Append(*edit)
	}

	send := data.SendData{
		TargetData: data.TargetData{
			ChatId: this.MySettings.Home,
		},
		ParseMode: data.ParseHTML,
		DisableNotification: true,
		ReplyMarkup: GetInlineKeyboardForEdit(post_info.Edit),
	}

	// figure out what we should say for what tags we're changing
	var edit_string bytes.Buffer
	if len(post_info.Edit.Prompt) != 0 {
		edit_string.WriteString(fmt.Sprintf("Manual fixes available:\n<pre>%s</pre>\n", html.EscapeString(post_info.Edit.Prompt.Flatten().APIString())))
	}
	if len(post_info.Edit.AutoFix) != 0 {
		edit_string.WriteString(fmt.Sprintf("Automatic fixes applied:\n<pre>%s</pre>\n", html.EscapeString(post_info.Edit.AutoFix.Flatten().APIString())))
	}

	// figure out what the message should be
	if post_info.PostType == "png" || post_info.PostType == "jpg" {
		send.Text = fmt.Sprintf("Post ID <pre>%d</pre>\n<a href=\"%s\">Image</a>\n<a href=\"https://" + api.Endpoint + "/posts/%d\">Post</a>\n%s", post_info.PostId, post_info.PostURL, post_info.PostId, edit_string.String())
	} else if post_info.PostType == "gif" {
		send.Text = fmt.Sprintf("Post ID <pre>%d</pre>\n<a href=\"%s\">Animation</a>\n<a href=\"https://" + api.Endpoint + "/posts/%d\">Post</a>\n%s", post_info.PostId, post_info.PostURL, post_info.PostId, edit_string.String())
	} else if post_info.PostType == "webm" {
		send.Text = fmt.Sprintf("Post ID <pre>%d</pre> (%s file, no preview available)\nView it using the links below.\n\n<a href=\"%s\">Video</a>\n<a href=\"https://" + api.Endpoint + "/posts/%d\">Post</a>\n%s", post_info.PostId, post_info.PostType, post_info.PostURL, post_info.PostId, edit_string.String())
	} else { // SWF, or any other unrecognized file type
		send.Text = fmt.Sprintf("Post ID <pre>%d</pre> (%s file, no preview available)\nView it using the links below.\n\n<a href=\"%s\">File</a>\n<a href=\"https://" + api.Endpoint + "/posts/%d\">Post</a>\n%s", post_info.PostId, post_info.PostType, post_info.PostURL, post_info.PostId, edit_string.String())
	}

	if !create_mode {
		if len(post_info.Edit.Prompt) + len(post_info.Edit.AutoFix) == 0 {
			bot.Remote.DeleteMessageAsync(data.ODelete{SourceChatId: post_info.ChatId, SourceMessageId: post_info.MsgId}, nil)
			return nil
		}

		source := data.SourceData{
			SourceChatId: post_info.ChatId,
			SourceMessageId: post_info.MsgId,
		}

		if post_info.Captioned {
			bot.Remote.EditMessageCaptionAsync(data.OCaptionEdit{
				SendData: send,
				SourceData: source,
			}, nil)
		} else {
			bot.Remote.EditMessageTextAsync(data.OMessageEdit{
				SendData: send,
				SourceData: source,
				DisableWebPagePreview: true,
			}, nil)
		}
	} else {
		if len(post_info.Edit.Prompt) + len(post_info.Edit.AutoFix) == 0 { return nil }

		var message *data.TMessage
		var err error
		if post_info.PostType == "png" || post_info.PostType == "jpg" {
			message, err = bot.Remote.SendPhoto(data.OPhoto{
				SendData: send,
				MediaData: data.MediaData{
					File: post_info.SampleURL,
					FileName: fmt.Sprintf("%s.%s", post_info.PostMd5, post_info.PostType),
				},
			})
		} else if post_info.PostType == "gif" {
			message, err = bot.Remote.SendAnimation(data.OAnimation{
				SendData: send,
				MediaData: data.MediaData{
					File: post_info.PostURL,
					FileName: fmt.Sprintf("%s.%s", post_info.PostMd5, post_info.PostType),
				},
				ResolutionData: data.ResolutionData{
					Width: post_info.PostWidth,
					Height: post_info.PostHeight,
				},
			})
		} else {
			message, err = bot.Remote.SendMessage(data.OMessage{
				SendData: send,
				DisableWebPagePreview: true,
			})
		}

		if err != nil {
			bot.ErrorLog.Println("Couldn't post message in PromptPost:", err.Error())
			return nil
		}

		post_info.MsgId = message.Id
		post_info.ChatId = message.Chat.Id
		post_info.Captioned = (message.Text == nil)
		post_info.Timestamp = time.Unix(message.Date, 0)
	}
	return post_info
}

func (this *Behavior) DismissPromptPost(tx storage.DBLike, bot *gogram.TelegramBot, post_info *storage.PromptPostInfo, diff tags.TagDiff) error {
	if post_info == nil { return nil }

	bot.Remote.DeleteMessageAsync(data.ODelete{SourceChatId: post_info.ChatId, SourceMessageId: post_info.MsgId}, nil)

	if !diff.IsZero() || len(post_info.Edit.AutoFix) > 0 {
		api_string := diff.APIString()
		edit_string := fmt.Sprintf("Applied the following tags:\n<pre>%s</pre>", ternary(len(api_string) != 0, html.EscapeString(api_string), "[no changes made]"))
		message := ""
		// figure out what the message should be
		if post_info.PostType == "png" || post_info.PostType == "jpg" {
			message = fmt.Sprintf("Post ID %d\n<a href=\"%s\">Image</a>\n<a href=\"https://" + api.Endpoint + "/posts/%d\">Post</a>\n%s", post_info.PostId, post_info.PostURL, post_info.PostId, edit_string)
		} else if post_info.PostType == "gif" {
			message = fmt.Sprintf("Post ID %d\n<a href=\"%s\">Animation</a>\n<a href=\"https://" + api.Endpoint + "/posts/%d\">Post</a>\n%s", post_info.PostId, post_info.PostURL, post_info.PostId, edit_string)
		} else if post_info.PostType == "webm" {
			message = fmt.Sprintf("Post ID %d (%s file, no preview available)\nView it using the links below.\n\n<a href=\"%s\">Video</a>\n<a href=\"https://" + api.Endpoint + "/posts/%d\">Post</a>\n%s", post_info.PostId, post_info.PostType, post_info.PostURL, post_info.PostId, edit_string)
		} else { // SWF, or any other unrecognized file type
			message = fmt.Sprintf("Post ID %d (%s file, no preview available)\nView it using the links below.\n\n<a href=\"%s\">File</a>\n<a href=\"https://" + api.Endpoint + "/posts/%d\">Post</a>\n%s", post_info.PostId, post_info.PostType, post_info.PostURL, post_info.PostId, edit_string)
		}

		bot.Remote.SendMessageAsync(data.OMessage{SendData: data.SendData{TargetData: data.TargetData{ChatId: post_info.ChatId}, Text: message, ParseMode: data.ParseHTML, DisableNotification: true}, DisableWebPagePreview: true}, nil)
	}

	return storage.SavePromptPost(tx, post_info.PostId, nil)
}

func (this *Behavior) ClearPromptPostsOlderThan(bot *gogram.TelegramBot, time_ago time.Duration) error {
	return storage.DefaultTransact(func(tx storage.DBLike) error {
		post_infos, err := storage.FindPromptPostsOlderThan(tx, time_ago)
		if err != nil { return err }

		for _, post_info := range post_infos {
			err = this.DismissPromptPost(tx, bot, &post_info, tags.TagDiff{})
			if err != nil { return err }
		}

		return nil
	})
}

type QuerySettings struct {
	debugmode      bool
	resultsperpage int
	settingsbutton string
}

func (this *Behavior) ProcessMessage(ctx *gogram.MessageCtx) {
	if this.MySettings.DebugMediaReceived {
		if ctx.Msg.Photo != nil {
			ctx.Bot.Log.Printf("Photo: %+v\n", ctx.Msg.Photo)
		}
	}

	this.ForwardTo.ProcessMessage(ctx)
}

// inline query, do tag search.
func (this *Behavior) ProcessInlineQuery(ctx *gogram.InlineCtx) {
	var q QuerySettings
	q.resultsperpage = this.MySettings.ResultsPerPage

	var new_query []string
	for _, tok := range(strings.Split(ctx.Query.Query, " ")) {
		// tokens which have the fsbdebug: prefix, or which are blank, are stripped from the query
		if strings.HasPrefix(strings.ToLower(tok), "fsbdebug:") {
			ctx.Bot.Log.Println(tok)
			tok = tok[len("fsbdebug:"):] // blindly chop prefix off, since it's case insensitive
			if strings.ToLower(tok) == "postdetails" {
				q.debugmode = (ctx.Query.From.Id == this.MySettings.Owner)
			} else if strings.HasPrefix(strings.ToLower(tok), "override:") {
				tok = tok[len("override:"):] // blindly chop prefix off, since it's case insensitive
				toks := strings.SplitN(tok, ":", 2)
				for len(toks) < 2 { toks = append(toks, "") }
				if strings.ToLower(toks[0]) == "resultsperpage" {
					if n, e := strconv.Atoi(toks[1]); e == nil && ctx.Query.From.Id == this.MySettings.Owner { q.resultsperpage = n }
				}
			}
		} else if strings.TrimSpace(tok) == "" {
		} else {
			new_query = append(new_query, tok)
		}
	}
	ctx.Query.Query = strings.Join(new_query, " ")

	var debugstr string
	if q.debugmode { debugstr = ", DEBUG" }
	ctx.Bot.Log.Printf("[behavior] Received inline query (from %d %s%s): %s", ctx.Query.From.Id, ctx.Query.From.UsernameString(), debugstr, ctx.Query.Query)

	var creds storage.UserCreds
	creds, err := storage.GetUserCreds(nil, ctx.Query.From.Id)
	if err == storage.ErrNoLogin {
		creds = this.MySettings.DefaultSearchCredentials()
	} else if err != nil {
		ctx.Bot.ErrorLog.Println("Error reading credentials: ", err.Error())
	}

	var settings *storage.UserSettings
	err = storage.DefaultTransact(func(tx storage.DBLike) error { settings, err = storage.GetUserSettings(tx, ctx.Query.From.Id); return err })

	var blacklist string
	if settings.BlacklistMode == bottypes.BLACKLIST_ON {
		if now := time.Now(); creds.BlacklistFetched.Add(time.Hour).Before(now) {
			user, success, err := api.TestLogin(creds.User, creds.ApiKey)
			if success {
				creds.Blacklist = user.Blacklist
				creds.BlacklistFetched = now
			} else if err != nil {
				ctx.Bot.ErrorLog.Println("Error testing login: ", err.Error())
			}
			err = storage.DefaultTransact(func(tx storage.DBLike) error { return storage.WriteUserCreds(tx, creds) })
			if err != nil {
				ctx.Bot.ErrorLog.Println("Error writing credentials: ", err.Error())
			}
		}

		blacklist = creds.Blacklist
	}

	allowed_ratings := apiextra.Ratings{Safe: true, Questionable: true, Explicit: true}
	q.settingsbutton = "Search Settings"
	if settings.RatingMode == bottypes.FILTER_EXPLICIT {
		allowed_ratings = apiextra.Ratings{Safe: true, Questionable: true, Explicit: false}
	} else if settings.RatingMode == bottypes.FILTER_QUESTIONABLE {
		allowed_ratings = apiextra.Ratings{Safe: true, Questionable: false, Explicit: false}
		q.settingsbutton += " [SFW mode]"
	}

	force_rating := apiextra.RatingsFromString(ctx.Query.Query).And(allowed_ratings).RatingTag()

	var iqa data.OInlineQueryAnswer

	offset, err := proxify.Offset(ctx.Query.Offset)
	if err == nil {
		search_results, err := api.ListPosts(creds.User, creds.ApiKey, apitypes.ListPostOptions{SearchQuery: ctx.Query.Query + " " + force_rating, Page: apitypes.Page(offset + 1), Limit: q.resultsperpage})
		errorlog.ErrorLog(ctx.Bot.ErrorLog, "api", "api.TagSearch", err)
		iqa = this.ApiResultsToInlineResponse(ctx.Query.Query, blacklist, search_results, offset, err, q)
	} else {
		errorlog.ErrorLog(ctx.Bot.ErrorLog, "proxify", "proxify.Offset", errors.New(fmt.Sprintf("Bad Offset: %s (%s)", ctx.Query.Offset, err.Error())))
		iqa = this.ApiResultsToInlineResponse(ctx.Query.Query, blacklist, nil, 0, err, q)
	}

	ctx.AnswerAsync(iqa, nil)
}

func (this *Behavior) ApiResultsToInlineResponse(query, blacklist string, search_results apitypes.TPostInfoArray, current_offset int, err error, q QuerySettings) data.OInlineQueryAnswer {
	iqa := data.OInlineQueryAnswer{CacheTime: 30, IsPersonal: true, SwitchPMText: q.settingsbutton, SwitchPMParam: "settings"}
	if err != nil {
		if placeholder := this.GetErrorPlaceholder(); placeholder != nil {
			iqa.Results = append(iqa.Results, placeholder)
		}
	} else if len(search_results) == 0 && current_offset == 0 {
		if placeholder := this.GetNoResultsPlaceholder(query); placeholder != nil {
			iqa.Results = append(iqa.Results, placeholder)
		}
	} else if len(search_results) == q.resultsperpage {
		iqa.NextOffset = strconv.FormatInt(int64(current_offset + 1), 10)
	}

	for _, r := range search_results {
		if r.MatchesBlacklist(blacklist) { continue }
		new_result := proxify.ConvertApiResultToTelegramInline(r, proxify.ContainsSafeRatingTag(query), query, q.debugmode, this.MySettings.CaptionSettings)

		if (new_result != nil) {
			iqa.Results = append(iqa.Results, new_result)
		}
	}

	if len(iqa.Results) == 0 {
		iqa.NextOffset = ""
		if placeholder := this.GetBlacklistedPlaceholder(query); placeholder != nil {
			iqa.Results = append(iqa.Results, placeholder)
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
	if this.MySettings.NoResultsPhotoID == "" { return nil }
	return &data.TInlineQueryResultCachedPhoto{
		Type: "photo",
		Id: "no-results",
		PhotoId: this.MySettings.NoResultsPhotoID,
		InputMessageContent: &data.TInputMessageTextContent{
			MessageText: fmt.Sprintf("There are no results on " + api.ApiName + " for <code>%s</code> :(", html.EscapeString(query)),
			ParseMode: data.ParseHTML,
		},
	}
}

func (this *Behavior) GetBlacklistedPlaceholder(query string) *data.TInlineQueryResultCachedPhoto {
	if this.MySettings.NoResultsPhotoID == "" { return nil }
	return &data.TInlineQueryResultCachedPhoto{
		Type: "photo",
		Id: "blacklisted-results",
		PhotoId: this.MySettings.BlacklistedPhotoID,
		InputMessageContent: &data.TInputMessageTextContent{
			MessageText: fmt.Sprintf("There are results on " + api.ApiName + " for <code>%s</code>, but my blacklist filtered them all!", html.EscapeString(query)),
			ParseMode: data.ParseHTML,
		},
	}
}

func (this *Behavior) ProcessInlineQueryResult(ctx *gogram.InlineResultCtx) {
	ctx.Bot.Log.Printf("[behavior] Inline selection: %s (by %d %s)\n", ctx.Result.ResultId, ctx.Result.From.Id, ctx.Result.From.UsernameString())

	if !strings.HasSuffix(ctx.Result.ResultId, "_cvt") {
		return
	}

	go proxify.HandleWebmConversionRequest(ctx, this.MySettings.DefaultSearchCredentials())
}
