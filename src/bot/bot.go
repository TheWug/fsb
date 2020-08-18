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
`.public.example.wizard.post.advanced.faq.login.contact. Hello! I'm the <b>` + api.ApiName + ` Telegram Bot</b>!
.public.example.wizard.post.advanced.faq.login.contact.
.public.example.wizard.post.advanced.faq.login.contact.  Content on ` + api.ApiName + ` may be unsuitable for kids.
.public.example.wizard.post.advanced.faq.login.contact.  <b>You must be 18 or older to use this bot.</b>
.public. 
.public. This bot's commands and help messages should be used via PM.
.advanced. 
.advanced. <b>General Usage</b>
.advanced. I work like @gif! Simply type my name, followed by search terms.
.advanced. <code>@fsb frosted_butts</code>
.advanced. All tags which are supported on the site work here!
login.advanced.janitor. 
login.advanced.janitor. <b>Using Your Account</b>
login.advanced.janitor. Some of my features require you to connect your ` + api.ApiName + ` account.
login.advanced.janitor. <code>/login [user] [apikey] -</code> connect to your ` + api.ApiName + ` account.
login.advanced.janitor. <code>/logout                -</code> disconnect your account.
.advanced. 
.advanced. <b>Posting</b>
.advanced. Upload or edit posts. You must connect your ` + api.ApiName + ` account.
.advanced. <code>/post        ... -</code> posts a new file.
.advanced. <code>/settagrules ... -</code> updates your tag rules.
janitor. 
janitor. <b>Janitor Commands</b>
janitor. For a full description of any command, use <code>/help [command]</code>.
janitor.cats. <code>/cats</code>
cats. A <i>CAT</i> is a tag which is two other tags concatenated together. These tags are typos, and are added to posts by accident (if it isn't an accident, it's not a <i>CAT</i>). This command helps search for <i>CAT</i>s, resolve them to their correct tags, and automatically apply them to posts.
cats. <i>Listing</i> options:
cats. <i>CAT</i>s are not shown if they are already listed in the database. <i>BLIT</i>s are also not elligible to be part of <i>CAT</i>s.
cats. <code> (no arguments)    -</code> list a few random possible <i>CAT</i>s
cats. <code> --all,     -a     -</code> include known and <i>BLIT CAT</i>s
cats. <code> --inspect, -i TAG -</code> List all possible <i>CAT</i>s including <code>TAG</code>
cats. <code> --ratio,   -r N   -</code> <i>CAT</i>s must be <code>N</code> times rarer than base tags
cats. <i>Selection</i> options:
cats. <code> --entry,  -e N     -</code> reply to listing to select <i>CAT</i> N
cats. <code> --select, -s T1 T2 -</code> manually specify <i>CAT</i> formed by <code>T1</code> + <code>T2</code>
cats. <i>Editing</i> options:
cats. These options all apply to any previous selection options, since the last edit option was specified. They "forget" the list of specified cats when they are used. <code>--fix</code> can be paired with any of the other options, otherwise they are all mutually exclusive.
cats. <code> --ignore,  -I -</code> not a cat, no corrective action or listings
cats. <code> --notify,  -N -</code> matching posts will prompt for corrective action
cats. <code> --autofix, -X -</code> matching posts will be automatically corrected
cats. <code> --delete,  -D -</code> remove this entry from the database
cats. <code> --fix,     -x -</code> fix matching posts right now
janitor.blits. <code>/blits</code>	
blits. A <i>BLIT</i> is a tag that is not eligible to be part of a <i>CAT</i>. Such tags are usually very short, and occur coincidentally at the start or end of other tags. All tags 2 characters or shorter are <i>BLIT</i>s by default, and all tags longer than that are not. This can be overridden or clarified for specific tags using this command.
blits. Listing options:
blits. <code> (no arguments) -</code> list some candidate <i>BLIT</i>s with no status
blits. <code> --list-yes, -y -</code> list known <i>BLIT</i>s
blits. <code> --list-no,  -n -</code> list known non-<i>BLIT</i>s
blits. <code> --list,     -l -</code> shorthand for -y and -n
blits. Editing options:
blits. <code> --mark,   -M TAG -</code> mark <code>TAG</code> as a <i>BLIT</i>
blits. <code> --ignore, -I TAG -</code> mark <code>TAG</code> as a non-<i>BLIT</i>
blits. <code> --delete, -D TAG -</code> clear <code>TAG</code> entirely from <i>BLIT</i> list
janitor.syncposts. <code>/syncposts</code>
syncposts. This command is used to keep my internal index of ` + api.ApiName + ` data up to date. This index is used to speed up operations like searching for similar tag names and listing all posts with a tag. Some metadata is maintained for each tag, each tag alias, and each post, on the site.
syncposts. <i>Control</i> options:
syncposts. <code> (no arguments) -</code> incremental sync of tags and posts (default)
syncposts. <code> --full         -</code> discard local database and sync from scratch
syncposts. <code> --aliases      -</code> sync tag aliases as well
syncposts. <code> --recount      -</code> tally post tag counts afterwards
syncposts. You do not normally need to use this command. Commands which push changes to ` + api.ApiName + ` should apply them locally as well, and an incremental sync is performed by the bot's internal maintenance routine every five minutes (with an alias sync and a tag recount happening every 60 minutes).
najitor.indextags. <code>/indextags</code>
indextags. This command syncs new changes on ` + api.ApiName + ` to the local tag database.
indextags. <i>Control</i> options:
indextags. <code> (no arguments) -</code> perform an incremental sync of tags
indextags. <code> --full         -</code> discard local database and sync from scratch
indextags. This operation is invoked by <code>/syncposts</code>, which passes <code>--full</code> to this command if it is present.
janitor.indextagaliases. <code>/indextagaliases</code>
indextagaliases. This command syncs tag aliases between ` + api.ApiName + ` and the local alias database. Because of how aliases are listed on ` + api.ApiName + `, an incremental sync is not possible, and a full sync is always performed. This command takes no options. It is invoked by <code>/syncposts</code> if <code>--aliases</code> is specified.
janitor.findtagtypos. <code>/findtagtypos</code>
findtagtypos. This command searches for likely typos of a tag, as determined by their edit distance to other tags. The way you should use this command is broadly at first, listing all typos, and then more and more specifically as you investigate each possible option on the site, adding selection options until you have a comprehensive, accurate listing of typos, then apply them to the site by issuing the command again with <code>--fix</code> and using <code>--save</code> to store them for auto-fixing in the future.
findtagtypos. <i>Listing</i> options:
findtagtypos. <code> [other args] TAG  -</code> find typos of <code>TAG</code> (required)
findtagtypos. <code> --all,         -a -</code> include known or ignored possible typos
findtagtypos. <code> --all-posts,   -p -</code> include deleted posts
findtagtypos. <code> --show-short,  -s -</code> include tags less than 3 characters long
findtagtypos. <code> --show-zero,   -z -</code> show tags with zero posts
findtagtypos. <code> --only-general,-g -</code> hide specialty tags (<code>--include</code> overrides)
findtagtypos. <code> --threshhold,-t N -</code> show tags with edit distance <code>N</code> or less
findtagtypos. <i>Selection</i> options:
findtagtypos. <code> --exclude,  -e E -</code> mark <code>E</code> as not a typo
findtagtypos. <code> --distinct, -d E -</code> excludes all tags closer to <code>E</code> than <code>TAG</code>
findtagtypos. <code> --include,  -i I -</code> mark <code>I</code> as a typo
findtagtypos. <i>Editing</i> options:
findtagtypos. Saving changes will store <code>--exclude</code> tags for ignore behavior, and <code>--include</code> tags with prompt-to-fix behavior by default.
findtagtypos. <code> --save,    -S -</code> store changes for corrective action
findtagtypos. <code> --autofix, -X -</code> tags are marked for autofix instead of prompt
findtagtypos. <code> --fix,     -x -</code> push changes to ` + api.ApiName + ` now
findtagtypos. <code> --reason,-r R -</code> use edit reason <code>R</code> instead of "likely typo"
janitor.recounttags. <code>/recounttags</code>
recounttags. This command recounts the cached tag counts, providing an accurate count (the site itself becomes desynced sometimes and its counts are not always accurate). It does so for both visible and deleted posts. It takes no arguments. It is invoked by <code>/syncposts</code> if the <code>--recount</code> option is specified.
janitor.resyncdeleted. <code>/resyncdeleted</code>
resyncdeleted. <s>This command is disabled.</s> You should not need to use it. It enumerates all deleted posts from ` + api.ApiName + ` and updates the local database's deleted status. It exists because at one point, that information was not stored, but it affects certain parts of the API (namely, ordinary users can no longer edit deleted posts) and it needed to be re-imported. It takes no options. If you need to use it again, you should clear the deleted status of all posts manually from the database console first.
janitor.resynclist. <code>/resynclist</code>
resynclist. Use this command captioned on an uploaded file, containing whitespace delimited post ids (and comments beginning with #). The bot will perform a local DB sync on each post listed in the file.
post. 
post. Post Command
post. Posting a file to ` + api.ApiName + ` requires gathering some information. This command pulls everything together, and then does an upload. You must connect to your ` + api.ApiName + ` account to use this.
post. <code>/post</code>
post. <code>/post (reply to file)</code>
post. <code>/post (file caption)</code> 
post. <code>/post [url]</code>
post. 
post. The following subcommands exist:
post. <code>/cancel          -</code> cancel this post.
post. <code>/file, /f        -</code> set the post file.
post. <code>/tag, /t         -</code> set the post tags.
post. <code>/wizard, /w      -</code> use the tagging wizard.
post. <code>/rating, /r      -</code> set the post rating.
post. <code>/source, /s      -</code> set the source.
post. <code>/description, /d -</code> set the description.
post. <code>/parent, /p      -</code> set the parent post.
post. <code>/reset           -</code> reset everything and start over.
post. <code>/preview         -</code> show all of the collected info.
post. <code>/upload          -</code> upload the file and finish!
edit.
edit. Edit Command
edit. Editing a post on ` + api.ApiName + ` requires gathering some information. This command pulls everything together, then does an update. You must connect to your ` + api.ApiName + ` account to use this.
edit. <code>/edit [OPTIONS] (reply to message with ` + api.ApiName + ` post URL)</code>
edit. <code>/edit [FILE ID] [OPTIONS] </code>
edit. <code>/edit [` + api.ApiName + ` URL] [OPTIONS]</code>
edit.
edit. OPTIONS can be any of the following:
edit. <code>--tags [T]    -</code> space delimited list of tags
edit. <code>--sources     -</code> newline delimited list of sources
edit. <code>--rating      -</code> one of safe, questionable, explicit
edit. <code>--description -</code> post description (<a href="https://` + api.Endpoint + `/help/dtext">dtext help</a>)
edit. <code>--parent      -</code> parent post id, or "none"
edit. <code>--reason      -</code> edit reason, shown in edit history
edit. <code>--file        -</code> replace post from file
edit. <code>--url         -</code> replace post from url
edit. <code>--commit      -</code> save changes immediately
edit.
edit. Replacing posts is not yet supported. although the options are present.
edit.
edit. To cancel an edit in process, you can use <code>/cancel</code>, or the discard button.
edit.
edit. Use the buttons on the edit wizard to configure the edit you wish to make. The default for every option is "leave it the way it was".
edit. Notes: tags support three prefixes. <code>+</code> (implied if no prefix is present) adds a tag, <code>-</code> removes it, and <code>=</code> resets to "no change". Sources also operate as a diff, supporting the + and - prefixes.
.wizard. 
.wizard. <b>The tag wizard</b>
.wizard. The tag wizard is probably my most powerful feature. With this, you can specify your own suggestions for tagging posts, and the wizard will guide you through them, one at a time.
. To read more about the tag wizard, use <code>/help wizard</code>.
wizard. The syntax for tag rules is as follows:
wizard. <code>[prereq ...] [control ...] . [option ...]</code>
wizard. <code>option  -</code> tags to display. can be toggled on and off.
wizard. <code>prereq  -</code> tags which must be present to show the options.
wizard. <code>control -</code> special tags which affect behavior.
wizard. The following control options exist:
wizard. <code>sort:{no|alpha}:{asc|desc}   -</code> Automatically sort options (default = alpha, asc)
wizard. <code>prompt:"....."               -</code> A helpful message describing the options.
dev. <code>auto:{no|add|remove|replace} -</code> Automatically apply all options here.
wizard. Also, you can prepend the following bits to an option, with the following effects:
wizard. <code>meta:   -</code> display-only. useful for categorizing, doesn't generate a tag.
wizard. <code>rating: -</code> sets the post rating. if there's more than one, the worst is used.
wizard. <code>x:      -</code> automatically set this option
wizard. <code>o:      -</code> automatically clear this option
wizard. <code>h:      -</code> hide this option from the list
wizard. You can use more than one, like <code>xh:meta:foobar</code>.
wizard. Your tag rules can be up to 100KB.
wizard. To see a tag rule example, use <code>/help example</code>.
example. 
example. <b>Tag Rule Example</b>
example. <pre>prompt:"Overview:" . meta:SFW meta:NSFW meta:Kinky_stuff
example. meta:SFW . fur scales background hx:rating:s
example. meta:NSFW . sex penis pussy penetration hx:rating:e
example. meta:Kinky_stuff . pet_play food_fetish vore</pre>
.faq. 
.faq. <b>Important Info and FAQ</b>
.faq. <code>*</code> Before posting to ` + api.ApiName + `, please make sure you read the site's rules.
.faq. <code>*</code> Your account standing is your own responsibility.
.faq. <code>*</code> Your ` + api.ApiName + ` API key is NOT your password. To find it, go to your <a href="https://` + api.Endpoint + `/users/home">Account Settings</a> and click "Manage API Access".
.faq. <code>*</code> To report a bug, see <code>/help contact.</code>
contact. 
contact. <b>Contacting the author</b>
contact. You may be contacted by the bot author for more information.
contact. <code>/operator [what's wrong] -</code> Flag something for review.
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
		this.Behavior.ClearPromptPostsOlderThan(bot, time.Hour * 24)
	}()
}

func (this *AutofixState) Handle(ctx *gogram.MessageCtx) {
	return // ignore messages
}

func (this *AutofixState) HandleCallback(ctx *gogram.CallbackCtx) {
	go func() {
		defer ctx.AnswerAsync(data.OCallback{}, nil) // non-specific acknowledge if we return without answering explicitly
		// don't bother with any callbacks that are so old their message is no longer available
		if ctx.MsgCtx == nil { return }

		var err error
		settings := storage.UpdaterSettings{}
		settings.Transaction, err = storage.NewTxBox()
		if err != nil {
			ctx.Bot.Log.Println("Error in maintenance loop:", err.Error())
			return
		}
		defer settings.Transaction.Finalize(true)

		user, api_key, janitor, err := storage.GetUserCreds(storage.UpdaterSettings{}, ctx.Cb.From.Id)
		if err == storage.ErrNoLogin {
			ctx.AnswerAsync(data.OCallback{Notification: "\U0001F512 You need to login to do that!\n(use /login, in PM)", ShowAlert: true}, nil)
			return
		}

		if !janitor {
			ctx.AnswerAsync(data.OCallback{Notification: "\U0001F512 Sorry, this feature is currently limited to janitors.", ShowAlert: true}, nil)
			return
		}

		post_info, err := storage.FindPromptPostByMessage(ctx.MsgCtx.Msg.Chat.Id, ctx.MsgCtx.Msg.Id, storage.UpdaterSettings{})
		if post_info == nil { return }

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
				if err != nil { return } // this should only happen if users are spoofing callback data, so ignore it.
				if set {
					post_info.Edit.Select(ctx.Cmd.Args[0], index)
				} else {
					post_info.Edit.Deselect(ctx.Cmd.Args[0], index)
				}
			}
			err = storage.SavePromptPost(post_info.PostId, post_info, settings)
			this.Behavior.PromptPost(ctx.Bot, post_info, post_info.PostId, nil, nil)
		} else if ctx.Cmd.Command == "/af-commit" {
			diff := post_info.Edit.GetChangeToApply()

			if diff.IsZero() {
				ctx.AnswerAsync(data.OCallback{Notification: "\u2139 No changes to apply."}, nil)
				this.Behavior.DismissPromptPost(ctx.Bot, post_info, diff, settings)
			} else {
				reason := "Manual tag cleanup: typos and concatenations (via KnottyBot)"
				post, err := api.UpdatePost(user, api_key, post_info.PostId, diff, nil, nil, nil, nil, &reason)
				if err != nil {
					ctx.AnswerAsync(data.OCallback{Notification: "\u26A0 An error occurred when trying to update the post! Try again later."}, nil)
					return
				}

				post_info.Edit.Apply()
				this.Behavior.DismissPromptPost(ctx.Bot, post_info, diff, settings)
				ctx.AnswerAsync(data.OCallback{Notification: "\U0001F539 Changes saved."}, nil)

				if post != nil {
					err = storage.UpdatePost(*post, settings)
					if err != nil { ctx.Bot.ErrorLog.Println("Failed to locally update post:", err.Error()) }
				}
			}
		} else if ctx.Cmd.Command == "/af-dismiss" {
			err = this.Behavior.DismissPromptPost(ctx.Bot, post_info, tags.TagDiff{}, settings)
			if err != nil {
				if err != nil { ctx.Bot.ErrorLog.Println("Failed to dismiss prompt post:", err.Error()) }
				return
			}
			ctx.AnswerAsync(data.OCallback{Notification: "\U0001F539 Dismissed without changes."}, nil)
		}

		settings.Transaction.MarkForCommit()
	}()
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

	user, api_key, _, err := storage.GetUserCreds(storage.UpdaterSettings{}, from.Id)
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
		id, err = strconv.Atoi(cmd.Args[0])
		if err != nil {
			response.Text = fmt.Sprintf("Error parsing post ID: %s", err.Error())
			return response, true
		}
	} else {
		if reply_message != nil {
			text := reply_message.Text
			if text == nil { text = reply_message.Caption }
			if text != nil {
				potential_id := apiextra.GetPostIDFromText(*text)
				if potential_id != apiextra.NONEXISTENT_POST {
					// if we are a reply to a message, AND that message has text or a caption, AND that text contains a post URL, yoink it.
					id = potential_id
				}
			}
		}
	}

	// if after all that, the id is still the zero value, that means we didn't find one, so die
	if id == 0 {
		response.Text = "You must to specify a post ID."
		return response, true
	}

	if cmd.Command == "/upvote" {
		if this.MarkAndTestRecentlyVoted(from.Id, apitypes.Upvote, id) {
			err = api.UnvotePost(user, api_key, id)
			if err != nil {
				response.Text = "An error occurred when removing your vote! (Is " + api.ApiName + " down?)"
				bot.ErrorLog.Printf("Error when unvoting post %d: %s\n", id, err.Error())
			} else {
				response.Text = "\U0001F5D1 You have deleted your vote."
			}
		} else {
			_, err := api.VotePost(user, api_key, id, apitypes.Upvote, true)
			if err != nil {
				response.Text = "An error occurred when voting! (Is " + api.ApiName + " down?)"
				bot.ErrorLog.Printf("Error when voting post %d: %s\n", id, err.Error())
			} else {
				response.Text = "\U0001F7E2 You have upvoted this post! (Click again to cancel your vote)"
			}
		}
	} else if cmd.Command == "/downvote" {
		if this.MarkAndTestRecentlyVoted(from.Id, apitypes.Downvote, id) {
			err = api.UnvotePost(user, api_key, id)
			if err != nil {
				response.Text = "An error occurred when removing your vote! (Is " + api.ApiName + " down?)"
				bot.ErrorLog.Printf("Error when unvoting post %d: %s\n", id, err.Error())
			} else {
				response.Text = "\U0001F5D1 You have deleted your vote."
			}
		} else {
			_, err := api.VotePost(user, api_key, id, apitypes.Downvote, true)
			if err != nil {
				response.Text = "An error occurred when voting! (Is " + api.ApiName + " down?)"
				bot.ErrorLog.Printf("Error when voting post %d: %s\n", id, err.Error())
			} else {
				response.Text = "\U0001F534 You have downvoted this post! (Click again to cancel your vote)"
			}
		}
	} else if cmd.Command == "/favorite" {
		if this.MarkAndTestRecentlyFaved(from.Id, id) {
			err = api.UnfavoritePost(user, api_key, id)
			if err != nil {
				response.Text = "An error occurred when unfavoriting the post! (Is " + api.ApiName + " down?)"
				bot.ErrorLog.Printf("Error when unfaving post %d: %s\n", id, err.Error())
			} else {
				response.Text = "\U0001F5D1 You have unfavorited this post."
			}
		} else {
			_, err = api.FavoritePost(user, api_key, id)
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
	if ctx.Cmd.Command == "/edit" && ctx.GetState() == nil {
		this.Edit(ctx)
	} else if ctx.Cmd.Command == "/cancel" {
		this.Cancel(ctx)
	} else {
		this.Freeform(ctx)
	}
}

func (this *EditState) HandleCallback(ctx *gogram.CallbackCtx) {
	txbox, err := storage.NewTxBox()
	if err != nil {
		ctx.Bot.ErrorLog.Println("Error occurred opening transaction: ", err.Error())
		return
	}
	settings := storage.UpdaterSettings{Transaction: txbox}
	defer settings.Transaction.Finalize(true)

	p, err := dialogs.LoadEditPrompt(settings, this.data.MsgId, this.data.ChatId)
	if err != nil {
		ctx.Bot.ErrorLog.Println("Error loading edit prompt: ", err.Error())
		return
	}

	p.HandleCallback(ctx, settings)

	if p.State == dialogs.SAVED {
		_, err := p.CommitEdit(this.data.User, this.data.ApiKey, gogram.NewMessageCtx(ctx.Cb.Message, false, ctx.Bot), settings)
		if err == nil {
			p.Finalize(settings, ctx.Bot, nil, dialogs.NewEditFormatter(!*ctx.Bot.Remote.GetMe().CanReadAllGroupMessages, nil))
			ctx.AnswerAsync(data.OCallback{Notification: "\U0001F7E2 Edit submitted."}, nil)
			ctx.SetState(nil)
		} else {
			ctx.AnswerAsync(data.OCallback{Notification: fmt.Sprintf("\U0001F534 %s", err.Error())}, nil)
			p.Prompt(settings, ctx.Bot, nil, dialogs.NewEditFormatter(!*ctx.Bot.Remote.GetMe().CanReadAllGroupMessages, err))
			p.State = dialogs.WAIT_MODE
		}
	} else if p.State == dialogs.DISCARDED {
		p.Finalize(settings, ctx.Bot, nil, dialogs.NewEditFormatter(!*ctx.Bot.Remote.GetMe().CanReadAllGroupMessages, nil))
	} else {
		p.Prompt(settings, ctx.Bot, nil, dialogs.NewEditFormatter(!*ctx.Bot.Remote.GetMe().CanReadAllGroupMessages, nil))
	}
	settings.Transaction.MarkForCommit()
}

func (this *EditState) Freeform(ctx *gogram.MessageCtx) {
	txbox, err := storage.NewTxBox()
	if err != nil {
		ctx.Bot.ErrorLog.Println("Error occurred opening transaction: ", err.Error())
		return
	}
	settings := storage.UpdaterSettings{Transaction: txbox}
	defer settings.Transaction.Finalize(true)

	p, err := dialogs.LoadEditPrompt(settings, this.data.MsgId, this.data.ChatId)
	if err != nil {
		ctx.Bot.ErrorLog.Println("Error occurred loading edit prompt: ", err.Error())
		return
	}

	p.HandleFreeform(ctx)

	p.Prompt(settings, ctx.Bot, nil, dialogs.NewEditFormatter(!*ctx.Bot.Remote.GetMe().CanReadAllGroupMessages, nil))
	ctx.DeleteAsync(nil)
	settings.Transaction.MarkForCommit()
}

func (this *EditState) Cancel(ctx *gogram.MessageCtx) {
	txbox, err := storage.NewTxBox()
	if err != nil {
		ctx.Bot.ErrorLog.Println("Error occurred opening transaction: ", err.Error())
		return
	}
	settings := storage.UpdaterSettings{Transaction: txbox}
	defer settings.Transaction.Finalize(true)

	p, err := dialogs.LoadEditPrompt(settings, this.data.MsgId, this.data.ChatId)
	if err != nil { ctx.Bot.ErrorLog.Println(err.Error()) }
	if p != nil {
		p.State = dialogs.DISCARDED
		p.Prefix = ""
		p.Finalize(settings, ctx.Bot, nil, dialogs.NewEditFormatter(!*ctx.Bot.Remote.GetMe().CanReadAllGroupMessages, nil))
	}
	ctx.SetState(nil)
	settings.Transaction.MarkForCommit()
}

func (this *EditState) Edit(ctx *gogram.MessageCtx) {
	var e dialogs.EditPrompt

	if ctx.Msg.From == nil { return }

	user, api_key, _, err := storage.GetUserCreds(storage.UpdaterSettings{}, ctx.Msg.From.Id)
	if err != nil {
		ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: "You need to be logged in to use this command!"}}, nil)
		if err != storage.ErrNoLogin {
			ctx.Bot.ErrorLog.Println("Error while checking credentials: ", err.Error())
		}
		return
	}

	savenow, err := e.ParseArgs(ctx)
	if err != nil {
		ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: err.Error()}}, nil)
		return
	}

	if e.PostId <= 0 {
		ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: "Sorry, I can't figure out which post you're talking about!\n\nYou can reply to a message with a post URL, or you can pass an ID or a link directly."}}, nil)
		return
	}

	e.OrigSources = make(map[string]int)

	post_data, err := storage.PostByID(e.PostId, storage.UpdaterSettings{})
	if post_data != nil {
		for _, s := range post_data.Sources {
			e.SeeSource(s)
			e.OrigSources[s] = 1
		}
	}

	if savenow {
		e.CommitEdit(user, api_key, ctx, storage.UpdaterSettings{})
	} else {
		e.ResetState()
		prompt := e.Prompt(storage.UpdaterSettings{}, ctx.Bot, ctx, dialogs.NewEditFormatter(!*ctx.Bot.Remote.GetMe().CanReadAllGroupMessages, nil))
		ctx.SetState(EditStateFactoryWithData(nil, this.StateBasePersistent, esp{
			User: user,
			ApiKey: api_key,
			MsgId: prompt.Msg.Id,
			ChatId: prompt.Msg.Chat.Id,
		}))
	}
}

type LoginState struct {
	gogram.StateBase

	user	string
	apikey	string
}

func (this *LoginState) Handle(ctx *gogram.MessageCtx) {
	if ctx.Msg.From == nil {
		return
	}
	if ctx.Cmd.Command == "/cancel" {
		ctx.RespondAsync(data.OMessage{SendData: data.SendData{Text: "Command cancelled."}}, nil)
		ctx.SetState(nil)
		return
	} else if ctx.Msg.Chat.Type != "private" && ctx.Cmd.Command == "/login" {
		ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: "You should only use this command in private, to protect the security of your account.\n\nIf you accidentally posted your API key publicly, <a href=\"https://" + api.Endpoint + "/users/home\">open your account settings</a> and go to \"Manage API Access\" to revoke it.", ParseMode: data.ParseHTML}}, nil)
		ctx.SetState(nil)
		return
	} else if ctx.Cmd.Command == "/logout" {
		storage.WriteUserCreds(storage.UpdaterSettings{}, ctx.Msg.From.Id, "", "")
		ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: "You are now logged out."}}, nil)
		ctx.SetState(nil)
		return
	} else {
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
				success, err := api.TestLogin(this.user, this.apikey)
				if success && err == nil {
					ctx.RespondAsync(data.OMessage{SendData: data.SendData{Text: fmt.Sprintf("You are now logged in as <code>%s</code>.\n\nTo protect the security of your account, I have deleted the message containing your API key.", this.user), ParseMode: data.ParseHTML}}, nil)
					storage.WriteUserCreds(storage.UpdaterSettings{}, ctx.Msg.From.Id, this.user, this.apikey)
					ctx.SetState(nil)
				} else if err != nil {
					ctx.RespondAsync(data.OMessage{SendData: data.SendData{Text: fmt.Sprintf("An error occurred when testing if you were logged in! (%s)", html.EscapeString(err.Error())), ParseMode: data.ParseHTML}}, nil)
					ctx.SetState(nil)
				} else if !success {
					ctx.RespondAsync(data.OMessage{SendData: data.SendData{Text: "Login failed! (api key invalid?)\n\nLet's try again. Please send your " + api.Endpoint + " username."}}, nil)
					this.user = ""
					this.apikey = ""
				}
				return
			}
		}

		if this.user == "" {
			ctx.RespondAsync(data.OMessage{SendData: data.SendData{Text: "Please send your " + api.ApiName + " username."}}, nil)
		} else if this.apikey == "" {
			account, err := api.FetchUser(this.user, "")
			if err != nil {
				ctx.RespondAsync(data.OMessage{SendData: data.SendData{Text: "There was an error looking up that username."}}, nil)
				ctx.Bot.ErrorLog.Printf("Error looking up user [%s]: %s\n", this.user, err.Error())
			} else if account == nil {
				ctx.RespondAsync(data.OMessage{SendData: data.SendData{Text: "That username doesn't seem to exist. Please double check, and send it again."}}, nil)
				this.user = ""
			} else {
				ctx.RespondAsync(data.OMessage{SendData: data.SendData{Text: fmt.Sprintf("Please send your " + api.ApiName + " API Key.\n\nYour api key can be found <a href=\"https://" + api.Endpoint + "/users/%d/api_key\">in your account settings</a>. It is used to allow bots and services (such as me!) to access site features on your behalf. Do not share it in public.", account.Id), ParseMode: data.ParseHTML}}, nil)
			}
		}
		return
	}
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
			ctx.RespondAsync(data.OMessage{SendData: data.SendData{Text: fmt.Sprintf("%s isn't a plain text file.", html.EscapeString(*doc.FileName)), ParseMode: data.ParseHTML}}, nil)
			return
		}
		file, err := ctx.Bot.Remote.GetFile(data.OGetFile{Id: doc.Id})
		if err != nil || file == nil || file.FilePath == nil {
			ctx.RespondAsync(data.OMessage{SendData: data.SendData{Text: fmt.Sprintf("Error while fetching %s, try sending it again?", html.EscapeString(*doc.FileName)), ParseMode: data.ParseHTML}}, nil)
			return
		}
		file_data, err := ctx.Bot.Remote.DownloadFile(data.OFile{FilePath: *file.FilePath})
		if err != nil || file_data == nil {
			ctx.RespondAsync(data.OMessage{SendData: data.SendData{Text: fmt.Sprintf("Error while downloading %s, try sending it again?", html.EscapeString(*doc.FileName)), ParseMode: data.ParseHTML}}, nil)
			return
		}
		b, err := ioutil.ReadAll(file_data)
		if err != nil || b == nil {
			ctx.RespondAsync(data.OMessage{SendData: data.SendData{Text: fmt.Sprintf("Error while reading %s, try sending it again?", html.EscapeString(*doc.FileName)), ParseMode: data.ParseHTML}}, nil)
			return
		}

		if len(b) > 102400 {
			ctx.RespondAsync(data.OMessage{SendData: data.SendData{Text: fmt.Sprintf("%s is too large (100kB max), edit and try again.", html.EscapeString(*doc.FileName)), ParseMode: data.ParseHTML}}, nil)
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
		storage.WriteUserTagRules(storage.UpdaterSettings{}, ctx.Msg.From.Id, this.tagrulename, this.tagwizardrules)
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
	storage.WriteUserTagRules(storage.UpdaterSettings{}, my_id, name, tagrules)
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
			p.Finalize(settings, ctx.Bot, nil, dialogs.NewPostFormatter(!*ctx.Bot.Remote.GetMe().CanReadAllGroupMessages, upload_result))
			ctx.AnswerAsync(data.OCallback{Notification: "\U0001F7E2 Edit submitted."}, nil)
			ctx.SetState(nil)
		} else if err != nil {
			ctx.AnswerAsync(data.OCallback{Notification: fmt.Sprintf("\U0001F534 %s", err.Error())}, nil)
			p.Prompt(settings, ctx.Bot, nil, dialogs.NewPostFormatter(!*ctx.Bot.Remote.GetMe().CanReadAllGroupMessages, nil))
			p.State = dialogs.WAIT_MODE
		} else if upload_result != nil && !upload_result.Success {
			if upload_result.Reason == nil { upload_result.Reason = new(string) }
			ctx.AnswerAsync(data.OCallback{Notification: fmt.Sprintf("\U0001F534 Error: %s", *upload_result.Reason)}, nil)
			p.Prompt(settings, ctx.Bot, nil, dialogs.NewPostFormatter(!*ctx.Bot.Remote.GetMe().CanReadAllGroupMessages, upload_result))
			p.State = dialogs.WAIT_MODE
		}
	} else if p.State == dialogs.DISCARDED {
		p.Finalize(settings, ctx.Bot, nil, dialogs.NewPostFormatter(!*ctx.Bot.Remote.GetMe().CanReadAllGroupMessages, nil))
	} else {
		p.Prompt(settings, ctx.Bot, nil, dialogs.NewPostFormatter(!*ctx.Bot.Remote.GetMe().CanReadAllGroupMessages, nil))
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

	user, api_key, _, err := storage.GetUserCreds(storage.UpdaterSettings{}, ctx.Msg.From.Id)
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

	if postnow {
		p.CommitPost(user, api_key, ctx, storage.UpdaterSettings{})
	} else {
		tagrules, err := storage.GetUserTagRules(storage.UpdaterSettings{}, ctx.Msg.From.Id, "main")
		if err != nil {
			ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: "Couldn't load your tag rules for some reason."}}, nil)
			return
		}

		p.TagWizard.SetNewRulesFromString(tagrules)
		p.ResetState()
		prompt := p.Prompt(storage.UpdaterSettings{}, ctx.Bot, ctx, dialogs.NewPostFormatter(!*ctx.Bot.Remote.GetMe().CanReadAllGroupMessages, nil))
		ctx.SetState(PostStateFactoryWithData(nil, this.StateBasePersistent, psp{
			User: user,
			ApiKey: api_key,
			MsgId: prompt.Msg.Id,
			ChatId: prompt.Msg.Chat.Id,
		}))
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
		p.Prefix = ""
		p.Finalize(settings, ctx.Bot, nil, dialogs.NewPostFormatter(!*ctx.Bot.Remote.GetMe().CanReadAllGroupMessages, nil))
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

	p.Prompt(settings, ctx.Bot, nil, dialogs.NewPostFormatter(!*ctx.Bot.Remote.GetMe().CanReadAllGroupMessages, nil))
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

	user, apikey, janitor, err := storage.GetUserCreds(storage.UpdaterSettings{}, ctx.Msg.From.Id)
	if !janitor {
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
	} else if ctx.Cmd.Command == "/findtagtypos" {
		go tagindex.FindTagTypos(ctx)
	} else if ctx.Cmd.Command == "/recounttags" {
		go tagindex.RecountTagsCommand(ctx)
	} else if ctx.Cmd.Command == "/resyncdeleted" {
		go tagindex.RefetchDeletedPostsCommand(ctx)
	} else if ctx.Cmd.Command == "/resynclist" {
		go tagindex.ResyncListCommand(ctx)
	} else if ctx.Cmd.Command == "/editposttest" {
		post := 2893902 // https://api-host/post/show/2893902
		newtags := "1:1 2021 anthro beastars canid canine canis clothed clothing fur grey_body grey_fur hi_res javigameboy legoshi_(beastars) male mammal simple_background solo teeth wolf"
		oldtags := "1:1 2021 anthro beastars canid canine canis clothed clothing fur grey_body grey_fur hi_res javigameboy legoshi_(beastars) male mammal simple_background solo teeth wolf"
		sources := []string{"https://twitter.com/Javigameboy/status/1429921007721062401"}
		description := ""
		parent_post := -1
		rating := "safe"

		reason := "API Update Test (should be NOOP)"
		api.UpdatePost(user, apikey, post, tags.TagDiff{}, &rating, &parent_post, sources, &description, &reason)
	}
}
