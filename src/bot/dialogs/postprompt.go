package dialogs

import (
	"api/tags"
	"storage"

	"github.com/thewug/gogram/data"
	"github.com/thewug/gogram/dialog"

	"encoding/json"
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
