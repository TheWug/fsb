package dialogs

import (
	"api/tags"
	"storage"

	"github.com/thewug/gogram/data"
	"github.com/thewug/gogram/dialog"

	"encoding/json"
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

