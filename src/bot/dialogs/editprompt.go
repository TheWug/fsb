package dialogs

import (
	"storage"

	"api"
	"api/tags"
	"api/types"
	"apiextra"

	"github.com/thewug/gogram"
	"github.com/thewug/gogram/data"
	"github.com/thewug/gogram/dialog"

	"bytes"
	"encoding/json"
	"database/sql"
	"errors"
	"fmt"
	"html"
	"net/url"
	"path"
	"strconv"
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
	SizeBytes int64 `json:"pf_size"`
}

func (this *PostFile) SetTelegramFile(id data.FileID, name string, size int64) {
	this.Mode = PF_FROM_TELEGRAM
	this.FileId = id
	this.FileName = name
	this.Url = ""
	this.SizeBytes = size
}

func filenameFromURL(u string) string {
	uo, err := url.Parse(u)

	if err != nil || uo == nil {
		return "[unknown]"
	}

	return path.Base(uo.Path)
}

func (this *PostFile) SetUrl(url string, size int64) {
	this.Mode = PF_FROM_URL
	this.Url = url
	this.FileId = ""
	this.FileName = filenameFromURL(url)
	this.SizeBytes = size
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
	Status string `json:"status"`
	State string `json:"state"`

	// stuff to generate the post info
	TagChanges tags.TagDiff `json:"tag_changes"`
	SourceChanges tags.StringDiff `json:"source_changes"` // not actually tags, but you can treat them the same.
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
	if state == WAIT_TAGS || state == WAIT_ALL {
		this.TagChanges.Clear()
	}

	if state == WAIT_SOURCE || state == WAIT_ALL {
		this.SourceChanges.Clear()
		this.SeenSources = nil
		this.SeenSourcesReverse = nil
		for s, _ := range this.OrigSources {
			this.SeeSource(s)
		}
	}

	if state == WAIT_RATING || state == WAIT_ALL {
		this.Rating = ""
	}

	if state == WAIT_DESC || state == WAIT_ALL {
		this.Description = ""
	}

	if state == WAIT_PARENT || state == WAIT_ALL {
		this.Parent = PARENT_RESET
	}

	if state == WAIT_REASON || state == WAIT_ALL {
		this.Reason = ""
	}

	if state == WAIT_FILE || state == WAIT_ALL {
		this.File.Clear()
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
		this.SourceChanges.Add(source)
	} else if live && !pick {
		this.SourceChanges.Remove(source)
	} else {
		this.SourceChanges.Reset(source)
	}
}

func (this *EditPrompt) SourceStringPrefixed(source string) {
	if strings.HasPrefix(source, "-") {
		this.SourceChanges.Remove(source[1:])
	} else if strings.HasPrefix(source, "=") {
		this.SourceChanges.Reset(source[1:])
	} else {
		if strings.HasPrefix(source, "+") {
			source = source[1:]
		}

		this.SeeSource(source)
		this.SourceChanges.Add(source)
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

func (this *EditPrompt) Prompt(tx *sql.Tx, bot *gogram.TelegramBot, ctx *gogram.MessageCtx, frmt EditFormatter) (*gogram.MessageCtx) {
	var send data.SendData
	send.Text = frmt.GenerateMessage(this)
	send.ParseMode = data.ParseHTML
	send.ReplyMarkup = frmt.GenerateMarkup(this)
	if this.TelegramDialogPost.IsUnset() {
		// no existing message, send a new one
		if ctx != nil {
			prompt, err := ctx.Reply(data.OMessage{SendData: send})
			if err != nil { bot.ErrorLog.Println("Error sending prompt: ", err.Error()) }
			err = this.FirstSave(tx, prompt.Msg.Id, prompt.Msg.Chat.Id, time.Unix(prompt.Msg.Date, 0), this)
			if err != nil { bot.ErrorLog.Println("Error sending prompt: ", err.Error()) }
			return prompt
		} else {
			panic("You must pass a context to reply to for the initial post!")
		}
	} else {
		// message already exists, update it
		prompt, err := this.Ctx(bot).EditText(data.OMessageEdit{SendData: send})
		if err != nil { bot.ErrorLog.Println("Error sending prompt: ", err.Error()) }
		this.Save(tx)
		return prompt
	}
}

func (this *EditPrompt) Finalize(tx *sql.Tx, bot *gogram.TelegramBot, ctx *gogram.MessageCtx, frmt EditFormatter) (*gogram.MessageCtx) {
	var send data.SendData
	send.Text = frmt.GenerateMessage(this)
	send.ParseMode = data.ParseHTML
	send.ReplyMarkup = nil
	
	var prompt *gogram.MessageCtx
	var err error
	if this.TelegramDialogPost.IsUnset() {
		prompt, err = ctx.Reply(data.OMessage{SendData: send, DisableWebPagePreview: true})
	} else {
		prompt, err = this.Ctx(bot).EditText(data.OMessageEdit{SendData: send, DisableWebPagePreview: true})
		this.Delete(tx)
	}

	if err != nil { bot.ErrorLog.Println("Error sending prompt: ", err.Error()) }

	return prompt
}

func (this *EditPrompt) ResetState() {
	this.State = WAIT_MODE
	this.Status = "What would you like to edit? Pick a button from below."
}

func (this *EditPrompt) IsNoop() bool {
	return len(this.Rating) == 0 &&
               this.TagChanges.IsZero() &&
               this.SourceChanges.IsZero() &&
               len(this.Description) == 0 &&
               this.Parent == 0
}

func (this *EditPrompt) IsComplete() error {
	if len(this.Rating) == 0 {
		return errors.New("You must specify a rating!")
	} else if len(this.TagChanges.AddList) < 6 {
		return errors.New("You must specify at least six tags!")
	} else if len(this.TagChanges.RemoveList) != 0 {
		return errors.New("You can't remove tags, this is a brand new post!")
	} else if this.File.Mode == PF_UNSET {
		return errors.New("You must specify a file!")
	}

	return nil
}

// i stole this
func byteCountIEC(b int64) string {
    const unit = 1024
    if b < unit {
        return fmt.Sprintf("%d B", b)
    }
    div, exp := int64(unit), 0
    for n := b / unit; n >= unit; n /= unit {
        div *= unit
        exp++
    }
    return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE??????????????????"[exp])
}

func (this *EditPrompt) PostStatus(b *bytes.Buffer) {
	no_changes := true
	if this.File.Mode == PF_FROM_TELEGRAM {
		b.WriteString("File: <i>")
		b.WriteString(html.EscapeString(this.File.FileName))
		b.WriteString("</i> (")
		b.WriteString(byteCountIEC(this.File.SizeBytes))
		b.WriteString(")\n")
	} else if this.File.Mode == PF_FROM_URL {
		b.WriteString("File: <a href=\"")
		b.WriteString(html.EscapeString(this.File.Url))
		b.WriteString("\">")
		b.WriteString(html.EscapeString(this.File.FileName))
		b.WriteString("</a> (")
		b.WriteString(byteCountIEC(this.File.SizeBytes))
		b.WriteString(")\n")
	}
	if len(this.Rating) != 0 {
		b.WriteString("Rating: <code>")
		b.WriteString(api.RatingNameString(this.Rating))
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
		b.WriteString(html.EscapeString(this.SourceChanges.StringWithDelimiter("\n  ")))
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

func (this *EditPrompt) CommitEdit(user, api_key string, ctx *gogram.MessageCtx, settings storage.UpdaterSettings) (*types.TPostInfo, error) {
	if this.IsNoop() {
		return nil, errors.New("This edit is a no-op.")
	}
	var rating *string
	var parent *int
	var description *string
	var reason *string

	if this.Rating != "" { rating = &this.Rating }
	if this.Parent != 0 { parent = &this.Parent }
	if this.Description != "" { description = &this.Description }
	if this.Reason != "" { reason = &this.Reason }

	update, err := api.UpdatePost(user, api_key, this.PostId, this.TagChanges, rating, parent, this.SourceChanges.Array(), description, reason)
	if err != nil {
		return nil, err
	}

	if update != nil {
		err_extra := storage.DefaultTransact(func(tx storage.DBLike) error { return storage.UpdatePost(tx, *update) })
		// don't overwrite original error since we're now past the point of no return
		if err_extra != nil {
			ctx.Bot.ErrorLog.Println("Error updating internal post: ", err_extra.Error())
		}
	}

	return update, err
}

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

func (this *EditPrompt) ParseArgs(ctx *gogram.MessageCtx) (bool, error) {
	doc := ctx.Msg.Document
	if doc == nil && ctx.Msg.ReplyToMessage != nil {
		doc = ctx.Msg.ReplyToMessage.Document
	}

	if doc != nil {
		name := doc.FileName
		if name == nil { name = new(string) }
		size := new(int64)
		if doc.FileSize != nil { *size = int64(*doc.FileSize) }
		this.File.SetTelegramFile(doc.Id, *name, *size)
	}

	// if we replied to a message, search it for a post id
	if ctx.Msg.ReplyToMessage != nil {
		this.PostId = apiextra.GetPostIDFromMessage(ctx.Msg.ReplyToMessage)
	}

	var err error
	var mode int
	var commitnow bool

	for _, token := range ctx.Cmd.Args {
		if mode != root {
			if mode == posttags {
				this.TagChanges.ApplyString(token)
			} else if mode == postsource {
				this.SourceChanges.ApplyArray(strings.Split(token, "\n"))
			} else if mode == postrating {
				this.Rating, err = api.SanitizeRatingForEdit(token)
				if err != nil {
					return false, errors.New("Please try again with a valid rating.")
				}
			} else if mode == postdescription {
				this.Description = token // at some point i should make it convert telegram markup to dtext but i can't do that right now
			} else if mode == postparent {
				this.Parent = apiextra.GetParentPostFromText(token)

				if this.Parent == apiextra.NONEXISTENT_PARENT {
					return false, errors.New("Please try again wth a valid parent post.")
				}
			} else if mode == postfileurl {
				this.File.SetUrl(token, 0)
			} else if mode == editreason {
				this.Reason = token
			}
			mode = root
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
		} else if token == "--url" {
			mode = postfileurl
		} else if token == "--commit" {
			commitnow = true
		} else {
			this.PostId = apiextra.GetPostIDFromText(token)
			if err != nil {
				return false, errors.New("Nonsense post ID, please specify a number.")
			}
		}
	}

	return commitnow, nil
}

func (this *EditPrompt) HandleCallback(ctx *gogram.CallbackCtx, settings storage.UpdaterSettings) {
	// TODO: this code mishandles settings, it should create its own subtransaction if settings is blank, but it won't.
	switch ctx.Cmd.Command {
	case "/reset":
		if len(ctx.Cmd.Args) != 1 { return }
		this.ApplyReset(ctx.Cmd.Args[0])
	case "/tags":
		this.Status = "Enter a list of tag changes, seperated by spaces. You can clear tags by prefixing them with a minus (-) and reset them by prefixing with an equals (=)."
		this.State = WAIT_TAGS
	case "/sources":
		if len(ctx.Cmd.Args) == 2 {
			index, err := strconv.Atoi(ctx.Cmd.Args[0])
			pick := ctx.Cmd.Args[1] == "true"
			if err != nil { return }
			this.SourceButton(index, pick)
		}
		this.Status = "Post some source changes, seperated by newlines. You can remove sources by prefixing them with a minus (-)."
		this.State = WAIT_SOURCE
	case "/rating":
		if len(ctx.Cmd.Args) == 1 {
			rating, err := api.SanitizeRatingForEdit(ctx.Cmd.Args[0])

			if err == nil {
				this.Rating = rating
			}
		}
		this.Status = "Post the new rating."
		this.State = WAIT_RATING
	case "/description":
		this.Status = `Post the new description. You can use <a href="https://" + api.Endpoint + "/help/dtext">dtext</a>.`
		this.State = WAIT_DESC
	case "/parent":
		if len(ctx.Cmd.Args) == 1 {
			parent := apiextra.GetParentPostFromText(ctx.Cmd.Args[0])
			if parent != apiextra.NONEXISTENT_PARENT {
				this.Parent = parent
			}
		}
		this.Status = `Post the new parent.`
		this.State = WAIT_PARENT
	case "/reason":
		this.Status = "Why are you editing this post? Post an edit reason, 250 characters max."
		this.State = WAIT_REASON
	case "/file":
		this.Status = `Upload a file.`
		this.State = WAIT_FILE
	case "/save":
		this.Status = ""
		this.State = SAVED
	case "/discard":
		ctx.AnswerAsync(data.OCallback{Notification: "\U0001F534 Edit discarded."}, nil) // finalize dialog post and discard edit
		this.Status = ""
		this.State = DISCARDED
		ctx.SetState(nil)
	default:
	}
}

func (this *EditPrompt) HandleFreeform(ctx *gogram.MessageCtx) {
	if this.State == WAIT_TAGS {
		this.TagChanges.ApplyString(ctx.Msg.PlainText())
		this.Status = "Got it. Continue sending more tag changes, and pick a button from below when you're done."
	} else if this.State == WAIT_SOURCE {
		for _, source := range strings.Split(ctx.Msg.PlainText(), "\n") {
			this.SourceStringPrefixed(source)
		}
		this.Status = "Got it. Continue sending more source changes, and pick a button from below when you're done."
	} else if this.State == WAIT_RATING {
		rating, err := api.SanitizeRatingForEdit(ctx.Msg.PlainText())

		if err != nil {
			this.Status = "Please enter a <i>valid</i> rating. (Pick from <code>explicit</code>, <code>questionable</code>, <code>safe</code>, or <code>original</code>.)"
		} else {
			this.Rating = rating
			this.ResetState()
		}
	} else if this.State == WAIT_DESC {
		this.Description = ctx.Msg.PlainText() // TODO: convert telegram markup to dtext
		this.ResetState()
	} else if this.State == WAIT_PARENT {
		parent := apiextra.GetParentPostFromText(ctx.Msg.PlainText())

		if parent == apiextra.NONEXISTENT_PARENT {
			this.Status = "Please enter a <i>valid</i> parent post. (You can either send a link to an " + api.ApiName + " post, a bare numeric ID, 'none' for no parent, or 'original' to not attempt to update the parent at all.)"
		} else {
			this.Parent = parent
			this.ResetState()
		}
	} else if this.State == WAIT_REASON {
		this.Reason = ctx.Msg.PlainText()
		this.ResetState()
	} else if this.State == WAIT_FILE {
		done := false
		_, err := url.Parse(ctx.Msg.PlainText())
		if err == nil { // it IS a URL
			this.File.SetUrl(ctx.Msg.PlainText(), 0)
			done = true
		}

		doc := ctx.Msg.Document
		if doc == nil && ctx.Msg.ReplyToMessage != nil {
			doc = ctx.Msg.ReplyToMessage.Document
		}

		if doc != nil {
			name := doc.FileName
			if name == nil { name = new(string) }
			size := new(int64)
			if doc.FileSize != nil { *size = int64(*doc.FileSize) }
			this.File.SetTelegramFile(doc.Id, *name, *size)
			done = true
		}

		if done {
			this.ResetState()
		} else {
			this.Status = "Please send a new file. You can upload a new one, reply to or forward an existing one, or send a URL to upload from. (Only certain whitelisted domains can be used for URL uploads, see <a href=\"https://" + api.Endpoint + "/upload_whitelists\">" + api.ApiName + "'s upload whitelist</a>.)"
		}
	} else {
		return
	}
}

func EditPromptID() data.DialogID {
	return EDIT_PROMPT_ID
}

func LoadEditPrompt(tx *sql.Tx, msg_id data.MsgID, chat_id data.ChatID) (*EditPrompt, error) {
	found, err := storage.FetchDialogPost(tx, msg_id, chat_id)
	if err != nil { return nil, err }
	if found == nil { return nil, nil }
	if found.DialogId != EditPromptID() { return nil, dialog.ErrDialogTypeMismatch }

	var ep EditPrompt
	err = json.Unmarshal(found.DialogData, &ep)
	ep.TelegramDialogPost.Load(found, EditPromptID(), &ep)
	return &ep, nil
}
