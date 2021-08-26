package proxify

import (
	"github.com/thewug/gogram/data"

	"api/types"
	"strings"
	"strconv"
	"log"
	"fmt"
)

func WugURL(url string) (string) {
	if strings.HasSuffix(url, ".png") {
		url += ".jpg"
	}

	if strings.HasPrefix(url, "https://") {
		url = strings.Replace(url, "https://", "http://", 1)
	} else {
		url = "http://" + url
	}

	url = strings.Replace(url, api.StaticEndpoint, "home.wuggl.es", 1)
	return url
}

func ContainsSafeRatingTag(tags string) (bool) {
	taglist := strings.Split(tags, " ")

	for _, b := range taglist {
		if strings.HasPrefix(strings.ToLower(b), "rating:s") {
			return true
		}
	}
	return false
}

func ConvertApiResultToTelegramInline(result types.TSearchResult, force_safe bool, query string, debugmode bool) (interface{}) {
	salt := "x_"
	postURL := ""
	if force_safe {
		postURL = fmt.Sprintf("https://%s/post/show/%d", api.FilteredEndpoint, result.Id)
	} else {
		postURL = fmt.Sprintf("https://%s/post/show/%d", api.Endpoint, result.Id)
	}

	raw_url := result.File_url
	width := result.Width
	height := result.Height

	caption := fmt.Sprintf("Full res: %s\n(search: %s)", postURL, query)

	if result.File_ext == "gif" {
		foo := data.TInlineQueryResultGif{
			Type:         "gif",
			Id:           salt + result.Md5,
			Gif_url:      raw_url,
			Thumb_url:    result.Preview_url,
			Gif_width:    &width,
			Gif_height:   &height,
			Caption:      &caption,
			Title:        &caption,
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
			Type:         "photo",
			Id:           salt + result.Md5,
			Photo_url:    raw_url,
			Thumb_url:    result.Preview_url,
			Photo_width:  &width,
			Photo_height: &height,
			Caption:      &caption,
			Title:        &caption,
		}

		if debugmode { GenerateDebugText(&foo, result) }
		return foo
	}
	return nil
}

func Offset(last string) (y int) {
	var e error
	y, e = strconv.Atoi(last)
	if e != nil && last == "" {
		y = 1
	} else if e != nil {
		y = -1
	}
	return
}

func GenerateDebugText(iqr interface{}, result types.TSearchResult) {

	imt := data.TInputMessageTextContent{
		Message_text: "",
		Parse_mode: "Markdown",
		No_preview: true,
	}

	switch v := iqr.(type) {
	case *data.TInlineQueryResultPhoto:
		imt.Message_text = fmt.Sprintf("`ID:    `%d\n`MD5:   `%s\n`Size:  `%dx%d\n`Full:  `%s\n`Thumb: `%s\n", result.Id, result.Md5, *v.Photo_width, *v.Photo_height, v.Photo_url, v.Thumb_url)
		v.Input_message_content = &imt
	case *data.TInlineQueryResultGif:
		imt.Message_text = fmt.Sprintf("`ID:    `%d\n`MD5:   `%s\n`Size:  `%dx%d\n`Full:  `%s\n`Thumb: `%s\n", result.Id, result.Md5, *v.Gif_width, *v.Gif_height, v.Gif_url, v.Thumb_url)
		v.Input_message_content = &imt
	}
}
