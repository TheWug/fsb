package proxify

import (
	"github.com/thewug/gogram/data"

	"api/types"
	"api"
	"strings"
	"strconv"
	"log"
	"fmt"
	"net/url"
	"html"
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

// takes an api URL and transforms the domain to filtered api.
// building a filtered api URL from scratch is more efficient than using this,
// so this should be used primarily for URLs retrieved from the API,
// not ones assembled locally.

func MaybeSafeify(u string, safe bool) string {
	if !safe { return u }

	URL, err := url.Parse(u)
	if err != nil { return "" }

	URL.Host = strings.Replace(URL.Host, api.Endpoint, api.FilteredEndpoint, -1)
	return URL.String()
}

func domain(safe bool) string {
	if safe { return api.FilteredEndpoint }
	return api.Endpoint
}

func artistLink(artist string, safe bool) string {
	return fmt.Sprintf(`<a href="https://%s/artists/show_or_new?name=%s">%s</a>`, domain(safe), url.QueryEscape(artist), html.EscapeString(strings.Replace(artist, "_", " ", -1)))
}

func characterLink(character string, safe bool) string {
	return fmt.Sprintf(`<a href="https://%s/wiki_pages/show_or_new?title=%s">%s</a>`, domain(safe), url.QueryEscape(character), html.EscapeString(strings.Replace(character, "_", " ", -1)))
}

func sourceLine(url, display string) string {
	return fmt.Sprintf(`<a href="%s">%s</a>`, url, display)
}

func sourcesList(sources []string) []string {
	var output_source_list, all_sources []string
	var unknown int

	telegram_sticker_source := false

	for _, source := range(sources) {
		u, err := url.Parse(source)
		if err != nil {
			unknown++
			continue
		}

		hostname := u.Hostname()
		path := u.EscapedPath()

		source_entry := sourceLine(source, "Source")

		// figure out a way to pretty-print sources

		if len(source_entry) == 0 {
			unknown++
			continue
		} else {
			output_source_list = append(output_source_list, source_entry)
		}
	}

	if unknown != 0 { output_source_list = append(output_source_list, fmt.Sprintf(`%d more...`, unknown)) }
	return append(all_sources, "Sources: " + strings.Join(output_source_list, ", "))
}

func GenerateCaption(result types.TPostInfo, force_safe bool, query string) string {
	post_url := fmt.Sprintf("https://%s/posts/%d", domain(force_safe), result.Id)
	image_url := MaybeSafeify(result.File_url, force_safe)

	var caption []string
	// add the post and image links
	caption = append(caption, fmt.Sprintf(`View <a href="%s">Post</a>, <a href="%s">Image</a>`, post_url, image_url))

	// add the artist links
	if len(result.Artist) == 0 {
		caption = append(caption, fmt.Sprintf(`Art by %s`, artistLink("unknown_artist", force_safe)))
	} else if len(result.Artist) <= 10 {
		var artist_links []string
		for _, artist := range(result.Artist) { artist_links = append(artist_links, artistLink(artist, force_safe)) }
		caption = append(caption, fmt.Sprintf(`Art by %s`, strings.Join(artist_links, ", ")))
	} else if len(result.Artist) > 10 {
		caption = append(caption, "Art by more than 10 artists (see post)")
	}

	// add the character links
	if len(result.Character) > 0 && len(result.Character) <= 10 {
		var character_links []string
		for _, char := range(result.Character) { character_links = append(character_links, characterLink(char, force_safe)) }
		caption = append(caption, fmt.Sprintf(`Featuring %s`, strings.Join(character_links, ", ")))
	} else if len(result.Character) > 10 {
		caption = append(caption, "Featuring more than 10 characters (see post)")
	}

	// add generic source links
	caption = append(caption, sourcesList(result.Sources)...)

	// add search query
	if query == "" {
		caption = append(caption, fmt.Sprintf(`(from the front page)`))
	} else {
		caption = append(caption, fmt.Sprintf(`(search: %s)`, html.EscapeString(query)))
	}

	return strings.Join(caption, "\n")
}

// https://api/artists/show_or_new?name=dizzyvixen

func ConvertApiResultToTelegramInline(result types.TPostInfo, force_safe bool, query string, debugmode bool) (interface{}) {
	salt := "x_"

	width := result.Width
	height := result.Height

	caption := GenerateCaption(result, force_safe, query)

	if result.File_ext == "gif" {
		foo := data.TInlineQueryResultGif{
			Type:        "gif",
			Id:          salt + result.Md5,
			GifUrl:      result.File_url,
			ThumbUrl:    result.Preview_url,
			GifWidth:    &width,
			GifHeight:   &height,
			ParseMode:   data.ParseHTML,
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
		file_url := result.File_url
		if width * height > 13000000 { // images larger than 13MP will use the sample image instead of the full res
			file_url = result.Sample_url
			width = result.Sample_width
			height = result.Sample_height
		}

		foo := data.TInlineQueryResultPhoto{
			Type:        "photo",
			Id:          salt + result.Md5,
			PhotoUrl:    file_url,
			ThumbUrl:    result.Preview_url,
			PhotoWidth:  &width,
			PhotoHeight: &height,
			ParseMode:   data.ParseHTML,
			Caption:     &caption,
			Title:       &caption,
		}

		if debugmode { GenerateDebugText(&foo, result) }

		s2p := func(s string) *string { return &s }
		foo.ReplyMarkup = &data.TInlineKeyboard{
			Buttons: [][]data.TInlineKeyboardButton{
				[]data.TInlineKeyboardButton{
					data.TInlineKeyboardButton{Text: fmt.Sprintf("\U0001F44D %d", result.Upvotes), Data: s2p(fmt.Sprintf("/upvote %d", result.Id))},
					data.TInlineKeyboardButton{Text: fmt.Sprintf("\U0001F44E %d", result.Downvotes), Data: s2p(fmt.Sprintf("/downvote %d", result.Id))},
					data.TInlineKeyboardButton{Text: "\u2764\uFE0F Fav", Data: s2p(fmt.Sprintf("/favorite %d", result.Id))},
				},
			},
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
	t := true
	imt := data.TInputMessageTextContent{
		MessageText: "",
		ParseMode: data.ParseMarkdown,
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
