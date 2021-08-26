package tagindex

import (
	"github.com/thewug/gogram"
	"github.com/thewug/gogram/data"
	"github.com/meirf/gopart"

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
	"math/rand"
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

	err := SyncTags(ctx, storage.UpdaterSettings{Full: full}, nil, nil)
	if err == storage.ErrNoLogin {
		ctx.ReplyOrPMAsync(data.OMessage{Text: "You need to be logged in to " + api.ApiName + " to use this command (see <code>/help login</code>)", ParseMode: data.HTML}, nil)
		return
	} else if err != nil {
		log.Println("Error occurred syncing tags: %s", err.Error())
	}
}

func SyncTags(ctx *gogram.MessageCtx, settings storage.UpdaterSettings, msg, sfx chan string) (error) {
	user, api_key, janitor, err := storage.GetUserCreds(storage.UpdaterSettings{}, ctx.Msg.From.Id)
	if err != nil || !janitor { return err }

	m := "Syncing tag database..."
	if settings.Full { m = "Full syncing tag database..." }
	if msg == nil || sfx == nil {
		msg, sfx = ProgressMessage(ctx, m, "")
		defer close(msg)
		defer close(sfx)
	} else {
		msg <- m
	}

	if settings.Full {
		storage.ClearTagIndex(settings)
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
	last, err := storage.GetLastTag(settings)
	if last != nil { last_existing_tag_id = last.Id }

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		storage.TagUpdater(fixed_tags, settings)
		wg.Done()
	}()
	defer api_timeout.Stop()
	defer storage.FixTagsWhichTheAPIManglesByAccident(settings)

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
	storage.ResolvePhantomTags(settings)

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

const (
	SYNC_BEGIN = 0
	SYNC_BEGIN_FULL = iota
	SYNC_BEGIN_INCREMENTAL = iota

	SYNC_RESETSTAGING = iota
	SYNC_TAGCHECKPOINT = iota
	SYNC_VISIBLE = iota
	SYNC_DELETED = iota
	SYNC_GHOSTED = iota
	SYNC_INCREMENTAL_HISTORY = iota
	SYNC_TAGS = iota
	SYNC_TAGNORMALIZE = iota

	SYNC_STAGEPROMOTE = iota

	SYNC_DONE = iota
	SYNC_ERROR = iota
	SYNC_BREAK = iota
)

type SearchChanBox struct {
	Channel chan types.TSearchResult
}

func (this *SearchChanBox) Close() {
	close(this.Channel)
}

func SyncPosts(ctx *gogram.MessageCtx) {
	user, api_key, janitor, err := storage.GetUserCreds(storage.UpdaterSettings{}, ctx.Msg.From.Id)
	if err != nil || !janitor {
		ctx.ReplyAsync(data.OMessage{Text: "You need to be logged in to " + api.ApiName + " to use this command (see <code>/help login</code>)", ParseMode: data.HTML}, nil)
		return
	}

	settings := storage.UpdaterSettings{}
	for _, token := range ctx.Cmd.Args {
		if token == "--full" {
			settings.Full = true
		}
	}

	msg, sfx := ProgressMessage(ctx, "Syncing post database...", "")
	defer close(msg)
	defer close(sfx)

	state, substate := SYNC_BEGIN, 0
	checkpoint, err := storage.GetTagHistoryCheckpoint()

	if err != nil {
		m := fmt.Sprintf("Error during storage.GetTagHistoryCheckpoint: %s", err.Error())
		ctx.Bot.ErrorLog.Println(m)
		msg <- m
		return
	}

	// we don't need to do a full sync if one wasn't requested, or if we have a saved checkpoint.
	if !settings.Full && checkpoint != 0 {
		// attempt to resume a sync in progress
		state, substate, settings.Full = storage.GetMigrationState(settings)

		// if we're resuming a full sync, bodge in the correct staging suffix.
		if settings.Full {
			settings.TableSuffix = "staging"
		}
	}

	for {
		switch state {

		// beginning states (non-resumes SHOULD start in one of these)
		case SYNC_BEGIN:
			if settings.Full {
				state = SYNC_BEGIN_FULL
			} else {
				state = SYNC_BEGIN_INCREMENTAL
			}
			storage.SetMigrationState(settings, state, 0, settings.Full)
		case SYNC_BEGIN_FULL:
			settings.TableSuffix = "staging"
			state = SYNC_RESETSTAGING
			storage.SetMigrationState(settings, state, 0, settings.Full)
		case SYNC_BEGIN_INCREMENTAL:
			state = SYNC_INCREMENTAL_HISTORY
			storage.SetMigrationState(settings, state, 0, settings.Full)

		// full sync states
		case SYNC_RESETSTAGING:
			msg <- "Preparing sync environment..."
			storage.SetupSyncStagingEnvironment(settings)
			state = SYNC_TAGCHECKPOINT
			storage.SetMigrationState(settings, state, 0, settings.Full)
		case SYNC_TAGCHECKPOINT:
			msg <- "Updating tag history checkpoint..."
			hist, err := api.ListTagHistory(user, api_key, 1, nil, nil)
			if err != nil || len(hist) == 0 {
				if err == nil { err = errors.New("Got zero tag history entries from a successful request!") }
				msg <- html.EscapeString(fmt.Sprintf("An error occurred: %s", err.Error()))
				return
			}

			storage.SetTagHistoryCheckpoint(hist[0].Id, settings)
			checkpoint = hist[0].Id

			state = SYNC_VISIBLE
			storage.SetMigrationState(settings, state, 0, settings.Full)
		case SYNC_VISIBLE:
			success := func() bool {
			msg <- "Downloading visible posts..."
			api_timeout := time.NewTicker(750 * time.Millisecond)

			defer api_timeout.Stop()

			before := substate
			cont := true
			max_post_id := storage.GetHighestStagedPostID(settings)
			var list types.TResultArray
			var err error
			var wg sync.WaitGroup

			for {
				list, cont, before, err = api.ListOnePageOfPosts(user, api_key, before)
				if err != nil {
					m := fmt.Sprintf("Error when calling api: %s", err.Error())
					msg <- m
					ctx.Bot.ErrorLog.Println(m)
					return false
				}

				posts := make(chan types.TSearchResult)
				wg.Add(1)
				go func(_state, _substate int) {
					storage.PostUpdater(posts, settings)
					wg.Done()
					storage.SetMigrationState(settings, _state, _substate, settings.Full)
				}(state, before)

				for i, p := range list {
					if max_post_id < p.Id {
						max_post_id = p.Id
					}
					if i == 0 { sfx <- Percent(max_post_id - p.Id, max_post_id) }
					posts <- p
				}

				close(posts)

				if !cont { break }
				<- api_timeout.C
			}

			sfx <- "done."
			wg.Wait()
			state = SYNC_DELETED
			substate = 0
			storage.SetMigrationState(settings, state, 0, settings.Full)
			return true
			}()
			if !success { state = SYNC_ERROR }
		case SYNC_DELETED:
			success := func() bool {
			msg <- "Downloading deleted posts..."
			api_timeout := time.NewTicker(750 * time.Millisecond)

			defer api_timeout.Stop()

			page := substate
			if page == 0 { page = 1 }
			cont := true
			max_post_id := 0
			var list types.TResultArray
			var err error
			var wg sync.WaitGroup

			for {
				list, cont, page, err = api.ListOnePageOfDeletedPosts(user, api_key, page)
				if err != nil {
					m := fmt.Sprintf("Error when calling api: %s", err.Error())
					msg <- m
					ctx.Bot.ErrorLog.Println(m)
					return false
				}

				posts := make(chan types.TSearchResult)
				wg.Add(1)
				go func(_state, _substate int) {
					storage.PostUpdater(posts, settings)
					wg.Done()
					storage.SetMigrationState(settings, _state, _substate, settings.Full)
				}(state, page)

				for i, p := range list {
					if max_post_id < p.Id {
						max_post_id = p.Id
					}
					if i == 0 { sfx <- fmt.Sprintf(" Page %d", page) }
					posts <- p
				}

				close(posts)

				if !cont { break }
				<- api_timeout.C
			}

			sfx <- " done."
			wg.Wait()
			state = SYNC_GHOSTED
			substate = 0
			storage.SetMigrationState(settings, state, 0, settings.Full)
			return true
			}()
			if !success { state = SYNC_ERROR }
		case SYNC_GHOSTED:
			// this process is intrinsically resumable and no special logic is needed to accomplish that
			success := func() bool {
			msg <- "Scanning post gaps for stragglers..."
			api_timeout := time.NewTicker(750 * time.Millisecond)

			defer api_timeout.Stop()

			gap_ids, err := storage.FindPostGaps(substate, settings)
			if (err != nil) {
				m := fmt.Sprintf("An error occurred when fetching post gaps: %s", err.Error())
				msg <- m
				ctx.Bot.ErrorLog.Println(m)
				return false
			}

			var wg sync.WaitGroup

			for partition := range gopart.Partition(len(gap_ids), 10) {
				posts := make(chan types.TSearchResult)
				wg.Add(1)
				go func(_state, _substate int) {
					storage.PostUpdater(posts, settings)
					wg.Done()
					storage.SetMigrationState(settings, _state, _substate, settings.Full)
				}(state, gap_ids[partition.High - 1])

				for i, id := range gap_ids[partition.Low: partition.High] {
					post, err := api.FetchOnePost(user, api_key, id)
					if err != nil {
						m := fmt.Sprintf("Error when calling api: %s", err.Error())
						msg <- m
						ctx.Bot.ErrorLog.Println(m)
						close(posts)
						return false
					}

					if post != nil {
						posts <- *post
					}

					sfx <- Percent(i + partition.Low, len(gap_ids))

					<- api_timeout.C
				}

				close(posts)
			}

			sfx <- " done."
			wg.Wait()
			state = SYNC_INCREMENTAL_HISTORY
			substate = 0
			storage.SetMigrationState(settings, state, 0, settings.Full)
			return true
			}()
			if !success { state = SYNC_ERROR }

		// incremental sync states
		case SYNC_INCREMENTAL_HISTORY:
			success := func() bool {
			msg <- "Fetching history delta since last checkpoint..."
			api_timeout := time.NewTicker(750 * time.Millisecond)
			var wg sync.WaitGroup

			defer api_timeout.Stop()

			history_updates := make(map[int]int)

			var new_checkpoint int
			var last *int
			if substate != 0 { last = &substate }
			for {
				history, err := api.ListTagHistory(user, api_key, 10000, nil, last)
				if len(history) == 0 { err = errors.New("api.ListTagHistory produced an empty list") }
				if err != nil {
					m := fmt.Sprintf("Error when calling api: %s", err.Error())
					msg <- m
					ctx.Bot.ErrorLog.Println(m)
					return false
				}

				last = &history[len(history) - 1].Id

				posts := make(chan types.TSearchResult)
				wg.Add(1)
				go func(_state, _substate int) {
					storage.PostUpdater(posts, settings)
					wg.Done()
					storage.SetMigrationState(settings, _state, _substate, settings.Full)
				}(state, *last)

				if new_checkpoint == 0 { new_checkpoint = history[0].Id }

				var h types.TTagHistory
				for _, h = range history {
					if h.Id <= checkpoint { break }

					_, ok := history_updates[h.Post_id]
					if ok { continue }

					history_updates[h.Post_id] = h.Post_id
					posts <- types.TSearchResult{Id: h.Post_id, Tags: h.Tags}
				}
				sfx <- Percent(new_checkpoint - *last, new_checkpoint - checkpoint)
				close(posts)
				if h.Id == checkpoint { break }

				<- api_timeout.C
			}

			sfx <- " done."

			msg <- "Writing new checkpoint..."
			storage.SetTagHistoryCheckpoint(new_checkpoint, settings)
			wg.Wait()
			state = SYNC_TAGS
			return true
			}()
			if !success { state = SYNC_ERROR }
		case SYNC_TAGS:
			SyncTags(ctx, settings, msg, sfx)
			state = SYNC_TAGNORMALIZE
			storage.SetMigrationState(settings, state, 0, settings.Full)
		case SYNC_TAGNORMALIZE:
			msg <- "Resolving post tags..."
			err := storage.ImportPostTagsFromNameToID(settings, sfx)
			if (err != nil) {
				m := fmt.Sprintf("An error occurred when normalizing post tags: %s", err.Error())
				msg <- m
				ctx.Bot.ErrorLog.Println(m)
				state = SYNC_BREAK
				break
			}
			state = SYNC_STAGEPROMOTE
			storage.SetMigrationState(settings, state, 0, settings.Full)

		// promotion stages
		case SYNC_STAGEPROMOTE:
			msg <- "Promoting staging tablespace..."
			// this is a no-op if you're not using a staging environment, e.g. if you do an incremental
			// sync by itself (which is instead clumped into one giant transaction)
			err := storage.ResetIntermediateEnvironment(settings)
			if err != nil {
				err = storage.PromoteStagingEnvironment(settings)
			}
			if (err != nil) {
				m := fmt.Sprintf("An error occurred when promoting stage space: %s", err.Error())
				msg <- m
				ctx.Bot.ErrorLog.Println(m)
				state = SYNC_BREAK
				break
			}
			state = SYNC_DONE
			storage.SetMigrationState(settings, state, 0, settings.Full)

		// finish-success stage
		case SYNC_DONE:
			RecountTags(ctx, msg, sfx)
			CalculateAliasedCounts(ctx, msg, sfx)
			sfx <- " done."
			storage.SetMigrationState(settings, SYNC_BEGIN, 0, false)
			state = SYNC_BREAK
			break

		// finish-error stage
		case SYNC_ERROR:
			// put some proper error reporting mechanism here
			state = SYNC_BREAK
			break

		}

		if state == SYNC_BREAK {
			break
		}
	}
}

func UpdateAliases(ctx *gogram.MessageCtx) {
	user, api_key, janitor, err := storage.GetUserCreds(storage.UpdaterSettings{}, ctx.Msg.From.Id)
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
	STATE_COUNT = iota
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
		go storage.TagUpdater(fixed_tags, storage.UpdaterSettings{})

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
	user, api_key, janitor, err := storage.GetUserCreds(storage.UpdaterSettings{}, ctx.Msg.From.Id)
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
	MODE_FREQ_RATIO = iota
	MODE_IGNORE = iota
	MODE_UNIGNORE = iota
	MODE_FIX = iota
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

	user, api_key, janitor, err := storage.GetUserCreds(storage.UpdaterSettings{}, ctx.Msg.From.Id)
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
	user, api_key, janitor, err := storage.GetUserCreds(storage.UpdaterSettings{}, ctx.Msg.From.Id)
	if err != nil || !janitor { return }

	var cats []Triplet
	header := "Here are some random concatenated tags:"
	if ctx.Msg.Reply_to_message != nil {
		text := ctx.Msg.Reply_to_message.Text
		if text != nil {
			prev_cats := strings.Split(*text, "\n")
			if prev_cats[0] == header {
				prev_cats = prev_cats[1:]
				for _, line := range prev_cats {
					var t Triplet
					tokens := strings.Split(line, " ")
					t.subtag1.Name = strings.TrimSuffix(tokens[1], ",")
					t.subtag2.Name = tokens[2]
					t.tag.Name = t.subtag1.Name + t.subtag2.Name
					cats = append(cats, t)
				}
			}
		}
	}

	ratio := 10
	mode := MODE_READY
	var fix_list, ignore_list []int
	var manual_ignore, manual_unignore []string

	for _, token := range ctx.Cmd.Args {
		token = strings.Replace(strings.ToLower(token), "\uFE0F", "", -1)
		if token == "--frequency-ratio" {
			mode = MODE_FREQ_RATIO
		} else if token == "--ignore" {
			mode = MODE_IGNORE
		} else if token == "--unignore" {
			mode = MODE_UNIGNORE
		} else if token == "--fix" {
			mode = MODE_FIX
		} else if mode == MODE_FREQ_RATIO {
			temp, err := strconv.Atoi(token)
			if err == nil { ratio = temp }
		} else if mode == MODE_IGNORE {
			manual_ignore = append(manual_ignore, token)
			temp, err := strconv.Atoi(token)
			if err == nil { ignore_list = append(ignore_list, temp) }
		} else if mode == MODE_UNIGNORE {
			manual_unignore = append(manual_unignore, token)
		} else if mode == MODE_FIX {
			temp, err := strconv.Atoi(token)
			if err == nil { fix_list = append(fix_list, temp) }
		}
	}

	if cats == nil {
		var message bytes.Buffer
		for _, tag := range manual_ignore {
			storage.SetCatsException(tag)
			message.WriteString(fmt.Sprintf("Adding to ignore list: <code>%s</code>\n", tag))
		}
		for _, tag := range manual_unignore {
			storage.ClearCatsException(tag)
			message.WriteString(fmt.Sprintf("Removing from ignore list: <code>%s</code>\n", tag))
		}

		if manual_ignore != nil || manual_unignore != nil {
			ctx.ReplyAsync(data.OMessage{Text: message.String(), ParseMode: data.HTML}, nil)
			return
		}

		tags, _ := storage.EnumerateAllTags(storage.EnumerateControl{})
		exceptions, _ := storage.EnumerateCatsExceptions()

		tagmap := make(map[string]types.TTagData, len(tags))
		exceptionmap := make(map[string]bool, len(exceptions))

		for _, t := range exceptions {
			exceptionmap[t] = true
		}

		for _, t := range tags {
			if !exceptionmap[t.Name] && len(t.Name) > 3 {
				tagmap[t.Name] = t
			}
		}

		var candidates []Triplet

		for k, v := range tagmap {
			if v.Count == 0 { continue } // skip anything with no posts.
			if v.Type != types.General { continue } // skip anything that's not a general tag.
			runes := []rune(k)
			for i := 1; i < len(runes) - 1; i++ {
				prefix, prefix_ok := tagmap[string(runes[:i])]
				suffix, suffix_ok := tagmap[string(runes[i:])]
				if prefix_ok && suffix_ok && ratio * v.Count < prefix.Count && ratio * v.Count < suffix.Count && v.Type == types.General {
					candidates = append(candidates, Triplet{tag: v, subtag1: prefix, subtag2: suffix})
				}
			}
		}

		message.WriteString(header + "\n")

		for i := 0; i < 10; i++ {
			cats = append(cats, candidates[rand.Intn(len(candidates))])
		}

		for i, t := range cats {
			message.WriteString(fmt.Sprintf("%d: <code>%s</code>, <code>%s</code> (%d)\n", i, t.subtag1.Name, t.subtag2.Name, t.tag.Count))
		}

		ctx.ReplyAsync(data.OMessage{Text: message.String(), ParseMode: data.HTML}, nil)
		return
	}

	msg, sfx := ProgressMessage(ctx, "", "")
	var message bytes.Buffer
	for _, i := range ignore_list {
		storage.SetCatsException(cats[i].tag.Name)
		msg <- fmt.Sprintf("Adding %d to ignore list: <code>%s</code>\n", i, cats[i].tag.Name)
	}
	msg <- "\nUpdating posts which need fixing... "

	api_timeout := time.NewTicker(750 * time.Millisecond)
	updated := 1
	for _, i := range fix_list {
		t, err := storage.GetTag(cats[i].tag.Name, storage.EnumerateControl{})
		cats[i].tag = *t
		posts, err := storage.LocalTagSearch(cats[i].tag)
		if err != nil {
			sfx <- fmt.Sprintf(" (error: %s)", err.Error())
			return
		}

		reason := fmt.Sprintf("Bulk retag: %s --> %s, %s (fixed concatenated tags)", cats[i].tag.Name, cats[i].subtag1.Name, cats[i].subtag2.Name)
		for _, p := range posts {
			newtags := NewTagsFromOldTags(p.Tags, map[string]bool{cats[i].tag.Name: true}, map[string]bool{cats[i].subtag1.Name: true, cats[i].subtag2.Name: true})
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

			sfx <- fmt.Sprintf(" (%d/%d %d: <code>%s</code> -> <code>%s</code>, <code>%s</code>)", updated, -1, p.Id, cats[i].tag.Name, cats[i].subtag1.Name, cats[i].subtag2.Name)

			<- api_timeout.C
			updated++
		}
		sfx <- " done."
		api_timeout.Stop()
		message.WriteString(fmt.Sprintf("Fixing %d: <code>%s</code> -> <code>%s, %s</code>\n", i, cats[i].tag.Name, cats[i].subtag1.Name, cats[i].subtag2.Name))
	}
	ctx.ReplyAsync(data.OMessage{Text: message.String(), ParseMode: data.HTML}, nil)
}

// func BulkSearch(searchtags string)

/*
func BulkRetag(searchtags, applytags string) {
        expression_tokens := Tokenize(searchtags)
        search_expression := Parse(expression_tokens)
        sql_format_string, sql_tokens := Serialize(search_expression)
        for k, v := range sql_tokens {
            sql_tokens[k] = pq.QuoteLiteral(v)
        }
        replace["tag_id"] = "tag_id"
        replace["tag_index"] = "tag_index"
        replace["tag_name"] = "tag_name"
        replace["temp"] = "x"
        var buf bytes.Buffer
        template.Must(template.New("decoder").Parse(sql_format_string)).Execute(&buf, sql_tokens)
        sql_substring := buf.String()

        // TODO finish this
}
*/
