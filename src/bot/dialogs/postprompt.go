package dialogs

import (
	"api"
	"api/tags"
	"api/tags/wizard"
	"storage"
	"apiextra"

	"github.com/thewug/gogram"
	"github.com/thewug/gogram/data"
	"github.com/thewug/gogram/dialog"

	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const POST_PROMPT_ID data.DialogID = "postprompt"

func PostPromptID() data.DialogID {
	return POST_PROMPT_ID
}

func LoadPostPrompt(settings storage.UpdaterSettings, msg_id data.MsgID, chat_id data.ChatID, user_id data.UserID, api_user string) (*PostPrompt, error) {
	found, err := storage.FetchDialogPost(settings, msg_id, chat_id)
	if err != nil { return nil, err }
	if found == nil { return nil, nil }
	if found.DialogId != PostPromptID() { return nil, dialog.ErrDialogTypeMismatch }

	tagrules, err := storage.GetUserTagRules(settings, user_id, "upload")
	if err != nil { return nil, err }

	var pp PostPrompt
	pp.TagWizard.SetNewRulesFromString(tagrules)
	err = json.Unmarshal(found.DialogData, &pp)
	pp.TelegramDialogPost.Load(found, PostPromptID(), &pp)
	return &pp, nil
}

type PostPrompt struct {
	dialog.TelegramDialogPost `json:"-"`

	PostId int `json:"post_id"`
	Status string `json:"status"`
	State string `json:"state"`

	// stuff to generate the post info
	TagWizard wizard.TagWizard `json:"tagwiz"`
	Sources tags.StringSet `json:"sources"` // not actually tags, but you can treat them the same.
	SeenSources map[string]int `json:"source_seen"`
	SeenSourcesReverse []string `json:"source_seen_rev"`
	Parent int `json:"parent"`
	Rating string `json:"rating"`
	Description string `json:"description"`
	File PostFile `json:"file"`
}

func (this *PostPrompt) JSON() (string, error) {
	bytes, err := json.Marshal(this)
	return string(bytes), err
}

func (this *PostPrompt) ID() data.DialogID {
	return PostPromptID()
}

func (this *PostPrompt) ApplyReset(state string) {
	if state == WAIT_TAGS || state == WAIT_ALL {
		this.TagWizard.Reset()
		this.Status = this.TagWizard.Prompt()
	}

	if state == WAIT_SOURCE || state == WAIT_ALL {
		this.Sources.Reset()
		this.SeenSources = nil
		this.SeenSourcesReverse = nil
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

	if state == WAIT_FILE || state == WAIT_ALL {
		this.File.Clear()
	}
}

func (this *PostPrompt) SourceButton(n int, pick bool) {
	// buttons will never not have been seen before so just ignore (but handle gracefully) buttons which are for invalid indexes, as they shouldn't happen
	if n < 0 || n >= len(this.SeenSourcesReverse) { return }
	this.SourceStringLiteral(this.SeenSourcesReverse[n], pick)
}

func (this *PostPrompt) SourceStringLiteral(source string, pick bool) {
	if pick { // make sure we've seen it if we're adding it
		this.SeeSource(source)
	}

	if pick {
		this.Sources.Set(source)
	} else if !pick {
		this.Sources.Clear(source)
	}
}

func (this *PostPrompt) SourceStringPrefixed(source string) {
	if strings.HasPrefix(source, "-") {
		this.Sources.Clear(source[1:])
	} else {
		if strings.HasPrefix(source, "+") {
			source = source[1:]
		}

		this.SeeSource(source)
		this.Sources.Set(source)
	}
}

func (this *PostPrompt) SeeSource(source string) {
	if _, ok := this.SeenSources[source]; ok { return } // already seen
	if this.SeenSources == nil { this.SeenSources = make(map[string]int) }
	index := len(this.SeenSources)
	this.SeenSources[source] = index
	this.SeenSourcesReverse = append(this.SeenSourcesReverse, source)
}

func (this *PostPrompt) ResetState() {
	this.State = WAIT_MODE
	this.Status = "What would you like to edit? Pick a button from below."
}

func (this *PostPrompt) PostStatus(b *bytes.Buffer) {
	no_changes := true
	if this.File.Mode == PF_FROM_TELEGRAM {
		b.WriteString("File: <i>")
		b.WriteString(html.EscapeString(this.File.FileName))
		b.WriteString("</i> (")
		b.WriteString(byteCountIEC(this.File.SizeBytes))
		b.WriteString(")\n")
	} else if this.File.Mode == PF_FROM_URL {
		b.WriteString("File: <a href=\"")
		b.WriteString(this.File.Url)
		b.WriteString("\">")
		b.WriteString(html.EscapeString(this.File.FileName))
		b.WriteString("</a>\n")
	}
	if len(this.Rating) != 0 {
		b.WriteString("Rating: <code>")
		b.WriteString(api.RatingNameString(this.TestRating()))
		b.WriteString("</code>\n")
		no_changes = false
	}
	if this.TagWizard.Len() != 0 {
		b.WriteString("Tags: <code>")
		b.WriteString(html.EscapeString(this.TagWizard.Tags().String()))
		b.WriteString("</code>\n")
		no_changes = false
	}
	if this.Sources.Len() != 0 {
		b.WriteString("Sources:\n<pre>  ")
		b.WriteString(html.EscapeString(this.Sources.StringWithDelimiter("\n  ")))
		b.WriteString("</pre>\n")
		no_changes = false
	}
	if len(this.Description) != 0 {
		b.WriteString("Description:\n<pre>")
		b.WriteString(html.EscapeString(this.Description))
		b.WriteString("</pre>\n")
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
}

func (this *PostPrompt) TestRating() string {
	if this.Rating == "" {
		return this.TagWizard.Rating()
	}

	return this.Rating
}

func (this *PostPrompt) IsComplete() error {
	if len(this.TestRating()) == 0 {
		return errors.New("You must specify a rating!")
	} else if this.TagWizard.Len() < 6 {
		return errors.New("You must specify at least six tags!")
	} else if this.File.Mode == PF_UNSET {
		return errors.New("You must specify a file!")
	}

	return nil
}

func (this *PostPrompt) CommitPost(user, api_key string, ctx *gogram.MessageCtx, settings storage.UpdaterSettings) (*api.UploadCallResult, error) {
	err := this.IsComplete()
	if err != nil {
		return nil, err
	}

	var post_url string
	var post_filedata io.ReadCloser
	var parent *int

	if this.Parent != 0 { parent = &this.Parent }

	if this.File.Mode == PF_FROM_URL {
		post_url = this.File.Url
	} else {
		file, err := ctx.Bot.Remote.GetFile(data.OGetFile{Id: this.File.FileId})
		if err != nil || file == nil || file.FilePath == nil {
			return nil, errors.New("Error while fetching file, try sending it again?")
		}
		post_filedata, err = ctx.Bot.Remote.DownloadFile(data.OFile{FilePath: *file.FilePath})
		if err != nil || post_filedata == nil {
			return nil, errors.New("Error while downloading file, try sending it again?")
		}
	}

	status, err := api.UploadFile(post_filedata, post_url, this.TagWizard.Tags(), this.TestRating(), this.Sources.StringWithDelimiter("\n"), this.Description, parent, user, api_key)
	if err != nil {
		ctx.Bot.ErrorLog.Println("Error updating post: ", err.Error())
		return nil, errors.New("An error occurred when editing the post! Double check your info, or try again later.")
	}

	return status, nil
}

func (this *PostPrompt) Prompt(settings storage.UpdaterSettings, bot *gogram.TelegramBot, ctx *gogram.MessageCtx, frmt PostFormatter) (*gogram.MessageCtx) {
	var send data.SendData
	send.Text = frmt.GenerateMessage(this)
	send.ParseMode = data.ParseHTML
	send.ReplyMarkup = frmt.GenerateMarkup(this)
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

func (this *PostPrompt) Finalize(settings storage.UpdaterSettings, bot *gogram.TelegramBot, ctx *gogram.MessageCtx, frmt PostFormatter) (*gogram.MessageCtx) {
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
		this.Delete(settings)
	}

	if err != nil { bot.ErrorLog.Println("Error sending prompt: ", err.Error()) }

	return prompt
}

func (this *PostPrompt) ParseArgs(ctx *gogram.MessageCtx) (bool, error) {
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
				this.TagWizard.MergeTagsFromString(token)
			} else if mode == postsource {
				this.Sources.ApplyArray(strings.Split(token, "\n"))
			} else if mode == postrating {
				this.Rating, err = api.SanitizeRating(token)
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
		} else if token == "--url" {
			mode = postfileurl
		} else if token == "--commit" {
			commitnow = true
		} else {
			this.PostId = apiextra.GetPostIDFromText(token)
			if err != nil {
				return false, errors.New("Please try again with a valid post id or md5.")
			}
		}
	}

	return commitnow, nil
}

func (this *PostPrompt) HandleCallback(ctx *gogram.CallbackCtx, settings storage.UpdaterSettings) {
	// TODO: this code mishandles settings, it should create its own subtransaction if settings is blank, but it won't.
	switch ctx.Cmd.Command {
	case "/reset":
		if len(ctx.Cmd.Args) != 1 { return }
		this.ApplyReset(ctx.Cmd.Args[0])
	case "/tags":
		this.Status = this.TagWizard.Prompt()
		this.State = WAIT_TAGS
	case "/sources":
		if len(ctx.Cmd.Args) == 2 {
			index, err := strconv.Atoi(ctx.Cmd.Args[0])
			pick := ctx.Cmd.Args[1] == "true"
			if err != nil { return }
			this.SourceButton(index, pick)
		}
		this.Status = "Post some sources, seperated by newlines. You can remove sources by prefixing them with a minus (-)."
		this.State = WAIT_SOURCE
	case "/rating":
		if len(ctx.Cmd.Args) == 1 {
			rating, err := api.SanitizeRating(ctx.Cmd.Args[0])

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
	case wizard.CMD_NEXT, wizard.CMD_RESTART, wizard.CMD_DONE, wizard.CMD_TAGS:
		this.TagWizard.ButtonPressed(ctx.Cb.Data)
		this.Status = this.TagWizard.Prompt()
	default:
	}
}

func (this *PostPrompt) HandleFreeform(ctx *gogram.MessageCtx) {
	if this.State == WAIT_TAGS {
		this.TagWizard.MergeTagsFromString(ctx.Msg.PlainText())
		this.Status = "Got it. Continue sending more tag changes, and pick a button from below when you're done."
	} else if this.State == WAIT_SOURCE {
		for _, source := range strings.Split(ctx.Msg.PlainText(), "\n") {
			this.SourceStringPrefixed(source)
		}
		this.Status = "Got it. Continue sending more source changes, and pick a button from below when you're done."
	} else if this.State == WAIT_RATING {
		rating, err := api.SanitizeRating(ctx.Msg.PlainText())

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

