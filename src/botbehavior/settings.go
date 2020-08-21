package botbehavior

import (
	"fsb/proxify"
	"storage"

	"github.com/thewug/gogram"
	"github.com/thewug/gogram/data"

	"encoding/json"

	"log"
	"os"
)

const MAX_RESULTS_PER_PAGE = 50
const MAX_ARTISTS = 10
const MAX_CHARS = 10
const MAX_SOURCES = 10

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

	NoResultsPhotoID   data.FileID `json:"no_results_photo_id"`
	BlacklistedPhotoID data.FileID `json:"blacklisted_photo_id"`
	ErrorPhotoID       data.FileID `json:"error_photo_id"`

	SourceMap        json.RawMessage `json:"source_map"`

	proxify.CaptionSettings
}

func (s Settings) GetSourceMap() json.RawMessage {
	return s.SourceMap
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
	if this.ResultsPerPage < 1 || this.ResultsPerPage > MAX_RESULTS_PER_PAGE { this.ResultsPerPage = MAX_RESULTS_PER_PAGE }
	if this.MaxArtists < 1 || this.MaxArtists > MAX_ARTISTS { this.MaxArtists = MAX_ARTISTS }
	if this.MaxChars < 1 || this.MaxChars > MAX_CHARS { this.MaxChars = MAX_CHARS }
	if this.MaxSources < 1 || this.MaxSources > MAX_SOURCES { this.MaxSources = MAX_SOURCES }

	e := this.RedirectLogs(bot)
	if e != nil { return e }

	e = storage.DBInit(this.DbUrl)
	if e != nil { return e }

	bot.Remote.SetAPIKey(this.ApiKey)
	e = bot.Remote.Test()
	return e
}

func (this *Settings) DefaultSearchCredentials() (storage.UserCreds) {
	return storage.UserCreds{
		User: this.SearchUser,
		ApiKey: this.SearchAPIKey,
	}
}
