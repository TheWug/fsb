package proxify

import (
	"api"
	"telegram"
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

func ConvertApiResultToTelegramInline(result api.TSearchResult, force_safe bool) (interface{}) {
	postURL := ""
	if force_safe {
		postURL = fmt.Sprintf("https://%s/post/show/%d", api.FilteredEndpoint, result.Id)
	} else {
		postURL = fmt.Sprintf("https://%s/post/show/%d", api.Endpoint, result.Id)
	}
	if result.File_ext == "gif" {
		return telegram.TInlineQueryResultGif{
			Type:         "gif",
			Id:           result.Md5,
			Gif_url:      WugURL(result.File_url),
			Thumb_url:    WugURL(result.Preview_url),
			Gif_width:    &result.Width,
			Gif_height:   &result.Height,
			Caption:      &postURL,
			Title:        &postURL,
		}
	} else if result.File_ext == "webm" || result.File_ext == "swf" {
		// not handled yet, so do nothing
		log.Printf("[Wug     ] Not handling result ID %d (it's an incompatible animation)\n", result.Id)
		return nil
	} else if (result.File_ext == "png" || result.File_ext == "jpg" || result.File_ext == "jpeg"){
		return telegram.TInlineQueryResultPhoto{
			Type:         "photo",
			Id:           result.Md5,
			Photo_url:    WugURL(result.File_url),
			Thumb_url:    WugURL(result.Preview_url),
			Photo_width:  &result.Width,
			Photo_height: &result.Height,
			Caption:      &postURL,
			Title:        &postURL,
		}
	}
	return nil
}

func Offset(last string) (y int) {
	var e error
	y, e = strconv.Atoi(last)
	if e != nil && last == "" {
		y = 0
	} else if e != nil {
		y = -1
	}
	return
}
