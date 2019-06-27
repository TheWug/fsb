package tagindex

import (
	"github.com/thewug/gogram"
	"github.com/thewug/gogram/data"

	"api"
	"api/types"
	"storage"
	"wordset"

	"strconv"
	"time"
	"fmt"
	"log"
	"unicode/utf8"
	"bytes"
	"html"
	"errors"
	"sort"
	"strings"
	"sync"
)

func DownloadMessage(id, max_id int, name string) (string) {
	if max_id == 0 {
		return fmt.Sprintf("Downloading %s list...", name)
	} else if id == 0 {
		return fmt.Sprintf("Downloading %s list (done).", name)
	} else {
		percentage := int(100 * (1 - float32(id) / float32(max_id)))
		return fmt.Sprintf("Downloading %s list (%d%%)...", name, percentage)
	}
}

func DBMessage(done bool, name string) (string) {
	if !done {
		return fmt.Sprintf("%s\nUpdating %s database...", DownloadMessage(0, 1, name), name)
	} else {
		return fmt.Sprintf("%s\nUpdating %s database (done).", DownloadMessage(0, 1, name), name)
	}
}

// launches a message thing and repeatedly edits it with updates that are passed into channels. expects to live in a goroutine.
func ProgressMessageRoutine(ctx *gogram.MessageCtx, initial_status, initial_suffix string, new_status, new_suffix chan string) {
	var err error
	var sent_message *gogram.MessageCtx

	if initial_status != "" {
		sent_message, err = ctx.Reply(data.OMessage{Text: fmt.Sprintf("%s%s", initial_status, initial_suffix), ParseMode: data.HTML})
	}

	edit_timer := time.NewTicker(1000 * time.Millisecond)
	message := initial_status
	suffix := initial_suffix
	changed := false
	last_update := time.Now()

	update := func(force bool) {
		// don't update if unchanged, even if force is set
		if err != nil || !changed { return }

		now := time.Now()
		delta := now.Sub(last_update)
		if delta > 5 * time.Second || force {
			// always sleep at least 500ms, even if forced
			if delta < 500 * time.Millisecond {
				time.Sleep(500 * time.Millisecond - delta)
			}

			if sent_message == nil {
				sent_message, err = ctx.Reply(data.OMessage{Text: fmt.Sprintf("%s%s", message, suffix), ParseMode: data.HTML})
			} else {
				sent_message.EditTextAsync(data.OMessage{Text: fmt.Sprintf("%s%s", message, suffix), ParseMode: data.HTML}, nil)
			}
			last_update = now
			changed = false
		}
	}

	var nsuffix, nstatus string
	status_ok, suffix_ok := true, true
	for status_ok && suffix_ok {
		select {
			case nstatus, status_ok = <-new_status:
				if !status_ok { continue }
				message = fmt.Sprintf("%s\n%s", message, nstatus)
				suffix = ""
				changed = true
				update(true)
			case nsuffix, suffix_ok = <-new_suffix:
				if !suffix_ok { continue }
				changed = changed || suffix != nsuffix
				suffix = nsuffix
				update(false)
			case <- edit_timer.C:
				update(false)
		}
	}
	edit_timer.Stop()
	update(true)
}

func ProgressMessage(ctx *gogram.MessageCtx, initial_status, initial_suffix string) (chan string, chan string) {
	new_status := make(chan string)
	new_suffix := make(chan string)
	go ProgressMessageRoutine(ctx, initial_status, initial_suffix, new_status, new_suffix)
	return new_status, new_suffix
}

func SyncTagsExternal(ctx *gogram.MessageCtx) {
	full := false
	for _, token := range ctx.Cmd.Args {
		if token == "--full" {
			full = true
		}
	}

	err := SyncTags(ctx, full, nil, nil)
	if err == storage.ErrNoLogin {
		ctx.ReplyOrPMAsync(data.OMessage{Text: "You need to be logged in to " + api.ApiName + " to use this command (see <code>/help login</code>)", ParseMode: data.HTML}, nil)
		return
	} else if err != nil {
		log.Println("Error occurred syncing tags: %s", err.Error())
	}
}

func SyncTags(ctx *gogram.MessageCtx, full bool, msg, sfx chan string) (error) {
	user, api_key, janitor, err := storage.GetUserCreds(ctx.Msg.From.Id)
	if err != nil || !janitor { return err }

	m := "Syncing tag database..."
	if full { m = "Full syncing tag database..." }
	if msg == nil || sfx == nil {
		msg, sfx = ProgressMessage(ctx, m, "")
		defer close(msg)
		defer close(sfx)
	} else {
		msg <- m
	}

	if full {
		storage.ClearTagIndex()
	}

	api_timeout := time.NewTicker(750 * time.Millisecond)
	fixed_tags := make(chan types.TTagData)

	var list types.TTagInfoArray
	var cont bool
	page := 1
	last_id := -1
	max_tag_id := 0
	last_existing_tag_id := -1
	consecutive_errors := 0
	last, err := storage.GetLastTag()
	if last != nil { last_existing_tag_id = last.Id }

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		storage.TagUpdater(fixed_tags)
		wg.Done()
	}()
	defer api_timeout.Stop()
	defer storage.FixTagsWhichTheAPIManglesByAccident()

	for {
		list, cont, page, err = api.ListOnePageOfTags(user, api_key, page, nil)
		if err != nil {
			fmt.Printf("Error when calling api, waiting 30 seconds before retrying (%s)", err.Error())
			if consecutive_errors++; consecutive_errors == 10 {
				// transient API errors are okay, they might be because of network issues or whatever, but give up if they last too long.
			}
			time.Sleep(30 * time.Second)
			continue
		}

		consecutive_errors = 0

		for i, t := range list {
			if t.Id <= last_existing_tag_id {
				cont = false
				break
			}
			if max_tag_id < t.Id {
				last_id = t.Id + 1
				max_tag_id = t.Id
			}
			if i == 0 { sfx <- Percent(max_tag_id - t.Id, max_tag_id - last_existing_tag_id) }
			if t.Id < last_id {
				fixed_tags <- t
			}
		}

		if !cont { break }
		
		<- api_timeout.C
		_ = page
	}

	close(fixed_tags)
	wg.Wait()

	msg <- "Resolving phantom tags..."
	storage.ResolvePhantomTags()

	sfx <- " done."
	return nil
}

func RecountTagsExternal(ctx *gogram.MessageCtx) {
	real_counts := false
	alias_counts := false
	for _, token := range ctx.Cmd.Args {
		if token == "--real" {
			real_counts = true
		} else if token == "--alias" {
			alias_counts = true
		}
	}

	msg, sfx := ProgressMessage(ctx, "", "")
	defer close(msg)
	defer close(sfx)

	if (real_counts) {
		RecountTags(ctx, msg, sfx)
	}
	if (alias_counts) {
		CalculateAliasedCounts(ctx, msg, sfx)
	}
}

func RecountTags(ctx *gogram.MessageCtx, msg, sfx chan string) (error) {
	m := "Recounting tags..."
	if msg == nil || sfx == nil {
		msg, sfx = ProgressMessage(ctx, m, "")
		defer close(msg)
		defer close(sfx)
	} else {
		msg <- m
	}

	err := storage.CountTags(sfx)
	if err != nil {
		sfx <- fmt.Sprintf(" (error: %s)", err.Error())
	} else {
		sfx <- " done."
	}

	return err
}

func SyncPosts(ctx *gogram.MessageCtx) {
	user, api_key, janitor, err := storage.GetUserCreds(ctx.Msg.From.Id)
	if err != nil || !janitor {
		ctx.ReplyAsync(data.OMessage{Text: "You need to be logged in to " + api.ApiName + " to use this command (see <code>/help login</code>)", ParseMode: data.HTML}, nil)
		return
	}

	var full, restart bool
	for _, token := range ctx.Cmd.Args {
		if token == "--full" {
			full = true
		} else if token == "--restart" {
			restart = true
		}
	}

	msg, sfx := ProgressMessage(ctx, "Syncing post database...", "")
	defer close(msg)
	defer close(sfx)

	checkpoint, err := storage.GetTagHistoryCheckpoint()

	state, _ := storage.GetMigrationState()
	if restart { state = 0 }
	if full || state != 0 || checkpoint == 0 {
		if state == 0 {
			state = 1
			storage.SetMigrationState(state, 0)
		}
		if state == 1 {
			msg <- "Resetting post database..."
 			storage.ResetPostTags()
			state += 1
			storage.SetMigrationState(state, 0)
		}
		if state == 2 {
			msg <- "Updating tag history checkpoint..."
			hist, err := api.ListTagHistory(user, api_key, 1, nil, nil)
			if err != nil || len(hist) == 0 {
				if err == nil { err = errors.New("Got zero tag history entries from a successful request!") }
				msg <- html.EscapeString(fmt.Sprintf("An error occurred: %s", err.Error()))
				return
			}

			storage.SetTagHistoryCheckpoint(hist[0].Id)
			checkpoint = hist[0].Id

			state += 1
			storage.SetMigrationState(state, 0)
		}
		if state == 3 {
			msg <- "Downloading visible posts..."
			api_timeout := time.NewTicker(750 * time.Millisecond)
			posts := make(chan types.TSearchResult)

			defer api_timeout.Stop()
			go storage.PostUpdater(posts, false)

			before := 0
			cont := true
			max_post_id := 0
			var list types.TResultArray
			var err error

			for {
				list, cont, before, err = api.ListOnePageOfPosts(user, api_key, before)
				if err != nil {
					fmt.Printf("Error when calling api: %s", err.Error())
				}

				for i, p := range list {
					if max_post_id < p.Id {
						max_post_id = p.Id
					}
					if i == 0 { sfx <- Percent(max_post_id - p.Id, max_post_id) }
					posts <- p
				}

				if !cont { break }
				<- api_timeout.C
			}

			close(posts)
			sfx <- "done."
			state += 1
			storage.SetMigrationState(state, 0)
		}
		if state == 4 {
			msg <- "Downloading deleted posts..."
			api_timeout := time.NewTicker(750 * time.Millisecond)
			posts := make(chan types.TSearchResult)

			defer api_timeout.Stop()
			go storage.PostUpdater(posts, false)

			page := 1
			cont := true
			max_post_id := 0
			var list types.TResultArray
			var err error

			for {
				list, cont, page, err = api.ListOnePageOfDeletedPosts(user, api_key, page)
				if err != nil {
					fmt.Printf("Error when calling api: %s", err.Error())
				}

				for i, p := range list {
					if max_post_id < p.Id {
						max_post_id = p.Id
					}
					if i == 0 { sfx <- fmt.Sprintf(" Page %d", page) }
					posts <- p
				}

				if !cont { break }
				<- api_timeout.C
			}

			close(posts)
			sfx <- " done."
			state += 1
			storage.SetMigrationState(state, 0)
		}
		if state == 5 {
			msg <- "Scanning post gaps for stragglers..."
			api_timeout := time.NewTicker(750 * time.Millisecond)
			posts := make(chan types.TSearchResult)

			defer api_timeout.Stop()
			go storage.PostUpdater(posts, false)

			gap_ids, err := storage.FindPostGaps()
			if (err != nil) {
				msg <- fmt.Sprintf("An error occurred when fetching post gaps: %s", err.Error())
				return
			}

			for i, id := range gap_ids {
				post, err := api.FetchOnePost(user, api_key, id)
				if err != nil {
					fmt.Printf("Error when calling api: %s", err.Error())
				}

				if post != nil {
					posts <- *post
				}

				sfx <- Percent(i, len(gap_ids))

				<- api_timeout.C
			}

			close(posts)
			sfx <- " done."
			state += 1
			storage.SetMigrationState(state, 0)
		}
		if state == 6 {
			SyncTags(ctx, false, msg, sfx)
			state += 1
			storage.SetMigrationState(state, 0)
		}
		if state == 7 {
			msg <- "Resolving post tags..."
			err := storage.ImportPostTagsFromNameToID(sfx)
			if (err != nil) {
				msg <- fmt.Sprintf("An error occurred when normalizing post tags: %s", err.Error())
				return
			}
			state += 1
			storage.SetMigrationState(state, 0)
		}
		sfx <- " done."
		storage.SetMigrationState(0, 0)
	}

	msg <- "Fetching history delta since last checkpoint..."
	history_updates := make(map[int]int)

	posts := make(chan types.TSearchResult)
	go storage.PostUpdater(posts, true)

	var new_checkpoint int
	var last *int
	for {
		history, err := api.ListTagHistory(user, api_key, 10000, nil, last)
		if len(history) == 0 || err != nil {
			sfx <- fmt.Sprintf(" (error: %s)", err.Error())
			return
		}

		if new_checkpoint == 0 { new_checkpoint = history[0].Id }

		var h types.TTagHistory
		for _, h = range history {
			if h.Id <= checkpoint { break }

			_, ok := history_updates[h.Post_id]
			if ok { continue }

			history_updates[h.Post_id] = h.Post_id
			posts <- types.TSearchResult{Id: h.Post_id, Tags: h.Tags}
		}
		last = &h.Id
		sfx <- Percent(new_checkpoint - *last, new_checkpoint - checkpoint)
		if h.Id == checkpoint { break }
	}
	close(posts)

	SyncTags(ctx, false, msg, sfx)

	msg <- "Resolving post tags..."
	err = storage.ImportPostTagsFromNameToID(sfx)
	if (err != nil) {
		msg <- fmt.Sprintf("An error occurred when normalizing post tags: %s", err.Error())
		return
	}

	msg <- "Writing new checkpoint..."
	storage.SetTagHistoryCheckpoint(new_checkpoint)

	RecountTags(ctx, msg, sfx)
	CalculateAliasedCounts(ctx, msg, sfx)
	sfx <- " done."
}

func UpdateAliases(ctx *gogram.MessageCtx) {
	user, api_key, janitor, err := storage.GetUserCreds(ctx.Msg.From.Id)
	if err != nil || !janitor {
		ctx.ReplyAsync(data.OMessage{Text: "You need to be logged in to " + api.ApiName + " to use this command (see <code>/help login</code>)", ParseMode: data.HTML}, nil)
		return
	}

	sent_message, err := ctx.Reply(data.OMessage{Text: DownloadMessage(0, 0, "alias"), ParseMode: data.HTML})
	if err != nil {
		ctx.Bot.ErrorLog.Printf("Couldn't send message to update requestor. ???")
		return
	}

	list, cont, page, err := api.ListOnePageOfAliases(user, api_key, 1, nil)
	if err != nil {
		fmt.Printf("Error when calling api: %s", err.Error())
	}
	if len(list) != 0 {
		for cont {
			if page % 5 == 0 { // every 5 pages, update the percentage
				sent_message.EditTextAsync(data.OMessage{Text: DownloadMessage(list[len(list) - 1].Id, list[0].Id, "alias"), ParseMode: data.HTML}, nil)
			}
			time.Sleep(750 * time.Millisecond) // 750ms per request, just above the hard limit of 500
			list, cont, page, err = api.ListOnePageOfAliases(user, api_key, page, list)
		}
		sent_message.EditTextAsync(data.OMessage{Text: DBMessage(false, "alias"), ParseMode: data.HTML}, nil)
	}

	storage.ClearAliasIndex()

	if len(list) != 0 {
		fixed_aliases := make(chan types.TAliasData)
		go storage.AliasUpdater(fixed_aliases)

		last_id := list[0].Id + 1
		for _, x := range list {
			// entries should be fetched in descending order by ID but because of the non-atomicity of the fetching process
			// duplicate runs of entries can exist, and you can weed them out by ignoring entries where the ID suddenly jumps back upwards.
			if x.Id < last_id {
				fixed_aliases <- x
				last_id = x.Id
			}
		}
		close(fixed_aliases)
	}
	sent_message.EditTextAsync(data.OMessage{Text: DBMessage(true, "alias"), ParseMode: data.HTML}, nil)
}

const (
	STATE_READY = 0
	STATE_COUNT = 1
)

func Percent(current, max int) (string) {
	return fmt.Sprintf(" (%.1f%%)", float32(current * 100) / float32(max))
}

func RecountNegativeReal(user, api_key string, skip_update bool, broken_tags types.TTagInfoArray, message string, sent_message *gogram.MessageCtx) (string) {
	for i, tag := range broken_tags {
		if i % 5 == 0 { // every 5 pages, update the percentage
			sent_message.EditTextAsync(data.OMessage{Text: fmt.Sprintf("%s (%s)...", message, Percent(i, len(broken_tags))), ParseMode: data.HTML}, nil)
		}
		err := api.FixPostcountForTag(user, api_key, tag.Name)
		if err != nil {
			sent_message.Bot.ErrorLog.Printf("Error jiggling tag: %s\n", err.Error())
			continue
		}
		time.Sleep(750 * time.Millisecond)
	}


	if !skip_update {
		// you have to wait a bit for the number to update, so force a delay.
		time.Sleep(3250 * time.Millisecond)

		fixed_tags := make(chan types.TTagData)
		go storage.TagUpdater(fixed_tags)

		message = fmt.Sprintf("%s (done).\nRefetching jiggled tags", message)

		for i, tag := range broken_tags {
			if i % 5 == 0 { // every 5 pages, update the percentage
				sent_message.EditTextAsync(data.OMessage{Text: fmt.Sprintf("%s (%s)...", message, Percent(i, len(broken_tags))), ParseMode: data.HTML}, nil)
			}
			td, err := api.GetTagData(user, api_key, tag.Id)
			if err != nil {
				sent_message.Bot.ErrorLog.Printf("Error updating tag: %s\n", err.Error())
				continue
			}
			time.Sleep(750 * time.Millisecond)

			fixed_tags <- *td
		}
		close(fixed_tags)
	}

	message = fmt.Sprintf("%s (done).\n", message)
	sent_message.EditTextAsync(data.OMessage{Text: message, ParseMode: data.HTML}, nil)
	return message
}

func RecountNegative(ctx *gogram.MessageCtx) {
	user, api_key, janitor, err := storage.GetUserCreds(ctx.Msg.From.Id)
	if err != nil || !janitor {
		ctx.ReplyAsync(data.OMessage{Text: "You need to be logged in to " + api.ApiName + " to use this command (see <code>/help login</code>)", ParseMode: data.HTML}, nil)
		return
	}

	skip_update := false
	aliased := false
	count := 0
	state := STATE_READY

	for _, token := range ctx.Cmd.Args {
		if state == STATE_COUNT {
			temp, err := strconv.Atoi(token)
			if err == nil { count = temp }
			state = STATE_READY
		} else if token == "--skipupdate" {
			skip_update = true
		} else if token == "--lessthan" {
			state = STATE_COUNT
		} else if token == "--aliased" {
			aliased = true
		}
	}

	var broken_tags types.TTagInfoArray
	m := ""
	if aliased {
		broken_tags, err = storage.GetAliasedTags()
		m = fmt.Sprintf("Jiggling tags that are aliases of other tags")
	} else {
		broken_tags, err = storage.GetTagsWithCountLess(count)
		m = fmt.Sprintf("Jiggling tags with (count &lt; %d)", count)
	}
	if err != nil {
		ctx.Bot.ErrorLog.Printf("Couldn't enumerate tags.")
		return
	}

	sent_message, err := ctx.Reply(data.OMessage{Text: fmt.Sprintf("%s...", m), ParseMode: data.HTML})
	if err != nil {
		ctx.Bot.ErrorLog.Printf("Couldn't send message to update requestor. %s\n", err.Error())
		return
	}

	RecountNegativeReal(user, api_key, skip_update, broken_tags, m, sent_message)
}

const (
	MINLENGTH = 4
	POPFACTOR = 250
	TOLERANCE = 2
)

func Traverse(words []string, solutions chan []string, so_far []string, combined string) {
	so_far = append(so_far, "")
	for _, v := range words {
		if v == so_far[0] { continue } // skip the identity tag
		so_far[len(so_far) - 1] = v
		//time.Sleep(10 * time.Millisecond)
		str1 := combined + v
		str2, leftovers := wordset.Utf8Split(so_far[0], utf8.RuneCountInString(str1))
		distance := wordset.Levenshtein(str1, str2)
		tolerance := TOLERANCE
		if utf8.RuneCountInString(str1) / MINLENGTH < tolerance { tolerance = utf8.RuneCountInString(str1) / MINLENGTH }
		if distance <= tolerance {
			if (len(leftovers) == 0 && distance <= TOLERANCE) ||
			   (utf8.RuneCountInString(leftovers) + distance <= TOLERANCE + 1 && wordset.Levenshtein(str1, so_far[0]) <= TOLERANCE) {
				var out []string
				for _, x := range so_far { out = append(out, x) }
				solutions <- out
				log.Printf("CANDIDATE SOLUTION: %v", out)
			} else {
				log.Printf("Trying: %v", so_far)
				Traverse(words, solutions, so_far, str1)
			}
		}
	}
}

func FindTagConcatenations(ctx *gogram.MessageCtx) {
	tags, err := storage.EnumerateAllTags(storage.EnumerateControl{})
	if err != nil {
		log.Printf("Error enumerating all tags: %s", err.Error())
		return
	}

	var long_general_tags []string
	var long_tags []string
	for _, t := range tags {
		if len(t.Name) == 0 { continue }
		if utf8.RuneCountInString(t.Name) >= MINLENGTH && t.Type == types.General {
			long_general_tags = append(long_general_tags, t.Name)
		}
		if utf8.RuneCountInString(t.Name) >= MINLENGTH {
			long_tags = append(long_tags, t.Name)
		}
	}

	solutions := make(chan []string)

	go func(){
		so_far := []string{""}
		for _, v := range long_general_tags {
			so_far[0] = v
			Traverse(long_tags, solutions, so_far, "")
		}
	}()

	for _ = range solutions {
	}
}

func CalculateAliasedCounts(ctx *gogram.MessageCtx, msg, sfx chan string) (error) {
	m := "Mapping counts between aliased tags..."
	if msg == nil || sfx == nil {
		msg, sfx = ProgressMessage(ctx, m, "")
		defer close(msg)
		defer close(sfx)
	} else {
		msg <- m
	}

	err := storage.RecalculateAliasedCounts()
	if err != nil {
		sfx <- fmt.Sprintf(" (error: %s)", err.Error())
	} else {
		sfx <- " done."
	}

	return err
}

const (
	MODE_READY = iota
	MODE_DISTINCT = iota
	MODE_EXCLUDE = iota
	MODE_INCLUDE = iota
	MODE_THRESHHOLD = iota
)

type TagEditBox struct {
	EditDistance int
	Tag types.TTagData
}

func NewTagsFromOldTags(oldtags string, deltags, addtags map[string]bool) (string) {
	var tags []string
	for _, tag := range strings.Split(oldtags, " ") {
		found := deltags[tag] || addtags[tag]
		if tag == "" || found { continue }
		tags = append(tags, tag)
	}
	for tag, v := range addtags {
		if !v { continue }
		tags = append(tags, tag)
	}
	return strings.Join(tags, " ")
}

func FindTagTypos(ctx *gogram.MessageCtx) {
	mode := MODE_READY
	var distinct, include, exclude []string
	var threshhold int
	var save, allow_short, fix, show_zero bool
	var start_tag string
	results := make(map[string]TagEditBox)

	for _, token := range ctx.Cmd.Args {
		token = strings.ToLower(token)
		if mode == MODE_DISTINCT {
			distinct = append(distinct, token)
		} else if mode == MODE_EXCLUDE {
			exclude = append(exclude, token)
		} else if mode == MODE_INCLUDE {
			include = append(include, token)
		} else if mode == MODE_THRESHHOLD {
			t, err := strconv.Atoi(token)
			if err == nil { threshhold = t }
		} 

		if mode != MODE_READY {
			mode = MODE_READY
		} else {
			if token == "--distinct" {
				mode = MODE_DISTINCT
			} else if token == "--exclude" {
				mode = MODE_EXCLUDE
			} else if token == "--include" {
				mode = MODE_INCLUDE
			} else if token == "--threshhold" {
				mode = MODE_THRESHHOLD
			} else if token == "--save" {
				save = true
			} else if token == "--show-zero" {
				show_zero = true
			} else if token == "--allow-short" {
				allow_short = true
			} else if token == "--fix" {
				fix = true
			} else {
				start_tag = token
			}
		}
	}

	user, api_key, janitor, err := storage.GetUserCreds(ctx.Msg.From.Id)
	if (err != nil || !janitor) && fix {
		ctx.ReplyAsync(data.OMessage{Text: "You need to be logged in to " + api.ApiName + " to use this command (see <code>/help login</code>)", ParseMode: data.HTML}, nil)
		return
	}

	if start_tag == "" {
		ctx.ReplyAsync(data.OMessage{Text: "You must specify a tag.", ParseMode: data.HTML}, nil)
		return
	}

	temp1 := utf8.RuneCountInString(start_tag)
	if threshhold == 0 {
		if temp1 < 8 {
			threshhold = 1
		} else if temp1 < 16 {
			threshhold = 2
		} else if temp1 < 32 {
			threshhold = 3
		} else {
			threshhold = 4
		}
	}

	t1, err := storage.GetTag(ctx.Cmd.Args[0], storage.EnumerateControl{CreatePhantom: true})
	if err != nil { log.Printf("Error occurred when looking up tag: %s", err.Error()) }
	if t1 == nil {
		ctx.ReplyAsync(data.OMessage{Text: fmt.Sprintf("Tag doesn't exist: %s.", start_tag)}, nil)
		return
	}

	msg, sfx := ProgressMessage(ctx, "Checking for duplicates...", "(enumerate tags)")

	tags, _ := storage.EnumerateAllTags(storage.EnumerateControl{OrderByCount: true})

	for _, t2 := range tags {
		if t1.Name == t2.Name { continue } // skip tag = tag

		// shortest name first (in terms of codepoints)
		var t1n, t2n string
		var t1l, t2l int
		temp2 := utf8.RuneCountInString(t2.Name)
		if temp1 > temp2 {
			t1n, t2n = t2.Name, t1.Name
			t1l, t2l = temp2, temp1
		} else {
			t1n, t2n = t1.Name, t2.Name
			t1l, t2l = temp1, temp2
		}

		if t1l < 3 && !allow_short { continue } // skip short tags.

		// the length difference is a lower bound on the edit distance so if the lengths are too dissimilar, skip.
		if t2l - t1l > threshhold { continue }

		// check the edit distance and bail if it's not low.
		distance := wordset.Levenshtein(t1n, t2n)
		if distance > threshhold { continue }

		// these tags are similar!
		results[t2.Name] = TagEditBox{Tag: t2, EditDistance: distance}
	}

	// remove any matches that were manually excluded.
	sfx <- "(remove excluded)"
	for _, item := range exclude {
		delete(results, item)
	}

	// now remove any matches which are more closely matched by the distinct list.
	sfx <- "(remove distinct)"
	for _, item := range distinct {
		for k, v := range results {
			if wordset.Levenshtein(item, k) < v.EditDistance { delete(results, k) }
		}
	}

	// now remove any matches which are already aliased to the target tag.
	sfx <- "(remove aliases)"
	aliases, err := storage.GetAliasesFor(start_tag)
	if err != nil { log.Printf("Error when searching for aliases to %s: %s", start_tag, err.Error()) }
	for _, item := range aliases {
		delete(results, item.Name)
	}

	// aaaaaand finally add any matches manually included.
	sfx <- "(merge included)"
	for _, item := range include {
		t, _ := storage.GetTag(item, storage.EnumerateControl{})
		if t != nil { results[item] = TagEditBox{Tag: *t, EditDistance: wordset.Levenshtein(t1.Name, t.Name)} }
	}

	sfx <- "done."

	total_posts := 0
	for _, v := range results {
		total_posts += v.Tag.Count
	}

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Possible typos of <code>%s</code>: %d (%d estimated posts)\n<pre>", html.EscapeString(start_tag), len(results), total_posts))
	for _, v := range results {
		alert := "  "
		if !show_zero && v.Tag.Count == 0 { continue }
		if v.Tag.Type != types.General { alert = "!!" }
		buf.WriteString(fmt.Sprintf("%6d %s %s\n", v.Tag.Count, alert, html.EscapeString(v.Tag.Name)))
	}
	buf.WriteString("</pre>")
	if fix {
		buf.WriteString("\nFixing tags...")
	}
	msg <- buf.String()

	if fix {
		updated := 1
		api_timeout := time.NewTicker(750 * time.Millisecond)
		for _, v := range results {
			posts, err := storage.LocalTagSearch(v.Tag)
			if err != nil {
				sfx <- fmt.Sprintf(" (error: %s)", err.Error())
				return
			}

			reason := fmt.Sprintf("Bulk retag: %s --> %s (likely typo)", v.Tag.Name, start_tag)
			for _, p := range posts {
				newtags := NewTagsFromOldTags(p.Tags, map[string]bool{v.Tag.Name: true}, map[string]bool{start_tag: true})
				newp, err := api.UpdatePost(user, api_key, p.Id, &p.Tags, &newtags, nil, nil, nil, nil, &reason)
				err = nil
				if err != nil {
					sfx <- fmt.Sprintf(" (error: %s)", err.Error())
					return
				}

				if newp != nil {
					err = storage.UpdatePost(p, *newp)
					if err != nil {
						sfx <- fmt.Sprintf(" (error: %s)", err.Error())
						return
					}
				}

				sfx <- fmt.Sprintf(" (%d/%d %d: <code>%s</code> -> <code>%s</code>)", updated, total_posts, p.Id, v.Tag.Name, start_tag)

				<- api_timeout.C
				updated++
			}
		}
		sfx <- " done."
		api_timeout.Stop()
	}

	_ = save
}

func Blits(ctx *gogram.MessageCtx) {
	mode := MODE_READY
	include, exclude := make(map[string]bool), make(map[string]bool)

	for _, token := range ctx.Cmd.Args {
		token = strings.Replace(strings.ToLower(token), "\uFE0F", "", -1)
		if token == "--exclude" {
			mode = MODE_EXCLUDE
		} else if token == "--mark" {
			mode = MODE_INCLUDE
		} else if mode == MODE_EXCLUDE {
			exclude[token] = true
		} else if mode == MODE_INCLUDE {
			include[token] = true
		}
	}

	tags, _ := storage.EnumerateAllTags(storage.EnumerateControl{})
	var intermediate, blits types.TTagInfoArray
	for _, t := range tags {
		if utf8.RuneCountInString(t.Name) <= 2 {
			intermediate = append(intermediate, t)
		}
	}

	for _, t := range intermediate {
		if include[t.Name] {
			storage.MarkBlit(t.Id, true)
		} else if exclude[t.Name] {
			storage.MarkBlit(t.Id, false)
		}
	}
	allknownblits := make(map[int]bool)
	for _, b := range storage.GetMarkedAndUnmarkedBlits() {
		allknownblits[b] = true
	}

	for _, b := range intermediate {
		if !allknownblits[b.Id] { blits = append(blits, b) }
	}

	sort.Slice(blits, func(i, j int) (bool) {
		return blits[i].Count > blits[j].Count
	})

	var buf bytes.Buffer
	for _, t := range blits {
		if t.Count <= 0 { continue }
		tagtype := "UNKNOWN"
		if t.Type == types.General {
			tagtype = "GENERAL"
		} else if t.Type == types.Species {
			tagtype = "SPECIES"
		} else if t.Type == types.Artist {
			tagtype = "ARTIST "
		} else if t.Type == types.Copyright {
			tagtype = "CPYRIGT"
		} else if t.Type == types.Character {
			tagtype = "CHARCTR"
		}
		newstr := fmt.Sprintf("%5d (%s) %s\n", t.Count, tagtype, t.Name)
		if len(newstr) + buf.Len() > 4096 - 12 { break }
		buf.WriteString(html.EscapeString(newstr))
	}
	ctx.ReplyAsync(data.OMessage{Text: "<pre>" + buf.String() + "</pre>", ParseMode: data.HTML}, nil)
}

type Triplet struct {
	tag, subtag1, subtag2 types.TTagData
}

func Concatenations(ctx *gogram.MessageCtx) {
	tags, _ := storage.EnumerateAllTags(storage.EnumerateControl{})
	tagmap := make(map[string]types.TTagData, len(tags))
	var candidates []Triplet
	for _, t := range tags {
		tagmap[t.Name] = t
	}

	for k, v := range tagmap {
		if v.Count == 0 { continue } // skip anything with no posts.
		runes := []rune(k)
		for i := 1; i < len(runes) - 1; i++ {
			prefix, prefix_ok := tagmap[string(runes[:i])]
			suffix, suffix_ok := tagmap[string(runes[i:])]
			if prefix_ok && suffix_ok && 10*v.Count < prefix.Count && 10*v.Count < suffix.Count && v.Type == types.General {
				candidates = append(candidates, Triplet{tag: v, subtag1: prefix, subtag2: suffix})
			}
		}
	}

	sort.Slice(candidates, func(i, j int) (bool) {
		return len(candidates[i].tag.Name) > len(candidates[j].tag.Name)
	})

	for i, triplet := range candidates {
		if i > 500 { break }
		log.Printf("%7d %s: %s + %s\n", triplet.tag.Count, triplet.tag.Name, triplet.subtag1.Name, triplet.subtag2.Name)
	}
}
