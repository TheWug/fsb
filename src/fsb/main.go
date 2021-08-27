package main

import (
	"bot"
	"botbehavior"
	"storage"

	"github.com/thewug/gogram"
	"github.com/thewug/gogram/persist"

	"fmt"
	"os"
)

// This bot runs by polling telegram for updates, and then making synchronous calls to api. Because of the latency
// involved, it won't handle high query volumes very well.

// Config is read from a config file, which is passed as argument 1 (or ~/.fsb.json if none is specified)
// It should contain a json object and supports the following keys: "logfile", "apikey"

func main() {
	// handle command line arguments
	settingsFile := "~/.fsb.json"
	if len(os.Args) > 1 && os.Args[1] != "" {
		if os.Args[1] == "--help" || os.Args[1] == "-h" {
			fmt.Printf("Usage: %s [CONFIGFILE]\n", os.Args[0])
			fmt.Println("  CONFIGFILE  - Read this file for settings. (if omitted, use " + settingsFile + ")")
			botbehavior.ShowHelp()
			os.Exit(0)
		} else {
			settingsFile = os.Args[1]
		}
	}

	var thebot gogram.TelegramBot
	machine := gogram.NewMessageStateMachine()
	var settings botbehavior.Settings
	var behavior botbehavior.Behavior
	behavior.ForwardTo = machine

	e := thebot.Init(settingsFile, &settings)
	if e != nil {
		fmt.Println(e.Error())
		os.Exit(1)
	}

	api.Init(settings)
	if e != nil {
		fmt.Println(e.Error())
		os.Exit(1)
	}

	apiextra.Init(settings)
	if e != nil {
		fmt.Println(e.Error())
		os.Exit(1)
	}

	e = proxify.Init(settings)
	if e != nil {
		fmt.Println(e.Error())
		os.Exit(1)
	}

	behavior.MySettings = settings

	p := persist.InitStatePersistence(storage.Db_pool, "state_persistence")

	help := bot.HelpState{StateBase: gogram.StateBase{StateMachine: machine}}
	login := bot.LoginState{StateBase: gogram.StateBase{StateMachine: machine}}
	janitor := bot.JanitorState{StateBase: gogram.StateBase{StateMachine: machine}}
	votes := bot.VoteState{StateBase: gogram.StateBase{StateMachine: machine}}
	autofix := bot.AutofixState{StateBase: gogram.StateBase{StateMachine: machine}, Behavior: &behavior}
	post := bot.PostState{StateBasePersistent: persist.Register(p, machine, "post", bot.PostStateFactory)}
	edit := bot.EditState{StateBasePersistent: persist.Register(p, machine, "edit", bot.EditStateFactory)}

	machine.AddCommand("/help", &help)
	machine.AddCommand("/login", &login)
	machine.AddCommand("/logout", &login)
	machine.AddCommand("/post", &post)
	machine.AddCommand("/indextags", &janitor)
	machine.AddCommand("/indextagaliases", &janitor)
	machine.AddCommand("/recountnegative", &janitor)
	machine.AddCommand("/cats", &janitor)
	machine.AddCommand("/blits", &janitor)
	machine.AddCommand("/findtagtypos", &janitor)
	machine.AddCommand("/recounttags", &janitor)
	machine.AddCommand("/syncposts", &janitor)
	machine.AddCommand("/resynclist", &janitor)
	machine.AddCommand("/editposttest", &janitor)
	machine.AddCommand("/parseexpression", &janitor)
	machine.AddCommand("/upvote", &votes)
	machine.AddCommand("/downvote", &votes)
	machine.AddCommand("/favorite", &votes)
	machine.AddCommand("/af-commit", &autofix)
	machine.AddCommand("/af-dismiss", &autofix)
	machine.AddCommand("/af-toggle", &autofix)
	machine.AddCommand("/edit", &edit)

	thebot.SetMessageCallback(machine)
	thebot.SetStateMachine(machine)
	thebot.SetCallbackCallback(machine)
	thebot.SetInlineCallback(&behavior)
	thebot.AddMaintenanceCallback(&behavior)
	thebot.AddMaintenanceCallback(&votes)
	thebot.AddMaintenanceCallback(&autofix)

	err := p.LoadAllStates(machine)
	if err != nil { thebot.ErrorLog.Println(err.Error()) }

	thebot.MainLoop()
	os.Exit(0)
}
