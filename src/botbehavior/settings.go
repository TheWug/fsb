package botbehavior

import (
	"telegram/telebot"
	"os"
	"log"
	"storage"
)

type Settings struct {
	telebot.InitSettings

	Logfile string `json:"logfile"`
	ApiKey  string `json:"apikey"`
	DbUrl   string `json:"dburl"`
	ApiName             string `json:"api_name"`
	ApiEndpoint         string `json:"api_endpoint"`
	ApiFilteredEndpoint string `json:"api_filtered_endpoint"`
	ApiStaticPrefix     string `json:"api_static_prefix"`

	Bot    *telebot.TelegramBot
}

func (s Settings) GetApiName() string {
	return s.ApiName
}

func (s Settings) GetApiEndpoint() string {
	return s.ApiEndpoint
}

func (s Settings) GetApiFilteredEndpoint() string {
	return s.ApiFilteredEndpoint
}

func (s Settings) GetApiStaticPrefix() string {
	return s.ApiStaticPrefix
}

func (this *Settings) RedirectLogs() (error) {
	newLogHandle, err := os.OpenFile(this.Logfile, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0600)
	if err != nil {
		return err
	}

	this.Bot.Log = log.New(newLogHandle, "", log.LstdFlags)
	this.Bot.ErrorLog = log.New(newLogHandle, "", log.LstdFlags | log.Llongfile)
	log.SetOutput(newLogHandle)
	this.Bot.Log.Printf("%s opened for logging.\n", this.Logfile)
	return nil
}

func (this Settings) InitializeAll() (error) {
	e := this.RedirectLogs()
	if e != nil { return e }

	e = storage.DBInit(this.DbUrl)
	if e != nil { return e }

	this.Bot.Remote.SetAPIKey(this.ApiKey)
	e = this.Bot.Remote.Test()
	return e
}
