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
	"bytes"
	"log"
	"strconv"
	"sync"
	"storage"
	"errors"
	"time"
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
	return 60
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
				continue
			}

			update_chan := make(chan []apitypes.TPostInfo)
			var updated_post_ids []int
			updated_posts := make(map[int]apitypes.TPostInfo)

			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				for posts := range update_chan {
					for _, p := range posts {
						updated_post_ids = append(updated_post_ids, p.Id)
						updated_posts[p.Id] = p
					}
				}
				wg.Done()
			}()

			go func() {
				tagindex.SyncPostsInternal(this.MySettings.SearchUser, this.MySettings.SearchAPIKey, settings, extra_expensive, extra_expensive, nil, nil, update_chan)
				close(update_chan)
				wg.Done()
				if err != nil { log.Println(err.Error()) }
			}()

			wg.Wait()

			settings.Transaction.MarkForCommit()
			settings.Transaction.Finalize(true)

			updated_post_ids = []int{2278061, 2278890}
			updated_posts = make(map[int]apitypes.TPostInfo)

			for _, pid := range updated_post_ids {
				post, err := api.FetchOnePost(this.MySettings.SearchUser, this.MySettings.SearchAPIKey, pid)
				if err != nil { log.Println(err.Error()) }
				updated_posts[pid] = *post
			}

			bot.Log.Println("Maintenance sync complete. Processing updated posts...")

			settings.Transaction, err = storage.NewTxBox()
			if err != nil {
				bot.Log.Println("Error creating transaction:", err.Error())
				continue
			}

			edits, err := storage.GetSuggestedPostEdits(updated_post_ids, settings)
			if err != nil {
				bot.Log.Println("Error in GetSuggestedPostEdits:", err.Error())
				continue
			}

			autofixes, err := storage.GetAutoFixHistoryForPosts(updated_post_ids, settings) // map of id to array of tag diff
			if err != nil {
				bot.Log.Println("Error in GetAutoFixHistoryForPosts:", err.Error())
				continue
			}

			log.Println("updated ids:", updated_post_ids)
			log.Println("updated posts:", updated_posts)
			log.Println("edits:", edits)
			log.Println("autofixes:", autofixes)

			for id, edit := range edits {
				log.Println("edit:", edit)
				// remove any recent autofix changes from the autofix list, bit by bit.
				// also filter out any edits which become no-ops
				autofix := autofixes[id]
				var new_autofix []apitypes.TagDiff
				for i, _ := range edit.AutoFix {
					for _, already_done := range autofix {
						edit.AutoFix[i] = edit.AutoFix[i].Difference(already_done)
					}
					if !edit.AutoFix[i].IsZero() { new_autofix = append(new_autofix, edit.AutoFix[i]) }
				}
				edit.AutoFix = new_autofix
				log.Println("edit:", edit)

				// automatically apply any autofix edits that were made

				// generate a prompt post, or find an existing one and edit it
				post_info, err := storage.FindPromptPost(id, settings)
				if err != nil {
					bot.Log.Println("Error in FindPromptPost:", err.Error())
					continue
				}
				post_info = this.PromptPost(bot, post_info, id, updated_posts[id], edit)
				err = storage.SavePromptPost(id, post_info, settings)
				if err != nil {
					bot.Log.Println("Error in SavePromptPost:", err.Error())
					continue
				}
			}

			settings.Transaction.MarkForCommit()
			settings.Transaction.Finalize(true)

			bot.Log.Println("Update processing complete.")
		}
	}()
	return channel
}

func ternary(b bool, x, y string) string {
	if b { return x }
	return y
}

func GetInlineKeyboardForEdit(edit *storage.PostSuggestedEdit) (*data.TInlineKeyboard) {
	const DIAMOND string = "\U0001F539"
	const RED_DOT string = "\U0001F534"
	const GREEN_DOT string = "\U0001F7E2"

	var keyboard data.TInlineKeyboard
	keyboard.Buttons = append(keyboard.Buttons, []data.TInlineKeyboardButton{data.TInlineKeyboardButton{
		Text: DIAMOND + " Commit",
		Data: func(s string) *string {return &s}("/af-commit"),
	},data.TInlineKeyboardButton{
		Text: DIAMOND + " Dismiss",
		Data: func(s string) *string {return &s}("/af-dismiss"),
	}})
	keyboard.Buttons = append(keyboard.Buttons, []data.TInlineKeyboardButton{data.TInlineKeyboardButton{
		Text: ternary(len(edit.AutoFix) + len(edit.Prompt) == len(edit.SelectedEdits),
				fmt.Sprintf("%s Clear all", strings.Repeat(RED_DOT, 3)),
				fmt.Sprintf("%s Apply all", strings.Repeat(GREEN_DOT, 3))),
		Data: func(s string) *string {return &s}("/af-toggle everything"),
	}})
	for i, diff := range edit.AutoFix {
		api_string := diff.APIString()
		_, ok := edit.SelectedEdits[api_string]
		keyboard.Buttons = append(keyboard.Buttons, []data.TInlineKeyboardButton{data.TInlineKeyboardButton{
			Text: fmt.Sprintf("%s %s", ternary(ok, GREEN_DOT, RED_DOT), api_string),
			Data: func(s string) *string {return &s}(fmt.Sprintf("/af-toggle autofix %d", i)),
		}})
	}
	for i, diff := range edit.Prompt {
		api_string := diff.APIString()
		_, ok := edit.SelectedEdits[api_string]
		keyboard.Buttons = append(keyboard.Buttons, []data.TInlineKeyboardButton{data.TInlineKeyboardButton{
			Text: fmt.Sprintf("%s %s", ternary(ok, GREEN_DOT, RED_DOT), api_string),
			Data: func(s string) *string {return &s}(fmt.Sprintf("/af-toggle autofix %d", i)),
		}})
	}

	return &keyboard
}

func (this *Behavior) PromptPost(bot *gogram.TelegramBot, post_info *storage.PromptPostInfo, id int, post apitypes.TPostInfo, edit storage.PostSuggestedEdit) (*storage.PromptPostInfo) {
	if post_info != nil {
		edit = post_info.Edit.Append(edit)
	}
	markup := GetInlineKeyboardForEdit(&edit)

	send := data.SendData{
		TargetData: data.TargetData{
			ChatId: data.ChatID(-1001429698294), // project 621
		},
		ParseMode: data.ParseHTML,
		DisableNotification: true,
		ReplyMarkup: markup,
	}

	// figure out what we should say for what tags we're changing
	var edit_string bytes.Buffer
	if len(edit.Prompt) != 0 {
		edit_string.WriteString(fmt.Sprintf("Manual fixes available:\n<pre>%s</pre>\n", edit.Prompt.Flatten().APIString()))
	}
	if len(edit.AutoFix) != 0 {
		edit_string.WriteString(fmt.Sprintf("Automatic fixes applied:\n<pre>%s</pre>\n", edit.AutoFix.Flatten().APIString()))
	}

	// figure out what the message should be
	if post.File_ext == "png" || post.File_ext == "jpg" {
		send.Text = fmt.Sprintf("<a href=\"%s\">Image</a>\n<a href=\"https://" + api.Endpoint + "/posts/%d\">Post</a>\n%s", post.File_url, post.Id, edit_string.String())
	} else if post.File_ext == "gif" {
		send.Text = fmt.Sprintf("<a href=\"%s\">Animation</a>\n<a href=\"https://" + api.Endpoint + "/posts/%d\">Post</a>\n%s", post.File_url, post.Id, edit_string.String())
	} else if post.File_ext == "webm" {
		send.Text = fmt.Sprintf("Post ID %d (%s file, no preview available)\nView it using the links below.\n\n<a href=\"%s\">Video</a>\n<a href=\"https://" + api.Endpoint + "/posts/%d\">Post</a>\n%s", post.Id, post.File_ext, post.File_url, post.Id, edit_string.String())
	} else { // SWF, or any other unrecognized file type
		send.Text = fmt.Sprintf("Post ID %d (%s file, no preview available)\nView it using the links below.\n\n<a href=\"%s\">File</a>\n<a href=\"https://" + api.Endpoint + "/posts/%d\">Post</a>\n%s", post.Id, post.File_ext, post.File_url, post.Id, edit_string.String())
	}

	if post_info != nil {
		if len(edit.Prompt) + len(edit.AutoFix) == 0 {
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
		post_info.Edit = &edit
	} else {
		if len(edit.Prompt) + len(edit.AutoFix) == 0 { return nil }

		var message *data.TMessage
		var err error
		if post.File_ext == "png" || post.File_ext == "jpg" {
			message, err = bot.Remote.SendPhoto(data.OPhoto{
				SendData: send,
				MediaData: data.MediaData{
					File: post.Sample_url,
					FileName: fmt.Sprintf("%s.%s", post.Md5, post.File_ext),
				},
			})
		} else if post.File_ext == "gif" {
			message, err = bot.Remote.SendAnimation(data.OAnimation{
				SendData: send,
				MediaData: data.MediaData{
					File: post.File_url,
					FileName: fmt.Sprintf("%s.%s", post.Md5, post.File_ext),
				},
				ResolutionData: data.ResolutionData{
					Width: post.Width,
					Height: post.Height,
				},
			})
		} else {
			send.Text = fmt.Sprintf("%s", post.Id, post.File_ext, send.Text)
			message, err = bot.Remote.SendMessage(data.OMessage{
				SendData: send,
				DisableWebPagePreview: true,
			})
		}

		if err != nil {
			bot.ErrorLog.Println(err.Error())
			return nil
		}

		post_info = &storage.PromptPostInfo{
			PostId: id,
			MsgId: message.Id,
			ChatId: message.Chat.Id,
			Timestamp: time.Unix(message.Date, 0),
			Captioned: message.Text == nil,
			Edit: &edit,
		}
	}
	return post_info
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
