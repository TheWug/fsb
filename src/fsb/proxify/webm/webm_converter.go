package webm

import (
	"botbehavior/settings"
	"api/types"
	"storage"

	"github.com/thewug/gogram"
	"github.com/thewug/gogram/data"
	"github.com/thewug/reqtify"

	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path"
)

type webmToTelegramMp4Converter struct {
	convert_requests chan *webm2Mp4Req
	bot *gogram.TelegramBot
	settings *settings.Settings
}

var converter *webmToTelegramMp4Converter

func ConfigureWebmToTelegramMp4Converter(bot *gogram.TelegramBot, settings *settings.Settings) {
	converter = &webmToTelegramMp4Converter{
		bot: bot,
		settings: settings,
		convert_requests: make(chan *webm2Mp4Req),
	}

	go converter.convertRoutine()
}

type webm2Mp4Req struct {
	output chan *data.FileID
	post  *types.TPostInfo
}


// waits for an mp4 file id to become available, possibly triggering a
// conversion and blocking until it completes.
func GetMp4ForWebm(result *types.TPostInfo) *data.FileID {
	req := webm2Mp4Req{
		output: make(chan *data.FileID),
		post: result,
	}
	converter.convert_requests <- &req
	return <- req.output
}

// checks if an mp4 file id is available, returning it immediately if so
// and returning nil if not.
func CheckMp4ForWebm(tx *sql.Tx, result *types.TPostInfo) *data.FileID {
	cached, err := storage.FindCachedMp4ForWebm(tx, result.Md5)
	if err != nil {
		converter.bot.ErrorLog.Println("Error in storage.FindCachedMp4ForWebm: ", err.Error())
		return nil
	}
	return cached
}

// synchronous converter routine.
func (this webmToTelegramMp4Converter) convertRoutine() {
	for req := range this.convert_requests {
		_ := storage.DefaultTransact(func(tx *sql.Tx) error {
			// within this function, return = continue outer loop
			// so I can use defer to process stuff at end of iteration
			defer func() { close(req.output) }()

			cached, err := storage.FindCachedMp4ForWebm(tx, req.post.Md5)
			if err != nil { return fmt.Errorf("storage.FindCachedMp4ForWebm: %w", err) }

			if cached != nil {
				req.output <- cached
				return nil
			}

			converted_file, err := this.convertFile(req.post.File_url, true)
			if err != nil { return fmt.Errorf("webm.convertFile: %w", err) }

			cached, err = this.uploadConvertedFileToTelegram(converted_file)
			if err != nil { return fmt.Errorf("webm.uploadConvertedFileToTelegram: %w", err) }

			req.output <- cached

			err = storage.SaveCachedMp4ForWebm(tx, req.post.Md5, *cached)
			if err != nil { return fmt.Errorf("storage.SaveCachedMp4ForWebm: %w", err) }

			return nil
		})

		// TODO handle error
	}
}

func (this webmToTelegramMp4Converter) convertFile(url string, strip_audio bool) (reqtify.FormFile, error) {
	var file reqtify.FormFile
	resp, err := http.Get(url)
	if err != nil {
		return file, err
	} else if resp.StatusCode != 200 {
		return file, errors.New("Request failed: " + resp.Status)
	} else if resp.Body == nil {
		return file, errors.New("Request succeeded, but has empty body!")
	}

	defer resp.Body.Close()

	base_name := path.Base(url) + map[bool]string{false: ".mp4", true: ".silent.mp4"}[strip_audio]
	out_name := this.settings.MediaConvertDirectory + base_name
	cmd := exec.Command(this.settings.Webm2Mp4ConvertScript, out_name,
	                    map[bool]string{false: "audio", true: "noaudio"}[strip_audio])
	cmd.Stdin = resp.Body
	err = cmd.Run()
	if err != nil {
		os.Remove(out_name)
		return file, err
	}

	converted, err := os.Open(out_name)
	if err != nil {
		os.Remove(out_name)
		return file, err
	}

	err = os.Remove(out_name)
	if err != nil {
		converted.Close()
		return file, err
	}

	file.Name = base_name
	file.Data = converted
	return file, err
}

func (this webmToTelegramMp4Converter) uploadConvertedFileToTelegram(file reqtify.FormFile) (*data.FileID, error) {
	message, err := this.bot.Remote.SendAnimation(data.OAnimation{
		SendData: data.SendData{
			TargetData: data.TargetData{
				ChatId: this.settings.MediaStoreChannel,
			},
			Text: file.Name,
		},
		MediaData: data.MediaData{
			File: file,
		},
	})

	if err != nil {
		return nil, err
	} else if message != nil && message.Animation != nil {
		return &message.Animation.Id, nil
	} else if message == nil {
		return nil, errors.New("Nil message returned for seemingly successful call?")
	} else if message.Animation == nil {
		return nil, errors.New("message sent successfully, but not of type animation?")
	} else {
		return nil, errors.New("Unexpected error condition")
	}
}
