package bot

import (
	"bot/dialogs"
	"botbehavior"
	"api"
	"api/tagindex"
	apitypes "api/types"
	"apiextra"
	"storage"

	"github.com/kballard/go-shellquote"
	"github.com/thewug/gogram"
	"github.com/thewug/gogram/data"

	"bytes"
	"errors"
	"fmt"
	"html"
	"io"
	"io/ioutil"
	"regexp"
	"sort"
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

const (
	sort_alpha = iota
	sort_none
	sort_popularity

	sort_asc = false
	sort_desc = true
)

const (
	auto_no = iota
	auto_add
	auto_remove
	auto_replace
)

var apiurlmatch *regexp.Regexp
var wizard_rule_done WizardRule = WizardRule{prompt: "You've finished the tag wizard."}

type settings interface {
	GetApiEndpoint() string
}

func Init(s settings) error {
	var err error
	apiurlmatch, err = regexp.Compile(fmt.Sprintf(`https?://(www\.)?(%s|%s)/(posts|post/show)/(\d+)`,
	                                              regexp.QuoteMeta(s.GetApiEndpoint()),
	                                              regexp.QuoteMeta(s.GetApiFilteredEndpoint())))
	return err
}

type WizardRule struct {
	prereqs     []string
	options     []string
	prompt        string
	sort          int
	sortdirection bool
	auto          int
	visited       int
}

func (this *WizardRule) Prompt() (string) {
	if this.prompt == "" { return "Choose or type some tags." }
	return this.prompt
}

func (this *WizardRule) Len() (int) {
	return len(this.options)
}

func (this *WizardRule) Less(i, j int) (bool) {
	if this.sort == sort_none { return (i < j) == (this.sortdirection == sort_asc) }
	if this.sort == sort_popularity { return (i < j) == (this.sortdirection == sort_asc) }
	if this.sort == sort_alpha { return (this.options[i] < this.options[j]) == (this.sortdirection == sort_asc) }
	panic("invalid sort mode")
}

func (this *WizardRule) Swap(i, j int) {
	temp := this.options[i]
	this.options[i] = this.options[j]
	this.options[j] = temp
}

func (this *WizardRule) ParseFromString(line string) (error) {
	*this = WizardRule{}
	tokens, err := shellquote.Split(line)
	found_dot := false
	if err != nil { return err }
	for _, t := range tokens {
		l := strings.ToLower(t)
		if l == "" { continue }
		if !found_dot && strings.HasPrefix(l, "auto:") {
			if l[5:] == "add" { this.auto = auto_add }
			if l[5:] == "remove" { this.auto = auto_remove }
			if l[5:] == "replace" { this.auto = auto_replace }
		} else if !found_dot && strings.HasPrefix(l, "sort:") {
			for i, x := range strings.Split(l, ":") {
				if i == 1 && x == "alpha" { this.sort = sort_alpha }
				if i == 1 && x == "popularity" { this.sort = sort_popularity }
				if i == 1 && x == "none" { this.sort = sort_none }
				if i == 2 && x == "asc" { this.sortdirection = sort_asc }
				if i == 2 && x == "desc" { this.sortdirection = sort_desc }
			}
		} else if !found_dot && strings.HasPrefix(l, "prompt:") {
			this.prompt = l[7:]
		} else if !found_dot && l == "." {
			found_dot = true
		} else if !found_dot {
			this.prereqs = append(this.prereqs, l)
		} else if found_dot {
			this.options = append(this.options, t)
		}
	}

	if this.sort != sort_none { sort.Sort(this) }
	return nil
}

func (this *WizardRule) PrereqsSatisfied(tags *api.TagSet) (bool) {
	for _, tag := range this.prereqs {
		if _, ok := tags.Tags[tag]; !ok {
			return false
		}
	}

	return true
}

func TagWizardMarkupHelper(tag string) (bool, bool, bool, string) {
	tokens := strings.SplitN(tag, ":", 2)
	var hide, set, unset bool
	if len(tokens) == 2 {
		for _, c := range tokens[0] {
			if c == 'h' {
				hide = true
			} else if c == 'x' {
				set = true
			} else if c == 'o' {
				unset = true
			// no-op, just in case a tag like 'oh:snap' is actually used somehow,
			// which would then be specified as 'n:oh:snap'
			} else if c == 'n' {
			} else { return false, false, false, tag }
		}
		return hide, set, unset, tokens[1]
	}
	return false, false, false, tag
}

func (this *WizardRule) DoImplicit(tags *api.TagSet) {
	for _, o := range this.options {
		_, set, unset, tag := TagWizardMarkupHelper(o)
		if set { tags.SetTag(tag) }
		if unset { tags.ClearTag(tag) }
	}
}

func strPtr(a string) (*string) {
	return &a
}

func (this *WizardRule) GetButtons(tags *api.TagSet) ([]data.TInlineKeyboardButton) {
	var out []data.TInlineKeyboardButton
	for _, o := range this.options {
		hide, _, _, tag := TagWizardMarkupHelper(o)
		if !hide {
			var decor string
			if tags.IsSet(tag) {
				decor = "\u2705" // green box checkmark
			} else {
				decor = "\u26d4" // red circle strikethru
			}
			tag_display := tag
			if strings.HasPrefix(strings.ToLower(tag), "meta:") { tag_display = strings.Replace(tag_display[5:], "_", " ", -1) }
			btn := data.TInlineKeyboardButton{Text: decor + " " + tag_display + " " + decor, Data: strPtr("/z " + tag)}
			out = append(out, btn)
		}
	}
	return out
}

func NewWizardRuleFromString(line string) (*WizardRule) {
	var w WizardRule
	w.ParseFromString(line)
	return &w
}

type WizardRuleset struct {
	interactive_rules []WizardRule
	auto_rules        []WizardRule
	visitval            int
}

func (this *WizardRuleset) AddRule(r *WizardRule) {
	if r.auto != auto_no {
		this.auto_rules = append(this.auto_rules, *r)
	} else {
		this.interactive_rules = append(this.interactive_rules, *r)
	}
}

type TagWizard struct {
	ctx          *gogram.MessageCtx
	tags         *api.TagSet
	rules         WizardRuleset
	current_rule *WizardRule
	wizard_ctx   *gogram.MessageCtx
}

func (this *TagWizard) SetNewRulesFromString(rulestring string) (error) {
	this.rules.interactive_rules = nil
	this.rules.auto_rules = nil
	for _, line := range strings.Split(rulestring, "\n") {
		line = strings.TrimSpace(strings.Replace(line, "\r", "", -1))
		if line == "" { continue }
		rule := NewWizardRuleFromString(line)
		this.rules.AddRule(rule)
	}
	return nil
}

func (this *TagWizard) SelectRule() (bool) {
	for i, _ := range this.rules.interactive_rules {
		if this.rules.interactive_rules[i].visited == this.rules.visitval { continue }
		if !this.rules.interactive_rules[i].PrereqsSatisfied(this.tags) { continue }
		this.rules.interactive_rules[i].visited = this.rules.visitval
		this.current_rule = &this.rules.interactive_rules[i]
		return true
	}
	this.current_rule = &wizard_rule_done
	return false
}

func (this *TagWizard) MergeTagsFromString(tagstr string) {
	var tags []string
	for _, t := range strings.Split(tagstr, " ") {
		for _, tt := range strings.Split(t, "\n") {
			if tt == "" { continue }
			tags = append(tags, tt)
		}
	}
	this.MergeTags(tags)
}

func (this *TagWizard) ToggleTagsFromString(tagstr string) {
	var tags []string
	for _, t := range strings.Split(tagstr, " ") {
		for _, tt := range strings.Split(t, "\n") {
			if tt == "" { continue }
			tags = append(tags, tt)
		}
	}
	this.ToggleTags(tags)
}

func (this *TagWizard) MergeTags(tags []string) {
	this.tags.MergeTags(tags)
}

func (this *TagWizard) ToggleTags(tags []string) {
	this.tags.ToggleTags(tags)
}

func (this *TagWizard) Abort() {
	this.Reset()
}

func (this *TagWizard) Reset() {
	if this.tags == nil { this.tags = api.NewTagSet() }
	if this.wizard_ctx != nil { this.wizard_ctx.DeleteAsync(nil) }
	*this = TagWizard{tags: this.tags, rules: this.rules, ctx: this.ctx}
	this.tags.Reset()
	this.rules.visitval += 1
}

func (this *TagWizard) Len() (int) {
	if this.tags == nil { this.tags = api.NewTagSet() }
	return this.tags.Len()
}

func (this *TagWizard) Rating() (string) {
	if this.tags == nil { this.tags = api.NewTagSet() }
	return this.tags.Rating()
}

func (this *TagWizard) TagString() (string) {
	if this.tags == nil { return "" }

	builder := bytes.NewBuffer(nil)
	for k, v := range this.tags.Tags {
		if v == 0 { continue }
		t := strings.ToLower(k)
		if strings.HasPrefix(t, "rating:") { continue }
		if strings.HasPrefix(t, "meta:") { continue }
		if builder.Len() != 0 { builder.WriteRune(' ') }
		builder.WriteString(k)
	}

	return builder.String()
}

func (this *TagWizard) SetTag(tag string) {
	this.tags.SetTag(tag)
}

func (this *TagWizard) ClearTag(tag string) {
	this.tags.ClearTag(tag)
}

func (this *TagWizard) SendWizard(chat_id int64) {
	if this.wizard_ctx != nil { this.wizard_ctx.DeleteAsync(nil) }
	this.wizard_ctx = nil

	if this.current_rule == nil { this.NextMenuInternal() }
	this.UpdateMenu()
	return
}

func (this *TagWizard) NextMenu() {
	this.NextMenuInternal()
	this.UpdateMenu()
}

func (this *TagWizard) NextMenuInternal() (bool) {
	found := this.SelectRule()
	if !found { return false }
	this.current_rule.visited = this.rules.visitval
	this.current_rule.DoImplicit(this.tags)
	return true
}

func (this *TagWizard) UpdateMenu() {
	if this.current_rule == nil { return }
	if (this.wizard_ctx != nil) {
		var kbd data.TInlineKeyboard
		kbd.AddButton(data.TInlineKeyboardButton{Text: "\u27a1 Next", Data: strPtr("/next")})
		kbd.AddButton(data.TInlineKeyboardButton{Text: "\U0001f501 Start Over", Data: strPtr("/again")})
		if this.current_rule == &wizard_rule_done {
			kbd.AddButton(data.TInlineKeyboardButton{Text: "\U0001f197 Done", Data: strPtr("/finish")})
		} else { 
			for _, b := range this.current_rule.GetButtons(this.tags) {
				kbd.AddRow()
				kbd.AddButton(b)
			}
		}
		this.wizard_ctx.EditTextAsync(data.OMessageEdit{SendData: data.SendData{Text: this.current_rule.Prompt(), ParseMode: data.ParseHTML, ReplyMarkup: kbd}}, nil)
	} else {
		prompt := "Time for tags!\n\n" + this.current_rule.Prompt()
		var kbd data.TInlineKeyboard
		kbd.AddButton(data.TInlineKeyboardButton{Text: "\u27a1 Next", Data: strPtr("/next")})
		kbd.AddButton(data.TInlineKeyboardButton{Text: "\U0001f501 Start Over", Data: strPtr("/again")})
		if this.current_rule == &wizard_rule_done {
			kbd.AddButton(data.TInlineKeyboardButton{Text: "\U0001f197 Done", Data: strPtr("/finish")})
		} else { 
			for _, b := range this.current_rule.GetButtons(this.tags) {
				kbd.AddRow()
				kbd.AddButton(b)
			}
		}
		this.wizard_ctx, _ = this.ctx.Respond(data.OMessage{SendData: data.SendData{Text: prompt, ParseMode: data.ParseHTML, ReplyMarkup: kbd}})
	}
}

func (this *TagWizard) FinishMenu() { // updates menu with no buttons
	if this.wizard_ctx != nil { this.wizard_ctx.EditTextAsync(data.OMessageEdit{SendData: data.SendData{Text: fmt.Sprintf("Tags:\n\n<pre>%s</pre>", this.TagString()), ParseMode: data.ParseHTML}}, nil) }
	this.wizard_ctx = nil
}

func (this *TagWizard) DoOver() {
	this.rules.visitval += 1
	this.UpdateMenu()
}

func NewTagWizard(ctx *gogram.MessageCtx) (*TagWizard) {
	w := TagWizard{tags: api.NewTagSet(), rules:WizardRuleset{visitval: 1}, ctx: ctx}
	return &w
}

type PostFile struct {
	mode int
	file_id data.FileID
	url string
}

type UserState struct {
	my_id		data.UserID

	postmode	int
	postfile	PostFile
	postrating	string
	posttagwiz	bool
	postwizard	TagWizard
	postsource      string
	postdescription string
	postparent      int
	postready	bool
	postdone	bool
	postreviewed	bool

}

func (this *UserState) SetTagRulesByName(name string) {
	if name == "" { name = "main" }
	rules, _ := storage.GetUserTagRules(storage.UpdaterSettings{}, this.my_id, name)
	this.postwizard.SetNewRulesFromString(rules)
}

func (this *UserState) WriteUserTagRules(tagrules, name string) {
	storage.WriteUserTagRules(storage.UpdaterSettings{}, this.my_id, name, tagrules)
}

func (this *UserState) Reset() {
	*this = UserState{my_id: this.my_id, postwizard: this.postwizard}
	this.postwizard.Reset()
}

func NewUserState(ctx *gogram.MessageCtx) (*UserState) {
	u := UserState{my_id: ctx.Msg.From.Id}
	u.postwizard = *NewTagWizard(ctx)
	rules, err := storage.GetUserTagRules(storage.UpdaterSettings{}, ctx.Msg.From.Id, "main")
	if err != nil { ctx.Bot.ErrorLog.Println(err.Error()) }
	if err == nil { u.postwizard.SetNewRulesFromString(rules) }
	return &u
}

var states_by_user map[data.UserID]*UserState

func GetUserState(ctx *gogram.MessageCtx) (*UserState) {
	if states_by_user == nil {
		states_by_user = make(map[data.UserID]*UserState)
	}

	if ctx.Msg.From == nil {
		return nil
	}

	state := states_by_user[ctx.Msg.From.Id]
	if state == nil {
		state = NewUserState(ctx)
		states_by_user[ctx.Msg.From.Id] = state
	}

	return state
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
edit. <code>/edit (reply to message with ` + api.ApiName + ` post URL)</code>
edit. <code>/edit [file ID]</code>
edit. <code>/edit [` + api.ApiName + ` post URL]</code>
edit.
edit. The following subcommand exists:
edit. <code>/cancel -</code> cancel this edit.
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
			err = this.Behavior.DismissPromptPost(ctx.Bot, post_info, apitypes.TagDiff{}, settings)
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
				potential_id := apiextra.GetPostIDFromURLInText(*text)
				if potential_id != nil {
					// if we are a reply to a message, AND that message has text or a caption, AND that text contains a post URL, yoink it.
					id = *potential_id
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
	PostId int `json:"postid"`
}

type EditState struct {
	gogram.StateBase

	data esp
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

	switch ctx.Cmd.Command {
	case "/tags":
		p.Prefix = "Enter a list of tag changes, seperated by spaces. You can clear tags by prefixing them with a minus (-) and reset them by prefixing with an equals (=)."
		p.State = dialogs.WAIT_TAGS
	case "/sources":
		p.Prefix = "Post some source changes, seperated by newlines. You can remove sources by prefixing them with a minus (-)."
		p.State = dialogs.WAIT_SOURCE
	case "/rating":
		p.Prefix = "Post the new rating."
		p.State = dialogs.WAIT_RATING
	case "/description":
		p.Prefix = `Post the new description. You can use <a href="https://` + api.Endpoint + `/help/dtext">dtext</a>.`
		p.State = dialogs.WAIT_DESC
	case "/parent":
		p.Prefix = `Post the new parent.`
		p.State = dialogs.WAIT_PARENT
	case "/reason":
		p.Prefix = "Why are you editing this post? Post an edit reason, 250 characters max."
		p.State = dialogs.WAIT_REASON
	case "/file":
		p.Prefix = `Upload a file.`
		p.State = dialogs.WAIT_FILE
	case "/save":
		p.Prefix = ""
		p.State = dialogs.SAVED
		p.Finalize(settings, ctx.Bot, nil)
		ctx.Answer(data.OCallback{Notification: "\U0001F7E2 Edit submitted."})
		if !p.IsNoop() {
			var rating *string
			var parent *int
			var description *string
			var reason *string

			if p.Rating != "" { rating = &p.Rating }
			if p.Parent != 0 { parent = &p.Parent }
			if p.Description != "" { description = &p.Description }
			if p.Reason != "" { reason = &p.Reason }

			update, err := api.UpdatePost(this.data.User, this.data.ApiKey, p.PostId, p.TagChanges, rating, parent, p.SourceChanges, description, reason)
			if err != nil {
				ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: "An error occurred when editing the post! Try again later."}}, nil)
				ctx.Bot.ErrorLog.Println("Error updating post: ", err.Error())
				return
			}

			if update != nil {
				err = storage.UpdatePost(*update, settings)
				if err != nil {
					ctx.Bot.ErrorLog.Println("Error updating internal post: ", err.Error())
					return
				}
			}
		}
		ctx.SetState(nil)
		settings.Transaction.MarkForCommit()
		return
	case "/discard":
		p.Prefix = ""
		p.State = dialogs.DISCARDED
		p.Finalize(settings, ctx.Bot, nil)
		ctx.AnswerAsync(data.OCallback{Notification: "\U0001F534 Edit discarded."}, nil) // finalize dialog post and discard edit
		ctx.SetState(nil)
		settings.Transaction.MarkForCommit()
		return
	default:
	}

	p.Prompt(settings, ctx.Bot, nil)
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

	if p.State == dialogs.WAIT_TAGS {
		p.TagChanges.ApplyString(ctx.Msg.PlainText())
		p.Prefix = "Got it. Continue sending more tag changes, and pick a button from below when you're done."
	} else if p.State == dialogs.WAIT_SOURCE {
		p.SourceChanges = append(p.SourceChanges, strings.Split(ctx.Msg.PlainText(), "\n")...)
		p.Prefix = "Got it. Continue sending more source changes, and pick a button from below when you're done."
	} else if p.State == dialogs.WAIT_RATING {
		rating, err := api.SanitizeRatingForEdit(ctx.Msg.PlainText())

		if err != nil {
			p.Prefix = "Please enter a <i>valid</i> rating. (Pick from <code>explicit</code>, <code>questionable</code>, <code>safe</code>, or <code>original</code>.)"
		} else {
			p.Rating = rating
			p.ResetState()
		}
	} else if p.State == dialogs.WAIT_DESC {
		p.Description = ctx.Msg.PlainText() // TODO: convert telegram markup to dtext
		p.ResetState()
	} else if p.State == dialogs.WAIT_PARENT {
		parent := scrapePostIdFromMessage(ctx.Msg)

		if parent == 0 {
			parent, err = strconv.Atoi(ctx.Msg.PlainText())
		}

		if parent <= 0 {
			err = errors.New("negative parent post")
		} else if strings.ToLower(ctx.Msg.PlainText()) == "none" {
			parent = -1
			err = nil
		} else if strings.ToLower(ctx.Msg.PlainText()) == "original" {
			parent = 0
			err = nil
		}

		if err != nil {
			p.Prefix = "Please enter a <i>valid</i> parent post. (You can either send a link to a post, a bare numeric ID, 'none' for no parent, or 'original' to not attempt to update the parent at all.)"
		} else {
			p.Parent = parent
			p.ResetState()
		}
	} else if p.State == dialogs.WAIT_REASON {
		p.Reason = ctx.Msg.PlainText()
		p.ResetState()
	} else {
		return
	}

	p.Prompt(settings, ctx.Bot, nil)
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
	p.State = dialogs.DISCARDED
	p.Prefix = ""
	p.Finalize(settings, ctx.Bot, nil)
	ctx.SetState(nil)
	settings.Transaction.MarkForCommit()
}

func scrapePostIdFromMessage(msg *data.TMessage) (int) {
	id, err := api.ApiURLToPostID(msg.PlainText())
	if err == nil { return id }

	for _, entity := range msg.GetEntities() {
		if entity.Url != nil {
			id, err := api.ApiURLToPostID(*entity.Url)
			if err == nil { return id }
		}
	}

	return 0
}

func (this *EditState) Edit(ctx *gogram.MessageCtx) {
	var post int
	var mode int
	var e dialogs.EditPrompt

	if ctx.Msg.From == nil { return }

	if ctx.Msg.ReplyToMessage != nil {
		post = scrapePostIdFromMessage(ctx.Msg.ReplyToMessage)
	}

	user, api_key, _, err := storage.GetUserCreds(storage.UpdaterSettings{}, ctx.Msg.From.Id)
	if err != nil {
		ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: "You need to be logged in to use this command!"}}, nil)
		if err != storage.ErrNoLogin {
			ctx.Bot.ErrorLog.Println("Error while checking credentials: ", err.Error())
		}
		return
	}

	for _, token := range ctx.Cmd.Args {
		if mode != root {
			if mode == posttags {
				e.TagChanges.ApplyString(token)
			} else if mode == postsource {
				e.SourceChanges = append(e.SourceChanges, strings.Split(token, "\n")...)
			} else if mode == postrating {
				rating, err := api.SanitizeRatingForEdit(token)
				if err != nil {
					ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: "Please try again with a valid rating."}}, nil)
					return
				} else {
					e.Rating = rating
				}
			} else if mode == postdescription {
				e.Description = token // at some point i should make it convert telegram markup to dtext but i can't do that right now
			} else if mode == postparent {
				submatches := apiurlmatch.FindStringSubmatch(token)
				var err error
				if len(submatches) == 5 {
					e.Parent, err = strconv.Atoi(submatches[4])
				}

				if len(submatches) != 5 || err != nil {
					e.Parent, err = strconv.Atoi(token)
				}

				if err != nil {
					ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: "Please try again wth a valid parent post."}}, nil)
					return
				}
			} else if mode == editreason {
				e.Reason = token
			}
			mode = root
		}
		if token == "" {
		} else if token == "--tags" {
			mode = posttags
		} else if token == "--sources" {
			mode = postsource
		} else if token == "--rating" {
			mode = postrating
		} else if token == "--description" {
			mode = postdescription
		} else if token == "--parent" {
			mode = postparent
		} else if token == "--reason" {
			mode = editreason
		} else {
			temp, err := strconv.Atoi(token)
			if err == nil { post = temp }
		}
	}

	if post <= 0 {
		ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: "Sorry, I can't figure out which post you're talking about!\n\nYou can reply to a message with a post URL, or you can pass an ID or a link directly."}}, nil)
		return
	}

	e.PostId = post
	e.ResetState()

	prompt := e.Prompt(storage.UpdaterSettings{}, ctx.Bot, ctx)
	this = &EditState{
		data: esp{
			User: user,
			ApiKey: api_key,
			MsgId: prompt.Msg.Id,
			ChatId: prompt.Msg.Chat.Id,
			PostId: post,
		},
	}

	ctx.SetState(this)
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
		this.tagwizardrules = string(b)
	}
	if ctx.Cmd.Argstr != "" {
		this.tagrulename = ctx.Cmd.Argstr
	} else {
		ctx.RespondAsync(data.OMessage{SendData: data.SendData{Text: "Send some new tag rules in a text file."}}, nil)
		return
	}

	if this.tagwizardrules != "" {
		if this.tagrulename == "" { this.tagrulename = "main" }
		this.tagwizardrules = strings.Replace(this.tagwizardrules, "\r", "", -1) // pesky windows carriage returns
		storage.WriteUserTagRules(storage.UpdaterSettings{}, ctx.Msg.From.Id, this.tagrulename, this.tagwizardrules)
		if err := NewTagWizard(ctx).SetNewRulesFromString(this.tagwizardrules); err != nil {
			ctx.RespondAsync(data.OMessage{SendData: data.SendData{Text: fmt.Sprintf("Error while parsing tag rules: %s", html.EscapeString(err.Error())), ParseMode: data.ParseHTML}}, nil)
			return
		} else {
			ctx.RespondAsync(data.OMessage{SendData: data.SendData{Text: "Set new tag rules."}}, nil)
			ctx.SetState(nil)
			return
		}
	}
}

type PostState struct {
	gogram.StateBase

	postmode	int
	postfile	PostFile
	postrating	string
	posttagwiz	bool
	postwizard	TagWizard
	postsource      string
	postdescription string
	postparent     *int
	postready	bool
	postdone	bool
	postreviewed	bool
}

func (this *PostState) Reset() {
	this.postwizard.Reset()
}

func (this *PostState) SetTagRulesByName(my_id data.UserID, name string) {
	if name == "" { name = "main" }
	rules, _ := storage.GetUserTagRules(storage.UpdaterSettings{}, my_id, name)
	this.postwizard.SetNewRulesFromString(rules)
}

func (this *PostState) WriteUserTagRules(my_id data.UserID, tagrules, name string) {
	storage.WriteUserTagRules(storage.UpdaterSettings{}, my_id, name, tagrules)
}

func (this *PostState) HandleCallback(ctx *gogram.CallbackCtx) {
	newstate, response := this.HandleCmd(&ctx.Cmd, &ctx.Cb.From, ctx.GetState(), nil, ctx.MsgCtx.Msg, ctx.Bot)
	ctx.SetState(newstate)
	if response.Text != "" {
		ctx.Respond(response)
	}
}

func (this *PostState) Handle(ctx *gogram.MessageCtx) {
	if ctx.Msg.From == nil { return }
	newstate, response := this.HandleCmd(&ctx.Cmd, ctx.Msg.From, ctx.GetState(), ctx.Msg, nil, ctx.Bot)
	ctx.SetState(newstate)
	if response.Text != "" {
		ctx.Respond(response)
	}
}

func (this *PostState) HandleCmd(cmd *gogram.CommandData,
                                 from *data.TUser,
                                 current_state gogram.State,
                                 inbound_message *data.TMessage,
                                 context_message *data.TMessage,
                                 bot *gogram.TelegramBot) (gogram.State, data.OMessage) {

	endstate := current_state
	var response data.OMessage
	response.ParseMode = data.ParseHTML

	if cmd.Command == "/cancel" {
		response.Text = "Command cancelled."
		endstate = nil
		return endstate, response
	}

	if current_state == nil {
		this = &PostState{}
		m := inbound_message
		if m == nil {
			m = &data.TMessage{Chat: data.TChat{Id: data.ChatID(from.Id)}}
		}
		this.postwizard = *NewTagWizard(gogram.NewMessageCtx(m, false, bot))
		rules, err := storage.GetUserTagRules(storage.UpdaterSettings{}, from.Id, "main")
		if err != nil { bot.ErrorLog.Println(err.Error()) }
		if err == nil { this.postwizard.SetNewRulesFromString(rules) }
		endstate = this
	}

	if cmd.Command == "/post" || cmd.Command == "/file" || cmd.Command == "/f" {
		this.postfile.mode = none
		this.posttagwiz = false
		this.postmode = postfile
	} else if cmd.Command == "/tag" || cmd.Command == "/t" {
		this.posttagwiz = false
		this.postmode = posttags
	} else if cmd.Command == "/wizard" || cmd.Command == "/w" {
		this.posttagwiz = true
		this.postmode = postwizard
		this.SetTagRulesByName(from.Id, cmd.Argstr)
	} else if cmd.Command == "/rating" || cmd.Command == "/r" {
		this.posttagwiz = false
		this.postrating = ""
		this.postmode = postrating
	} else if cmd.Command == "/source" || cmd.Command == "/src" || cmd.Command == "/s" {
		this.posttagwiz = false
		this.postsource = ""
		this.postmode = postsource
	} else if cmd.Command == "/description" || cmd.Command == "/desc" || cmd.Command == "/d" {
		this.posttagwiz = false
		this.postdescription = ""
		this.postmode = postdescription
	} else if cmd.Command == "/parent" || cmd.Command == "/p" {
		this.posttagwiz = false
		this.postparent = nil
		this.postmode = postparent
	} else if cmd.Command == "/upload" {
		this.postmode = postupload
	} else if cmd.Command == "/help" {
		response.Text = ShowHelp("post")
		return endstate, response
	} else if cmd.Command == "/z" && this.postmode == postwizard {
		this.postwizard.ToggleTagsFromString(cmd.Argstr)
		this.postwizard.UpdateMenu()
	} else if cmd.Command == "/next" && this.postmode == postwizard {
		this.postwizard.NextMenu()
	} else if cmd.Command == "/finish" && this.postmode == postwizard {
		this.postwizard.FinishMenu()
		this.postmode = postnext
		response.Text = "Done tagging.\n\n"
	} else if cmd.Command == "/again" && this.postmode == postwizard {
		this.postwizard.DoOver()
	}

	if this.postmode == postfile {
		if inbound_message != nil && inbound_message.Photo != nil { // inline photo
			response.Text = "That photo was compressed by telegram, and its quality may be severely degraded.  Send it as a file instead if you're sure.\n\n"
		} else if inbound_message != nil && inbound_message.Document != nil { // inline file
			response.Text = "Preparing to post file sent in this message.\n\n"
			this.postfile.mode = file_id
			this.postfile.file_id = inbound_message.Document.Id
			this.postmode = postnext
		} else if inbound_message != nil && inbound_message.ReplyToMessage != nil && inbound_message.ReplyToMessage.Document != nil { // reply to file
			response.ReplyToId = &inbound_message.ReplyToMessage.Id
			response.Text = "Preparing to post file sent in this message.\n\n"
			this.postfile.mode = file_id
			this.postfile.file_id = inbound_message.ReplyToMessage.Document.Id
			this.postmode = postnext
		} else if strings.HasPrefix(cmd.Argstr, "http://") || strings.HasPrefix(cmd.Argstr, "https://") { // inline url
			response.Text = fmt.Sprintf("Preparing to post from <a href=\"%s\">this URL</a>.\n\n", cmd.Argstr)
			this.postfile.mode = url
			this.postfile.url = cmd.Argstr
			this.postmode = postnext
		} else if inbound_message != nil && inbound_message.ReplyToMessage != nil && inbound_message.ReplyToMessage.Photo != nil { // reply to photo
			response.ReplyToId = &inbound_message.ReplyToMessage.Id
			response.Text = "That photo was compressed by telegram, and its quality may be severely degraded.  Send it as a file instead if you're sure.\n\n"
		} else if inbound_message != nil && inbound_message.ReplyToMessage != nil || cmd.Argstr != "" { // reply to unknown, or unknown
			response.Text = "Sorry, I don't know what to do with that.\n\nPlease send me a file. Either send (or forward) one directly, reply to one you sent earlier, or send a URL."
		} else {
			this.postmode = postnext
		}
	} else if this.postmode == postwizard {
		if cmd.Command == "" {
			this.postwizard.MergeTagsFromString(cmd.Argstr)
			this.postwizard.UpdateMenu()
		}
		if cmd.Command == "/next" {
			this.postwizard.NextMenu()
		}
	} else if this.postmode == posttags {
		if cmd.Argstr == "" {
			response.Text = "Please send some new tags."
		} else {
			if this.postwizard.Len() != 0 {
				response.Text = fmt.Sprintf("Replaced previous tags.\n(%s)", this.postwizard.TagString())
			} else {
				response.Text = "Applied tags."
			}
			this.postwizard.Reset()
			this.postwizard.MergeTagsFromString(cmd.Argstr)
			this.postmode = postnext
		}
	} else if this.postmode == postrating {
		var err error
		this.postrating, err = api.SanitizeRating(cmd.Argstr)
		if cmd.Argstr == "" {
			this.postmode = postnext
		} else if err != nil {
			response.Text = "Sorry, that isn't a valid rating.\n\nPlease enter the post's rating! Safe, Questionable, or Explicit?"
		} else {
			response.Text = fmt.Sprintf("Set rating to %s.\n\n", this.postrating)
			this.postmode = postnext
		}
	} else if this.postmode == postsource {
		this.postsource = cmd.Argstr
		if this.postsource == "." { this.postsource = "" }
		this.postmode = postnext
		if cmd.Argstr == "" {
		} else if cmd.Argstr == "." {
			response.Text = "Cleared sources.\n\n"
		} else {
			response.Text = "Set sources.\n\n"
		}
	} else if this.postmode == postdescription {
		this.postdescription = cmd.Argstr
		if this.postdescription == "." { this.postdescription = "" }
		this.postmode = postnext
		if cmd.Argstr == "" {
		} else if cmd.Argstr == "." {
			response.Text = "Cleared description.\n\n"
		} else {
			response.Text = "Set description.\n\n"
		}
	} else if this.postmode == postparent {
		this.postmode = postnext
		if cmd.Argstr != "" {
			num, err := strconv.Atoi(cmd.Argstr)
			if err != nil {
				submatches := apiurlmatch.FindStringSubmatch(cmd.Argstr)
				if len(submatches) == 5 {
					num, err = strconv.Atoi(submatches[4])
				}
			}
			if err == nil {
				this.postparent = &num
				response.Text = "Set parent post.\n\n"
			} else {
				this.postparent = nil
				response.Text = "Cleared parent post.\n\n"
			}
		}
	} else if this.postmode == postupload {
		if this.postfile.mode == none || this.postwizard.Len() < 6 || this.postrating == "" {
			this.postmode = postnext
		} else {
			var post_url string
			var post_filedata io.ReadCloser
			if this.postfile.mode == url {
				post_url = this.postfile.url
			} else {
				file, err := bot.Remote.GetFile(data.OGetFile{Id: this.postfile.file_id})
				if err != nil || file == nil || file.FilePath == nil {
					response.Text = "Error while fetching file, try sending it again?"
					this.postmode = postnext
					return endstate, response
				}
				post_filedata, err = bot.Remote.DownloadFile(data.OFile{FilePath: *file.FilePath})
				if err != nil || post_filedata == nil {
					response.Text = "Error while downloading file, try sending it again?"
					this.postmode = postnext
					return endstate, response
				}
			}
			user, apikey, _, err := storage.GetUserCreds(storage.UpdaterSettings{}, from.Id)
			result, err := api.UploadFile(post_filedata, post_url, this.postwizard.TagString(), this.postrating, this.postsource, this.postdescription, this.postparent, user, apikey)
			if err != nil || !result.Success {
				if result.StatusCode == 403 {
					response.Text = "It looks like your api key isn't valid, you need to login again."
					this.Reset()
					endstate = nil
				} else if result.Location != nil && result.StatusCode == 423 {
					response.Text = fmt.Sprintf("It looks like that file has already been posted. <a href=\"%s\">Check it out here.</a>", *result.Location)
					this.Reset()
					endstate = nil
				} else {
					response.Text = fmt.Sprintf("I'm having issues posting that file. (%s)", *result.Reason)
				}
			} else {
				response.Text = fmt.Sprintf("Upload complete! <a href=\"%s\">Check it out.</a>", *result.Location)
				this.Reset()
				endstate = nil
			}

			if endstate == nil { return endstate, response }
		}
	}

	if this.postmode == postnext {
		if this.postrating == "" {
			newrating := this.postwizard.Rating()
			if newrating != "" {
				this.postrating = newrating
				response.Text = fmt.Sprintf("%sThe tags imply the rating of this post is %s.\n\n", response.Text, this.postrating)
			}
		}

		if this.postfile.mode == none {
			response.Text = fmt.Sprintf("%s%s", response.Text,  "Please send me a file. Either send (or forward) one directly, reply to one you sent earlier, or send a URL.")
			this.postmode = postfile
		} else if this.postwizard.Len() == 0 {
			this.posttagwiz = true
			this.postmode = postwizard
		} else if this.postrating == "" {
			response.Text = "Please enter the post's rating! Safe, Questionable, or Explicit?"
			this.postmode = postrating
		} else {
			if this.postready == false {
				response.Text = fmt.Sprintf("%s%s", response.Text, "Your post now has enough information to submit!\n\n")
				this.postready = true
			}

			if this.postsource == "" {
				response.Text = fmt.Sprintf("%s%s", response.Text, "Please enter the post source links.")
				this.postmode = postsource
			} else if this.postdescription == "" {
				response.Text = fmt.Sprintf("%s%s", response.Text, "Please enter the description.\n<a href=\"https://" + api.Endpoint + "/help/show/dtext\">Remember, you can use DText.</a>")
				this.postmode = postdescription
			} else if this.postparent == nil {
				response.Text = fmt.Sprintf("%s%s", response.Text, "Please enter the parent post.")
				this.postmode = postparent
			} else if !this.postdone {
				response.Text = fmt.Sprintf("%sThat's it! You've entered all of the info.", response.Text)
				this.postdone = true
			}
		}
	}

	if this.posttagwiz && this.postmode == postwizard {
		this.postwizard.SendWizard(int64(from.Id))
		this.posttagwiz = false
	}

	return endstate, response
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
		api.UpdatePost(user, apikey, post, apitypes.TagDiff{}, &rating, &parent_post, sources, &description, &reason)
	}
}
