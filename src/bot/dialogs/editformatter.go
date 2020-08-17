package dialogs

import (
	"api"
	"api/tags"

	"github.com/thewug/gogram/data"

	"bytes"
	"fmt"
	"html"
)

type EditFormatterBase struct {
	private_mode bool
}

type EditFormatter struct {
	EditFormatterBase

	Error error
}

type PostFormatter struct {
	EditFormatterBase

	Result *api.UploadCallResult
}

func (this *EditFormatterBase) Warnings(b *bytes.Buffer, prompt *EditPrompt) {
	any := false

	any = true
	b.WriteString("<b>WARNING! editing is experimental right now.</b>\nDouble check your edits after committing to make sure you're not accidentally scrambling posts.\n")

	var tagset tags.TagSet
	for osource, _ := range prompt.OrigSources {
		tagset.Set(osource)
	}
	tagset.ApplyDiff(tags.TagDiff{StringDiff: prompt.SourceChanges})
	if tagset.Len() > 10 {
		b.WriteString("Too many sources, each post can only have 10! Remove some before committing.\n")
		any = true
	}

	for tag, _ := range tagset.Data {
		if len(tag) > 1024 {
			b.WriteString("One of the sources is too long, each source can only be 1024 characters long. Shorten them before committing.\n")
			any = true
			break
		}
	}

	if any {
		b.WriteString("\n")
	}
}

func NewEditFormatter(privacy_mode bool, err error) EditFormatter {
	return EditFormatter{EditFormatterBase{privacy_mode}, err}
}

func NewPostFormatter(privacy_mode bool, result *api.UploadCallResult) PostFormatter {
	return PostFormatter{EditFormatterBase{privacy_mode}, result}
}

func (this EditFormatter) GenerateMessage(prompt *EditPrompt) string {
	var b bytes.Buffer
	b.WriteString(prompt.Prefix)
	if b.Len() != 0 { b.WriteString("\n\n") }

	this.Warnings(&b, prompt)

	if prompt.State == SAVED {
		if (prompt.IsNoop()) {
			prompt.State = WAIT_MODE
			b.WriteString(fmt.Sprintf("<b>Nothing to do for <a href=\"https://" + api.Endpoint + "/posts/%d\">Post #%d</a></b>\n", prompt.PostId, prompt.PostId))
		} else {
			if this.Error == nil {
				b.WriteString(fmt.Sprintf("Changes made to <a href=\"https://" + api.Endpoint + "/posts/%d\">Post #%d</a>\n", prompt.PostId, prompt.PostId))
			} else {
				prompt.State = WAIT_MODE
				b.WriteString(fmt.Sprintf("<b>There was an error updating <a href=\"https://" + api.Endpoint + "/posts/%d\">Post #%d</a>.</b>\nYou can continue to edit and then try again, or discard it.\n", prompt.PostId, prompt.PostId))
			}
		}
		prompt.PostStatus(&b)
	} else if prompt.State == DISCARDED {
		b.WriteString(fmt.Sprintf("Changes discarded for <a href=\"https://" + api.Endpoint + "/posts/%d\">Post #%d</a>\n", prompt.PostId, prompt.PostId))
	} else {
		b.WriteString(fmt.Sprintf("Now editing <a href=\"https://" + api.Endpoint + "/posts/%d\">Post #%d</a>\n", prompt.PostId, prompt.PostId))
		prompt.PostStatus(&b)
	}
	if this.private_mode && !(prompt.State == SAVED || prompt.State == DISCARDED) {
		b.WriteString("\nBe sure to <b>reply</b> to my messages! (<a href=\"https://core.telegram.org/bots#privacy-mode\">why?</a>)")
	}
	return b.String()
}

func (this EditFormatter) GenerateMarkup(prompt *EditPrompt) interface{} {
	sptr := func(x string) (*string) {return &x }
	// no buttons for a prompt which has already been finalized
	if prompt.State == DISCARDED || prompt.State == SAVED { return nil }

	var kb data.TInlineKeyboard
	kb.Buttons = make([][]data.TInlineKeyboardButton, 4)
	kb.Buttons[0] = append(kb.Buttons[0], data.TInlineKeyboardButton{Text: "Tags", Data: sptr("/tags")})
	kb.Buttons[0] = append(kb.Buttons[0], data.TInlineKeyboardButton{Text: "Rating", Data: sptr("/rating")})
	kb.Buttons[0] = append(kb.Buttons[0], data.TInlineKeyboardButton{Text: "Parent", Data: sptr("/parent")})
	kb.Buttons[1] = append(kb.Buttons[1], data.TInlineKeyboardButton{Text: "Sources", Data: sptr("/sources")})
	kb.Buttons[1] = append(kb.Buttons[1], data.TInlineKeyboardButton{Text: "Description", Data: sptr("/description")})
	kb.Buttons[1] = append(kb.Buttons[1], data.TInlineKeyboardButton{Text: "Edit Reason", Data: sptr("/reason")})
	if prompt.State != WAIT_MODE {
		kb.Buttons[2] = append(kb.Buttons[2], data.TInlineKeyboardButton{Text: fmt.Sprintf("\u21A9\uFE0F Reset %s", GetNameOfState(prompt.State)), Data: sptr(fmt.Sprintf("/reset %s", prompt.State))})
	}
	kb.Buttons[2] = append(kb.Buttons[2], data.TInlineKeyboardButton{Text: fmt.Sprintf("\u2622\uFE0F Reset %s", GetNameOfState(WAIT_ALL)), Data: sptr(fmt.Sprintf("/reset %s", WAIT_ALL))})
	kb.Buttons[3] = append(kb.Buttons[3], data.TInlineKeyboardButton{Text: "\U0001F7E2 Save", Data: sptr("/save")})
	kb.Buttons[3] = append(kb.Buttons[3], data.TInlineKeyboardButton{Text: "\U0001F534 Discard", Data: sptr("/discard")})

	var extra_buttons [][]data.TInlineKeyboardButton
	if prompt.State == WAIT_RATING {
		extra_buttons = append(extra_buttons, nil)
		extra_buttons[0] = append(extra_buttons[0], data.TInlineKeyboardButton{Text: "\U0001F7E9 Safe", Data: sptr("/rating s")})
		extra_buttons[0] = append(extra_buttons[0], data.TInlineKeyboardButton{Text: "\U0001F7E8 Questionable", Data: sptr("/rating q")})
		extra_buttons[0] = append(extra_buttons[0], data.TInlineKeyboardButton{Text: "\U0001F7E5 Explicit", Data: sptr("/rating e")})
	} else if prompt.State == WAIT_SOURCE {
		for i, source := range prompt.SeenSourcesReverse {
			var selected bool
			if prompt.SourceChanges.Status(source) == tags.AddsTag {
				selected = true
			} else if prompt.SourceChanges.Status(source) == tags.RemovesTag {
				selected = false
			} else if _, ok := prompt.OrigSources[source]; ok {
				selected = true
			}

			prefixes := map[bool]string{true:"\U0001F7E9 ", false:"\U0001F7E5 "}
			extra_buttons = append(extra_buttons, append([]data.TInlineKeyboardButton(nil), data.TInlineKeyboardButton{Text: prefixes[selected] + source, Data: sptr(fmt.Sprintf("/sources %d %t", i, !selected))}))
		}
	} else if prompt.State == WAIT_PARENT {
		extra_buttons = append(extra_buttons, append([]data.TInlineKeyboardButton(nil), data.TInlineKeyboardButton{Text: "Delete parent", Data: sptr("/parent none")}))
	}
	kb.Buttons = append(extra_buttons, kb.Buttons...)
	return kb
}

func (this PostFormatter) GenerateMessage(prompt *EditPrompt) string {
	var b bytes.Buffer
	b.WriteString(prompt.Prefix)
	if b.Len() != 0 { b.WriteString("\n\n") }

	this.Warnings(&b, prompt)

	if prompt.State == SAVED {
		if this.Result == nil {
			prompt.State = WAIT_MODE
			b.WriteString("<b>Failed to post file, no explanation available. Sorry about that.</b>\n")
			prompt.PostStatus(&b)
		} else {
			if this.Result.Success {
				b.WriteString("You have successfully uploaded <i>")
				b.WriteString(html.EscapeString(prompt.File.FileName))
				b.WriteString("</i>")
				if this.Result != nil && this.Result.Location != nil {
					b.WriteString(", <a href=\"")
					b.WriteString(*this.Result.Location)
					b.WriteString("\">click here to open it</a>")
				}
				b.WriteString(".\n")
				prompt.PostStatus(&b)
			} else {
				prompt.State = WAIT_MODE
				b.WriteString("<b>Error uploading <i>")
				b.WriteString(html.EscapeString(prompt.File.FileName))
				b.WriteString("</i>: ")
				if this.Result != nil && this.Result.Reason != nil {
					b.WriteString(html.EscapeString(*this.Result.Reason))
					b.WriteString(" (")
					b.WriteString(html.EscapeString(this.Result.Status))
					b.WriteString(")")
				} else {
					b.WriteString("No explanation available.")
				}
				b.WriteString("</b>\nYou can continue to edit your upload and try again, or you can discard it.\n")
				prompt.PostStatus(&b)
			}
		}
	} else if prompt.State == DISCARDED {
		b.WriteString("New post discarded.")
	} else {
		b.WriteString("Now creating and editing a new post.\n")
		prompt.PostStatus(&b)
	}
	if this.private_mode && !(prompt.State == SAVED || prompt.State == DISCARDED) {
		b.WriteString("\nBe sure to <b>reply</b> to my messages! (<a href=\"https://core.telegram.org/bots#privacy-mode\">why?</a>)")
	}
	return b.String()
}

func (this PostFormatter) GenerateMarkup(prompt *EditPrompt) interface{} {
	sptr := func(x string) (*string) {return &x }
	// no buttons for a prompt which has already been finalized
	if prompt.State == DISCARDED || prompt.State == SAVED { return nil }

	var kb data.TInlineKeyboard
	kb.Buttons = make([][]data.TInlineKeyboardButton, 4)
	kb.Buttons[0] = append(kb.Buttons[0], data.TInlineKeyboardButton{Text: "Tags", Data: sptr("/tags")})
	kb.Buttons[0] = append(kb.Buttons[0], data.TInlineKeyboardButton{Text: "Rating", Data: sptr("/rating")})
	kb.Buttons[0] = append(kb.Buttons[0], data.TInlineKeyboardButton{Text: "Parent", Data: sptr("/parent")})
	kb.Buttons[1] = append(kb.Buttons[1], data.TInlineKeyboardButton{Text: "File", Data: sptr("/file")})
	kb.Buttons[1] = append(kb.Buttons[1], data.TInlineKeyboardButton{Text: "Sources", Data: sptr("/sources")})
	kb.Buttons[1] = append(kb.Buttons[1], data.TInlineKeyboardButton{Text: "Description", Data: sptr("/description")})
	if prompt.State != WAIT_MODE {
		kb.Buttons[2] = append(kb.Buttons[2], data.TInlineKeyboardButton{Text: fmt.Sprintf("\u21A9\uFE0F Reset %s", GetNameOfState(prompt.State)), Data: sptr(fmt.Sprintf("/reset %s", prompt.State))})
	}
	kb.Buttons[2] = append(kb.Buttons[2], data.TInlineKeyboardButton{Text: fmt.Sprintf("\u2622\uFE0F Reset %s", GetNameOfState(WAIT_ALL)), Data: sptr(fmt.Sprintf("/reset %s", WAIT_ALL))})
	kb.Buttons[3] = append(kb.Buttons[3], data.TInlineKeyboardButton{Text: "\U0001F7E2 Upload", Data: sptr("/save")})
	kb.Buttons[3] = append(kb.Buttons[3], data.TInlineKeyboardButton{Text: "\U0001F534 Discard", Data: sptr("/discard")})

	var extra_buttons [][]data.TInlineKeyboardButton
	if prompt.State == WAIT_RATING {
		extra_buttons = append(extra_buttons, nil)
		extra_buttons[0] = append(extra_buttons[0], data.TInlineKeyboardButton{Text: "\U0001F7E9 Safe", Data: sptr("/rating s")})
		extra_buttons[0] = append(extra_buttons[0], data.TInlineKeyboardButton{Text: "\U0001F7E8 Questionable", Data: sptr("/rating q")})
		extra_buttons[0] = append(extra_buttons[0], data.TInlineKeyboardButton{Text: "\U0001F7E5 Explicit", Data: sptr("/rating e")})
	} else if prompt.State == WAIT_SOURCE {
		for i, source := range prompt.SeenSourcesReverse {
			var selected bool
			if prompt.SourceChanges.Status(source) == tags.AddsTag {
				selected = true
			} else if prompt.SourceChanges.Status(source) == tags.RemovesTag {
				selected = false
			}

			prefixes := map[bool]string{true:"\U0001F7E9 ", false:"\U0001F7E5 "}
			extra_buttons = append(extra_buttons, append([]data.TInlineKeyboardButton(nil), data.TInlineKeyboardButton{Text: prefixes[selected] + source, Data: sptr(fmt.Sprintf("/sources %d %t", i, !selected))}))
		}
	} else if prompt.State == WAIT_PARENT {
		extra_buttons = append(extra_buttons, append([]data.TInlineKeyboardButton(nil), data.TInlineKeyboardButton{Text: "Delete parent", Data: sptr("/parent none")}))
	}
	kb.Buttons = append(extra_buttons, kb.Buttons...)
	return kb
}
