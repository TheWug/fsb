package bot

import (
	"storage"
	"bot/types"
	"api"

	"github.com/thewug/gogram"
	"github.com/thewug/gogram/data"

	"bytes"
	"fmt"
	"html"
	"strconv"
	"database/sql"
)

const SETTINGS = "/settings"
const BLACKLIST = "blacklist"
const RATING = "rating"
const VERIFY = "verify"
const VERIFYDONE = "doneverifying"
const VERIFYFAIL = "failverifying"

func Account(user string, bot_janitor bool) string {
	if user == "" {
		return "<i>Not connected</i>"
	}

	return html.EscapeString(user) + map[bool]string{false: "", true: " \u2728"}[bot_janitor]
}

func SettingsMessage(subcommand string, settings *storage.UserSettings, user string, bot_janitor bool) data.SendData {
	var d data.SendData
	var b bytes.Buffer

	prompt := ""
	if subcommand == BLACKLIST {
		prompt = "Please use the buttons to change your blacklist preferences.\n\nBy default, both blacklists are applied."
	} else if subcommand == RATING {
		prompt = "Please use the buttons to change your rating filter.\n\nBy default, only posts rated <b>safe</b> are shown. Users who have not verified their age can only view posts rated <b>safe</b> or <b>questionable</b>."
	} else if subcommand == VERIFY {
		prompt = "The fandom has a place for everyone regardless of age, but " + api.ApiName + " is not that place. Please don't lie about your age.\n\n<b>Help keep the fandom safe for everyone.</b>"
	} else if subcommand == VERIFYDONE {
		prompt = "Thanks for helping keep the fandom a safe place for everyone."
	}

	b.WriteString("<b>Your Settings</b>\n<code>Rating Filter:  </code>")
	b.WriteString(settings.RatingMode.Display())
	b.WriteString("\n<code>Blacklist Mode: </code>")
	b.WriteString(settings.BlacklistMode.Display())
	b.WriteString("\n<code>Age Status:     </code>")
	b.WriteString(settings.AgeStatus.Display())
	b.WriteString("\n\n<b>Your Account</b>\n<code>Telegram ID:  </code>")
	b.WriteString(strconv.Itoa(int(settings.TelegramId)))
	b.WriteString("\n<code>" + api.ApiName + " Account: </code>")
	b.WriteString(Account(user, bot_janitor))
	b.WriteString("\n\n")
	b.WriteString(prompt)

	d.Text = b.String()
	d.ParseMode = data.ParseHTML

	var k data.TInlineKeyboard
	k.AddRow()
	k.AddButton(data.TInlineKeyboardButton{Text: "Blacklist Settings", Data: sptr(SETTINGS + " " + BLACKLIST)})
	k.AddRow()
	k.AddButton(data.TInlineKeyboardButton{Text: "Rating Filter Settings", Data: sptr(SETTINGS + " " + RATING)})
	if settings.AgeStatus < types.AGE_VALIDATED {
		k.AddRow()
		k.AddButton(data.TInlineKeyboardButton{Text: "Verify Your Age", Data: sptr(SETTINGS + " " + VERIFY)})
	}

	if subcommand == BLACKLIST {
		k.AddRow()
		k.AddButton(data.TInlineKeyboardButton{Text: "Enabled", Data: sptr(SETTINGS + " " + BLACKLIST + " " + types.BLACKLIST_ON.String())})
		k.AddButton(data.TInlineKeyboardButton{Text: "Disabled", Data: sptr(SETTINGS + " " + BLACKLIST + " " + types.BLACKLIST_OFF.String())})
	} else if subcommand == RATING {
		k.AddRow()
		k.AddButton(data.TInlineKeyboardButton{Text: "Safe Only", Data: sptr(SETTINGS + " " + RATING + " " + types.FILTER_QUESTIONABLE.String())})
		k.AddButton(data.TInlineKeyboardButton{Text: "Safe, Questionable", Data: sptr(SETTINGS + " " + RATING + " " + types.FILTER_EXPLICIT.String())})
		k.AddButton(data.TInlineKeyboardButton{Text: "All Posts", Data: sptr(SETTINGS + " " + RATING + " " + types.FILTER_NONE.String())})
	} else if subcommand == VERIFY {
		k.AddRow()
		k.AddButton(data.TInlineKeyboardButton{Text: "\U0001F51E I understand, and I am 18 or older. \U0001F51E", Data: sptr(SETTINGS + " " + VERIFY + " yes")})
	}

	d.ReplyMarkup = k

	return d
}

type SettingsState struct {
	gogram.StateBase
}

func sptr(x string) *string { return &x }

func (this *SettingsState) Handle(ctx *gogram.MessageCtx) {
	err := storage.DefaultTransact(func(tx *sql.Tx) error { return this.HandleTx(tx, ctx) })
	if err != nil {
		ctx.Bot.ErrorLog.Println("Error in SettingsState.Handle: %s", err)
	}
}

func (this *SettingsState) HandleTx(tx *sql.Tx, ctx *gogram.MessageCtx) error {
	if ctx.Msg.From == nil { return nil }

	if ctx.Cmd.Command == "/settings" {
		settings, err := storage.GetUserSettings(tx, ctx.Msg.From.Id)
		if err != nil {
			ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: "Sorry! There was an error looking up your settings."}}, nil)
			return fmt.Errorf("Error looking up user settings for %d: %w", ctx.Msg.From.Id, err)
		}

		creds, err := storage.GetUserCreds(nil, ctx.Msg.From.Id)
		if err != nil && err != storage.ErrNoLogin {
			ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: "Sorry! There was an error looking up your " + api.ApiName + " account."}}, nil)
			return fmt.Errorf("Error looking up " + api.ApiName + " account for %d: %v", ctx.Msg.From.Id, err)
		}

		ctx.ReplyAsync(data.OMessage{SendData: SettingsMessage("", settings, creds.User, creds.Janitor)}, nil)
	} else if ctx.Cmd.Command == "/delete_my_data_and_forget_me" {
		if ctx.Cmd.Argstr == "Yes I'm sure!" {
			ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: fmt.Sprintf("I'll always remember you, %s!\n<i>MEMORY DELETED</i>", html.EscapeString(ctx.Msg.From.FirstName)), ParseMode: data.ParseHTML}}, nil)
			if err := storage.DeleteUserSettings(tx, ctx.Msg.From.Id); err != nil { ctx.Bot.ErrorLog.Println("Error deleting settings: ", err.Error()) }
			if err := storage.DeleteUserCreds(tx, ctx.Msg.From.Id); err != nil { ctx.Bot.ErrorLog.Println("Error deleting credentials: ", err.Error()) }
			if err := storage.DeleteUserTagRules(tx, ctx.Msg.From.Id); err != nil { ctx.Bot.ErrorLog.Println("Error deleting credentials: ", err.Error()) }
		} else if len(ctx.Cmd.Argstr) == 0 || ctx.Cmd.Argstr != "Yes I'm sure!" {
			ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: "<b>Forget all data and settings: are you sure?</b>\n\nIf you're sure, copy and paste the command\n<code>/delete_my_data_and_forget_me Yes I'm sure!</code>", ParseMode: data.ParseHTML}}, nil)
			return nil
		}
	}

	return nil
}

func (this *SettingsState) HandleCallback(ctx *gogram.CallbackCtx) {
	err := storage.DefaultTransact(func(tx *sql.Tx) error { return this.HandleCallbackTx(tx, ctx) })
	if err != nil {
		ctx.Bot.ErrorLog.Println("Error in SettingsState.HandleCallback: %s", err)
	}
}

func (this *SettingsState) HandleCallbackTx(tx *sql.Tx, ctx *gogram.CallbackCtx) error {
	var answer data.OCallback
	defer func() { ctx.Answer(answer) }()

	if ctx.Cmd.Command == "/settings" {
		settings, err := storage.GetUserSettings(tx, ctx.Cb.From.Id)
		if err != nil {
			answer.Notification, answer.ShowAlert = "Sorry! There was an error looking up your settings.", true
			return fmt.Errorf("Error looking up user settings for %d: %w", ctx.Cb.From.Id, err)
		}

		creds, err := storage.GetUserCreds(tx, ctx.Cb.From.Id)
		if err != nil && err != storage.ErrNoLogin {
			answer.Notification, answer.ShowAlert = "Sorry! There was an error looking up your " + api.ApiName + " account.", true
			return fmt.Errorf("Error looking up " + api.ApiName + " account for %d: %w", ctx.Cb.From.Id, err)
		}

		subcommand := ""
		if len(ctx.Cmd.Args) > 0 {
			subcommand = ctx.Cmd.Args[0]
		}

		if len(ctx.Cmd.Args) > 1 {
			answer.Notification, answer.ShowAlert = "OK!", false
			if subcommand == BLACKLIST {
				switch ctx.Cmd.Args[1] {
				case types.BLACKLIST_ON.String():
					settings.BlacklistMode = types.BLACKLIST_ON
				case types.BLACKLIST_OFF.String():
					settings.BlacklistMode = types.BLACKLIST_OFF
				}
			} else if subcommand == RATING {
				switch ctx.Cmd.Args[1] {
				case types.FILTER_QUESTIONABLE.String():
					settings.RatingMode = types.FILTER_QUESTIONABLE
				case types.FILTER_EXPLICIT.String():
					settings.RatingMode = types.FILTER_EXPLICIT
				case types.FILTER_NONE.String():
					if settings.AgeStatus > types.AGE_UNVALIDATED {
						settings.RatingMode = types.FILTER_NONE
					} else {
						answer.Notification, answer.ShowAlert = "You must verify your age to do this.", true
					}
				}
			} else if subcommand == VERIFY {
				if ctx.Cmd.Args[1] == "yes" {
					if settings.AgeStatus == types.AGE_UNVALIDATED {
						settings.AgeStatus = types.AGE_VALIDATED
						subcommand = VERIFYDONE
					} else if settings.AgeStatus == types.AGE_LOCKED {
						answer.Notification, answer.ShowAlert = "Your verification was revoked by an operator. Contact an operator with proof of age to restore it.", true
						subcommand = VERIFYFAIL
					} else {
						subcommand = ""
					}
				}
			}
		}

		err = storage.WriteUserSettings(tx, settings)
		if err != nil {
			answer.Notification, answer.ShowAlert = "Sorry! There was an error saving your settings.", true
			return fmt.Errorf("Error saving settings for %d: %w", ctx.Cb.From.Id, err)
		}

		gogram.NewMessageCtx(ctx.Cb.Message, false, ctx.Bot).EditText(data.OMessageEdit{SendData: SettingsMessage(subcommand, settings, creds.User, creds.Janitor)})
	}

	return nil
}
