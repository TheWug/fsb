package bot

import (
	"github.com/thewug/gogram"
	"github.com/thewug/gogram/data"

	"storage"
	"api"
	"api/tagindex"
	apitypes "api/types"
	"apiextra"

	"fmt"
	"time"
	"strings"
	"bytes"
	"regexp"
	"strconv"
	"sort"
	"html"
	"io/ioutil"
	"io"
	"sync"

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
	gogram.StateIgnoreMessages
}

func (this *HelpState) Handle(ctx *gogram.MessageCtx) {
	topic := "public"
	if ctx.Msg.Chat.Type == "private" {
		topic = ctx.Cmd.Argstr
	}
	ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: ShowHelp(topic), ParseMode: data.ParseHTML}}, nil)
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
	votes map[data.UserID]lookup_votes
	faves map[data.UserID]lookup_faves
	lock sync.Mutex
}

func (this *VoteState) GetInterval() int64 {
	return 30
}

func (this *VoteState) DoMaintenance() {
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

type LoginState struct {
	gogram.StateIgnoreCallbacks

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
	gogram.StateIgnoreCallbacks

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
		this.postrating = api.SanitizeRating(cmd.Argstr)
		if cmd.Argstr == "" {
			this.postmode = postnext
		} else if this.postrating == "" {
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
				if len(submatches) != 0 {
					num, err = strconv.Atoi(cmd.Argstr)
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
	gogram.StateIgnoreMessages
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
	} else if ctx.Cmd.Command == "/editposttest" {
		post := 2893902 // https://api-host/post/show/2893902
		newtags := "1:1 2021 anthro beastars canid canine canis clothed clothing fur grey_body grey_fur hi_res javigameboy legoshi_(beastars) male mammal simple_background solo teeth wolf"
		oldtags := "1:1 2021 anthro beastars canid canine canis clothed clothing fur grey_body grey_fur hi_res javigameboy legoshi_(beastars) male mammal simple_background solo teeth wolf"
		sources := "https://twitter.com/Javigameboy/status/1429921007721062401"
		description := ""
		parent_post := -1
		rating := "safe"

		reason := "API Update Test (should be NOOP)"
		api.UpdatePost(user, apikey, post, api.TagDiff{}, &rating, &parent_post, &sources, &description, &reason)
	}
}
