package bot

import (
	"telegram"
	"telegram/telebot"
	"storage"
	"api"

	"fmt"
	"strings"
	"bytes"
	"regexp"
	"strconv"
	"sort"

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
var apiName string

type settings interface {
	GetApiEndpoint() string
	GetApiName() string
}

func Init(s settings) error {
	apiName = s.GetApiName()
	if apiName == "" {
		return errors.New("missing required parameters")
	}

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
	mode		int

	user	string
	apikey	string

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

	tagwizardrules  string
	tagrulename     string
}

func (this *UserState) SetTagRulesByName(name string) {
	if name == "" { name = "main" }
	rules, _ := storage.GetUserTagRules(this.my_id, name)
	this.postwizard.SetNewRulesFromString(rules)
}

func (this *UserState) WriteUserCreds(e6Username, e6Apikey string) {
	storage.WriteUserCreds(this.my_id, e6Username, e6Apikey)
}

func (this *UserState) HasUserCreds() (bool) {
	user, apikey, err := storage.GetUserCreds(this.my_id)
	return user != "" && apikey != "" && err == nil
}

func (this *UserState) GetUserCreds() (string, string, error) {
	return storage.GetUserCreds(this.my_id)
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
	help :=
`.public.example.wizard.post.advanced.faq.login.contact. Hello! I'm the <b>` + apiName + ` Telegram Bot</b>!
.public.example.wizard.post.advanced.faq.login.contact. 
.public.example.wizard.post.advanced.faq.login.contact. Content on ` + apiName + ` may be unsuitable for kids.
.public.example.wizard.post.advanced.faq.login.contact. <b>You must be 18 or older to use this bot.</b>
.public. 
.public. This bot's commands and help messages should be used via PM.
.advanced. 
.advanced. <b>General Usage</b>
.advanced. I work like @gif! Simply type my name, followed by search terms.
.advanced. <code>@fsb frosted_butts</code>
.advanced. All tags which are supported on the site work here!
login.advanced. 
login.advanced. <b>Using Your Account</b>
login.advanced. Some of my features require you to connect your ` + apiName + ` account.
login.advanced. <code>/login [user] [apikey] -</code> connect to your ` + apiName + ` account.
login.advanced. <code>/logout                -</code> disconnect your account.
.advanced. 
.advanced. <b>Posting</b>
.advanced. Upload or edit posts. You must connect your ` + apiName + ` account.
.advanced. <code>/post        ... -</code> posts a new file.
.advanced. <code>/settagrules ... -</code> updates your tag rules.
post. 
post. Post Command
post. Posting a file to ` + apiName + ` requires gathering some information. This command pulls everything together, and then does an upload. You must connect to your ` + apiName + ` account to use this.
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
.faq. <code>*</code> Before posting to ` + apiName + `, please make sure you read the site's rules.
.faq. <code>*</code> Your account standing is your responsibility.
.faq. <code>*</code> Your ` + apiName + ` API key is NOT your password. Go here to find it.
.faq. <code>*</code> To report a bug, see <code>/help contact.</code>
contact. 
contact. <b>Contacting the author</b>
contact. You may be contacted by the bot author for more information.
contact. <code>/operator [what's wrong] -</code> Flag something for review.
birds. What <b>are</b> birds?
birds. We just don't know.`

	output := bytes.NewBuffer(nil)
	for _, line := range strings.Split(help, "\n") {
		chunks := strings.SplitN(line, ". ", 2)
		tokens := strings.Split(chunks[0], ".")
		for _, blip := range tokens {
			if blip == topic {
				output.WriteString(chunks[1])
				output.WriteRune('\n')
				break
			}
		}
	}

	out := output.String()
	if out == "" { return "Sorry, no help available for that." }
	return out
}

func Handle(bot *telebot.TelegramBot, message *telegram.TMessage, callback *telegram.TCallbackQuery) {
	var state *UserState
	var from telegram.TUser
	var cmd telebot.CommandData
	var err error

	if message != nil {
		// ignore messages without a sender completely.
		if message.From == nil { return }
		from = *message.From

		cmd, err = bot.ParseCommand(message)
	} else if callback != nil {
		from = callback.From
		if callback.Data != nil {
			cmd, err = bot.ParseCommandFromString(*callback.Data)
		}
	}

	state = GetUserState(&from, bot)

	if state.mode == root && message != nil { // general command accept state
		if err != nil { return }

		if message.Chat.Type != "private" {
			if cmd.Command == "/help" {
				bot.Remote.SendMessage(message.Chat.Id, ShowHelp("public"), nil, "HTML", nil, true)
			} else if cmd.Command == "/login" {
				bot.Remote.SendMessage(message.Chat.Id, "You should only use this command in private, to protect the security of your account.\n\nIf you accidentally posted your API key publicly, <a href=\"https://" + api.Endpoint + "/user/api_key\"go here to revoke it.</a>", &message.Message_id, "HTML", nil, false)
			} else if cmd.Command == "/logout" {
				state.mode = logout
			} else if cmd.Command == "/post" {
				if !state.HasUserCreds() {
					bot.Remote.SendMessage(from.Id, "You must connect your " + apiName + " account to use this feature.", nil, "HTML", nil, true)
					return
				}
				state.mode = post
				state.postmode = postpublic
			} else if cmd.Command == "/settagrules" {
				state.mode = settagrules
			}
		} else {
			if cmd.Command == "/help" {
				bot.Remote.SendMessage(message.Chat.Id, ShowHelp(cmd.Argstr), nil, "HTML", nil, false)
			} else if cmd.Command == "/login" {
				state.mode = login
			} else if cmd.Command == "/logout" {
				state.mode = logout
			} else if cmd.Command == "/post" {
				if !state.HasUserCreds() {
					bot.Remote.SendMessage(from.Id, "You must connect your " + apiName + " account to use this feature.", nil, "HTML", nil, true)
					return
				}
				state.mode = post
				state.postmode = postfile
			} else if cmd.Command == "/settagrules" {
				state.mode = settagrules
			}
		}

		if state.mode == root { return }	
	}

	if state.mode == login {
		if cmd.Command == "/cancel" {
			state.mode = root
			bot.Remote.SendMessage(from.Id, "Command cancelled.", nil, "HTML", nil, false)
			return
		}
		for _, token := range cmd.Args {
			if token == "" {
			} else if state.user == "" {
				state.user = token
			} else if state.apikey == "" {
				state.apikey = token
			}
			if state.user != "" && state.apikey != "" {
				success, err := api.TestLogin(state.user, state.apikey)
				if success && err == nil {
					bot.Remote.SendMessage(from.Id, fmt.Sprintf("You are now logged in as <code>%s</code>.", state.user), nil, "HTML", nil, true)
					state.WriteUserCreds(state.user, state.apikey)
				} else if err != nil {
					bot.Remote.SendMessage(from.Id, fmt.Sprintf("An error occurred when testing if you were logged in! (%s)", err.Error()), nil, "HTML", nil, true)
				} else if !success {
					bot.Remote.SendMessage(from.Id, "Login failed! (api key invalid?)", nil, "HTML", nil, true)
				}
				state.Reset()
				return
			}
		}

		if state.user == "" {
			bot.Remote.SendMessage(from.Id, "Please send your " + apiName + " username.", nil, "HTML", nil, true)
		} else if state.apikey == "" {
			bot.Remote.SendMessage(from.Id, "Please send your " + apiName + " <a href=\"https://" + api.Endpoint + "/user/api_key\">API Key</a>. (not your password!)", nil, "HTML", nil, true)
		}
		return
	} else if state.mode == logout {
		bot.Remote.SendMessage(from.Id, "You are now logged out.", nil, "HTML", nil, true)
		state.WriteUserCreds("", "")
		state.mode = root
		return
	} else if state.mode == settagrules {
		if cmd.Command == "/cancel" {
			state.Reset()
			bot.Remote.SendMessage(from.Id, "Command cancelled.", nil, "HTML", nil, true)
			return
		}

		var doc *telegram.TDocument
		if message != nil && message.Document != nil {
			doc = message.Document
		} else if message != nil && message.Reply_to_message != nil && message.Reply_to_message.Document != nil {
			doc = message.Reply_to_message.Document
		}

		if doc != nil {
			if !strings.HasSuffix(doc.File_name, ".txt") {
				bot.Remote.SendMessage(from.Id, fmt.Sprintf("%s isn't a plain text file.", doc.File_name), nil, "HTML", nil, true)
				return
			}
			file, err := bot.Remote.GetFile(doc.File_id)
			if err != nil || file == nil || file.File_path == nil {
				bot.Remote.SendMessage(from.Id, fmt.Sprintf("Error while fetching %s, try sending it again?", doc.File_name), nil, "HTML", nil, true)
				return
			}
			file_data, err := bot.Remote.DownloadFile(*file.File_path)
			if err != nil || file_data == nil {
				bot.Remote.SendMessage(from.Id, fmt.Sprintf("Error while downloading %s, try sending it again?", doc.File_name), nil, "HTML", nil, true)
				return
			}
			state.tagwizardrules = string(file_data)
		} else if cmd.Argstr != "" {
			state.tagrulename = cmd.Argstr
		} else {
			bot.Remote.SendMessage(from.Id, "Send some new tag rules in a text file.", nil, "HTML", nil, true)
			return
		}

		if state.tagwizardrules != "" {
			if state.tagrulename == "" { state.tagrulename = "main" }
			state.tagwizardrules = strings.Replace(state.tagwizardrules, "\r", "", -1) // pesky windows carriage returns
			state.WriteUserTagRules(state.tagwizardrules, state.tagrulename)
			if err := state.postwizard.SetNewRulesFromString(state.tagwizardrules); err != nil {
				bot.Remote.SendMessage(from.Id, fmt.Sprintf("Error while parsing tag rules: %s", err.Error()), nil, "HTML", nil, true)
				return
			} else {
				bot.Remote.SendMessage(from.Id, "Set new tag rules.", nil, "HTML", nil, true)
				state.Reset()
				return
			}
		}
	} else if state.mode == post {
		if cmd.Command == "/cancel" {
			state.Reset()
			bot.Remote.SendMessage(from.Id, "Command cancelled.", nil, "HTML", nil, true)
			return
		}

		var prompt, callback_message string
		var reply *int

		if cmd.Command == "/file" || cmd.Command == "/f" {
			state.postfile.mode = none
			state.posttagwiz = false
			state.postmode = postfile
		} else if cmd.Command == "/tag" || cmd.Command == "/t" {
			state.posttagwiz = false
			state.postmode = posttags
		} else if cmd.Command == "/wizard" || cmd.Command == "/w" {
			state.posttagwiz = true
			state.postmode = postwizard
			state.SetTagRulesByName(cmd.Argstr)
		} else if cmd.Command == "/rating" || cmd.Command == "/r" {
			state.posttagwiz = false
			state.postrating = ""
			state.postmode = postrating
		} else if cmd.Command == "/source" || cmd.Command == "/src" || cmd.Command == "/s" {
			state.posttagwiz = false
			state.postsource = ""
			state.postmode = postsource
		} else if cmd.Command == "/description" || cmd.Command == "/desc" || cmd.Command == "/d" {
			state.posttagwiz = false
			state.postdescription = ""
			state.postmode = postdescription
		} else if cmd.Command == "/parent" || cmd.Command == "/p" {
			state.posttagwiz = false
			state.postparent = 0
			state.postmode = postparent
		} else if cmd.Command == "/upload" {
			state.postmode = postupload
		} else if cmd.Command == "/help" {
			bot.Remote.SendMessage(from.Id, ShowHelp("post"), reply, "HTML", nil, true)
			return
		}

		if message != nil && (state.postmode == postfile || state.postmode == postpublic) {
			if message.Photo != nil { // inline photo
				prompt = "That photo was compressed by telegram, and its quality may be severely degraded.  Send it as a file instead.\n\n"
			} else if message.Document != nil { // inline file
				prompt = "Preparing to post file sent in this message.\n\n"
				state.postfile.mode = file_id
				state.postfile.file_id = message.Document.File_id
				if state.postmode == postpublic {
					fwd, err := bot.Remote.ForwardMessage(from.Id, message.Chat.Id, message.Message_id, true)
					if err == nil { reply = &fwd.Message_id }
				} else { reply = &message.Message_id }
				state.postmode = postnext
			} else if message.Reply_to_message != nil && message.Reply_to_message.Document != nil { // reply to file
				prompt = "Preparing to post file sent in this message.\n\n"
				state.postfile.mode = file_id
				state.postfile.file_id = message.Reply_to_message.Document.File_id
				if state.postmode == postpublic {
					fwd, err := bot.Remote.ForwardMessage(from.Id, message.Chat.Id, message.Reply_to_message.Message_id, true)
					if err == nil { reply = &fwd.Message_id }
				} else { reply = &message.Reply_to_message.Message_id }
				state.postmode = postnext
			} else if strings.HasPrefix(cmd.Argstr, "http://") || strings.HasPrefix(cmd.Argstr, "https://") { // inline url
				prompt = fmt.Sprintf("Preparing to post from <a href=\"%s\">this URL</a>.\n\n", cmd.Argstr)
				state.postfile.mode = url
				state.postfile.url = cmd.Argstr
				state.postmode = postnext
			} else if message.Reply_to_message != nil && message.Reply_to_message.Photo != nil { // reply to photo
				prompt = "That photo was compressed by telegram, and its quality may be severely degraded.  Send it as a file instead.\n\n"
				if state.postmode != postpublic { reply = &message.Reply_to_message.Message_id }
			} else if message.Reply_to_message != nil || cmd.Argstr != "" { // reply to unknown, or unknown
				prompt = "Sorry, I don't know what to do with that.\n\nPlease send me a file. Either send (or forward) one directly, reply to one you sent earlier, or send a URL."
				reply = &message.Message_id
			} else {
				state.postmode = postnext
			}
		} else if callback != nil {
			if cmd.Command == "/z" {
				state.postwizard.ToggleTagsFromString(cmd.Argstr)
				state.postwizard.UpdateMenu()
			} else if cmd.Command == "/next" {
				state.postwizard.NextMenu()
			} else if cmd.Command == "/finish" {
				state.postwizard.FinishMenu()
				state.postmode = postnext
				prompt = "Done tagging.\n\n"
			} else if cmd.Command == "/again" {
				state.postwizard.DoOver()
			}
		} else if message != nil && state.postmode == postwizard {
			if cmd.Command == "" {
				state.postwizard.MergeTagsFromString(cmd.Argstr)
				state.postwizard.UpdateMenu()
			}
			if cmd.Command == "/next" {
				state.postwizard.NextMenu()
			}
		} else if message != nil && state.postmode == posttags {
			if cmd.Argstr == "" {
				prompt = "Please send some new tags."
			} else {
				if state.postwizard.Len() != 0 {
					prompt = fmt.Sprintf("Replaced previous tags.\n(%s)", state.postwizard.TagString())
				} else {
					prompt = "Applied tags."
				}
				state.postwizard.Reset()
				state.postwizard.MergeTagsFromString(cmd.Argstr)
				state.postmode = postnext
			}
		} else if message != nil && state.postmode == postrating {
			state.postrating = api.SanitizeRating(cmd.Argstr)
			if cmd.Argstr == "" {
				state.postmode = postnext
			} else if state.postrating == "" {
				prompt = "Sorry, that isn't a valid rating.\n\nPlease enter the post's rating! Safe, Questionable, or Explicit?"
			} else {
				prompt = fmt.Sprintf("Set rating to %s.\n\n", state.postrating)
				state.postmode = postnext
			}
		} else if message != nil && state.postmode == postsource {
			state.postsource = cmd.Argstr
			if state.postsource == "." { state.postsource = "" }
			state.postmode = postnext
			if cmd.Argstr == "" {
			} else if cmd.Argstr == "." {
				prompt = "Cleared sources.\n\n"
			} else {
				prompt = "Set sources.\n\n"
			}
		} else if message != nil && state.postmode == postdescription {
			state.postdescription = cmd.Argstr
			if state.postdescription == "." { state.postdescription = "" }
			state.postmode = postnext
			if cmd.Argstr == "" {
			} else if cmd.Argstr == "." {
				prompt = "Cleared description.\n\n"
			} else {
				prompt = "Set description.\n\n"
			}
		} else if message != nil && state.postmode == postparent {
			state.postmode = postnext
			if cmd.Argstr != "" {
				num, err := strconv.Atoi(cmd.Argstr)
				if err != nil {
					submatches := apiurlmatch.FindStringSubmatch(cmd.Argstr)
					if len(submatches) != 0 {
						num, err = strconv.Atoi(cmd.Argstr)
					}
				}
				if err == nil {
					state.postparent = num
					prompt = "Set parent post.\n\n"
				} else {
					state.postparent = 0
					prompt = "Cleared parent post.\n\n"
				}
			}
		} else if state.postmode == postupload {
			if state.postfile.mode == none || state.postwizard.Len() < 6 || state.postrating == "" {
				state.postmode = postnext
			} else {
				var post_url string
				var post_filedata []byte
				if state.postfile.mode == url {
					post_url = state.postfile.url
				} else {
					file, err := bot.Remote.GetFile(state.postfile.file_id)
					if err != nil || file == nil || file.File_path == nil {
						bot.Remote.SendMessage(from.Id, fmt.Sprintf("Error while fetching %s, try sending it again?", state.postfile.file_id), nil, "HTML", nil, true)
						state.postmode = postnext
						return
					}
					post_filedata, err = bot.Remote.DownloadFile(*file.File_path)
					if err != nil || post_filedata == nil {
						bot.Remote.SendMessage(from.Id, fmt.Sprintf("Error while downloading %s, try sending it again?", state.postfile.file_id), nil, "HTML", nil, true)
						state.postmode = postnext
						return
					}
				}
				user, apikey, err := state.GetUserCreds()
				result, err := api.UploadFile(post_filedata, post_url, state.postwizard.TagString(), state.postrating, state.postsource, state.postdescription, state.postparent, user, apikey)
				if err != nil || !result.Success {
					if result.StatusCode == 403 {
						bot.Remote.SendMessage(from.Id, fmt.Sprintf("It looks like your api key isn't valid, you need to login again.", *result.Reason), nil, "HTML", nil, true)
						state.Reset()
					} else if result.Location != nil && result.StatusCode == 423 {
						bot.Remote.SendMessage(from.Id, fmt.Sprintf("It looks like that file has already been posted. <a href=\"%s\">Check it out here.</a>", *result.Location), nil, "HTML", nil, true)
						state.Reset()
					} else {
						bot.Remote.SendMessage(from.Id, fmt.Sprintf("I'm having issues posting that file. (%s)", *result.Reason), nil, "HTML", nil, true)
					}
				} else {
					bot.Remote.SendMessage(from.Id, fmt.Sprintf("Upload complete! <a href=\"%s\">Check it out.</a>", *result.Location), nil, "HTML", nil, true)
					state.Reset()
				}

				if state.mode == root { return }
			}
		}

		if state.postmode == postnext {
			if state.postrating == "" {
				newrating := state.postwizard.Rating()
				if newrating != "" {
					state.postrating = newrating
					prompt = fmt.Sprintf("%sThe tags imply the rating of this post is %s.\n\n", prompt, state.postrating)
				}
			}

			if state.postfile.mode == none {
				prompt = fmt.Sprintf("%s%s", prompt,  "Please send me a file. Either send (or forward) one directly, reply to one you sent earlier, or send a URL.")
				state.postmode = postfile
			} else if state.postwizard.Len() == 0 {
				state.posttagwiz = true
				state.postmode = postwizard
			} else if state.postrating == "" {
				prompt = "Please enter the post's rating! Safe, Questionable, or Explicit?"
				state.postmode = postrating
			} else {
				if state.postready == false {
					prompt = fmt.Sprintf("%s%s", prompt, "Your post now has enough information to submit!\n\n")
					state.postready = true
				}

				if state.postsource == "" {
					prompt = fmt.Sprintf("%s%s", prompt, "Please enter the post source links.")
					state.postmode = postsource
				} else if state.postdescription == "" {
					prompt = fmt.Sprintf("%s%s", prompt, "Please enter the description.\n<a href=\"https://" + api.Endpoint + "/help/show/dtext\">Remember, you can use DText.</a>")
					state.postmode = postdescription
				} else if state.postparent == 0 {
					prompt = fmt.Sprintf("%s%s", prompt, "Please enter the parent post.")
					state.postmode = postparent
				} else if !state.postdone {
					prompt = fmt.Sprintf("%sThat's it! You've entered all of the info.", prompt)
					state.postdone = true
				}
			}
		}

		if prompt != "" {
			bot.Remote.SendMessage(from.Id, prompt, reply, "HTML", nil, true)
		}

		if callback != nil {
			bot.Remote.AnswerCallbackQuery(callback.Id, callback_message, true)
		}

		if state.posttagwiz && state.postmode == postwizard {
			state.postwizard.SendWizard(int64(from.Id))
			state.posttagwiz = false
		}
	}
}
