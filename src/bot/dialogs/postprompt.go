package dialogs

import (
	"api"
	"api/tags"
	"storage"

	"github.com/thewug/gogram"
	"github.com/thewug/gogram/data"
	"github.com/thewug/gogram/dialog"

	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"strings"
)

const POST_PROMPT_ID data.DialogID = "postprompt"

func PostPromptID() data.DialogID {
	return POST_PROMPT_ID
}

func LoadPostPrompt(settings storage.UpdaterSettings, msg_id data.MsgID, chat_id data.ChatID) (*PostPrompt, error) {
	found, err := storage.FetchDialogPost(settings, msg_id, chat_id)
	if err != nil { return nil, err }
	if found == nil { return nil, nil }
	if found.DialogId != PostPromptID() { return nil, dialog.ErrDialogTypeMismatch }

	var pp PostPrompt
	err = json.Unmarshal(found.DialogData, &pp)
	pp.TelegramDialogPost.Load(found, PostPromptID(), &pp)
	return &pp, nil
}

type PostPrompt struct {
	dialog.TelegramDialogPost `json:"-"`

	PostId int `json:"post_id"`
	Prefix string `json:"prefix"`
	State string `json:"state"`

	// stuff to generate the post info
	Tags tags.TagSet `json:"tags"`
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
		this.Tags.Reset()
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
	this.Prefix = "What would you like to edit? Pick a button from below."
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
		b.WriteString(api.RatingNameString(this.Rating))
		b.WriteString("</code>\n")
		no_changes = false
	}
	if this.Tags.Len() != 0 {
		b.WriteString("Tags: <code>")
		b.WriteString(html.EscapeString(this.Tags.String()))
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

func (this *PostPrompt) IsComplete() error {
	if len(this.Rating) == 0 {
		return errors.New("You must specify a rating!")
	} else if this.Tags.Len() < 6 {
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

	status, err := api.UploadFile(post_filedata, post_url, this.Tags.String(), this.Rating, this.Sources.StringWithDelimiter("\n"), this.Description, parent, user, api_key)
	if err != nil {
		ctx.Bot.ErrorLog.Println("Error updating post: ", err.Error())
		return nil, errors.New("An error occurred when editing the post! Double check your info, or try again later.")
	}

	return status, nil
}
