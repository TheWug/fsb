package wizard

import (
	"github.com/thewug/fsb/pkg/api/tags"
	"github.com/thewug/fsb/pkg/api/types"

	"github.com/thewug/gogram/data"
	"github.com/kballard/go-shellquote"

	"encoding/json"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

/*
Here's how you use a tag wizard.

1. create a new tag wizard, however is convenient.

2. call tag_wizard.SetNewRulesFromString with a user's tag rules. there
   are no default tag rules so without at least putting something in,
   the wizard won't do anything.

2.5 if you have json-serialized one, it is safe to unserialize it now,
   although it will only work correctly if you use the same tag rules
   both before and after serialization.

3. the tag wizard guides you through one or more states, each preceeded by
   a call to Next. if Next returns false, that means there is no more tag
   exploration to be done and all applicable states have been visited.

3.5 you can also call tag_wizard.ButtonPressed with the command string from
   any command, and it will handle it for you, turning true normally and
   false if the "done" button was pressed.

4. after calling next, you can call tag_wizard.Prompt to get the current
   prompt string, and .Buttons to get the wizard suggestions. It traverses
   its internal library of decisions one at a time, visiting every
   relevant node once until there are none left, at which time Next
   returns false.

5. You can call tag_wizard.Tags to get a tagset of all of the real api
   tags (that is, with flow-control metatags and rating tags removed), and
   Rating to get the rating inferred from any rating tags present.

So, the way you'd probably want to handle it all is roughly:

func HandlerFromTelegram() {
	context = getbuttonpressed()
	tag_wizard.ButtonPressed

*/

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

var wizard_rule_done WizardRule = WizardRule{id: -1, prompt: "You've finished the tag wizard."}
var wizard_rule_norules WizardRule = WizardRule{id: -2, prompt: "You haven't set any tag rules!  Enter some tags manually."}

const (
	CMD_NEXT    = "/w-next"
	CMD_RESTART = "/w-restart"
	CMD_DONE    = "/w-done"
	CMD_TAGS    = "/w-tag"
	CMD_PREFIX  = "/w-"

	CMD_TAGS_SPACE = CMD_TAGS + " "
)

var wizard_cmd_prefix string  = CMD_PREFIX
var wizard_cmd_next string    = CMD_NEXT
var wizard_cmd_restart string = CMD_RESTART
var wizard_cmd_done string    = CMD_DONE
var wizard_cmd_tags string    = CMD_TAGS

type WizardRule struct {
	id            int
	prereqs     []string
	options     []string
	prompt        string
	sort          int
	sortdirection bool
	auto          int
	visited       int
}

func (this *WizardRule) Prompt() (string) {
	if this == nil || this.prompt == "" { return "Choose or type some tags." }
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
			this.prompt = t[7:]
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

func (this *WizardRule) PrereqsSatisfied(tags *tags.TagSet) (bool) {
	for _, tag := range this.prereqs {
		if _, ok := tags.Data[tag]; !ok {
			return false
		}
	}

	return true
}

func (this *WizardRule) Applicable(tags *tags.TagSet, visitval int) bool {
	return this.PrereqsSatisfied(tags) && this.visited != visitval
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

func (this *WizardRule) DoImplicit(tags *tags.TagSet) {
	for _, o := range this.options {
		_, set, unset, tag := TagWizardMarkupHelper(o)
		if set { tags.Set(tag) }
		if unset { tags.Clear(tag) }
	}
}

func strPtr(a string) (*string) {
	return &a
}

func (this *WizardRule) Buttons(t *tags.TagSet, w *TagWizard) ([]data.TInlineKeyboardButton) {
	var out []data.TInlineKeyboardButton
	for _, o := range this.options {
		hide, _, _, tag := TagWizardMarkupHelper(o)
		if !hide {
			var decor string
			if t.Status(tag) == tags.AddsTag {
				decor = "\U0001F7E9" // green square
			} else {
				decor = "\U0001F7E5" // red square
			}
			tag_display := tag
			if strings.HasPrefix(strings.ToLower(tag), "meta:") { tag_display = strings.Replace(tag_display[5:], "_", " ", -1) }
			btn := data.TInlineKeyboardButton{Text: decor + " " + tag_display, Data: strPtr(CMD_TAGS_SPACE + w.UID(tag))}
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
	visitval            int
}

func (this *WizardRuleset) Clear() {
	this.interactive_rules = nil
}

func (this *WizardRuleset) Visit(id int) {
	if this.visitval == 0 {
		this.visitval = 1
	}
	if id >= len(this.interactive_rules) || id < 0 { return }
	this.interactive_rules[id].visited = this.visitval
}

func (this *WizardRuleset) Rule(id int) *WizardRule {
	if len(this.interactive_rules) == 0 { return &wizard_rule_norules }
	if id == -1 { return &wizard_rule_done }
	if id >= len(this.interactive_rules) { return nil }
	return &this.interactive_rules[id]
}

func (this *WizardRuleset) Reset() {
	this.visitval++
}

func (this *WizardRuleset) Foreach(f func(*WizardRule) bool) {
	for i := 0; i < len(this.interactive_rules) && f(&this.interactive_rules[i]); i++ {}
}

func (this *WizardRuleset) AddRule(r *WizardRule) {
	r.id = len(this.interactive_rules)
	this.interactive_rules = append(this.interactive_rules, *r)
}

type TagWizard struct {
	tags     tags.TagSet
	rules    WizardRuleset
	current  int
	uids     map[string]int
	uid_tags []string
}

func (this TagWizard) Prompt() string {
	return this.Current().Prompt()
}

func (this *TagWizard) setCurrent(current int) {
	this.current = current
}

func (this *TagWizard) SetNewRulesFromString(rulestring string) {
	this.rules.Clear()
	for _, line := range strings.Split(rulestring, "\n") {
		line = strings.TrimSpace(line)
		if line == "" { continue }
		rule := NewWizardRuleFromString(line)
		this.rules.AddRule(rule)
		this.TagUIDs(rule)
	}
}

func (this *TagWizard) TagUIDs(r *WizardRule) {
	for _, o := range r.options {
		_, _, _, tag := TagWizardMarkupHelper(o)
		this.UID(tag)
	}
}

func (this *TagWizard) UID(tag string) string {
	tag = strings.ToLower(tag)
	id, ok := this.uids[tag]
	if !ok {
		if this.uids == nil {
			this.uids = make(map[string]int)
		}

		id = len(this.uid_tags)
		this.uids[tag] = id
		this.uid_tags = append(this.uid_tags, tag)
	}
	return strconv.Itoa(id)
}

func (this *TagWizard) TagByUID(sid string) string {
	id, err := strconv.Atoi(sid)
	if err != nil || id < 0 || id >= len(this.uid_tags) {
		return ""
	}

	return this.uid_tags[id]
}

func (this *TagWizard) Next() (bool) {
	if this.rules.visitval == 0 {
		this.rules.visitval = 1
	}
	returns := false
	this.rules.Visit(this.current)
	this.current = -1
	this.rules.Foreach(func(w *WizardRule) bool {
		if !w.Applicable(&this.tags, this.rules.visitval) {
			return true // continue
		}
		this.setCurrent(w.id)
		returns = true
		return false
	})

	this.Current().DoImplicit(&this.tags)
	return returns
}

func (this *TagWizard) Current() *WizardRule {
	return this.rules.Rule(this.current)
}

func (this *TagWizard) Rule(id int) *WizardRule {
	return this.rules.Rule(id)
}

var whitespace *regexp.Regexp = regexp.MustCompile(`\s+`)

func (this *TagWizard) MergeTagsFromString(tagstr string) {
	this.tags.ApplyArray(whitespace.Split(tagstr, -1))
}

func (this *TagWizard) ToggleTagsFromString(tagstr string) {
	this.tags.ToggleArray(whitespace.Split(tagstr, -1))
}

func (this *TagWizard) MergeTags(tags []string) {
	this.tags.ApplyArray(tags)
}

func (this *TagWizard) ToggleTags(tags []string) {
	this.tags.ToggleArray(tags)
}

func (this *TagWizard) Reset() {
	this.tags.Reset()
	this.rules.Reset()
	this.current = 0
}

func (this *TagWizard) Len() (int) {
	return this.Tags().Len()
}

func (this *TagWizard) Rating() (types.PostRating) {
	return types.RatingFromTagSet(this.tags)
}

func (this *TagWizard) Tags() (tags.TagSet) {
	t := this.tags.Clone()
	t.ApplyArray([]string{"-meta:*", "-rating:*"})
	return t
}

func (this *TagWizard) SetTag(tag string) {
	this.tags.Set(tag)
}

func (this *TagWizard) ClearTag(tag string) {
	this.tags.Clear(tag)
}

func (this *TagWizard) Buttons() data.TInlineKeyboard {
	var kbd data.TInlineKeyboard
	if this.Current() == &wizard_rule_norules {
		// do nothing, this stand-in rule has no buttons.
	} else if this.Current() == &wizard_rule_done {
		kbd.AddButton(data.TInlineKeyboardButton{Text: "\U0001f501 Start Over", Data: &wizard_cmd_restart})
	} else {
		kbd.AddButton(data.TInlineKeyboardButton{Text: "\u27a1 Next", Data: &wizard_cmd_next})
		kbd.AddButton(data.TInlineKeyboardButton{Text: "\U0001f501 Start Over", Data: &wizard_cmd_restart})
		for _, b := range this.Current().Buttons(&this.tags, this) {
			kbd.AddRow()
			kbd.AddButton(b)
		}
	}
	return kbd
}

func (this *TagWizard) DoOver() {
	this.rules.Reset()
	this.current = 0
}

func (this *TagWizard) ButtonPressed(cmd *string) {
	if cmd == nil { return }
	switch *cmd {
	case wizard_cmd_next:
		this.Next()
		return
	case wizard_cmd_restart:
		this.DoOver()
		return
	case wizard_cmd_done:
		return
	}

	if strings.HasPrefix(*cmd, wizard_cmd_tags) {
		tag := this.TagByUID(strings.TrimPrefix(*cmd, CMD_TAGS_SPACE))
		if tag != "" {
			this.ToggleTags([]string{tag})
		}
	}
}

type wizardSerialized struct {
	Visited []int
	Tags *tags.TagSet
	Current *int
}

func (this *TagWizard) MarshalJSON() ([]byte, error) {
	if this.rules.visitval == 0 {
		this.rules.visitval = 1
	}
	ws := wizardSerialized{Tags: &this.tags, Current: &this.current}
	this.rules.Foreach(func(w *WizardRule) bool {
		if w.visited == this.rules.visitval {
			ws.Visited = append(ws.Visited, w.id)
		}
		return true
	})

	return json.Marshal(ws)
}

func (this *TagWizard) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}

	ws := wizardSerialized{Tags: &this.tags, Current: &this.current}
	err := json.Unmarshal(data, &ws)

	if err != nil {
		return err
	}

	for _, x := range ws.Visited {
		this.rules.Visit(x)
	}

	return nil
}
