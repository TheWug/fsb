package dialogs

import (
	"storage"

	"api"
	apitypes "api/types"

	"github.com/thewug/gogram"
	"github.com/thewug/gogram/data"
	"github.com/thewug/gogram/dialog"

	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"strings"
	"time"
)

const EDIT_PROMPT_ID data.DialogID = "editprompt"

// state constants
const WAIT_MODE   string = ""
const WAIT_ALL    string = "wait_all"
const WAIT_TAGS   string = "wait_tags"
const WAIT_SOURCE string = "wait_source"
const WAIT_RATING string = "wait_rating"
const WAIT_DESC   string = "wait_desc"
const WAIT_PARENT string = "wait_parent"
const WAIT_REASON string = "wait_reason"
const WAIT_FILE   string = "wait_file"
const SAVED       string = "saved"
const DISCARDED   string = "discarded"

// special parent constants
const PARENT_NONE int = -1
const PARENT_RESET int = 0

var name_of_state = map[string]string{
	WAIT_TAGS:   "Tags",
	WAIT_SOURCE: "Sources",
	WAIT_RATING: "Rating",
	WAIT_DESC:   "Description",
	WAIT_PARENT: "Parent",
	WAIT_FILE:   "File",
	WAIT_REASON: "Edit Reason",
	WAIT_ALL:    "Everything",
}

func GetNameOfState(state string) string {
	return name_of_state[state]
}

const PF_UNSET int = 0
const PF_FROM_TELEGRAM int = 1
const PF_FROM_URL int = 2

type PostFile struct {
	Mode int `json:"pf_mode"`
	FileId data.FileID `json:"pf_tfid"`
	FileName string `json:"pf_tfname"`
	Url string `json:"pf_furl"`
}

func (this *PostFile) SetTelegramFile(id data.FileID, name string) {
	this.Mode = PF_FROM_TELEGRAM
	this.FileId = id
	this.FileName = name
	this.Url = ""
}

func (this *PostFile) SetUrl(url string) {
	this.Mode = PF_FROM_TELEGRAM
	this.Url = url
	this.FileId = ""
	this.FileName = ""
}

func (this *PostFile) Clear() {
	this.Mode = PF_UNSET
	this.FileId = ""
	this.FileName = ""
	this.Url = ""
}

type EditPrompt struct {
	dialog.TelegramDialogPost `json:"-"`

	PostId int `json:"post_id"`
	Prefix string `json:"prefix"`
	State string `json:"state"`

	// stuff to generate the post info
	TagChanges apitypes.TagDiff `json:"tag_changes"`
	SourceChanges apitypes.TagDiff `json:"source_changes"` // not actually tags, but you can treat them the same.
	OrigSources map[string]int `json:"sources_live"`
	SeenSources map[string]int `json:"source_seen"`
	SeenSourcesReverse []string `json:"source_seen_rev"`
	Parent int `json:"parent"`
	Rating string `json:"rating"`
	Description string `json:"description"`
	File PostFile `json:"file"`
	Reason string `json:"reason"`
}

func (this *EditPrompt) ApplyReset(state string) {
	switch state {
	case WAIT_ALL:
		this.TagChanges.Reset()
		this.SourceChanges.Reset()
		this.SeenSources = nil
		this.SeenSourcesReverse = nil
		for s, _ := range this.OrigSources {
			this.SeeSource(s)
		}
		this.Rating = ""
		this.Description = ""
		this.Parent = PARENT_RESET
		this.Reason = ""
		this.File = PostFile{}
	case WAIT_TAGS:
		this.TagChanges.Reset()
	case WAIT_SOURCE:
		this.SourceChanges.Reset()
		this.SeenSources = nil
		this.SeenSourcesReverse = nil
		for s, _ := range this.OrigSources {
			this.SeeSource(s)
		}
	case WAIT_RATING:
		this.Rating = ""
	case WAIT_DESC:
		this.Description = ""
	case WAIT_PARENT:
		this.Parent = PARENT_RESET
	case WAIT_REASON:
		this.Reason = ""
	case WAIT_FILE:
		this.File = PostFile{}
	default:
	}
}

func (this *EditPrompt) SourceButton(n int, pick bool) {
	// buttons will never not have been seen before so just ignore (but handle gracefully) buttons which are for invalid indexes, as they shouldn't happen
	if n < 0 || n >= len(this.SeenSourcesReverse) { return }
	this.SourceStringLiteral(this.SeenSourcesReverse[n], pick)
}

func (this *EditPrompt) SourceStringLiteral(source string, pick bool) {
	if pick { // make sure we've seen it if we're adding it
		this.SeeSource(source)
	}

	_, live := this.OrigSources[source]
	if !live && pick {
		this.SourceChanges.AddTag(source)
	} else if live && !pick {
		this.SourceChanges.RemoveTag(source)
	} else {
		this.SourceChanges.ResetTag(source)
	}
}

func (this *EditPrompt) SourceStringPrefixed(source string) {
	if strings.HasPrefix(source, "-") {
		this.SourceChanges.RemoveTag(source[1:])
	} else if strings.HasPrefix(source, "=") {
		this.SourceChanges.ResetTag(source[1:])
	} else {
		if strings.HasPrefix(source, "+") {
			source = source[1:]
		}

		this.SeeSource(source)
		this.SourceChanges.AddTag(source)
	}
}

func (this *EditPrompt) SeeSource(source string) {
	if _, ok := this.SeenSources[source]; ok { return } // already seen
	if this.SeenSources == nil { this.SeenSources = make(map[string]int) }
	index := len(this.SeenSources)
	this.SeenSources[source] = index
	this.SeenSourcesReverse = append(this.SeenSourcesReverse, source)
}

func (this *EditPrompt) JSON() (string, error) {
	bytes, err := json.Marshal(this)
	return string(bytes), err
}

func (this *EditPrompt) ID() data.DialogID {
	return EditPromptID()
}

func (this *EditPrompt) Prompt(settings storage.UpdaterSettings, bot *gogram.TelegramBot, ctx *gogram.MessageCtx) (*gogram.MessageCtx) {
	var send data.SendData
	send.Text = this.GenerateMessage(*bot.Remote.GetMe().CanReadAllGroupMessages)
	send.ParseMode = data.ParseHTML
	send.ReplyMarkup = this.GenerateMarkup()
	if this.TelegramDialogPost.IsUnset() {
		// no existing message, send a new one
		if ctx != nil {
			prompt, err := ctx.Reply(data.OMessage{SendData: send})
			if err != nil { bot.ErrorLog.Println("Error sending prompt: ", err.Error()) }
			err = this.FirstSave(settings, prompt.Msg.Id, prompt.Msg.Chat.Id, time.Unix(prompt.Msg.Date, 0), this)
			if err != nil { bot.ErrorLog.Println("Error sending prompt: ", err.Error()) }
			return prompt
		} else {
			panic("You must pass a context to reply to for the initial post!")
		}
	} else {
		// message already exists, update it
		prompt, err := this.Ctx(bot).EditText(data.OMessageEdit{SendData: send})
		if err != nil { bot.ErrorLog.Println("Error sending prompt: ", err.Error()) }
		this.Save(settings)
		return prompt
	}
}

func (this *EditPrompt) Finalize(settings storage.UpdaterSettings, bot *gogram.TelegramBot, ctx *gogram.MessageCtx) (*gogram.MessageCtx) {
	var send data.SendData
	send.Text = this.GenerateMessage(*bot.Remote.GetMe().CanReadAllGroupMessages)
	send.ParseMode = data.ParseHTML
	send.ReplyMarkup = nil
	
	// message already exists, update it
	prompt, err := this.Ctx(bot).EditText(data.OMessageEdit{SendData: send, DisableWebPagePreview: true})
	if err != nil { bot.ErrorLog.Println("Error sending prompt: ", err.Error()) }
	this.Delete(settings)
	return prompt
}

func (this *EditPrompt) ResetState() {
	this.State = WAIT_MODE
	this.Prefix = "What would you like to edit? Pick a button from below."
}

func (this *EditPrompt) IsNoop() bool {
	return len(this.Rating) == 0 &&
               this.TagChanges.IsZero() &&
               this.SourceChanges.IsZero() &&
               len(this.Description) == 0 &&
               this.Parent == 0
}

func (this *EditPrompt) PostStatus(b *bytes.Buffer) {
	no_changes := true
	if this.File.Mode == PF_FROM_TELEGRAM {
		b.WriteString("File: <i>")
		b.WriteString(html.EscapeString(this.File.FileName))
		b.WriteString("</i>\n")
	} else if this.File.Mode == PF_FROM_URL {
		b.WriteString("File: <a href=\"")
		b.WriteString(this.File.Url)
		b.WriteString("\">Fetch from here</a>\n")
	}
	if len(this.Rating) != 0 {
		b.WriteString("Rating: <code>")
		b.WriteString(this.Rating)
		b.WriteString("</code>\n")
		no_changes = false
	}
	if !this.TagChanges.IsZero() {
		b.WriteString("Tags: <code>")
		b.WriteString(html.EscapeString(this.TagChanges.APIString()))
		b.WriteString("</code>\n")
		no_changes = false
	}
	if !this.SourceChanges.IsZero() {
		b.WriteString("Sources:\n<pre>  ")
		b.WriteString(html.EscapeString(this.SourceChanges.APIString()))
		b.WriteString("</pre>\n")
		no_changes = false
	}
	if len(this.Description) != 0 {
		b.WriteString("Description: <code>")
		b.WriteString(html.EscapeString(this.Description))
		b.WriteString("</code>\n")
		no_changes = false
	}
	if this.Parent == -1 {
		b.WriteString("Parent post: <code>none</code>\n")
		no_changes = false
	} else if this.Parent != 0 {
		b.WriteString(fmt.Sprintf("Parent post: <a href=\"https://" + api.Endpoint + "/posts/%d\">Post #%d</a>\n", this.Parent, this.Parent))
		no_changes = false
	}
	if no_changes {
		b.WriteString("No changes so far.\n")
	}
	if len(this.Reason) != 0 {
		b.WriteString("Edit reason: <code>")
		b.WriteString(html.EscapeString(this.Reason))
		b.WriteString("</code>\n")
	}
}

func (this *EditPrompt) GenerateMessage(privacy_disabled bool) string {
	var b bytes.Buffer
	b.WriteString(this.Prefix)
	if b.Len() != 0 { b.WriteString("\n\n") }

	if this.State == SAVED {
		if (this.IsNoop()) {
			b.WriteString(fmt.Sprintf("Nothing done for <a href=\"https://" + api.Endpoint + "/posts/%d\">Post #%d</a>\n", this.PostId, this.PostId))
		} else {
			b.WriteString(fmt.Sprintf("Changes made to <a href=\"https://" + api.Endpoint + "/posts/%d\">Post #%d</a>\n", this.PostId, this.PostId))
			this.PostStatus(&b)
		}
	} else if this.State == DISCARDED {
		b.WriteString(fmt.Sprintf("Changes discarded for <a href=\"https://" + api.Endpoint + "/posts/%d\">Post #%d</a>\n", this.PostId, this.PostId))
	} else {
		b.WriteString(fmt.Sprintf("Now editing <a href=\"https://" + api.Endpoint + "/posts/%d\">Post #%d</a>\n", this.PostId, this.PostId))
		this.PostStatus(&b)
	}
	if !privacy_disabled && !(this.State == SAVED || this.State == DISCARDED) {
		b.WriteString("\nBe sure to <b>reply</b> to my messages! (<a href=\"https://core.telegram.org/bots#privacy-mode\">why?</a>)")
	}
	return b.String()
}

func sptr(x string) (*string) {return &x }

func (this *EditPrompt) GenerateMarkup() interface{} {
	// no buttons for a prompt which has already been finalized
	if this.State == DISCARDED || this.State == SAVED { return nil }

	var kb data.TInlineKeyboard
	kb.Buttons = make([][]data.TInlineKeyboardButton, 4)
	kb.Buttons[0] = append(kb.Buttons[0], data.TInlineKeyboardButton{Text: "Tags", Data: sptr("/tags")})
	kb.Buttons[0] = append(kb.Buttons[0], data.TInlineKeyboardButton{Text: "Rating", Data: sptr("/rating")})
	kb.Buttons[0] = append(kb.Buttons[0], data.TInlineKeyboardButton{Text: "Parent", Data: sptr("/parent")})
//	kb.Buttons[0] = append(kb.Buttons[0], data.TInlineKeyboardButton{Text: "File", Data: sptr("/file")})
	kb.Buttons[1] = append(kb.Buttons[1], data.TInlineKeyboardButton{Text: "Sources", Data: sptr("/sources")})
	kb.Buttons[1] = append(kb.Buttons[1], data.TInlineKeyboardButton{Text: "Description", Data: sptr("/description")})
	kb.Buttons[1] = append(kb.Buttons[1], data.TInlineKeyboardButton{Text: "Edit Reason", Data: sptr("/reason")})
	if this.State != WAIT_MODE {
		kb.Buttons[2] = append(kb.Buttons[2], data.TInlineKeyboardButton{Text: fmt.Sprintf("\u21A9\uFE0F Reset %s", GetNameOfState(this.State)), Data: sptr(fmt.Sprintf("/reset %s", this.State))})
	}
	kb.Buttons[2] = append(kb.Buttons[2], data.TInlineKeyboardButton{Text: fmt.Sprintf("\u2622\uFE0F Reset %s", GetNameOfState(WAIT_ALL)), Data: sptr(fmt.Sprintf("/reset %s", WAIT_ALL))})
	kb.Buttons[3] = append(kb.Buttons[3], data.TInlineKeyboardButton{Text: "\U0001F7E2 Save", Data: sptr("/save")})
	kb.Buttons[3] = append(kb.Buttons[3], data.TInlineKeyboardButton{Text: "\U0001F534 Discard", Data: sptr("/discard")})

	var extra_buttons [][]data.TInlineKeyboardButton
	if this.State == WAIT_RATING {
		extra_buttons = append(extra_buttons, nil)
		extra_buttons[0] = append(extra_buttons[0], data.TInlineKeyboardButton{Text: "\U0001F7E9 Safe", Data: sptr("/rating s")})
		extra_buttons[0] = append(extra_buttons[0], data.TInlineKeyboardButton{Text: "\U0001F7E8 Questionable", Data: sptr("/rating q")})
		extra_buttons[0] = append(extra_buttons[0], data.TInlineKeyboardButton{Text: "\U0001F7E5 Explicit", Data: sptr("/rating e")})
	} else if this.State == WAIT_SOURCE {
		for i, source := range this.SeenSourcesReverse {
			var selected bool
			if this.SourceChanges.TagStatus(source) == apitypes.AddsTag {
				selected = true
			} else if this.SourceChanges.TagStatus(source) == apitypes.RemovesTag {
				selected = false
			} else if _, ok := this.OrigSources[source]; ok {
				selected = true
			}

			prefixes := map[bool]string{true:"\U0001F7E9 ", false:"\U0001F7E5 "}
			extra_buttons = append(extra_buttons, append([]data.TInlineKeyboardButton(nil), data.TInlineKeyboardButton{Text: prefixes[selected] + source, Data: sptr(fmt.Sprintf("/sources %d %t", i, !selected))}))
		}
	} else if this.State == WAIT_PARENT {
		extra_buttons = append(extra_buttons, append([]data.TInlineKeyboardButton(nil), data.TInlineKeyboardButton{Text: "Delete parent", Data: sptr("/parent none")}))
	}
	kb.Buttons = append(extra_buttons, kb.Buttons...)
	return kb
}

func EditPromptID() data.DialogID {
	return EDIT_PROMPT_ID
}

func LoadEditPrompt(settings storage.UpdaterSettings, msg_id data.MsgID, chat_id data.ChatID) (*EditPrompt, error) {
	found, err := storage.FetchDialogPost(settings, msg_id, chat_id)
	if err != nil { return nil, err }
	if found == nil { return nil, nil }
	if found.DialogId != EditPromptID() { return nil, dialog.ErrDialogTypeMismatch }

	var ep EditPrompt
	err = json.Unmarshal(found.DialogData, &ep)
	ep.TelegramDialogPost.Load(found, EditPromptID(), &ep)
	return &ep, nil
}
