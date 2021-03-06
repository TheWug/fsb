package dialogs

import (
	"github.com/thewug/fsb/pkg/api"
	"github.com/thewug/fsb/pkg/api/tags"

	"github.com/thewug/gogram/data"

	"bytes"
	"fmt"
	"html"
	"strings"
)

type EditFormatterBase struct {
	request_reply bool
}

type EditFormatter struct {
	EditFormatterBase

	Error error
}

type PostFormatter struct {
	EditFormatterBase

	Result *api.UploadCallResult
}

func (this EditFormatterBase) WarningsBase(b *bytes.Buffer, warnings []string) {
	if len(warnings) == 0 { return }

	warning_emoji := "\u26A0" // ⚠️
	for _, w := range warnings {
		b.WriteString(warning_emoji)
		b.WriteRune(' ')
		b.WriteString(w)
		b.WriteRune('\n')
	}

	b.WriteRune('\n')
}

func NewEditFormatter(request_reply bool, err error) EditFormatter {
	return EditFormatter{EditFormatterBase{request_reply}, err}
}

func NewPostFormatter(request_reply bool, result *api.UploadCallResult) PostFormatter {
	return PostFormatter{EditFormatterBase{request_reply}, result}
}

func (this EditFormatter) Warnings(b *bytes.Buffer, prompt *EditPrompt) {
	var warnings []string
	warnings = append(warnings, "<b>WARNING! editing is experimental right now.</b>\nDouble check your edits after committing to make sure you're not accidentally scrambling posts.")

	var set tags.StringSet
	for osource, _ := range prompt.OrigSources {
		set.Set(osource)
	}
	set.ApplyDiff(prompt.SourceChanges)
	if set.Len() > 10 {
		warnings = append(warnings, "Too many sources, each post can only have 10! Remove some before committing.")
	}

	for tag, _ := range set.Data {
		if len(tag) > 1024 {
			warnings = append(warnings, "One of the sources is too long, each source can only be 1024 characters long. Shorten them before committing.")
			break
		}
	}

	if this.request_reply && !(prompt.State == SAVED || prompt.State == DISCARDED) {
		warnings = append(warnings, "Be sure to <b>reply</b> to my messages! (<a href=\"https://core.telegram.org/bots#privacy-mode\">why?</a>)")
	}

	this.WarningsBase(b, warnings)
}

func (this EditFormatter) GenerateMessage(prompt *EditPrompt) string {
	var b bytes.Buffer

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
		b.WriteRune('\n')

		this.Warnings(&b, prompt)
	} else if prompt.State == DISCARDED {
		b.WriteString(fmt.Sprintf("Changes discarded for <a href=\"https://" + api.Endpoint + "/posts/%d\">Post #%d</a>\n", prompt.PostId, prompt.PostId))
	} else {
		b.WriteString(fmt.Sprintf("Now editing <a href=\"https://" + api.Endpoint + "/posts/%d\">Post #%d</a>\nCurrently editing: <code>", prompt.PostId, prompt.PostId))
		b.WriteString(GetNameOfState(prompt.State))
		b.WriteString("</code>\n\n")

		prompt_string := strings.TrimSpace(prompt.Status)
		b.WriteString(prompt_string)
		if len(prompt_string) > 0 {
			b.WriteString("\n\n")
		}

		prompt.PostStatus(&b)
		b.WriteRune('\n')

		this.Warnings(&b, prompt)
	}
	return b.String()
}

func (this EditFormatter) GenerateMarkup(prompt *EditPrompt) interface{} {
	sptr := func(x string) (*string) {return &x }
	// no buttons for a prompt which has already been finalized
	if prompt.State == DISCARDED || prompt.State == SAVED { return nil }

	var kb data.TInlineKeyboard
	kb.AddRow()
	kb.AddButton(data.TInlineKeyboardButton{Text: "Tags", Data: sptr("/tags")})
	kb.AddButton(data.TInlineKeyboardButton{Text: "Rating", Data: sptr("/rating")})
	kb.AddButton(data.TInlineKeyboardButton{Text: "Parent", Data: sptr("/parent")})
	kb.AddRow()
	kb.AddButton(data.TInlineKeyboardButton{Text: "Sources", Data: sptr("/sources")})
	kb.AddButton(data.TInlineKeyboardButton{Text: "Description", Data: sptr("/description")})
	kb.AddButton(data.TInlineKeyboardButton{Text: "Edit Reason", Data: sptr("/reason")})
	kb.AddRow()
	if prompt.State != WAIT_MODE {
		kb.AddButton(data.TInlineKeyboardButton{Text: fmt.Sprintf("\u21A9\uFE0F Reset %s", GetNameOfState(prompt.State)), Data: sptr(fmt.Sprintf("/reset %s", prompt.State))})
	}
	kb.AddButton(data.TInlineKeyboardButton{Text: fmt.Sprintf("\u2622\uFE0F Reset %s", GetNameOfState(WAIT_ALL)), Data: sptr(fmt.Sprintf("/reset %s", WAIT_ALL))})
	kb.AddRow()
	kb.AddButton(data.TInlineKeyboardButton{Text: "\U0001F7E2 Save", Data: sptr("/save")})
	kb.AddButton(data.TInlineKeyboardButton{Text: "\U0001F534 Discard", Data: sptr("/discard")})

	var extra_buttons data.TInlineKeyboard
	if prompt.State == WAIT_RATING {
		extra_buttons.AddRow()
		extra_buttons.AddButton(data.TInlineKeyboardButton{Text: "\U0001F7E9 Safe", Data: sptr("/rating s")})
		extra_buttons.AddButton(data.TInlineKeyboardButton{Text: "\U0001F7E8 Questionable", Data: sptr("/rating q")})
		extra_buttons.AddButton(data.TInlineKeyboardButton{Text: "\U0001F7E5 Explicit", Data: sptr("/rating e")})
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
			extra_buttons.AddRow()
			extra_buttons.AddButton(data.TInlineKeyboardButton{Text: prefixes[selected] + source, Data: sptr(fmt.Sprintf("/sources %d %t", i, !selected))})
		}
	} else if prompt.State == WAIT_PARENT {
		extra_buttons.AddRow()
		extra_buttons.AddButton(data.TInlineKeyboardButton{Text: "Delete parent", Data: sptr("/parent none")})
	}
	kb.Buttons = append(extra_buttons.Buttons, kb.Buttons...)
	return kb
}

func (this PostFormatter) Warnings(b *bytes.Buffer, prompt *PostPrompt) {
	var warnings []string
	warnings = append(warnings, "<b>WARNING! uploading is experimental right now.</b>\nDouble check your post after committing to make sure it's complete and not missing anything.")

	if prompt.Sources.Len() > 10 {
		warnings = append(warnings, "Too many sources, each post can only have 10! Remove some before committing.")
	}

	for tag, _ := range prompt.Sources.Data {
		if len(tag) > 1024 {
			warnings = append(warnings, "One of the sources is too long, each source can only be 1024 characters long. Shorten them before committing.")
			break
		}
	}

	if prompt.File.Mode == PF_UNSET {
		warnings = append(warnings, "No file selected!")
	}

	if prompt.TagWizard.Len() < 6 {
		warnings = append(warnings, "Not enough tags, each post must have at least 6! Add some more before committing.")
	}

	if len(prompt.Rating) == 0 {
		warnings = append(warnings, "You must specify a rating!")
	}

	if this.request_reply && !(prompt.State == SAVED || prompt.State == DISCARDED) {
		warnings = append(warnings, "Be sure to <b>reply</b> to my messages in groups! (<a href=\"https://core.telegram.org/bots#privacy-mode\">why?</a>)")
	}

	this.WarningsBase(b, warnings)
}

func (this PostFormatter) GenerateMessage(prompt *PostPrompt) string {
	var b bytes.Buffer

	if prompt.State == SAVED {
		if this.Result == nil {
			prompt.State = WAIT_MODE
			b.WriteString("<b>Failed to post file, no explanation available. Sorry about that.</b>\nYou can continue editing and try again.\n\n")
		} else {
			if this.Result.Success {
				b.WriteString("Successfully uploaded <i>")
				b.WriteString(html.EscapeString(prompt.File.FileName))
				b.WriteString("</i>")
				if this.Result != nil && this.Result.Location != nil {
					b.WriteString(", <a href=\"")
					b.WriteString(api.LocationToURLWithRating(*this.Result.Location, prompt.Rating))
					b.WriteString("\">click here to open it</a>")
				}
				b.WriteString(".\n\n")
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
				b.WriteString("</b>\nYou can continue to edit your upload and try again, or you can discard it.\n\n")
			}
		}
		prompt.PostStatus(&b)
		b.WriteRune('\n')

		this.Warnings(&b, prompt)
	} else if prompt.State == DISCARDED {
		b.WriteString("New post discarded.")
	} else {
		b.WriteString("Preparing to post a new file.\nCurrently editing: <code>")
		b.WriteString(GetNameOfState(prompt.State))
		b.WriteString("</code>\n\n")

		prompt_string := strings.TrimSpace(prompt.Status)
		b.WriteString(prompt_string)
		if len(prompt_string) > 0 {
			b.WriteString("\n\n")
		}

		prompt.PostStatus(&b)
		b.WriteRune('\n')

		this.Warnings(&b, prompt)
	}

	return b.String()
}

func (this PostFormatter) GenerateMarkup(prompt *PostPrompt) interface{} {
	sptr := func(x string) (*string) {return &x }
	// no buttons for a prompt which has already been finalized
	if prompt.State == DISCARDED || prompt.State == SAVED { return nil }

	var kb data.TInlineKeyboard
	kb.AddRow()
	kb.AddButton(data.TInlineKeyboardButton{Text: "Tags", Data: sptr("/tags")})
	kb.AddButton(data.TInlineKeyboardButton{Text: "Rating", Data: sptr("/rating")})
	kb.AddButton(data.TInlineKeyboardButton{Text: "Parent", Data: sptr("/parent")})
	kb.AddRow()
	kb.AddButton(data.TInlineKeyboardButton{Text: "File", Data: sptr("/file")})
	kb.AddButton(data.TInlineKeyboardButton{Text: "Sources", Data: sptr("/sources")})
	kb.AddButton(data.TInlineKeyboardButton{Text: "Description", Data: sptr("/description")})
	kb.AddRow()
	if prompt.State != WAIT_MODE {
		kb.AddButton(data.TInlineKeyboardButton{Text: fmt.Sprintf("\u21A9\uFE0F Reset %s", GetNameOfState(prompt.State)), Data: sptr(fmt.Sprintf("/reset %s", prompt.State))})
	}
	kb.AddButton(data.TInlineKeyboardButton{Text: fmt.Sprintf("\u2622\uFE0F Reset %s", GetNameOfState(WAIT_ALL)), Data: sptr(fmt.Sprintf("/reset %s", WAIT_ALL))})
	kb.AddRow()
	kb.AddButton(data.TInlineKeyboardButton{Text: "\U0001F7E2 Upload", Data: sptr("/save")})
	kb.AddButton(data.TInlineKeyboardButton{Text: "\U0001F534 Discard", Data: sptr("/discard")})

	var extra_buttons data.TInlineKeyboard
	if prompt.State == WAIT_TAGS {
		extra_buttons = prompt.TagWizard.Buttons()
	} else if prompt.State == WAIT_RATING {
		extra_buttons.AddRow()
		extra_buttons.AddButton(data.TInlineKeyboardButton{Text: "\U0001F7E9 Safe", Data: sptr("/rating s")})
		extra_buttons.AddButton(data.TInlineKeyboardButton{Text: "\U0001F7E8 Questionable", Data: sptr("/rating q")})
		extra_buttons.AddButton(data.TInlineKeyboardButton{Text: "\U0001F7E5 Explicit", Data: sptr("/rating e")})
	} else if prompt.State == WAIT_SOURCE {
		for i, source := range prompt.SeenSourcesReverse {
			var selected bool
			if prompt.Sources.Status(source) == tags.AddsTag {
				selected = true
			} else if prompt.Sources.Status(source) == tags.RemovesTag {
				selected = false
			}

			prefixes := map[bool]string{true:"\U0001F7E9 ", false:"\U0001F7E5 "}
			extra_buttons.AddRow()
			extra_buttons.AddButton(data.TInlineKeyboardButton{Text: prefixes[selected] + source, Data: sptr(fmt.Sprintf("/sources %d %t", i, !selected))})
		}
	} else if prompt.State == WAIT_PARENT {
		extra_buttons.AddRow()
		extra_buttons.AddButton(data.TInlineKeyboardButton{Text: "Delete parent", Data: sptr("/parent none")})
	}
	kb.Buttons = append(extra_buttons.Buttons, kb.Buttons...)
	return kb
}
