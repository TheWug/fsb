package main

import (
	"fmt"
	"os"
	"botbehavior"
	"telegram/telebot"

	bbot "bot"
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

	var bot telebot.TelegramBot
	var settings botbehavior.Settings
	var behavior botbehavior.Behavior

	settings.Bot = &bot
	behavior.Bot = &bot

	e := bot.Init(settingsFile, &settings)
	if e != nil {
		fmt.Println(e.Error())
		os.Exit(1)
	}

	api.Init(settings)
	if e != nil {
		fmt.Println(e.Error())
		os.Exit(1)
	}

	bbot.Init(settings)
	if e != nil {
		fmt.Println(e.Error())
		os.Exit(1)
	}

	bot.SetMessageCallback(&behavior)
	bot.SetInlineCallback(&behavior)
	bot.SetCallbackCallback(&behavior)
	bot.MainLoop()
}
