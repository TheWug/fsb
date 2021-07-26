package cmd

import (
	"github.com/thewug/gogram"
)


type StartState struct {
	gogram.StateBase
}

// this command is one expected by telegram for the following functions:
// a user's first message ever to a bot
// a user pushing the switch-to-pm button in an inline query
// handle it by modifying the command and redispatching.
func (this *StartState) Handle(ctx *gogram.MessageCtx) {
	if len(ctx.Cmd.Args) == 0 { // just /start by itself
		ctx.Cmd.Command = "/help"
	} else if len(ctx.Cmd.Args) == 1 {
		if ctx.Cmd.Args[0] == "settings" {
			ctx.Cmd.Command = "/settings"
		}
		ctx.Cmd.Args = nil
		ctx.Cmd.Argstr = ""
	}
	this.StateMachine.Handle(ctx)
}
