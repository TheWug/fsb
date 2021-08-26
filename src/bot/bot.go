package bot

import (
	"telegram"
	"telegram/telebot"
	"storage"
	"api"
	"api/tagindex"

	"fmt"
	"strings"
	"bytes"
	"regexp"
	"strconv"
	"sort"
	"html"
	"io/ioutil"
	"io"

	"errors"

	"errors"

	"github.com/kballard/go-shellquote"
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
	apiurlmatch, err = regexp.Compile(fmt.Sprintf(`https?://%s/post/show/(\d+)`, s))
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

func (this *WizardRule) GetButtons(tags *api.TagSet) ([]telegram.TInlineKeyboardButton) {
	var out []telegram.TInlineKeyboardButton
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
			btn := telegram.TInlineKeyboardButton{Text: decor + " " + tag_display + " " + decor, Data: "/z " + tag}
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
	bot          *telebot.TelegramBot
	tags         *api.TagSet
	rules         WizardRuleset
	current_rule *WizardRule
	chat_id       int64
	wizard_id     int
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
	if this.chat_id != 0 && this.wizard_id != 0 { this.bot.Remote.DeleteMessage(this.chat_id, this.wizard_id) }
	*this = TagWizard{tags: this.tags, rules: this.rules, bot: this.bot}
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
	if this.chat_id != 0 && this.wizard_id != 0 { this.bot.Remote.DeleteMessage(this.chat_id, this.wizard_id) }
	this.chat_id = chat_id
	this.wizard_id = 0

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
	if (this.wizard_id != 0) {
		var kbd telegram.TInlineKeyboard
		kbd.AddButton(telegram.TInlineKeyboardButton{Text: "\u27a1 Next", Data: "/next"})
		kbd.AddButton(telegram.TInlineKeyboardButton{Text: "\U0001f501 Start Over", Data: "/again"})
		if this.current_rule == &wizard_rule_done {
			kbd.AddButton(telegram.TInlineKeyboardButton{Text: "\U0001f197 Done", Data: "/finish"})
		} else { 
			for _, b := range this.current_rule.GetButtons(this.tags) {
				kbd.AddRow()
				kbd.AddButton(b)
			}
		}
		_, err := this.bot.Remote.EditMessageText(this.chat_id, this.wizard_id, "", this.current_rule.Prompt(), "HTML", kbd, false)
		if err != nil { fmt.Printf("An error happened: %s\n", err.Error()) }
	} else {
		prompt := "Time for tags!\n\n" + this.current_rule.Prompt()
		var kbd telegram.TInlineKeyboard
		kbd.AddButton(telegram.TInlineKeyboardButton{Text: "\u27a1 Next", Data: "/next"})
		kbd.AddButton(telegram.TInlineKeyboardButton{Text: "\U0001f501 Start Over", Data: "/again"})
		if this.current_rule == &wizard_rule_done {
			kbd.AddButton(telegram.TInlineKeyboardButton{Text: "\U0001f197 Done", Data: "/finish"})
		} else { 
			for _, b := range this.current_rule.GetButtons(this.tags) {
				kbd.AddRow()
				kbd.AddButton(b)
			}
		}
		msg, _ := this.bot.Remote.SendMessage(this.chat_id, prompt, nil, "HTML", kbd, false)
		this.wizard_id = msg.Message_id
	}
}

func (this *TagWizard) FinishMenu() { // updates menu with no buttons
	if this.chat_id != 0 && this.wizard_id != 0 { this.bot.Remote.EditMessageText(this.chat_id, this.wizard_id, "", fmt.Sprintf("Tags:\n\n<pre>%s</pre>", this.TagString()), "HTML", nil, false) }
	this.wizard_id = 0
}

func (this *TagWizard) DoOver() {
	this.rules.visitval += 1
	this.UpdateMenu()
}

func NewTagWizard(bot *telebot.TelegramBot) (*TagWizard) {
	w := TagWizard{tags: api.NewTagSet(), rules:WizardRuleset{visitval: 1}, bot: bot}
	return &w
}

type PostFile struct {
	mode int
	file_id string
	url string
}

type UserState struct {
	my_id		int

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
	rules, _ := storage.GetUserTagRules(this.my_id, name)
	this.postwizard.SetNewRulesFromString(rules)
}

func (this *UserState) WriteUserTagRules(tagrules, name string) {
	storage.WriteUserTagRules(this.my_id, name, tagrules)
}

func (this *UserState) Reset() {
	*this = UserState{my_id: this.my_id, postwizard: this.postwizard}
	this.postwizard.Reset()
}

func NewUserState(user_id int, bot *telebot.TelegramBot) (*UserState) {
	u := UserState{my_id: user_id}
	u.postwizard = *NewTagWizard(bot)
	rules, err := storage.GetUserTagRules(user_id, "main")
	if err != nil { fmt.Println(err.Error()) }
	if err == nil { u.postwizard.SetNewRulesFromString(rules) }
	return &u
}

var states_by_user map[int]*UserState

func GetUserState(from *telegram.TUser, bot *telebot.TelegramBot) (*UserState) {
	if states_by_user == nil {
		states_by_user = make(map[int]*UserState)
	}

	if from == nil {
		return nil
	}

	state := states_by_user[from.Id]
	if state == nil {
		state = NewUserState(from.Id, bot)
		states_by_user[from.Id] = state
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
janitor. <b>Janitor features</b>
janitor. I have some powerful features for manipulating posts. Use of these features is limited, contact the operator to apply for access. Remember that with great power comes great responsibility: these features can perform automated bulk edits and should be used with great carefulness. [X] indicates a deprecated (likely to be removed) command, and [D] or [U] indicates a development feature which may/may not be finished. Ask before using them.
janitor. <code>/indextags        -</code> Syncs tag database.
janitor. <code>      --full      -</code> Discard local database first.
janitor. <code>/indextagaliases  -</code> Syncs alias database.
janitor. <code>      --full      -</code> [U] Discard local database first.
janitor. <code>/sync             -</code> [D] Fetches new data from the site.
janitor. <code>      --all       -</code> [D] Fetch everything. (Default behavior)
janitor. <code>      --posts     -</code> [D] Fetch posts and tags.
janitor. <code>      --tags      -</code> [D] Fetch aliases and tags.
janitor. <code>      --aliases   -</code> [D] Fetch tags only.
janitor. <code>      --refetch   -</code> [D] Refetch everything, discard local copy.
janitor. <code>/recountnegative  -</code> Triggers remote tagged post recount.
janitor. <code>  --skipupdate    -</code> Don't refetch counts.
janitor. <code>  --aliased       -</code> Apply to aliased tags instead of by count.
janitor. <code>  --lessthan N    -</code> Apply to tags with count &lt; N (default 0)
janitor. <code>/cats             -</code> [D] Deal with concatenated tags.
janitor. <code>    --exclude T   -</code> [D] Flag T as not-a-cat.
janitor. <code>    --mark C X Y  -</code> [D] Flag C as a cat of X and Y.
janitor. <code>    --list        -</code> [D] List known cats.
janitor. <code>    --fix C       -</code> [D] Fix all posts with known cat C.
janitor. <code>    --search N    -</code> [D] Find possible cats with count >= N.
janitor. <code>/findtagtypos TAG -</code> Find typos of TAG.
janitor. <code>   --fix          -</code> fix typo for all non-excluded posts. (!!!)
janitor. <code>   --mark         -</code> [U] saves listed typos for auto-flagging later.
janitor. <code>   --include TAG  -</code> force inclusion of known unlisted typo.
janitor. <code>   --exclude TAG  -</code> exclude minor false positive.
janitor. <code>   --distinct TAG -</code> exclude major false positive and similar tags.
janitor. <code>   --allow-short  -</code> don't refuse to run for very short tags.
janitor. <code>   --threshhold N -</code> use N instead of auto-chosen edit distance.
janitor. <code>   --show-zero    -</code> don't hide tags with no posts.
janitor. <code>/recounttags      -</code> Update saved postcounts for tags.
janitor. <code>   --real         -</code> Count actual posts by tag.
janitor. <code>   --alias        -</code> Set alias count to that of their counterparts.
janitor. <code>/syncposts        -</code> Update saved postcounts for tags.
janitor. <code>   --full         -</code> Do full refetch instead of incremental.
janitor. <code>   --restart      -</code> Abandon progress of full sync and start over.
janitor. What do I do with these? Ask the operator. Don't guess.
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
.faq. <code>*</code> Your ` + api.ApiName + ` API key is NOT your password. Go <a href="https://` + api.Endpoint + `/user/api_key">HERE</a> to find it.
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
}

func (this *HelpState) Handle(ctx *telebot.MsgContext) {
	topic := "public"
	if ctx.Msg.Chat.Type == "private" {
		topic = ctx.Cmd.Argstr
	}
	ctx.Bot.Remote.SendMessageAsync(ctx.Msg.Chat.Id, ShowHelp(topic), nil, "HTML", nil, false, nil)
}

type LoginState struct {
	user	string
	apikey	string
}

func (this *LoginState) Handle(ctx *telebot.MsgContext) {
	if ctx.Msg.From == nil {
		return
	}
	if ctx.Cmd.Command == "/cancel" {
		ctx.Bot.Remote.SendMessageAsync(ctx.Msg.From.Id, "Command cancelled.", nil, "HTML", nil, false, nil)
		ctx.SetState(nil)
		return
	} else if ctx.Msg.Chat.Type != "private" && ctx.Cmd.Command == "/login" {
		ctx.Bot.Remote.SendMessageAsync(ctx.Msg.Chat.Id, "You should only use this command in private, to protect the security of your account.\n\nIf you accidentally posted your API key publicly, <a href=\"https://" + api.Endpoint + "/user/api_key\">go here to revoke it.</a>", &ctx.Msg.Message_id, "HTML", nil, false, nil)
		ctx.SetState(nil)
		return
	} else if ctx.Cmd.Command == "/logout" {
		storage.WriteUserCreds(ctx.Msg.From.Id, "", "")
		ctx.Bot.Remote.SendMessageAsync(ctx.Msg.Chat.Id, "You are now logged out.", nil, "HTML", nil, true, nil)
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
			}
			if this.user != "" && this.apikey != "" {
				success, err := api.TestLogin(this.user, this.apikey)
				if success && err == nil {
					ctx.Bot.Remote.SendMessageAsync(ctx.Msg.Chat.Id, fmt.Sprintf("You are now logged in as <code>%s</code>.", this.user), nil, "HTML", nil, true, nil)
					storage.WriteUserCreds(ctx.Msg.From.Id, this.user, this.apikey)
				} else if err != nil {
					ctx.Bot.Remote.SendMessageAsync(ctx.Msg.Chat.Id, fmt.Sprintf("An error occurred when testing if you were logged in! (%s)", html.EscapeString(err.Error())), nil, "HTML", nil, true, nil)
				} else if !success {
					ctx.Bot.Remote.SendMessageAsync(ctx.Msg.Chat.Id, "Login failed! (api key invalid?)", nil, "HTML", nil, true, nil)
				}
				ctx.SetState(nil)
				return
			}
		}

		if this.user == "" {
			ctx.Bot.Remote.SendMessageAsync(ctx.Msg.Chat.Id, "Please send your " + api.ApiName + " username.", nil, "HTML", nil, true, nil)
		} else if this.apikey == "" {
			ctx.Bot.Remote.SendMessageAsync(ctx.Msg.Chat.Id, "Please send your " + api.ApiName + " <a href=\"https://" + api.Endpoint + "/user/api_key\">API Key</a>. (not your password!)", nil, "HTML", nil, true, nil)
		}
		return
	}
}

type TagRuleState struct {
	tagwizardrules string
	tagrulename string
}

func (this *TagRuleState) Handle(ctx *telebot.MsgContext) {
	if ctx.Msg.From == nil {
		return
	}
	if ctx.Cmd.Command == "/cancel" {
		ctx.Bot.Remote.SendMessageAsync(ctx.Msg.From.Id, "Command cancelled.", nil, "HTML", nil, false, nil)
		ctx.SetState(nil)
		return
	}

	if ctx.GetState() == nil {
		this = &TagRuleState{}
		ctx.SetState(this)
	}

	var doc *telegram.TDocument
	if ctx.Msg.Document != nil {
		doc = ctx.Msg.Document
	} else if ctx.Msg.Reply_to_message != nil && ctx.Msg.Reply_to_message.Document != nil {
		doc = ctx.Msg.Reply_to_message.Document
	}

	if doc != nil {
		if !strings.HasSuffix(doc.File_name, ".txt") {
			ctx.Bot.Remote.SendMessageAsync(ctx.Msg.Chat.Id, fmt.Sprintf("%s isn't a plain text file.", doc.File_name), nil, "HTML", nil, true, nil)
			return
		}
		file, err := ctx.Bot.Remote.GetFile(doc.File_id)
		if err != nil || file == nil || file.File_path == nil {
			ctx.Bot.Remote.SendMessageAsync(ctx.Msg.Chat.Id, fmt.Sprintf("Error while fetching %s, try sending it again?", doc.File_name), nil, "HTML", nil, true, nil)
			return
		}
		file_data, err := ctx.Bot.Remote.DownloadFile(*file.File_path)
		if err != nil || file_data == nil {
			ctx.Bot.Remote.SendMessageAsync(ctx.Msg.Chat.Id, fmt.Sprintf("Error while downloading %s, try sending it again?", doc.File_name), nil, "HTML", nil, true, nil)
			return
		}
		b, err := ioutil.ReadAll(file_data)
		if err != nil || b == nil {
			ctx.Bot.Remote.SendMessageAsync(ctx.Msg.Chat.Id, fmt.Sprintf("Error while reading %s, try sending it again?", doc.File_name), nil, "HTML", nil, true, nil)
			return
		}
		this.tagwizardrules = string(b)
	}
	if ctx.Cmd.Argstr != "" {
		this.tagrulename = ctx.Cmd.Argstr
	} else {
		ctx.Bot.Remote.SendMessageAsync(ctx.Msg.Chat.Id, "Send some new tag rules in a text file.", nil, "HTML", nil, true, nil)
		return
	}

	if this.tagwizardrules != "" {
		if this.tagrulename == "" { this.tagrulename = "main" }
		this.tagwizardrules = strings.Replace(this.tagwizardrules, "\r", "", -1) // pesky windows carriage returns
		storage.WriteUserTagRules(ctx.Msg.From.Id, this.tagrulename, this.tagwizardrules)
		if err := NewTagWizard(ctx.Bot).SetNewRulesFromString(this.tagwizardrules); err != nil {
			ctx.Bot.Remote.SendMessageAsync(ctx.Msg.Chat.Id, fmt.Sprintf("Error while parsing tag rules: %s", err.Error()), nil, "HTML", nil, true, nil)
			return
		} else {
			ctx.Bot.Remote.SendMessageAsync(ctx.Msg.Chat.Id, "Set new tag rules.", nil, "HTML", nil, true, nil)
			ctx.SetState(nil)
			return
		}
	}
}

type PostState struct {
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

func (this *PostState) Reset(ctx *telebot.MsgContext) {
	if this.postwizard.chat_id != 0 && this.postwizard.wizard_id != 0 {
		ctx.Bot.Remote.DeleteMessage(this.postwizard.chat_id, this.postwizard.wizard_id)
	}
	ctx.SetState(nil)
}

func (this *PostState) SetTagRulesByName(my_id int, name string) {
	if name == "" { name = "main" }
	rules, _ := storage.GetUserTagRules(my_id, name)
	this.postwizard.SetNewRulesFromString(rules)
}

func (this *PostState) WriteUserTagRules(my_id int, tagrules, name string) {
	storage.WriteUserTagRules(my_id, name, tagrules)
}

func (this *PostState) Handle(ctx *telebot.MsgContext) {
	if ctx.Msg.From == nil {
		return
	}
	if ctx.Cmd.Command == "/cancel" {
		ctx.Bot.Remote.SendMessageAsync(ctx.Msg.Chat.Id, "Command cancelled.", nil, "HTML", nil, false, nil)
		ctx.SetState(nil)
		return
	}

	if ctx.GetState() == nil {
		this = &PostState{}
		ctx.SetState(this)
	}

	var prompt string
	var reply *int

	if ctx.Cmd.Command == "/post" || ctx.Cmd.Command == "/file" || ctx.Cmd.Command == "/f" {
		this.postfile.mode = none
		this.posttagwiz = false
		this.postmode = postfile
	} else if ctx.Cmd.Command == "/tag" || ctx.Cmd.Command == "/t" {
		this.posttagwiz = false
		this.postmode = posttags
	} else if ctx.Cmd.Command == "/wizard" || ctx.Cmd.Command == "/w" {
		this.posttagwiz = true
		this.postmode = postwizard
		this.SetTagRulesByName(ctx.Msg.From.Id, ctx.Cmd.Argstr)
	} else if ctx.Cmd.Command == "/rating" || ctx.Cmd.Command == "/r" {
		this.posttagwiz = false
		this.postrating = ""
		this.postmode = postrating
	} else if ctx.Cmd.Command == "/source" || ctx.Cmd.Command == "/src" || ctx.Cmd.Command == "/s" {
		this.posttagwiz = false
		this.postsource = ""
		this.postmode = postsource
	} else if ctx.Cmd.Command == "/description" || ctx.Cmd.Command == "/desc" || ctx.Cmd.Command == "/d" {
		this.posttagwiz = false
		this.postdescription = ""
		this.postmode = postdescription
	} else if ctx.Cmd.Command == "/parent" || ctx.Cmd.Command == "/p" {
		this.posttagwiz = false
		this.postparent = nil
		this.postmode = postparent
	} else if ctx.Cmd.Command == "/upload" {
		this.postmode = postupload
	} else if ctx.Cmd.Command == "/help" {
		ctx.Bot.Remote.SendMessageAsync(ctx.Msg.From.Id, ShowHelp("post"), reply, "HTML", nil, true, nil)
		return
	} else if ctx.Cmd.Command == "/z" && this.postmode == postwizard {
		this.postwizard.ToggleTagsFromString(ctx.Cmd.Argstr)
		this.postwizard.UpdateMenu()
	} else if ctx.Cmd.Command == "/next" && this.postmode == postwizard {
		this.postwizard.NextMenu()
	} else if ctx.Cmd.Command == "/finish" && this.postmode == postwizard {
		this.postwizard.FinishMenu()
		this.postmode = postnext
		prompt = "Done tagging.\n\n"
	} else if ctx.Cmd.Command == "/again" && this.postmode == postwizard {
		this.postwizard.DoOver()
	}

	if this.postmode == postfile || this.postmode == postpublic {
		if ctx.Msg.Photo != nil { // inline photo
			prompt = "That photo was compressed by telegram, and its quality may be severely degraded.  Send it as a file instead if you're sure.\n\n"
		} else if ctx.Msg.Document != nil { // inline file
			prompt = "Preparing to post file sent in this message.\n\n"
			this.postfile.mode = file_id
			this.postfile.file_id = ctx.Msg.Document.File_id
			if this.postmode == postpublic {
				fwd, err := ctx.Bot.Remote.ForwardMessage(ctx.Msg.From.Id, ctx.Msg.Chat.Id, ctx.Msg.Message_id, true)
				if err == nil { reply = &fwd.Message_id }
			} else { reply = &ctx.Msg.Message_id }
			this.postmode = postnext
		} else if ctx.Msg.Reply_to_message != nil && ctx.Msg.Reply_to_message.Document != nil { // reply to file
			prompt = "Preparing to post file sent in this message.\n\n"
			this.postfile.mode = file_id
			this.postfile.file_id = ctx.Msg.Reply_to_message.Document.File_id
			if this.postmode == postpublic {
				fwd, err := ctx.Bot.Remote.ForwardMessage(ctx.Msg.From.Id, ctx.Msg.Chat.Id, ctx.Msg.Reply_to_message.Message_id, true)
				if err == nil { reply = &fwd.Message_id }
			} else { reply = &ctx.Msg.Reply_to_message.Message_id }
			this.postmode = postnext
		} else if strings.HasPrefix(ctx.Cmd.Argstr, "http://") || strings.HasPrefix(ctx.Cmd.Argstr, "https://") { // inline url
			prompt = fmt.Sprintf("Preparing to post from <a href=\"%s\">this URL</a>.\n\n", ctx.Cmd.Argstr)
			this.postfile.mode = url
			this.postfile.url = ctx.Cmd.Argstr
			this.postmode = postnext
		} else if ctx.Msg.Reply_to_message != nil && ctx.Msg.Reply_to_message.Photo != nil { // reply to photo
			prompt = "That photo was compressed by telegram, and its quality may be severely degraded.  Send it as a file instead.\n\n"
			if this.postmode != postpublic { reply = &ctx.Msg.Reply_to_message.Message_id }
		} else if ctx.Msg.Reply_to_message != nil || ctx.Cmd.Argstr != "" { // reply to unknown, or unknown
			prompt = "Sorry, I don't know what to do with that.\n\nPlease send me a file. Either send (or forward) one directly, reply to one you sent earlier, or send a URL."
			reply = &ctx.Msg.Message_id
		} else {
			this.postmode = postnext
		}
	} else if this.postmode == postwizard {
		if ctx.Cmd.Command == "" {
			this.postwizard.MergeTagsFromString(ctx.Cmd.Argstr)
			this.postwizard.UpdateMenu()
		}
		if ctx.Cmd.Command == "/next" {
			this.postwizard.NextMenu()
		}
	} else if this.postmode == posttags {
		if ctx.Cmd.Argstr == "" {
			prompt = "Please send some new tags."
		} else {
			if this.postwizard.Len() != 0 {
				prompt = fmt.Sprintf("Replaced previous tags.\n(%s)", this.postwizard.TagString())
			} else {
				prompt = "Applied tags."
			}
			this.postwizard.Reset()
			this.postwizard.MergeTagsFromString(ctx.Cmd.Argstr)
			this.postmode = postnext
		}
	} else if this.postmode == postrating {
		this.postrating = api.SanitizeRating(ctx.Cmd.Argstr)
		if ctx.Cmd.Argstr == "" {
			this.postmode = postnext
		} else if this.postrating == "" {
			prompt = "Sorry, that isn't a valid rating.\n\nPlease enter the post's rating! Safe, Questionable, or Explicit?"
		} else {
			prompt = fmt.Sprintf("Set rating to %s.\n\n", this.postrating)
			this.postmode = postnext
		}
	} else if this.postmode == postsource {
		this.postsource = ctx.Cmd.Argstr
		if this.postsource == "." { this.postsource = "" }
		this.postmode = postnext
		if ctx.Cmd.Argstr == "" {
		} else if ctx.Cmd.Argstr == "." {
			prompt = "Cleared sources.\n\n"
		} else {
			prompt = "Set sources.\n\n"
		}
	} else if this.postmode == postdescription {
		this.postdescription = ctx.Cmd.Argstr
		if this.postdescription == "." { this.postdescription = "" }
		this.postmode = postnext
		if ctx.Cmd.Argstr == "" {
		} else if ctx.Cmd.Argstr == "." {
			prompt = "Cleared description.\n\n"
		} else {
			prompt = "Set description.\n\n"
		}
	} else if this.postmode == postparent {
		this.postmode = postnext
		if ctx.Cmd.Argstr != "" {
			num, err := strconv.Atoi(ctx.Cmd.Argstr)
			if err != nil {
				submatches := apiurlmatch.FindStringSubmatch(ctx.Cmd.Argstr)
				if len(submatches) != 0 {
					num, err = strconv.Atoi(ctx.Cmd.Argstr)
				}
			}
			if err == nil {
				this.postparent = &num
				prompt = "Set parent post.\n\n"
			} else {
				this.postparent = nil
				prompt = "Cleared parent post.\n\n"
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
				file, err := ctx.Bot.Remote.GetFile(this.postfile.file_id)
				if err != nil || file == nil || file.File_path == nil {
					ctx.Bot.Remote.SendMessageAsync(ctx.Msg.From.Id, fmt.Sprintf("Error while fetching %s, try sending it again?", this.postfile.file_id), nil, "HTML", nil, true, nil)
					this.postmode = postnext
					return
				}
				post_filedata, err = ctx.Bot.Remote.DownloadFile(*file.File_path)
				if err != nil || post_filedata == nil {
					ctx.Bot.Remote.SendMessageAsync(ctx.Msg.From.Id, fmt.Sprintf("Error while downloading %s, try sending it again?", this.postfile.file_id), nil, "HTML", nil, true, nil)
					this.postmode = postnext
					return
				}
			}
			user, apikey, _, err := storage.GetUserCreds(ctx.Msg.From.Id)
			result, err := api.UploadFile(post_filedata, post_url, this.postwizard.TagString(), this.postrating, this.postsource, this.postdescription, this.postparent, user, apikey)
			if err != nil || !result.Success {
				if result.StatusCode == 403 {
					ctx.Bot.Remote.SendMessageAsync(ctx.Msg.From.Id, fmt.Sprintf("It looks like your api key isn't valid, you need to login again.", *result.Reason), nil, "HTML", nil, true, nil)
					this.Reset(ctx)
				} else if result.Location != nil && result.StatusCode == 423 {
					ctx.Bot.Remote.SendMessageAsync(ctx.Msg.From.Id, fmt.Sprintf("It looks like that file has already been posted. <a href=\"%s\">Check it out here.</a>", *result.Location), nil, "HTML", nil, true, nil)
					this.Reset(ctx)
				} else {
					ctx.Bot.Remote.SendMessageAsync(ctx.Msg.From.Id, fmt.Sprintf("I'm having issues posting that file. (%s)", *result.Reason), nil, "HTML", nil, true, nil)
				}
			} else {
				ctx.Bot.Remote.SendMessageAsync(ctx.Msg.From.Id, fmt.Sprintf("Upload complete! <a href=\"%s\">Check it out.</a>", *result.Location), nil, "HTML", nil, true, nil)
				this.Reset(ctx)
			}

			if ctx.GetState() == nil { return }
		}
	}

	if this.postmode == postnext {
		if this.postrating == "" {
			newrating := this.postwizard.Rating()
			if newrating != "" {
				this.postrating = newrating
				prompt = fmt.Sprintf("%sThe tags imply the rating of this post is %s.\n\n", prompt, this.postrating)
			}
		}

		if this.postfile.mode == none {
			prompt = fmt.Sprintf("%s%s", prompt,  "Please send me a file. Either send (or forward) one directly, reply to one you sent earlier, or send a URL.")
			this.postmode = postfile
		} else if this.postwizard.Len() == 0 {
			this.posttagwiz = true
			this.postmode = postwizard
		} else if this.postrating == "" {
			prompt = "Please enter the post's rating! Safe, Questionable, or Explicit?"
			this.postmode = postrating
		} else {
			if this.postready == false {
				prompt = fmt.Sprintf("%s%s", prompt, "Your post now has enough information to submit!\n\n")
				this.postready = true
			}

			if this.postsource == "" {
				prompt = fmt.Sprintf("%s%s", prompt, "Please enter the post source links.")
				this.postmode = postsource
			} else if this.postdescription == "" {
				prompt = fmt.Sprintf("%s%s", prompt, "Please enter the description.\n<a href=\"https://" + api.Endpoint + "/help/show/dtext\">Remember, you can use DText.</a>")
				this.postmode = postdescription
			} else if this.postparent == nil {
				prompt = fmt.Sprintf("%s%s", prompt, "Please enter the parent post.")
				this.postmode = postparent
			} else if !this.postdone {
				prompt = fmt.Sprintf("%sThat's it! You've entered all of the info.", prompt)
				this.postdone = true
			}
		}
	}

	if prompt != "" {
		ctx.Bot.Remote.SendMessageAsync(ctx.Msg.From.Id, prompt, reply, "HTML", nil, true, nil)
	}

	if this.posttagwiz && this.postmode == postwizard {
		this.postwizard.SendWizard(int64(ctx.Msg.From.Id))
		this.posttagwiz = false
	}
}

type JanitorState struct {
}

func (this *JanitorState) Handle(ctx *telebot.MsgContext) {
	if ctx.Msg.From == nil {
		// ignore messages not sent by a user.
		return
	}

	user, apikey, janitor, err := storage.GetUserCreds(ctx.Msg.From.Id)
	if !janitor {
		// commands from non-authorized users are silently ignored
		return
	}
	if err != nil {
		ctx.Bot.Remote.SendMessageAsync(ctx.Msg.From.Id, "You need to be logged in to " + api.ApiName + " to use this command (see <code>/help login</code>)", nil, "HTML", nil, true, nil)
		return
	}

	if ctx.Cmd.Command == "/indextags" {
		go tagindex.SyncTagsExternal(ctx.Bot, &ctx.Msg, ctx.Cmd)
	} else if ctx.Cmd.Command == "/indextagaliases" {
		go tagindex.UpdateAliases(ctx.Bot, &ctx.Msg)
	} else if ctx.Cmd.Command == "/recountnegative" {
		go tagindex.RecountNegative(ctx.Bot, &ctx.Msg, ctx.Cmd)
	} else if ctx.Cmd.Command == "/cats" {
		go tagindex.Concatenations(ctx.Bot, &ctx.Msg, ctx.Cmd)
	} else if ctx.Cmd.Command == "/blits" {
		go tagindex.Blits(ctx.Bot, &ctx.Msg, ctx.Cmd)
	} else if ctx.Cmd.Command == "/findtagtypos" {
		go tagindex.FindTagTypos(ctx.Bot, &ctx.Msg, ctx.Cmd)
	} else if ctx.Cmd.Command == "/recounttags" {
		go tagindex.RecountTagsExternal(ctx.Bot, &ctx.Msg, ctx.Cmd)
	} else if ctx.Cmd.Command == "/syncposts" {
		go tagindex.SyncPosts(ctx.Bot, &ctx.Msg, ctx.Cmd)
	} else if ctx.Cmd.Command == "/editposttest" {
		post := 2893902 // https://api-host/post/show/2893902
		newtags := "1:1 2021 anthro beastars canid canine canis clothed clothing fur grey_body grey_fur hi_res javigameboy legoshi_(beastars) male mammal simple_background solo teeth wolf"
		oldtags := "1:1 2021 anthro beastars canid canine canis clothed clothing fur grey_body grey_fur hi_res javigameboy legoshi_(beastars) male mammal simple_background solo teeth wolf"
		sources := "https://twitter.com/Javigameboy/status/1429921007721062401"
		description := ""
		parent_post := -1
		rating := "safe"
	
		reason := "API Update Test (should be NOOP)"
		api.UpdatePost(user, apikey, post, &oldtags, &newtags, &rating, &parent_post, &sources, &description, &reason)
	}
}
