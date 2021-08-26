package botbehavior

import (
	"github.com/thewug/gogram"
	"github.com/thewug/gogram/data"
	"os"
	"log"
	"storage"
)

const MAX_RESULTS_PER_PAGE = 50

type Settings struct {
	gogram.InitSettings

	Logfile string      `json:"logfile"`
	ApiKey  string      `json:"apikey"`
	DbUrl   string      `json:"dburl"`
	ApiName             string `json:"api_name"`
	ApiEndpoint         string `json:"api_endpoint"`
	ApiFilteredEndpoint string `json:"api_filtered_endpoint"`
	ApiStaticPrefix     string `json:"api_static_prefix"`

	Owner   data.UserID `json:"owner"`
	Home    data.ChatID `json:"home"`

	ResultsPerPage int `json:"results_per_page"`

	SearchUser   string `json:"search_user"`
	SearchAPIKey string `json:"search_apikey"`

	NoResultsPhotoID data.FileID `json:"no_results_photo_id"`
	ErrorPhotoID     data.FileID `json:"error_photo_id"`
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

func (this *Settings) RedirectLogs(bot *gogram.TelegramBot) (error) {
	newLogHandle, err := os.OpenFile(this.Logfile, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0600)
	if err != nil {
		return err
	}

	bot.Log = log.New(newLogHandle, "", log.LstdFlags)
	bot.ErrorLog = log.New(newLogHandle, "", log.LstdFlags | log.Llongfile)
	log.SetOutput(newLogHandle)
	bot.Log.Printf("%s opened for logging.\n", this.Logfile)
	return nil
}

func (this *Settings) InitializeAll(bot *gogram.TelegramBot) (error) {
	if this.ResultsPerPage < 1 || this.ResultsPerPage > MAX_RESULTS_PER_PAGE {
		this.ResultsPerPage = MAX_RESULTS_PER_PAGE
	}

	e := this.RedirectLogs(bot)
	if e != nil { return e }

	e = storage.DBInit(this.DbUrl)
	if e != nil { return e }

	bot.Remote.SetAPIKey(this.ApiKey)
	e = bot.Remote.Test()
	return e
}

func (this *Settings) DefaultSearchCredentials() (string, string) {
	return this.SearchUser, this.SearchAPIKey
}
