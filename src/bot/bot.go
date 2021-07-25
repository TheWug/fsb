package bot

import (
	"bot/dialogs"
	"botbehavior"
	"api"
	"api/tagindex"
	"api/tags"
	apitypes "api/types"
	"apiextra"
	"storage"

	"github.com/thewug/gogram"
	"github.com/thewug/gogram/data"
	"github.com/thewug/gogram/persist"

	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"io/ioutil"
	"strconv"
	"strings"
	"sync"
	"time"
	"database/sql"
)

const (
	root = iota
	login
	logout
	settagrules
	post
		postfile
		postfileurl
		postpublic
		posttags
		postwizard
		postrating
		postsource
		postdescription
		postparent
		postupload
		postnext
	editreason
)

const (
	none = iota
	url
	file_id
)

type PostFile struct {
	mode int
	file_id data.FileID
	url string
}

func ShowHelp(topic string) (string) {
	// this works by filtering the full help document by the provided substring
	// against the first token in the line, as a sort of poor man's help topic selector.
	// the
	help :=
`.public.janitor. Hello! I'm the <b>` + api.ApiName + ` Telegram Bot</b>!
.public.janitor.
.public.janitor. Content on ` + api.ApiName + ` may be unsuitable for kids.
.public.janitor. <b>You must be 18 or older to use this bot.</b>
.
. To view a detailed explanation of my features and functions, please <a href="https://telegra.ph/Knottybot-Usage-Guide-08-20">check out this post</a>.
.
. <b>Short story:</b>
. <code>* </code>Search inline for ` + api.ApiName + ` posts
. <code>* </code>Connect your ` + api.ApiName + ` account
. <code>* </code>Vote on and favorite posts
. <code>* </code>Upload new posts
. <code>* </code>Edit existing posts
. <code>* </code>All without leaving Telegram!
.
. <b>Important Info and FAQ</b>
. <code>* </code>Adjust your rating filter and blacklist from your search settings.
. <code>* </code>Before posting to ` + api.ApiName + `, please make sure you read the site's rules.
. <code>* </code>Your account standing is your own responsibility.
. <code>* </code>Your ` + api.ApiName + ` API key is NOT your password. To find it, go to your <a href="https://` + api.Endpoint + `/users/home">Account Settings</a> and click "Manage API Access".
. <code>* </code>To report a bug, see <code>/help report.</code>
security.abuse.report. <b>Reporting abuse, bugs, or other issues</b>
security.abuse.report. Use the following command to send a message to the janitor's chat. If your issue is private or security related, please send a report asking to be contacted back.
security.abuse.report.
security.abuse.report. <code>/operator [message for janitors]</code>
public.
public. This bot's commands and help messages should be used via PM.
janitor.
janitor. <b>Janitor Commands</b>
janitor. For a full description of any command, use <code>/help [command]</code>.
janitor.cats. <code>/cats</code>
cats. A <i>CAT</i> is a malformed tag formed from two valid tags accidentally concatenated together. These tags are typos, and are added to posts by accident (if it isn't an accident, it's not a <i>CAT</i>). This command helps search for <i>CAT</i>s, resolve them to their correct tags, and automatically apply them to posts.
cats. <b>General Listing options:</b>
cats. <code> --list-wild, -w -</code> show only unconfirmed <i>CAT</i>s
cats. <code> --list-yes,  -y -</code> show only confirmed <i>CAT</i>s
cats. <code> --list-no,   -n -</code> show only excluded <i>CAT</i>s
cats. <code> --list,      -l -</code> shorthand for -y and -n
cats. <code> (no arguments)  -</code> same as -w
cats. <b>Directed Listing options:</b>
cats. <code> --inspect,    -i T -</code> List all possible <i>CAT</i>s including tag <code>T</code>
cats. <code> --first,      -1   -</code> sed with -i, <code>T</code> must be a prefix
cats. <code> --second,     -2   -</code> Used with -i, <code>T</code> must be a suffix
cats. <code> --ratio,      -r N -</code> <i>CAT</i>s must be <code>N</code> times rarer than base tags
cats. <code> --with-blits, -b   -</code> include wild <i>CAT</i>s which include <i>BLIT</i>s
cats. <code> --with-empty, -0   -</code> include <i>CAT</i>s with no tagged posts
cats. <code> --with-typed, -t   -</code> include <i>CAT</i>s which aren't general tags
cats. <b>Selection options:</b>
cats. <code> --entry,  -e N     -</code> reply to listing to select <i>CAT</i> <code>N</code>
cats. <code> --select, -s T1 T2 -</code> manually specify <i>CAT</i> formed by <code>T1 + T2</code>
cats. <b>Editing options:</b>
cats. <code> --exclude, -E -</code> exclude selected (indicating a valid tag)
cats. <code> --prompt,  -P -</code> confirm selected, prompt to fix new posts
cats. <code> --autofix, -A -</code> confirm selected, automatically fix new posts
cats. <code> --delete,  -D -</code> remove selected from the database
cats. <code> --fix,     -F -</code> fix posts matching selected <i>CAT</i>s right now
cats. <code>                </code> (can be combined with any other editing option)
janitor.blits. <code>/blits</code>
blits. A <i>BLIT</i> is a tag that is not automatically eligible to be part of a <i>CAT</i>. A tag should be marked as a <i>BLIT</i> if it is both:
blits. <code> -</code> unlikely to ever be used in a valid way on a post
blits. <code> -</code> likely to appear as a prefix or suffix within another valid tag
blits. Tags which are 2 characters or shorter are automatically marked as <i>BLIT</i>s. However, a small number of short tags exist (such as m and f, aliases of male and female) which should be exempt from this classification. Additionally, many blits exist which are longer than 2 characters. This command manages or exports the <i>BLIT</i> list.
blits. Listing options:
blits. <code> --list-wild -w -</code> list wild (unmarked) <i>BLIT</i>s
blits. <code> --list-yes, -y -</code> list known <i>BLIT</i>s
blits. <code> --list-no,  -n -</code> list known non-<i>BLIT</i>s
blits. <code> --list,     -l -</code> shorthand for -y and -n
blits. <code> (no arguments) -</code> same as -w
blits. Editing options:
blits. <code> --include,-I TAG -</code> mark <code>TAG</code> as a <i>BLIT</i>
blits. <code> --exclude,-E TAG -</code> mark <code>TAG</code> as a non-<i>BLIT</i>
blits. <code> --delete, -D TAG -</code> clear <code>TAG</code> entirely from <i>BLIT</i> list
janitor.syncposts. <code>/syncposts</code>
syncposts. This command is used to keep my internal index of ` + api.ApiName + ` data up to date. This index is used to speed up operations like searching for similar tag names and listing all posts with a tag. Some metadata is maintained for each tag, each tag alias, and each post, on the site.
syncposts. <i>Control</i> options:
syncposts. <code> (no arguments) -</code> incremental sync of tags and posts (default)
syncposts. <code> --full         -</code> discard local database and sync from scratch
syncposts. <code> --aliases      -</code> sync tag aliases as well
syncposts. <code> --recount      -</code> tally post tag counts afterwards
syncposts. You do not normally need to use this command. Commands which push changes to ` + api.ApiName + ` should apply them locally as well, and an incremental sync is performed by the bot's internal maintenance routine every five minutes (with an alias sync and a tag recount happening every 60 minutes).
janitor.indextags. <code>/indextags</code>
indextags. This command syncs new changes on ` + api.ApiName + ` to the local tag database.
indextags. <i>Control</i> options:
indextags. <code> (no arguments) -</code> perform an incremental sync of tags
indextags. <code> --full         -</code> discard local database and sync from scratch
indextags. This operation is invoked by <code>/syncposts</code>, which passes <code>--full</code> to this command if it is present.
janitor.indextagaliases. <code>/indextagaliases</code>
indextagaliases. This command syncs tag aliases between ` + api.ApiName + ` and the local alias database. Because of how aliases are listed on ` + api.ApiName + `, an incremental sync is not possible, and a full sync is always performed. This command takes no options. It is invoked by <code>/syncposts</code> if <code>--aliases</code> is specified.
janitor.typos. <code>/typos</code>
typos. This command searches for likely typos of a tag, as determined by their edit distance to other tags. The way you should use this command is broadly at first, listing all typos, and then more and more specifically as you investigate each possible option on the site, adding selection options until you have a comprehensive, accurate listing of typos, then apply them to the site by issuing the command again with <code>--fix</code> and using <code>--include</code> or <code>--autofix</code> to register them for future auto-fixes.
typos. <i>Listing</i> options:
typos. <code> --no-auto,     -x -</code> do not automatically treat <code>START_TAG</code> alias
typos. <code> --list-wild,   -w -</code> show unconfirmed typos (default)
typos. <code> --list-yes,    -y -</code> show confirmed typos
typos. <code> --list-no,     -n -</code> show confirmed non-typos
typos. <code> --list,        -l -</code> same as -y -n
typos. <code> --show-blits,  -b -</code> include typos which are blits
typos. <code> --show-zero,   -z -</code> include typos with no tagged posts
typos. <code> --general-only,-g -</code> include typos which are non-general tags
typos. <code> --threshold, -t N -</code> show tags with edit distance <code>N</code> or less
typos. <i>Selection</i> options:
typos. <code> [args] START_TAG -</code> find typos of <code>TAG</code> (required)
typos. <code> --select,   -s T -</code> select a specific, arbitrary tag <code>T</code>
typos. <code> --skip,     -k T -</code> deselect a specific, arbitrary tag <code>T</code>
typos. <code> --alias,    -a T -</code> also search for typos near tag <code>T</code>
typos. <code> --distinct, -d T -</code> treat <code>T</code> as distinct, and ignore nearby typos
typos. <i>Submission</i> options:
typos. <code> --delete,  -D   -</code> delete these typo records, if they exist
typos. <code> --exclude, -E   -</code> register the selected tags as non-typos
typos. <code> --include, -I   -</code> register the selected tags as typos
typos. <code> --autofix, -A   -</code> automatically fix the selected typos
typos. <code> --fix,     -F   -</code> fix the selected typos now
typos. <code> --reason,  -r R -</code> include reason <code>R</code> when performing edits
janitor.recounttags. <code>/recounttags</code>
recounttags. This command recounts the cached tag counts, providing an accurate count (the site itself becomes desynced sometimes and its counts are not always accurate). It does so for both visible and deleted posts. It takes no arguments. It is invoked by <code>/syncposts</code> if the <code>--recount</code> option is specified.
janitor.resyncdeleted. <code>/resyncdeleted</code>
resyncdeleted. <s>This command is disabled.</s> You should not need to use it. It enumerates all deleted posts from ` + api.ApiName + ` and updates the local database's deleted status. It exists because at one point, that information was not stored, but it affects certain parts of the API (namely, ordinary users can no longer edit deleted posts) and it needed to be re-imported. It takes no options. If you need to use it again, you should clear the deleted status of all posts manually from the database console first.
janitor.resynclist. <code>/resynclist</code>
resynclist. Use this command captioned on an uploaded file, containing whitespace delimited post ids (and comments beginning with #). The bot will perform a local DB sync on each post listed in the file.
birds. What <b>are</b> birds?
birds. We just don't know.`

	topic = strings.ToLower(topic)
	output := bytes.NewBuffer(nil)
	for _, line := range strings.Split(help, "\n") {
		chunks := strings.SplitN(line, ". ", 2)
		if len(chunks) != 2 { continue } // discard malformed lines.
		tokens := strings.Split(chunks[0], ".")
		for _, blip := range tokens {
			if blip == topic {
				output.WriteString(strings.TrimSpace(chunks[1]))
				output.WriteRune('\n')
				break
			}
		}
	}

	out := output.String()
	if out == "" { return "Sorry, no help available for that." }
	return out
}

type HelpState struct {
	gogram.StateBase
}

func (this *HelpState) Handle(ctx *gogram.MessageCtx) {
	topic := "public"
	if ctx.Msg.Chat.Type == "private" {
		topic = ctx.Cmd.Argstr
	}
	ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: ShowHelp(topic), ParseMode: data.ParseHTML}}, nil)
}

type AutofixState struct {
	gogram.StateBase

	Behavior *botbehavior.Behavior
}

func (this *AutofixState) GetInterval() int64 {
	return 60 * 60 // 1 hour
}

func (this *AutofixState) DoMaintenance(bot *gogram.TelegramBot) {
	go func() {
		// clear prompt_post table of entries that are older than 24 hours
		err := this.Behavior.ClearPromptPostsOlderThan(bot, time.Hour * 24)
		if err != nil {
			bot.ErrorLog.Println("ClearPromptPostsOlderThan:", err)
		}
	}()
}

func (this *AutofixState) Handle(ctx *gogram.MessageCtx) {
	return // ignore messages
}

func (this *AutofixState) HandleCallback(ctx *gogram.CallbackCtx) {
	err := storage.DefaultTransact(func(tx storage.DBLike) error { return this.HandleCallbackTx(tx, ctx) })
	if err != nil {
		ctx.Bot.ErrorLog.Println("Error in SettingsState.HandleCallback:", err)
	}
}

func (this *AutofixState) HandleCallbackTx(tx storage.DBLike, ctx *gogram.CallbackCtx) error {
	defer ctx.AnswerAsync(data.OCallback{}, nil) // non-specific acknowledge if we return without answering explicitly
	// don't bother with any callbacks that are so old their message is no longer available
	if ctx.MsgCtx == nil { return nil }

	creds, err := storage.GetUserCreds(nil, ctx.Cb.From.Id)
	if err == storage.ErrNoLogin {
		ctx.AnswerAsync(data.OCallback{Notification: "\U0001F512 You need to login to do that!\n(use /login, in PM)", ShowAlert: true}, nil)
		return nil
	}

	if !creds.Janitor {
		ctx.AnswerAsync(data.OCallback{Notification: "\U0001F512 Sorry, this feature is currently limited to janitors.", ShowAlert: true}, nil)
		return nil
	}

	post_info, err := storage.FindPromptPostByMessage(tx, ctx.MsgCtx.Msg.Chat.Id, ctx.MsgCtx.Msg.Id)
	if post_info == nil { return nil }

	// handle toggling possible edits on and off
	if ctx.Cmd.Command == "/af-toggle" && len(ctx.Cmd.Args) == 3 {
		set := (ctx.Cmd.Args[2] == "1")
		if ctx.Cmd.Args[0] == "everything" {
			if set {
				post_info.Edit.SelectAll()
			} else {
				post_info.Edit.DeselectAll()
			}
		} else if ctx.Cmd.Args[0] == "autofix" || ctx.Cmd.Args[0] == "prompt" {
			index, err := strconv.Atoi(ctx.Cmd.Args[1])
			if err != nil { return fmt.Errorf("possibly spoofed callback data: %w", err) } // this should only happen if users are spoofing callback data, so ignore it.
			if set {
				post_info.Edit.Select(ctx.Cmd.Args[0], index)
			} else {
				post_info.Edit.Deselect(ctx.Cmd.Args[0], index)
			}
		}
		err = storage.SavePromptPost(tx, post_info.PostId, post_info)
		this.Behavior.PromptPost(ctx.Bot, post_info, post_info.PostId, nil, nil)
	} else if ctx.Cmd.Command == "/af-commit" {
		diff := post_info.Edit.GetChangeToApply()

		if diff.IsZero() {
			ctx.AnswerAsync(data.OCallback{Notification: "\u2139 No changes to apply."}, nil)
			this.Behavior.DismissPromptPost(tx, ctx.Bot, post_info, diff)
		} else {
			reason := "Manual tag cleanup: typos and concatenations (via KnottyBot)"
			post, err := api.UpdatePost(creds.User, creds.ApiKey, post_info.PostId, diff, nil, nil, nil, nil, &reason)
			if err != nil {
				ctx.AnswerAsync(data.OCallback{Notification: "\u26A0 An error occurred when trying to update the post! Try again later."}, nil)
				return err
			}

			post_info.Edit.Apply()
			this.Behavior.DismissPromptPost(tx, ctx.Bot, post_info, diff)
			ctx.AnswerAsync(data.OCallback{Notification: "\U0001F539 Changes saved."}, nil)

			if post != nil {
				err = storage.UpdatePost(tx, *post)
				if err != nil {
					ctx.Bot.ErrorLog.Println("Failed to locally update post:", err.Error())
					return err
				}
			}
		}
	} else if ctx.Cmd.Command == "/af-dismiss" {
		err = this.Behavior.DismissPromptPost(tx, ctx.Bot, post_info, tags.TagDiff{})
		if err != nil { return fmt.Errorf("DismissPrimptPost: %w", err) }
		ctx.AnswerAsync(data.OCallback{Notification: "\U0001F539 Dismissed without changes."}, nil)
	}
	
	return nil
}

type lookup_votes struct {
	user data.UserID
	score apitypes.PostVote
	date time.Time
}

type lookup_faves struct {
	user data.UserID
	date time.Time
}

type OperatorState struct {
	gogram.StateBase

	Behavior *botbehavior.Behavior
}

func (this *OperatorState) Handle(ctx *gogram.MessageCtx) {
	if ctx.Msg.Chat.Type == data.Private {
		ctx.Forward(data.OForward{SendData: data.SendData{TargetData: data.TargetData{ChatId: this.Behavior.MySettings.Home}}})
		ctx.Reply(data.OMessage{SendData: data.SendData{Text: "Your message has been forwarded to the janitor's chat.  You may be contacted for more information, if possible."}})
	}
}

type VoteState struct {
	gogram.StateBase

	votes map[data.UserID]lookup_votes
	faves map[data.UserID]lookup_faves
	lock sync.Mutex
}

func (this *VoteState) GetInterval() int64 {
	return 30
}

func (this *VoteState) DoMaintenance(bot *gogram.TelegramBot) {
	go func(){
		now := time.Now()

		this.lock.Lock()
		for k, v := range this.votes {
			if now.Sub(v.date) > 30 * time.Second { delete(this.votes, k) }
		}

		for k, v := range this.faves {
			if now.Sub(v.date) > 30 * time.Second { delete(this.faves, k) }
		}
		this.lock.Unlock()
	}()
}

func (this *VoteState) Handle(ctx *gogram.MessageCtx) {
	if ctx.Msg.From == nil { return }
	go func() {
		msg, _ := this.HandleCmd(ctx.Msg.From, &ctx.Cmd, ctx.Msg.ReplyToMessage, ctx.Bot)
		if msg.ReplyToId == nil { msg.ReplyToId = &ctx.Msg.Id }
		ctx.RespondAsync(msg, nil)
	}()
}

func (this *VoteState) HandleCallback(ctx *gogram.CallbackCtx) {
	go func() {
		msg, alert := this.HandleCmd(&ctx.Cb.From, &ctx.Cmd, nil, ctx.Bot)
		ctx.AnswerAsync(data.OCallback{Notification: msg.Text, ShowAlert: alert}, nil)
	}()
}

func (this *VoteState) MarkAndTestRecentlyVoted(tg_user data.UserID, vote apitypes.PostVote, post_id int) bool {
	this.lock.Lock()
	if this.votes == nil { this.votes = make(map[data.UserID]lookup_votes) }
	entry, voted := this.votes[tg_user]
	// true if there is an entry, AND the entry is less than 30 seconds old, AND the vote is the same
	voted = (voted && time.Now().Sub(entry.date) < 30 * time.Second && entry.score == vote)
	if voted {
		delete(this.votes, tg_user)
	} else {
		this.votes[tg_user] = lookup_votes{user: tg_user, score: vote, date: time.Now()}
	}
	this.lock.Unlock()
	return voted
}

func (this *VoteState) MarkAndTestRecentlyFaved(tg_user data.UserID, post_id int) bool {
	this.lock.Lock()
	if this.faves == nil { this.faves = make(map[data.UserID]lookup_faves) }
	entry, faved := this.faves[tg_user]
	// true if there is an entry, AND the entry is less than 30 seconds old
	faved = (faved && time.Now().Sub(entry.date) < 30 * time.Second)
	if faved {
		delete(this.faves, tg_user)
	} else {
		this.faves[tg_user] = lookup_faves{user: tg_user, date: time.Now()}
	}
	this.lock.Unlock()
	return faved
}

func (this *VoteState) HandleCmd(from *data.TUser, cmd *gogram.CommandData, reply_message *data.TMessage, bot *gogram.TelegramBot) (data.OMessage, bool) {
	var response data.OMessage

	creds, err := storage.GetUserCreds(nil, from.Id)
	if err == storage.ErrNoLogin {
		response.Text = "\U0001F512 You need to login to do that!\n(use /login, in PM)"
		return response, true
	} else if err != nil {
		bot.ErrorLog.Printf("Failed to get credentials for user %d: %s\n", from.Id, err.Error())
		response.Text = "An error occurred while fetching up your " + api.ApiName + " credentials."
		return response, true
	}

	var id int
	if len(cmd.Args) > 0 {
		id = apiextra.GetPostIDFromText(cmd.Args[0])
	} else if reply_message != nil {
		id = apiextra.GetPostIDFromMessage(reply_message)
	}

	// if after all that, the id is still the zero value, that means we didn't find one, so die
	if id == 0 {
		response.Text = "You must to specify a post ID."
		return response, true
	}

	if cmd.Command == "/upvote" {
		if this.MarkAndTestRecentlyVoted(from.Id, apitypes.Upvote, id) {
			err = api.UnvotePost(creds.User, creds.ApiKey, id)
			if err != nil {
				response.Text = "An error occurred when removing your vote! (Is " + api.ApiName + " down?)"
				bot.ErrorLog.Printf("Error when unvoting post %d: %s\n", id, err.Error())
			} else {
				response.Text = "\U0001F5D1 You have deleted your vote."
			}
		} else {
			_, err := api.VotePost(creds.User, creds.ApiKey, id, apitypes.Upvote, true)
			if err != nil {
				response.Text = "An error occurred when voting! (Is " + api.ApiName + " down?)"
				bot.ErrorLog.Printf("Error when voting post %d: %s\n", id, err.Error())
			} else {
				response.Text = "\U0001F7E2 You have upvoted this post! (Click again to cancel your vote)"
			}
		}
	} else if cmd.Command == "/downvote" {
		if this.MarkAndTestRecentlyVoted(from.Id, apitypes.Downvote, id) {
			err = api.UnvotePost(creds.User, creds.ApiKey, id)
			if err != nil {
				response.Text = "An error occurred when removing your vote! (Is " + api.ApiName + " down?)"
				bot.ErrorLog.Printf("Error when unvoting post %d: %s\n", id, err.Error())
			} else {
				response.Text = "\U0001F5D1 You have deleted your vote."
			}
		} else {
			_, err := api.VotePost(creds.User, creds.ApiKey, id, apitypes.Downvote, true)
			if err != nil {
				response.Text = "An error occurred when voting! (Is " + api.ApiName + " down?)"
				bot.ErrorLog.Printf("Error when voting post %d: %s\n", id, err.Error())
			} else {
				response.Text = "\U0001F534 You have downvoted this post! (Click again to cancel your vote)"
			}
		}
	} else if cmd.Command == "/favorite" {
		if this.MarkAndTestRecentlyFaved(from.Id, id) {
			err = api.UnfavoritePost(creds.User, creds.ApiKey, id)
			if err != nil {
				response.Text = "An error occurred when unfavoriting the post! (Is " + api.ApiName + " down?)"
				bot.ErrorLog.Printf("Error when unfaving post %d: %s\n", id, err.Error())
			} else {
				response.Text = "\U0001F5D1 You have unfavorited this post."
			}
		} else {
			_, err = api.FavoritePost(creds.User, creds.ApiKey, id)
			if err != nil {
				response.Text = "An error occurred when favoriting the post! (Is " + api.ApiName + " down?)"
				bot.ErrorLog.Printf("Error when faving post %d: %s\n", id, err.Error())
			} else {
				response.Text = "\U0001F49B You have favorited this post! (Click again to unfavorite)"
			}
		}
	}

	return response, false
}

type esp struct {
	User string `json:"user"`
	ApiKey string `json:"apikey"`

	MsgId data.MsgID `json:"msgid"`
	ChatId data.ChatID `json:"chatid"`
}

type EditState struct {
	persist.StateBasePersistent

	data esp
}

func EditStateFactory(jstr []byte, sbp persist.StateBasePersistent) gogram.State {
	return EditStateFactoryWithData(jstr, sbp, esp{})
}

func EditStateFactoryWithData(jstr []byte, sbp persist.StateBasePersistent, data esp) gogram.State {
	var e EditState
	e.StateBasePersistent = sbp
	e.StateBasePersistent.Persist = &e.data
	e.data = data
	json.Unmarshal(jstr, e.StateBasePersistent.Persist)
	return &e
}

func (this *EditState) Handle(ctx *gogram.MessageCtx) {
	if ctx.Cmd.Command == "/cancel" {
		// always react to cancel command
		this.Cancel(ctx)
		if ctx.Msg.Chat.Id != this.data.ChatId {
			ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: "Command cancelled."}}, nil)
		}
		return
	} else if ctx.Cmd.Command == "" && ctx.Msg.Chat.Id != this.data.ChatId {
		// completely ignore non-commands sent to other chats
		return
	} else if ctx.Cmd.Command != "" && ctx.Msg.Chat.Id != this.data.ChatId && this.data.ChatId != 0 {
		// warn users who try to use commands in another chat while this command is active already
		ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: "A command is already in progress somewhere else. To cancel it, use /cancel."}}, nil)
		return
	} else if ctx.Msg.Chat.Type == data.Private {
		// if it's a PM, always process it.
	} else if ctx.Msg.Chat.Type == data.Channel {
		// if it's a channel, never process it
		return
	} else if ctx.Cmd.Command != "" {
		// if it's a command, always process it
	} else if ctx.Cmd.Command == "" && ctx.Msg.ReplyToMessage == nil || ctx.Msg.ReplyToMessage.From.Id != ctx.Bot.Remote.GetMe().Id {
		// if it's not a command AND not a reply to a message sent by the bot, ignore it completely
		return
	}

	del := func() {
		if ctx.Msg.Document != nil && ctx.Msg.ForwardDate == nil {
			// keep the post around if it's a new, non-forwarded file upload
		} else {
			// otherwise, toss it.
			ctx.DeleteAsync(nil)
		}
	}

	if ctx.Cmd.Command == "/edit" && ctx.GetState() == nil {
		this.Edit(ctx)
	} else if ctx.Cmd.Command == "/reply" {
		newctx := gogram.NewMessageCtx(ctx.Msg.ReplyToMessage, false, ctx.Bot)
		if newctx != nil {
			this.Freeform(newctx)
		}
		del()
	} else {
		this.Freeform(ctx)
		del()
	}
}

func (this *EditState) HandleCallback(ctx *gogram.CallbackCtx) {
	err := storage.DefaultTransact(func(tx storage.DBLike) error { return this.HandleCallbackTx(tx, ctx) })
	if err != nil {
		ctx.Bot.ErrorLog.Println("Error in EditState.HandleCallbackTx:", err)
	}
}

func (this *EditState) HandleCallbackTx(tx storage.DBLike, ctx *gogram.CallbackCtx) error {
	p, err := dialogs.LoadEditPrompt(tx, this.data.MsgId, this.data.ChatId)
	if err != nil { fmt.Errorf("LoadEditPrompt: %w", err) }

	p.HandleCallback(ctx)

	if p.State == dialogs.SAVED {
		_, err := p.CommitEdit(tx, this.data.User, this.data.ApiKey, gogram.NewMessageCtx(ctx.Cb.Message, false, ctx.Bot))
		if err == nil {
			p.Finalize(tx, ctx.Bot, nil, dialogs.NewEditFormatter(ctx.Cb.Message.Chat.Type != data.Private, nil))
			ctx.AnswerAsync(data.OCallback{Notification: "\U0001F7E2 Edit submitted."}, nil)
			ctx.SetState(nil)
		} else {
			ctx.AnswerAsync(data.OCallback{Notification: fmt.Sprintf("\U0001F534 %s", err.Error())}, nil)
			p.Prompt(tx, ctx.Bot, nil, dialogs.NewEditFormatter(ctx.Cb.Message.Chat.Type != data.Private, err))
			p.State = dialogs.WAIT_MODE
		}
	} else if p.State == dialogs.DISCARDED {
		p.Finalize(tx, ctx.Bot, nil, dialogs.NewEditFormatter(ctx.Cb.Message.Chat.Type != data.Private, nil))
	} else {
		p.Prompt(tx, ctx.Bot, nil, dialogs.NewEditFormatter(ctx.Cb.Message.Chat.Type != data.Private, nil))
	}
	
	return nil
}

func (this *EditState) Freeform(ctx *gogram.MessageCtx) {
	err := storage.DefaultTransact(func(tx storage.DBLike) error {
		p, err := dialogs.LoadEditPrompt(tx, this.data.MsgId, this.data.ChatId)
		if err != nil { return fmt.Errorf("LoadEditPrompt: %w", err) }

		p.HandleFreeform(ctx)

		p.Prompt(tx, ctx.Bot, nil, dialogs.NewEditFormatter(ctx.Msg.Chat.Type != data.Private, nil))
		return nil
	})
	if err != nil {
		ctx.Bot.ErrorLog.Println(err)
	}
}

func (this *EditState) Cancel(ctx *gogram.MessageCtx) {
	err := storage.DefaultTransact(func(tx storage.DBLike) error {
		p, err := dialogs.LoadEditPrompt(tx, this.data.MsgId, this.data.ChatId)
		if err != nil { return fmt.Errorf("LoadEditPrompt: %w", err) }
		if p != nil {
			p.State = dialogs.DISCARDED
			p.Finalize(tx, ctx.Bot, nil, dialogs.NewEditFormatter(ctx.Msg.Chat.Type != data.Private, nil))
		}
		ctx.SetState(nil)
		return nil
	})
	if err != nil {
		ctx.Bot.ErrorLog.Println(err)
	}
}

func (this *EditState) Edit(ctx *gogram.MessageCtx) {
	err := storage.DefaultTransact(func(tx storage.DBLike) error {
		var e dialogs.EditPrompt

		if ctx.Msg.From == nil { return nil }

		creds, err := storage.GetUserCreds(nil, ctx.Msg.From.Id)
		if err != nil {
			ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: "You need to be logged in to use this command!"}}, nil)
			if err != storage.ErrNoLogin {
				ctx.Bot.ErrorLog.Println("Error while checking credentials: ", err.Error())
				err = nil
			} else {
				err = fmt.Errorf("GetUserCreds: %w", err)
			}
			return err
		}

		savenow, err := e.ParseArgs(ctx)
		if err != nil {
			ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: err.Error()}}, nil)
			return nil
		}

		if e.PostId <= 0 {
			ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: "Sorry, I can't figure out which post you're talking about!\n\nYou can reply to a message with a post URL, or you can pass an ID or a link directly."}}, nil)
			return nil
		}

		e.OrigSources = make(map[string]int)

		post_data, err := storage.PostByID(tx, e.PostId)
		if post_data != nil {
			for _, s := range post_data.Sources {
				e.SeeSource(s)
				e.OrigSources[s] = 1
			}
		}

		savestate := func(prompt *gogram.MessageCtx) {
			ctx.SetState(EditStateFactoryWithData(nil, this.StateBasePersistent, esp{
				User: creds.User,
				ApiKey: creds.ApiKey,
				MsgId: prompt.Msg.Id,
				ChatId: prompt.Msg.Chat.Id,
			}))
		}

		if savenow {
			_, err := e.CommitEdit(tx, creds.User, creds.ApiKey, ctx)
			if err == nil {
				e.State = dialogs.SAVED
				e.Finalize(tx, ctx.Bot, ctx, dialogs.NewEditFormatter(ctx.Msg.Chat.Type != data.Private, nil))
			} else {
				e.State = dialogs.SAVED
				savestate(e.Prompt(tx, ctx.Bot, ctx, dialogs.NewEditFormatter(ctx.Msg.Chat.Type != data.Private, err)))
			}
		} else {
			e.ResetState()
			savestate(e.Prompt(tx, ctx.Bot, ctx, dialogs.NewEditFormatter(ctx.Msg.Chat.Type != data.Private, nil)))
		}
		
		return nil
	})
	if err != nil {
		ctx.Bot.ErrorLog.Println(err)
	}
}

type LoginState struct {
	gogram.StateBase

	user	string
	apikey	string
}

func (this *LoginState) Handle(ctx *gogram.MessageCtx) {
	err := storage.DefaultTransact(func(tx storage.DBLike) error { return this.HandleTx(tx, ctx) })
	if err != nil {
		ctx.Bot.ErrorLog.Println(fmt.Errorf("LoginState.HandleTx: %w", err))
	}
}

func (this *LoginState) HandleTx(tx storage.DBLike, ctx *gogram.MessageCtx) error {
	if ctx.Msg.From == nil {
		return nil
	}
	if ctx.Cmd.Command == "/cancel" {
		ctx.RespondAsync(data.OMessage{SendData: data.SendData{Text: "Command cancelled."}}, nil)
		ctx.SetState(nil)
		return nil
	} else if ctx.Msg.Chat.Type != "private" && ctx.Cmd.Command == "/login" {
		ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: "You should only use this command in private, to protect the security of your account.\n\nIf you accidentally posted your API key publicly, <a href=\"https://" + api.Endpoint + "/users/home\">open your account settings</a> and go to \"Manage API Access\" to revoke it.", ParseMode: data.ParseHTML}}, nil)
		ctx.SetState(nil)
		return nil
	} else if ctx.Cmd.Command == "/logout" {
		storage.WriteUserCreds(nil, storage.UserCreds{TelegramId: ctx.Msg.From.Id})
		ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: "You are now logged out."}}, nil)
		ctx.SetState(nil)
		return nil
	} else if ctx.Cmd.Command == "/login" || ctx.Cmd.Command == "" {
		if ctx.GetState() == nil {
			this = &LoginState{}
			ctx.SetState(this)
		}
		for _, token := range ctx.Cmd.Args {
			if token == "" {
			} else if this.user == "" {
				this.user = token
			} else if this.apikey == "" {
				this.apikey = token
				ctx.DeleteAsync(nil)
			}
			if this.user != "" && this.apikey != "" {
				user, success, err := api.TestLogin(this.user, this.apikey)
				if success && err == nil {
					ctx.RespondAsync(data.OMessage{SendData: data.SendData{Text: fmt.Sprintf("You are now logged in as <code>%s</code>.\n\nTo protect the security of your account, I have deleted the message containing your API key.", this.user), ParseMode: data.ParseHTML}}, nil)
					err = storage.WriteUserCreds(tx, storage.UserCreds{
						TelegramId: ctx.Msg.From.Id,
						User: this.user,
						ApiKey: this.apikey,
						Blacklist: user.Blacklist,
						BlacklistFetched: time.Now(),
					})
					if err != nil {
						return fmt.Errorf("WriteUserCreds: %w", err)
					}
					ctx.SetState(nil)
				} else if err != nil {
					ctx.RespondAsync(data.OMessage{SendData: data.SendData{Text: fmt.Sprintf("An error occurred when testing if you were logged in! (%s)", err.Error())}}, nil)
					ctx.SetState(nil)
					return fmt.Errorf("api.TestLogin: %w", err)
				} else if !success {
					ctx.RespondAsync(data.OMessage{SendData: data.SendData{Text: "Login failed! (api key invalid?)\n\nLet's try again. Please send your " + api.Endpoint + " username."}}, nil)
					this.user = ""
					this.apikey = ""
				}
				return nil
			}
		}

		if this.user == "" {
			ctx.RespondAsync(data.OMessage{SendData: data.SendData{Text: "Please send your " + api.ApiName + " username."}}, nil)
		} else if this.apikey == "" {
			account, err := api.FetchUser(this.user, "")
			if err != nil {
				ctx.RespondAsync(data.OMessage{SendData: data.SendData{Text: "There was an error looking up that username."}}, nil)
				return fmt.Errorf("api.FetchUser: %w", err)
			} else if account == nil {
				ctx.RespondAsync(data.OMessage{SendData: data.SendData{Text: "That username doesn't seem to exist. Please double check, and send it again."}}, nil)
				this.user = ""
			} else {
				ctx.RespondAsync(data.OMessage{SendData: data.SendData{Text: fmt.Sprintf("Please send your " + api.ApiName + " API Key.\n\nYour api key can be found <a href=\"https://" + api.Endpoint + "/users/%d/api_key\">in your account settings</a>. It is used to allow bots and services (such as me!) to access site features on your behalf. Do not share it in public.", account.Id), ParseMode: data.ParseHTML}}, nil)
			}
		}
		return nil
	} else if ctx.Cmd.Command == "/sync" {
		creds, err := storage.GetUserCreds(nil, ctx.Msg.From.Id)
		if err == storage.ErrNoLogin {
			ctx.RespondAsync(data.OMessage{SendData: data.SendData{Text: "You have not connected your " + api.ApiName + " account."}}, nil)
			return nil
		} else if err != nil {
			ctx.RespondAsync(data.OMessage{SendData: data.SendData{Text: "An error occurred! Try again later."}}, nil)
			return fmt.Errorf("GetUserCreds: %w", err)
		}
		user, success, err := api.TestLogin(creds.User, creds.ApiKey)
		if err != nil {
			ctx.RespondAsync(data.OMessage{SendData: data.SendData{Text: "An error occurred while communicating with " + api.ApiName + "! Try again later."}}, nil)
			return fmt.Errorf("api.TestLogin: %w", err)
		} else if !success {
			ctx.RespondAsync(data.OMessage{SendData: data.SendData{Text: "Your API key is invalid or has expired, please update it."}}, nil)
			return nil
		}
		creds.Blacklist = user.Blacklist
		creds.BlacklistFetched = time.Now()
		err = storage.WriteUserCreds(tx, creds)
		if err != nil {
			ctx.RespondAsync(data.OMessage{SendData: data.SendData{Text: "An error occurred while saving your settings! Try again later."}}, nil)
			return fmt.Errorf("WriteUserCreds: %w", err)
		}

		ctx.RespondAsync(data.OMessage{SendData: data.SendData{Text: "Successfully resync'd your " + api.ApiName + " account settings."}}, nil)
	}
	
	return nil
}

type TagRuleState struct {
	gogram.StateBase

	tagwizardrules string
	tagrulename string
}

func (this *TagRuleState) Handle(ctx *gogram.MessageCtx) {
	if ctx.Msg.From == nil {
		return
	}
	if ctx.Cmd.Command == "/cancel" {
		ctx.RespondAsync(data.OMessage{SendData: data.SendData{Text: "Command cancelled."}}, nil)
		ctx.SetState(nil)
		return
	}

	if ctx.GetState() == nil {
		this = &TagRuleState{}
		ctx.SetState(this)
	}

	var doc *data.TDocument
	if ctx.Msg.Document != nil {
		doc = ctx.Msg.Document
	} else if ctx.Msg.ReplyToMessage != nil && ctx.Msg.ReplyToMessage.Document != nil {
		doc = ctx.Msg.ReplyToMessage.Document
	}

	if doc != nil {
		if doc.FileName == nil {
			ctx.RespondAsync(data.OMessage{SendData: data.SendData{Text: "File is missing name."}}, nil)
			return
		}
		if !strings.HasSuffix(*doc.FileName, ".txt") {
			ctx.RespondAsync(data.OMessage{SendData: data.SendData{Text: fmt.Sprintf("<i>%s</i> isn't a plain text file.", html.EscapeString(*doc.FileName)), ParseMode: data.ParseHTML}}, nil)
			return
		}
		file, err := ctx.Bot.Remote.GetFile(data.OGetFile{Id: doc.Id})
		if err != nil || file == nil || file.FilePath == nil {
			ctx.RespondAsync(data.OMessage{SendData: data.SendData{Text: fmt.Sprintf("Error while fetching <i>%s</i>, try sending it again?", html.EscapeString(*doc.FileName)), ParseMode: data.ParseHTML}}, nil)
			return
		}
		file_data, err := ctx.Bot.Remote.DownloadFile(data.OFile{FilePath: *file.FilePath})
		if err != nil || file_data == nil {
			ctx.RespondAsync(data.OMessage{SendData: data.SendData{Text: fmt.Sprintf("Error while downloading <i>%s</i>, try sending it again?", html.EscapeString(*doc.FileName)), ParseMode: data.ParseHTML}}, nil)
			return
		}
		b, err := ioutil.ReadAll(file_data)
		if err != nil || b == nil {
			ctx.RespondAsync(data.OMessage{SendData: data.SendData{Text: fmt.Sprintf("Error while reading <i>%s</i>, try sending it again?", html.EscapeString(*doc.FileName)), ParseMode: data.ParseHTML}}, nil)
			return
		}

		if len(b) > 102400 {
			ctx.RespondAsync(data.OMessage{SendData: data.SendData{Text: fmt.Sprintf("<i>%s</i> is too large (100kB max), edit and try again.", html.EscapeString(*doc.FileName)), ParseMode: data.ParseHTML}}, nil)
			return
		}
		this.tagwizardrules = string(b)
	}

	ctx.Cmd.Argstr = strings.ToLower(ctx.Cmd.Argstr)
	if ctx.Cmd.Argstr == "edit" || ctx.Cmd.Argstr == "upload" {
		this.tagrulename = ctx.Cmd.Argstr
	}

	if this.tagwizardrules == "" {
		ctx.RespondAsync(data.OMessage{SendData: data.SendData{Text: "Send some new tag rules in a text file."}}, nil)
	} else if this.tagrulename != "edit" && this.tagrulename != "upload" {
		ctx.RespondAsync(data.OMessage{SendData: data.SendData{Text: "How do you want to use these? You can choose 'edit' or 'upload'."}}, nil)
	}

	if this.tagwizardrules != "" && this.tagrulename != "" {
		this.tagwizardrules = strings.Replace(this.tagwizardrules, "\r", "", -1) // pesky windows carriage returns
		_ = storage.DefaultTransact(func(tx *sql.Tx) error { return storage.WriteUserTagRules(tx, ctx.Msg.From.Id, this.tagrulename, this.tagwizardrules) })
		ctx.RespondAsync(data.OMessage{SendData: data.SendData{Text: "Set new tag rules."}}, nil)
		ctx.SetState(nil)
	}
}

type psp struct {
	User string `json:"user"`
	ApiKey string `json:"apikey"`

	MsgId data.MsgID `json:"msgid"`
	ChatId data.ChatID `json:"chatid"`
}

type PostState struct {
	persist.StateBasePersistent

	data psp
}

func PostStateFactory(jstr []byte, sbp persist.StateBasePersistent) gogram.State {
	return PostStateFactoryWithData(jstr, sbp, psp{})
}

func PostStateFactoryWithData(jstr []byte, sbp persist.StateBasePersistent, data psp) gogram.State {
	var p PostState
	p.StateBasePersistent = sbp
	p.StateBasePersistent.Persist = &p.data
	p.data = data
	json.Unmarshal(jstr, p.StateBasePersistent.Persist)
	return &p
}

func (this *PostState) WriteUserTagRules(my_id data.UserID, tagrules, name string) {
	_ = storage.DefaultTransact(func(tx *sql.Tx) error { return storage.WriteUserTagRules(tx, my_id, name, tagrules) })
}

func (this *PostState) HandleCallback(ctx *gogram.CallbackCtx) {
	txbox, err := storage.NewTxBox()
	if err != nil {
		ctx.Bot.ErrorLog.Println("Error occurred opening transaction: ", err.Error())
		return
	}
	settings := storage.UpdaterSettings{Transaction: txbox}
	defer settings.Transaction.Finalize(true)

	p, err := dialogs.LoadPostPrompt(settings, this.data.MsgId, this.data.ChatId, ctx.Cb.From.Id, "upload")
	if err != nil {
		ctx.Bot.ErrorLog.Println("Error loading edit prompt: ", err.Error())
		return
	}

	p.HandleCallback(ctx, settings)

	if p.State == dialogs.SAVED {
		upload_result, err := p.CommitPost(this.data.User, this.data.ApiKey, gogram.NewMessageCtx(ctx.Cb.Message, false, ctx.Bot), settings)
		if err == nil && upload_result != nil && upload_result.Success {
			p.Finalize(settings, ctx.Bot, nil, dialogs.NewPostFormatter(ctx.Cb.Message.Chat.Type != data.Private, upload_result))
			ctx.AnswerAsync(data.OCallback{Notification: "\U0001F7E2 Edit submitted."}, nil)
			ctx.SetState(nil)
		} else if err != nil {
			ctx.AnswerAsync(data.OCallback{Notification: fmt.Sprintf("\U0001F534 %s", err.Error())}, nil)
			p.Prompt(settings, ctx.Bot, nil, dialogs.NewPostFormatter(ctx.Cb.Message.Chat.Type != data.Private, nil))
			p.State = dialogs.WAIT_MODE
		} else if upload_result != nil && !upload_result.Success {
			if upload_result.Reason == nil { upload_result.Reason = new(string) }
			ctx.AnswerAsync(data.OCallback{Notification: fmt.Sprintf("\U0001F534 Error: %s", *upload_result.Reason)}, nil)
			p.Prompt(settings, ctx.Bot, nil, dialogs.NewPostFormatter(ctx.Cb.Message.Chat.Type != data.Private, upload_result))
			p.State = dialogs.WAIT_MODE
		}
	} else if p.State == dialogs.DISCARDED {
		p.Finalize(settings, ctx.Bot, nil, dialogs.NewPostFormatter(ctx.Cb.Message.Chat.Type != data.Private, nil))
	} else {
		p.Prompt(settings, ctx.Bot, nil, dialogs.NewPostFormatter(ctx.Cb.Message.Chat.Type != data.Private, nil))
	}
	settings.Transaction.MarkForCommit()
}

func (this *PostState) Handle(ctx *gogram.MessageCtx) {
	if ctx.Cmd.Command == "/cancel" {
		// always react to cancel command
		this.Cancel(ctx)
		if ctx.Msg.Chat.Id != this.data.ChatId {
			ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: "Command cancelled."}}, nil)
		}
		return
	} else if ctx.Cmd.Command == "" && ctx.Msg.Chat.Id != this.data.ChatId {
		// completely ignore non-commands sent to other chats
		return
	} else if ctx.Cmd.Command != "" && ctx.Msg.Chat.Id != this.data.ChatId && this.data.ChatId != 0 {
		// warn users who try to use commands in another chat while this command is active already
		ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: "A command is already in progress somewhere else. To cancel it, use /cancel."}}, nil)
		return
	} else if ctx.Msg.Chat.Type == data.Private {
		// if it's a PM, always process it.
	} else if ctx.Msg.Chat.Type == data.Channel {
		// if it's a channel, never process it
		return
	} else if ctx.Cmd.Command != "" {
		// if it's a command, always process it
	} else if ctx.Cmd.Command == "" && ctx.Msg.ReplyToMessage == nil || ctx.Msg.ReplyToMessage.From.Id != ctx.Bot.Remote.GetMe().Id {
		// if it's not a command AND not a reply to a message sent by the bot, ignore it completely
		return
	}

	del := func() {
		if ctx.Msg.Document != nil && ctx.Msg.ForwardDate == nil {
			// keep the post around if it's a new, non-forwarded file upload
		} else {
			// otherwise, toss it.
			ctx.DeleteAsync(nil)
		}
	}

	if ctx.Cmd.Command == "/post" && ctx.GetState() == nil {
		this.Post(ctx)
		return
	} else if ctx.Cmd.Command == "/reply" {
		newctx := gogram.NewMessageCtx(ctx.Msg.ReplyToMessage, false, ctx.Bot)
		if newctx != nil {
			this.Freeform(newctx)
		}
		del()
	} else {
		this.Freeform(ctx)
		del()
	}
}

func (this *PostState) Post(ctx *gogram.MessageCtx) {
	var p dialogs.PostPrompt

	if ctx.Msg.From == nil { return }

	creds, err := storage.GetUserCreds(nil, ctx.Msg.From.Id)
	if err != nil {
		ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: "You need to be logged in to use this command!"}}, nil)
		if err != storage.ErrNoLogin {
			ctx.Bot.ErrorLog.Println("Error while checking credentials: ", err.Error())
		}
		return
	}

	postnow, err := p.ParseArgs(ctx)
	if err != nil {
		ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: err.Error()}}, nil)
		return
	}

	var tagrules string
	err = storage.DefaultTransact(func(tx *sql.Tx) error {
		var err error
		tagrules, err = storage.GetUserTagRules(tx, ctx.Msg.From.Id, "main")
		return err
	})
	if err != nil {
		ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: "Couldn't load your tag rules for some reason."}}, nil)
		return
	}

		savestate := func(prompt *gogram.MessageCtx) {
			p.TagWizard.SetNewRulesFromString(tagrules)
			ctx.SetState(PostStateFactoryWithData(nil, this.StateBasePersistent, psp{
				User: creds.User,
				ApiKey: creds.ApiKey,
				MsgId: prompt.Msg.Id,
				ChatId: prompt.Msg.Chat.Id,
			}))
		}

		if postnow {
			if err := p.IsComplete(); err != nil {
				p.Status = "Your post isn't ready for upload yet, please fix it and then try to upload it again."
				savestate(p.Prompt(tx, ctx.Bot, ctx, dialogs.NewPostFormatter(ctx.Msg.Chat.Type != data.Private, nil)))
			} else {
				upload_result, err := p.CommitPost(creds.User, creds.ApiKey, ctx)
				if err == nil && upload_result != nil && upload_result.Success {
					p.State = dialogs.SAVED
					p.Finalize(tx, ctx.Bot, ctx, dialogs.NewPostFormatter(ctx.Msg.Chat.Type != data.Private, upload_result))
				} else if err != nil {
					p.State = dialogs.SAVED
					savestate(p.Prompt(tx, ctx.Bot, ctx, dialogs.NewPostFormatter(ctx.Msg.Chat.Type != data.Private, nil)))
					return fmt.Errorf("p.CommitPost: %w", err)
				} else if upload_result != nil && !upload_result.Success {
					if upload_result.Reason == nil { upload_result.Reason = new(string) }
					p.State = dialogs.SAVED
					savestate(p.Prompt(tx, ctx.Bot, ctx, dialogs.NewPostFormatter(ctx.Msg.Chat.Type != data.Private, upload_result)))
				}
			}
		} else {
			upload_result, err := p.CommitPost(creds.User, creds.ApiKey, ctx, storage.UpdaterSettings{})
			if err == nil && upload_result != nil && upload_result.Success {
				p.State = dialogs.SAVED
				p.Finalize(storage.UpdaterSettings{}, ctx.Bot, ctx, dialogs.NewPostFormatter(ctx.Msg.Chat.Type != data.Private, upload_result))
			} else if err != nil {
				p.State = dialogs.SAVED
				savestate(p.Prompt(storage.UpdaterSettings{}, ctx.Bot, ctx, dialogs.NewPostFormatter(ctx.Msg.Chat.Type != data.Private, nil)))
			} else if upload_result != nil && !upload_result.Success {
				if upload_result.Reason == nil { upload_result.Reason = new(string) }
				p.State = dialogs.SAVED
				savestate(p.Prompt(storage.UpdaterSettings{}, ctx.Bot, ctx, dialogs.NewPostFormatter(ctx.Msg.Chat.Type != data.Private, upload_result)))
			}
		}
	} else {
		p.ResetState()
		savestate(p.Prompt(storage.UpdaterSettings{}, ctx.Bot, ctx, dialogs.NewPostFormatter(ctx.Msg.Chat.Type != data.Private, nil)))
	}
}

func (this *PostState) Cancel(ctx *gogram.MessageCtx) {
	txbox, err := storage.NewTxBox()
	if err != nil {
		ctx.Bot.ErrorLog.Println("Error occurred opening transaction: ", err.Error())
		return
	}
	settings := storage.UpdaterSettings{Transaction: txbox}
	defer settings.Transaction.Finalize(true)

	p, err := dialogs.LoadPostPrompt(settings, this.data.MsgId, this.data.ChatId, ctx.Msg.From.Id, "main")
	if err != nil { ctx.Bot.ErrorLog.Println(err.Error()) }
	if p != nil {
		p.State = dialogs.DISCARDED
		p.Finalize(settings, ctx.Bot, nil, dialogs.NewPostFormatter(ctx.Msg.Chat.Type != data.Private, nil))
	}
	ctx.SetState(nil)
	settings.Transaction.MarkForCommit()
}

func (this *PostState) Freeform(ctx *gogram.MessageCtx) {
	txbox, err := storage.NewTxBox()
	if err != nil {
		ctx.Bot.ErrorLog.Println("Error occurred opening transaction: ", err.Error())
		return
	}
	settings := storage.UpdaterSettings{Transaction: txbox}
	defer settings.Transaction.Finalize(true)

	p, err := dialogs.LoadPostPrompt(settings, this.data.MsgId, this.data.ChatId, ctx.Msg.From.Id, "main")
	if err != nil {
		ctx.Bot.ErrorLog.Println("Error occurred loading edit prompt: ", err.Error())
		return
	}

	p.HandleFreeform(ctx)

	p.Prompt(settings, ctx.Bot, nil, dialogs.NewPostFormatter(ctx.Msg.Chat.Type != data.Private, nil))
	settings.Transaction.MarkForCommit()
}

type JanitorState struct {
	gogram.StateBase
}

func (this *JanitorState) Handle(ctx *gogram.MessageCtx) {
	if ctx.Msg.From == nil {
		// ignore messages not sent by a user.
		return
	}

	creds, err := storage.GetUserCreds(nil, ctx.Msg.From.Id)
	if !creds.Janitor {
		// commands from non-authorized users are silently ignored
		return
	}
	if err != nil {
		ctx.ReplyOrPMAsync(data.OMessage{SendData: data.SendData{Text: "You need to be logged in to " + api.ApiName + " to use this command (see <code>/help login</code>)", ParseMode: data.ParseHTML}}, nil)
		return
	}

	if ctx.Cmd.Command == "/indextags" {
		go tagindex.SyncTagsCommand(ctx)
	} else if ctx.Cmd.Command == "/indextagaliases" {
		go tagindex.SyncAliasesCommand(ctx)
	} else if ctx.Cmd.Command == "/syncposts" {
		go tagindex.SyncPostsCommand(ctx)
	} else if ctx.Cmd.Command == "/cats" {
		go tagindex.Concatenations(ctx)
	} else if ctx.Cmd.Command == "/blits" {
		go tagindex.Blits(ctx)
	} else if ctx.Cmd.Command == "/typos" {
		go tagindex.Typos(ctx)
	} else if ctx.Cmd.Command == "/recounttags" {
		go tagindex.RecountTagsCommand(ctx)
	} else if ctx.Cmd.Command == "/resyncdeleted" {
		go tagindex.RefetchDeletedPostsCommand(ctx)
	} else if ctx.Cmd.Command == "/resynclist" {
		go tagindex.ResyncListCommand(ctx)
	}
}
