package proxify

import (
	"github.com/thewug/gogram/data"

	"api/types"
	"strings"
	"strconv"
	"log"
	"fmt"
)

func ContainsSafeRatingTag(tags string) (bool) {
	taglist := strings.Split(tags, " ")

	for _, b := range taglist {
		if strings.HasPrefix(strings.ToLower(b), "rating:s") {
			return true
		}
	}
	return false
}

func ConvertApiResultToTelegramInline(result types.TPostInfo, force_safe bool, query string, debugmode bool) (interface{}) {
	salt := "x_"
	postURL := ""
	if force_safe {
		postURL = fmt.Sprintf("https://%s/posts/%d", api.FilteredEndpoint, result.Id)
	} else {
		postURL = fmt.Sprintf("https://%s/posts/%d", api.Endpoint, result.Id)
	}

	raw_url := result.File_url
	width := result.Width
	height := result.Height

	caption := fmt.Sprintf("Full res: %s\n(search: %s)", postURL, query)

	if result.File_ext == "gif" {
		foo := data.TInlineQueryResultGif{
			Type:        "gif",
			Id:          salt + result.Md5,
			GifUrl:      raw_url,
			ThumbUrl:    result.Preview_url,
			GifWidth:    &width,
			GifHeight:   &height,
			Caption:     &caption,
			Title:       &caption,
		}
		if debugmode { GenerateDebugText(&foo, result) }
		return foo
	} else if result.File_ext == "webm" || result.File_ext == "swf" {
		// not handled yet, so do nothing
		log.Printf("[Wug     ] Not handling result ID %d (it's an incompatible animation)\n", result.Id)
		return nil
	} else if (result.File_ext == "png" || result.File_ext == "jpg" || result.File_ext == "jpeg"){
		// telegram's logic about what files bots can send is fucked. it's tied to web previewing logic somehow,
		// and the limits seem to kick in long before the posted limits on the bot api say they should.
		// here is a shitty heuristic which will hopefilly be good enough to at least make most of them display SOMETHING.
		if width * height > 13000000 { // images larger than 13MP will use the sample image instead of the full res
			raw_url = result.Sample_url
			width = result.Sample_width
			height = result.Sample_height
		}

		foo := data.TInlineQueryResultPhoto{
			Type:        "photo",
			Id:          salt + result.Md5,
			PhotoUrl:    raw_url,
			ThumbUrl:    result.Preview_url,
			PhotoWidth:  &width,
			PhotoHeight: &height,
			Caption:     &caption,
			Title:       &caption,
		}

		if debugmode { GenerateDebugText(&foo, result) }

		s2p := func(s string) *string { return &s }

		if debugmode {
			foo.ReplyMarkup = &data.TInlineKeyboard{
				Buttons: [][]data.TInlineKeyboardButton{
					[]data.TInlineKeyboardButton{
						data.TInlineKeyboardButton{Text: "Upvote \U0001F44D", Data: s2p(fmt.Sprintf("/upvote %d", result.Id))},
						data.TInlineKeyboardButton{Text: "Downvote \U0001F44E", Data: s2p(fmt.Sprintf("/downvote %d", result.Id))},
						data.TInlineKeyboardButton{Text: "Favorite \u2764\uFE0F", Data: s2p(fmt.Sprintf("/favorite %d", result.Id))},
					},
				},
			}
		}
		return foo
	}
	return nil
}

func Offset(last string) (int, error) {
	if last == "" {
		last = "0"
	}

	return strconv.Atoi(last)
}

func GenerateDebugText(iqr interface{}, result types.TPostInfo) {
	md := data.ParseMarkdown
	t := true
	imt := data.TInputMessageTextContent{
		MessageText: "",
		ParseMode: &md,
		NoPreview: &t,
	}

	switch v := iqr.(type) {
	case *data.TInlineQueryResultPhoto:
		imt.MessageText = fmt.Sprintf("`ID:    `%d\n`MD5:   `%s\n`Size:  `%dx%d\n`Full:  `%s\n`Thumb: `%s\n", result.Id, result.Md5, *v.PhotoWidth, *v.PhotoHeight, v.PhotoUrl, v.ThumbUrl)
		v.InputMessageContent = &imt
	case *data.TInlineQueryResultGif:
		imt.MessageText = fmt.Sprintf("`ID:    `%d\n`MD5:   `%s\n`Size:  `%dx%d\n`Full:  `%s\n`Thumb: `%s\n", result.Id, result.Md5, *v.GifWidth, *v.GifHeight, v.GifUrl, v.ThumbUrl)
		v.InputMessageContent = &imt
	}
}
