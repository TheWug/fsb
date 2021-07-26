package bot

import (
	"botbehavior"

	"github.com/thewug/gogram"
	"github.com/thewug/gogram/data"

	"fmt"
)


type ManageState struct {
	gogram.StateBase

	Behavior *botbehavior.Behavior
}

// this command is one expected by telegram for the following functions:
// a user's first message ever to a bot
// a user pushing the switch-to-pm button in an inline query
// handle it by modifying the command and redispatching.
func (this *ManageState) Handle(ctx *gogram.MessageCtx) {
	// unceremoniously ignore non-owners
	if ctx.Msg.From == nil || this.Behavior.MySettings.Owner != ctx.Msg.From.Id { return }

	photo := ctx.Msg.Photo
	if (photo == nil || *photo == nil) && ctx.Msg.ReplyToMessage != nil {
		photo = ctx.Msg.ReplyToMessage.Photo
	}

	if !(photo == nil || *photo == nil) {
		ctx.ReplyOrPMAsync(data.OMessage{SendData: data.SendData{Text: fmt.Sprintf("<pre>PHOTO\nID: %s</pre>", (*photo)[0].Id), ParseMode: data.ParseHTML}}, nil)
		return
	}

	doc := ctx.Msg.Document
	if doc == nil && ctx.Msg.ReplyToMessage != nil {
		doc = ctx.Msg.ReplyToMessage.Document
	}

	if doc != nil {
		ctx.ReplyOrPMAsync(data.OMessage{SendData: data.SendData{Text: fmt.Sprintf("<pre>DOCUMENT\nID: %s</pre>", doc.Id), ParseMode: data.ParseHTML}}, nil)
		return
	}
}
