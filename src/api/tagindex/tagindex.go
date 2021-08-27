package tagindex

import (
	"api"
	"api/tags"
	"api/types"

	"storage"
	"wordset"

	"github.com/thewug/gogram"
	"github.com/thewug/gogram/data"

	"bufio"
	"bytes"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
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
		sent_message, err = ctx.Reply(data.OMessage{SendData: data.SendData{Text: fmt.Sprintf("%s%s", initial_status, initial_suffix), ParseMode: data.ParseHTML}})
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
				sent_message, err = ctx.Reply(data.OMessage{SendData: data.SendData{Text: fmt.Sprintf("%s%s", message, suffix), ParseMode: data.ParseHTML}})
			} else {
				sent_message.EditTextAsync(data.OMessageEdit{SendData: data.SendData{Text: fmt.Sprintf("%s%s", message, suffix), ParseMode: data.ParseHTML}}, nil)
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

type ProgMessage struct {
	// externally accessible fields
	status, notice string
	previous string
	active string
	running bool

	// shared fields
	text_updates chan string
	initialized sync.WaitGroup
	err error

	// public shared fields. These must be initialized before anything calls.
	UpdateInterval time.Duration
	MessageCallback func(string, data.MessageParseMode) (*data.TMessage, error)
	ParseMode data.MessageParseMode
	Bot *gogram.TelegramBot

	// public shared fields which are initialized after the first push.
	Ctx *gogram.MessageCtx

	// internally accessible fields
	target, actual string
	updater <- chan time.Time
}

func (this *ProgMessage) Respin(previous, notice, status string) string {
	lenprev, lenline := len(previous), len(status)
	spacer_1, spacer_2 := ternary(lenprev > 0, "\n", ""), ternary(lenline > 0, " ", "")
	return previous + spacer_1 + notice + spacer_2 + status
}

func ternary(b bool, x, y string) string {
	if b { return x }
	return y
}

// go-routine service host for a ProgMessage
func (this *ProgMessage) run() {
	for {
		select {
		case <- this.updater:
			this.update()
		case fetched, ok := <- this.text_updates:
			if !ok {
				this.update()
				return
			}
			this.target = fetched
			if this.updater == nil {
				this.update()
			}
		}
	}
}

// an update is needed, maybe.
// if our target message is different from the last one we wrote,
// trigger an update. otherwise, reset the timer to wait for a change.
func (this *ProgMessage) update() {
	if this.target != this.actual {
		if this.Ctx == nil {
			msg, err := this.MessageCallback(this.target, this.ParseMode)
			this.Ctx = gogram.NewMessageCtx(msg, false, this.Bot)
			if this.Ctx == nil {
				this.err = err
				if this.err == nil { this.err = errors.New("ProgMessage:update(): MessageCallback() returned nil") }
				this.initialized.Done()
			} else {
				this.initialized.Done()
				this.actual = this.target
			}
		} else {
			this.Ctx.EditTextAsync(data.OMessageEdit{
				SendData: data.SendData{
					Text: this.target,
					ParseMode: this.ParseMode,
				},
			}, nil)
			this.actual = this.target
		}
		this.updater = time.NewTimer(this.UpdateInterval).C
	} else {
		this.updater = nil
	}
}

func (this *ProgMessage) Push(text string) (error) {
	if this.err != nil {
		return this.err
	} else if !this.running {
		this.running = true
		this.text_updates = make(chan string, 4)
		this.initialized.Add(1)
		go this.run()
		this.text_updates <- text
		this.initialized.Wait()
		if this.err != nil { this.Close() }
		return this.err
	} else {
		this.text_updates <- text
	}
	return nil
}

func (this *ProgMessage) AppendNotice(text string) (error) {
	if len(this.previous) > 0 {
		this.previous = this.previous + "\n" + this.notice
	} else {
		this.previous = this.notice
	}
	this.status = ""
	this.notice = text
	this.active = this.Respin(this.previous, this.notice, this.status)
	return this.Push(this.active)
}

func (this *ProgMessage) ReplaceNotice(text string) (error) {
	this.notice = text
	this.active = this.Respin(this.previous, this.notice, this.status)
	return this.Push(this.active)
}

func (this *ProgMessage) SetStatus(text string) (error) {
	this.status = text
	this.active = this.Respin(this.previous, this.notice, this.status)
	return this.Push(this.active)
}

func (this *ProgMessage) SetMessage(text string) (error) {
	this.active = text
	this.previous = ""
	this.status = ""
	this.notice = text
	return this.Push(this.active)
}

func (this *ProgMessage) Active() string {
	return this.active
}

func (this *ProgMessage) Close() {
	if this.running {
		close(this.text_updates)
		this.running = false
	}
}

func ProgressMessage2(message_factory func(string, data.MessageParseMode) (*data.TMessage, error),
		      initial_text string,
		      pm data.MessageParseMode,
		      interval time.Duration,
		      bot *gogram.TelegramBot) (*ProgMessage, error) {
	x := ProgMessage{
		UpdateInterval: interval,
		MessageCallback: message_factory,
		ParseMode: pm,
		Bot: bot,
	}

	err := x.SetMessage(initial_text)
	if err != nil { return nil, err }
	return &x, nil
}

type UserError struct {
	err string
}

func (this UserError) Error() string {
	return this.err
}

func ResyncListCommand(ctx *gogram.MessageCtx) {
	err := ResyncList(ctx, storage.UpdaterSettings{}, nil, nil)
	if err == storage.ErrNoLogin {
		ctx.ReplyOrPMAsync(data.OMessage{SendData: data.SendData{Text: "You need to be logged in to " + api.ApiName + " to use this command (see <code>/help login</code>)", ParseMode: data.ParseHTML}}, nil)
		return
	} else if err != nil {
		if u, ok := err.(UserError); ok {
			ctx.ReplyOrPMAsync(data.OMessage{SendData: data.SendData{Text: u.Error(), ParseMode: data.ParseHTML}}, nil)
		} else {
			ctx.Bot.Log.Println("Error occurred syncing tags: %s", err.Error())
		}
	}
}

func ResyncList(ctx *gogram.MessageCtx, settings storage.UpdaterSettings, msg, sfx chan string) (error) {
	user, api_key, janitor, err := storage.GetUserCreds(settings, ctx.Msg.From.Id)
	if err != nil || !janitor { return err }

	doc := ctx.Msg.Document
	if doc == nil {
		return UserError{err: "This command requires an input file."}
	}

	file, err := ctx.Bot.Remote.GetFile(data.OGetFile{Id: doc.Id})
	if err != nil {
		return UserError{err: "This command requires an input file."}
	}

	if file.FilePath == nil {
		return UserError{err: "Couldn't read file data. Maybe it's too large?"}
	}

	file_data, err := ctx.Bot.Remote.DownloadFile(data.OFile{FilePath: *file.FilePath})
	if file_data == nil || err != nil {
		return UserError{err: "Couldn't download file?"}
	}

	defer file_data.Close()

	if msg == nil || sfx == nil {
		msg, sfx = ProgressMessage(ctx, "", "")
		defer close(msg)
		defer close(sfx)
	}

	return ResyncListInternal(user, api_key, settings, file_data, msg, sfx)
}


func ResyncListInternal(user, api_key string, settings storage.UpdaterSettings, file_data io.Reader, msg, sfx chan string) (error) {
	message := func(x string) {
		if msg != nil {
			msg <- x
		}
	}
	suffix := func(x string) {
		if sfx != nil {
			sfx <- x
		}
	}

	message("Updating posts from list...")

	idpipe := make(chan string)
	go func() {
		buf := bufio.NewReader(file_data)
		for {
			str, err := buf.ReadString('\n')
			tokens := strings.Split(str, " ")
			for _, tok := range tokens {
				tok = strings.TrimSpace(tok)
				// everything after a token beginning in a hash sign is a comment.
				if strings.HasPrefix(tok, "#") { break }

				// errors are silently discarded, and valid input continues to be processed.
				_, e := strconv.Atoi(tok)
				if e == nil { idpipe <- tok }
			}
			if err == io.EOF { break }
		}
		close(idpipe)
	} ()


	fixed_posts := make(chan types.TPostInfo)

	limit := 100
	consecutive_errors := 0

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		err := storage.PostUpdater(fixed_posts, settings)
		wg.Done()
		if err != nil { log.Println(err.Error()) }
	}()

	var ids []string
	for {
		id, ok := <- idpipe
		if ok {
			ids = append(ids, id)
		}
		if !ok && len(ids) == 0 { break }
		for !ok && len(ids) > 0 || len(ids) == limit {
			list, err := api.ListPosts(user, api_key, types.ListPostOptions{Limit: limit, SearchQuery: "status:any id:" + strings.Join(ids, ",")})
			if err != nil {
				if consecutive_errors++; consecutive_errors == 10 {
					// transient API errors are okay, they might be because of network issues or whatever, but give up if they last too long.
					close(fixed_posts)
					return errors.New(fmt.Sprintf("Repeated failure while calling " + api.ApiName + " API (%s)", err.Error()))
				}
				time.Sleep(30 * time.Second)
				continue
			}

			consecutive_errors = 0

			for _, post := range list {
				fixed_posts <- post
			}

			// breaks the loop
			ids = nil
		}
	}

	close(fixed_posts)
	wg.Wait()

	suffix("done.")
	return nil
}

func SyncTagsCommand(ctx *gogram.MessageCtx) {
	full := false
	for _, token := range ctx.Cmd.Args {
		if token == "--full" {
			full = true
		}
	}

	err := SyncTags(ctx, storage.UpdaterSettings{Full: full}, nil, nil)
	if err == storage.ErrNoLogin {
		ctx.ReplyOrPMAsync(data.OMessage{SendData: data.SendData{Text: "You need to be logged in to " + api.ApiName + " to use this command (see <code>/help login</code>)", ParseMode: data.ParseHTML}}, nil)
		return
	} else if err != nil {
		ctx.Bot.Log.Println("Error occurred syncing tags: %s", err.Error())
	}
}

func SyncTags(ctx *gogram.MessageCtx, settings storage.UpdaterSettings, msg, sfx chan string) (error) {
	user, api_key, janitor, err := storage.GetUserCreds(storage.UpdaterSettings{}, ctx.Msg.From.Id)
	if err != nil || !janitor { return err }

	if msg == nil || sfx == nil {
		msg, sfx = ProgressMessage(ctx, "", "")
		defer close(msg)
		defer close(sfx)
	}

	return SyncTagsInternal(user, api_key, settings, msg, sfx)
}


func SyncTagsInternal(user, api_key string, settings storage.UpdaterSettings, msg, sfx chan string) (error) {
	message := func(x string) {
		if msg != nil {
			msg <- x
		}
	}
	suffix := func(x string) {
		if sfx != nil {
			sfx <- x
		}
	}

	m := "Syncing tag database..."
	if settings.Full {
		m = "Full syncing tag database..."
	}

	message(m)

	if settings.Full {
		storage.ClearTagIndex(settings)
	}

	fixed_tags := make(chan types.TTagData)

	limit := 1000
	last_existing_tag_id := 0
	consecutive_errors := 0
	last, err := storage.GetLastTag(settings)
	if err != nil { return err }
	if last != nil { last_existing_tag_id = last.Id }
	if last_existing_tag_id < 0 { last_existing_tag_id = 0 }

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		err := storage.TagUpdater(fixed_tags, settings)
		if err != nil { log.Println(err.Error()) }
		wg.Done()
	}()

	for {
		list, err := api.ListTags(user, api_key, types.ListTagsOptions{Page: types.After(last_existing_tag_id), Order: types.TSONewest, Limit: limit})
		if err != nil {
			if consecutive_errors++; consecutive_errors == 10 {
				// transient API errors are okay, they might be because of network issues or whatever, but give up if they last too long.
				close(fixed_tags)
				return errors.New(fmt.Sprintf("Repeated failure while calling " + api.ApiName + " API (%s)", err.Error()))
			}
			time.Sleep(30 * time.Second)
			continue
		}

		consecutive_errors = 0

		if len(list) == 0 { break }

		last_existing_tag_id = list[0].Id
		for _, t := range list {
			fixed_tags <- t
		}

		if len(list) < limit { break }
	}

	close(fixed_tags)
	wg.Wait()

	message("Resolving phantom tags...")
	storage.ResolvePhantomTags(settings)

	suffix(" done.")
	return nil
}

func RecountTagsCommand(ctx *gogram.MessageCtx) {
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

	err := RecountTags(ctx, storage.UpdaterSettings{}, msg, sfx, real_counts, alias_counts)
	if err == storage.ErrNoLogin {
		ctx.ReplyOrPMAsync(data.OMessage{SendData: data.SendData{Text: "You need to be logged in to " + api.ApiName + " to use this command (see <code>/help login</code>)", ParseMode: data.ParseHTML}}, nil)
		return
	} else if err != nil {
		ctx.Bot.Log.Println("Error occurred syncing tags: %s", err.Error())
	}
}

func RecountTags(ctx *gogram.MessageCtx, settings storage.UpdaterSettings, msg, sfx chan string, real_counts, alias_counts bool) (error) {
	_, _, janitor, err := storage.GetUserCreds(storage.UpdaterSettings{}, ctx.Msg.From.Id)
	if err != nil { return err }
	if !janitor { return errors.New("You need to be a janitor to use this command.") }

	if msg == nil || sfx == nil {
		msg, sfx = ProgressMessage(ctx, "", "")
		defer close(msg)
		defer close(sfx)
	}

	if real_counts {
		err = RecountTagsInternal(settings, msg, sfx)
		if err != nil { return err }
	}

	if alias_counts {
		err = CalculateAliasedCountsInternal(settings, msg, sfx)
		if err != nil { return err }
	}

	return nil
}

func RecountTagsInternal(settings storage.UpdaterSettings, msg, sfx chan string) (error) {
	message := func(x string) {
		if msg != nil {
			msg <- x
		}
	}
	suffix := func(x string) {
		if sfx != nil {
			sfx <- x
		}
	}

	message("Recounting tags...")

	err := storage.CountTags(settings, sfx)
	if err != nil {
		suffix(fmt.Sprintf(" (error: %s)", err.Error()))
	} else {
		suffix(" done.")
	}

	return err
}

func CalculateAliasedCountsInternal(settings storage.UpdaterSettings, msg, sfx chan string) (error) {
	message := func(x string) {
		if msg != nil {
			msg <- x
		}
	}
	suffix := func(x string) {
		if sfx != nil {
			sfx <- x
		}
	}

	message("Mapping counts between aliased tags...")

	err := storage.RecalculateAliasedCounts(settings)
	if err != nil {
		suffix(fmt.Sprintf(" (error: %s)", err.Error()))
	} else {
		suffix(" done.")
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
	Channel chan types.TPostInfo
}

func (this *SearchChanBox) Close() {
	close(this.Channel)
}

func SyncPostsCommand(ctx *gogram.MessageCtx) {
	full := false
	aliases := false
	recount := false
	for _, token := range ctx.Cmd.Args {
		if token == "--full" {
			full = true
		}
		if token == "--aliases" {
			aliases = true
		}
		if token == "--recount" {
			recount = true
		}
	}

	var err error
	settings := storage.UpdaterSettings{Full: full}
	settings.Transaction, err = storage.NewTxBox()
	if err != nil { log.Println(err.Error(), "newtxbox") }

	err = SyncPosts(ctx, settings, aliases, recount, nil, nil)
	if err == storage.ErrNoLogin {
		ctx.ReplyOrPMAsync(data.OMessage{SendData: data.SendData{Text: "You need to be logged in to " + api.ApiName + " to use this command (see <code>/help login</code>)", ParseMode: data.ParseHTML}}, nil)
		return
	} else if err != nil {
		ctx.Bot.Log.Println("Error occurred syncing psts: %s", err.Error())
		return
	}

	settings.Transaction.MarkForCommit()
	settings.Transaction.Finalize(true)
}

func SyncPosts(ctx *gogram.MessageCtx, settings storage.UpdaterSettings, aliases_too, recount_too bool, msg, sfx chan string) (error) {
	user, api_key, janitor, err := storage.GetUserCreds(settings, ctx.Msg.From.Id)
	if err != nil || !janitor { return err }

	if msg == nil || sfx == nil {
		msg, sfx = ProgressMessage(ctx, "", "")
		defer close(msg)
		defer close(sfx)
	}

	return SyncPostsInternal(user, api_key, settings, aliases_too, recount_too, msg, sfx, nil)
}

func SyncOnlyPostsInternal(user, api_key string, settings storage.UpdaterSettings, msg, sfx chan string, post_updates chan []types.TPostInfo) (error) {
	message := func(x string) {
		if msg != nil {
			msg <- x
		}
	}
	suffix := func(x string) {
		if sfx != nil {
			sfx <- x
		}
	}
	update := func(p []types.TPostInfo) {
		if post_updates != nil {
			post_updates <- p
		}
	}

	message("Syncing posts... ")

	if settings.Full {
		err := storage.ClearPosts(settings)
		if err != nil {
			log.Println(err.Error(), "clearposts")
		}
	}

	fixed_posts := make(chan types.TPostInfo)

	limit := 320
	latest_change_seq := 0
	consecutive_errors := 0
	last, err := storage.GetMostRecentlyUpdatedPost(settings)
	if err != nil { return err }
	if last != nil { latest_change_seq = last.Change }

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		err := storage.PostUpdater(fixed_posts, settings)
		wg.Done()
		if err != nil { log.Println(err.Error()) }
	}()

	for {
		list, err := api.ListPosts(user, api_key, types.ListPostOptions{Limit: limit, SearchQuery: types.PostsAfterChangeSeq(latest_change_seq)})
		if err != nil {
			if consecutive_errors++; consecutive_errors == 10 {
				// transient API errors are okay, they might be because of network issues or whatever, but give up if they last too long.
				close(fixed_posts)
				return errors.New(fmt.Sprintf("Repeated failure while calling " + api.ApiName + " API (%s)", err.Error()))
			}
			time.Sleep(30 * time.Second)
			continue
		}

		consecutive_errors = 0

		if len(list) == 0 { break }

		// unlike most other calls, quirks of api require that this call return results in ID:ascending
		// order instead of ID:descending, so the highest ID is the last one, not the first.
		latest_change_seq = list[len(list) - 1].Change
		i := 0
		for _, p := range list {
			fixed_posts <- p
			i++
		}
		update(list)

		if len(list) < limit { break }
	}

	close(fixed_posts)
	wg.Wait()

	suffix(" done.")
	return nil
}

func SyncPostsInternal(user, api_key string, settings storage.UpdaterSettings, aliases_too, recount_too bool, msg, sfx chan string, post_updates chan []types.TPostInfo) (error) {
	message := func(x string) {
		if msg != nil {
			msg <- x
		}
	}
	suffix := func(x string) {
		if sfx != nil {
			sfx <- x
		}
	}

	message("Syncing activity... ")

	if err := SyncOnlyPostsInternal(user, api_key, settings, msg, sfx, post_updates); err != nil { return err }
	if err := SyncTagsInternal(user, api_key, settings, msg, sfx); err != nil { return err }

	if aliases_too {
		if err := SyncAliasesInternal(user, api_key, settings, msg, sfx); err != nil { return err }
	}

	message("Resolving post tags...")

	if err := storage.ImportPostTagsFromNameToID(settings, sfx); err != nil { return err }

	if recount_too {
		if err := RecountTagsInternal(settings, msg, sfx); err != nil { return err }
		if err := CalculateAliasedCountsInternal(settings, msg, sfx); err != nil { return err }
	}
	suffix(" done.")

	return nil
}

func SyncAliasesCommand(ctx *gogram.MessageCtx) {
	err := SyncAliases(ctx, storage.UpdaterSettings{}, nil, nil)
	if err == storage.ErrNoLogin {
		ctx.ReplyOrPMAsync(data.OMessage{SendData: data.SendData{Text: "You need to be logged in to " + api.ApiName + " to use this command (see <code>/help login</code>)", ParseMode: data.ParseHTML}}, nil)
		return
	} else if err != nil {
		ctx.Bot.Log.Println("Error occurred syncing tags: %s", err.Error())
	}
}

func SyncAliases(ctx *gogram.MessageCtx, settings storage.UpdaterSettings, msg, sfx chan string) (error) {
	user, api_key, janitor, err := storage.GetUserCreds(settings, ctx.Msg.From.Id)
	if err != nil || !janitor { return err }

	if msg == nil || sfx == nil {
		msg, sfx = ProgressMessage(ctx, "", "")
		defer close(msg)
		defer close(sfx)
	}

	return SyncAliasesInternal(user, api_key, settings, msg, sfx)
}

func SyncAliasesInternal(user, api_key string, settings storage.UpdaterSettings, msg, sfx chan string) (error) {
	message := func(x string) {
		if msg != nil {
			msg <- x
		}
	}
	suffix := func(x string) {
		if sfx != nil {
			sfx <- x
		}
	}

	message("Syncing alias list...")

	storage.ClearAliasIndex(settings)

	consecutive_errors := 0
	page := types.After(0)

	fixed_aliases := make(chan types.TAliasData)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		err := storage.AliasUpdater(fixed_aliases, settings)
		if err != nil { log.Println(err.Error()) }
		wg.Done()
	}()

	for {
		list, err := api.ListTagAliases(user, api_key, types.ListTagAliasOptions{Limit: 10000, Page: page, Order: types.ASOCreated, Status: types.ASActive})
		if err != nil {
			if consecutive_errors++; consecutive_errors == 10 {
				// transient API errors are okay, they might be because of network issues or whatever, but give up if they last too long.
				close(fixed_aliases)
				return errors.New(fmt.Sprintf("Repeated failure while calling " + api.ApiName + " API (%s)", err.Error()))
			}
			time.Sleep(30 * time.Second)
			continue
		}

		consecutive_errors = 0

		if len(list) == 0 { break }

		page = types.After(list[0].Id)
		for _, a := range list {
			fixed_aliases <- a
		}
	}

	close(fixed_aliases)
	wg.Wait()

	suffix("done.")
	return nil
}

const (
	STATE_READY = 0
	STATE_COUNT = iota
)

func Percent(current, max int) (string) {
	return fmt.Sprintf(" (%.1f%%)", float32(current * 100) / float32(max))
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
		if utf8.RuneCountInString(t.Name) >= MINLENGTH && t.Type == types.TCGeneral {
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

const (
	MODE_READY = iota
	MODE_DISTINCT = iota
	MODE_EXCLUDE = iota
	MODE_INCLUDE = iota
	MODE_LIST = iota
	MODE_THRESHHOLD = iota
	MODE_FREQ_RATIO = iota
	MODE_IGNORE = iota
	MODE_UNIGNORE = iota
	MODE_FIX = iota
	MODE_REASON = iota
)

type TagEditBox struct {
	EditDistance int
	Tag types.TTagData
	Mode storage.CorrectionMode
}

func NewTagsFromOldTags(oldtags []string, deltags, addtags map[string]bool) (string) {
	var tags []string
	for _, tag := range oldtags {
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
	var threshhold int = -1
	var fix, save, autofix bool
	var show_short, show_zero, show_all, show_all_posts, only_general bool
	var start_tag string
	reason := "likely typo"
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
		} else if mode == MODE_REASON {
			reason = token
		}

		if mode != MODE_READY {
			mode = MODE_READY
		} else {
			if !strings.HasPrefix(token, "-") {
				start_tag = token
			} else if token == "--all"         || token == "-a" {
				show_all = true
			} else if token == "--all-posts"   || token == "-p" {
				show_all_posts = true
			} else if token == "--show-short"  || token == "-s" {
				show_short = true
			} else if token == "--show-zero"   || token == "-z" {
				show_zero = true
			} else if token == "--only-general"|| token == "-g" {
				only_general = true
			} else if token == "--threshhold"  || token == "-t" {
				mode = MODE_THRESHHOLD
			} else if token == "--exclude"     || token == "-e" {
				mode = MODE_EXCLUDE
			} else if token == "--distinct"    || token == "-d" {
				mode = MODE_DISTINCT
			} else if token == "--include"     || token == "-i" {
				mode = MODE_INCLUDE
			} else if token == "--save"        || token == "-S" {
				save = true
			} else if token == "--autofix"     || token == "-X" {
				autofix = true
			} else if token == "--fix"         || token == "-x" {
				fix = true
			} else if token == "--reason"      || token == "-r" {
				mode = MODE_REASON
			}
		}
	}


	txbox, err := storage.NewTxBox()
	if err != nil {
		ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: fmt.Sprintf("Error opening DB transaction: %s.", err.Error())}}, nil)
		return
	}

	ctrl := storage.EnumerateControl{
		Transaction: txbox,
		CreatePhantom: true,
		OrderByCount: true,
		IncludeDeleted: show_all_posts,
	}

	defer ctrl.Transaction.Finalize(true)

	user, api_key, janitor, err := storage.GetUserCreds(storage.UpdaterSettings{Transaction: ctrl.Transaction}, ctx.Msg.From.Id)
	if (err != nil || !janitor) && fix {
		ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: "You need to be logged in to " + api.ApiName + " to use this command (see <code>/help login</code>)", ParseMode: data.ParseHTML}}, nil)
		return
	}

	if start_tag == "" {
		ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: "You must specify a tag.", ParseMode: data.ParseHTML}}, nil)
		return
	}

	len1 := utf8.RuneCountInString(start_tag)
	if threshhold == -1 {
		if len1 < 8 {
			threshhold = 1
		} else if len1 < 16 {
			threshhold = 2
		} else if len1 < 32 {
			threshhold = 3
		} else {
			threshhold = 4
		}
	}

	t1, err := storage.GetTag(start_tag, ctrl)
	if err != nil { log.Printf("Error occurred when looking up tag: %s", err.Error()) }
	if t1 == nil {
		ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: fmt.Sprintf("Tag doesn't exist: %s.", start_tag)}}, nil)
		return
	}

	progress, err := ProgressMessage2(func(t string, m data.MessageParseMode) (*data.TMessage, error) {
		nctx, err := ctx.Reply(data.OMessage{SendData: data.SendData{Text: t, ParseMode: m}, DisableWebPagePreview: true})
		return nctx.Msg, err
	}, "Checking for typos...", data.ParseHTML, 3 * time.Second, ctx.Bot)
	if err != nil {
		ctx.Bot.ErrorLog.Println("ProgressMessage2() failed:", err.Error())
		return
	}
	defer progress.Close()

	alltags, err := storage.EnumerateAllTags(ctrl)
	blits, err := storage.EnumerateAllBlits(ctrl)
	typos, err := storage.GetTagTypos(start_tag, ctrl)

	for _, t2 := range alltags {
		if t1.Name == t2.Name { continue } // skip tag = tag

		is_blit2, reg_blit2 := blits[t2.Name]

		// shortest name first (in terms of codepoints)
		var t1n, t2n string
		var t1l, t2l int
		len2 := utf8.RuneCountInString(t2.Name)
		if len1 > len2 {
			t1n, t2n = t2.Name, t1.Name
			t1l, t2l = len2, len1
		} else {
			t1n, t2n = t1.Name, t2.Name
			t1l, t2l = len1, len2
		}

		tooshort := (t1l < 3) // tag length too short
		blit := reg_blit2 && is_blit2
		notblit := reg_blit2 && !is_blit2
		zero := t2.ApparentCount(show_all_posts) <= 0

		if tooshort && !notblit && !show_short ||
		   blit && !show_all ||
		   zero && !show_zero {
			// if it's too short and not a confirmed non-blit, AND the short override isn't specified OR
			// if it's a blit, and the all override isn't specified OR
			// if it's got no posts and the no-post override isn't specified
			// skip
			continue
		}

		// if it doesn't match, skip
		// the length difference is a lower bound on the edit distance so if the lengths are too dissimilar, skip.
		if t2l - t1l > threshhold { continue }

		// check the edit distance and bail if it's not low.
		distance := wordset.Levenshtein(t1n, t2n)
		if distance > threshhold { continue }

		// these tags are similar!
		results[t2.Name] = TagEditBox{Tag: t2, EditDistance: distance}
	}

	for name, value := range typos {
		if !show_all {
			// if override isn't specified and we already have a rule for this one, skip it
			delete(results, name)
		} else {
			// if override IS specified, always show all rules
			results[name] = TagEditBox{Tag: value.Tag, EditDistance: wordset.Levenshtein(start_tag, name), Mode: value.Mode}
		}
	}

	// now for selectors, which take priority over all of the blanket filter options
	// remove any matches that were manually excluded.
	progress.SetStatus("remove excluded")
	for _, item := range exclude {
		delete(results, item)
	}

	// now remove any matches which are more closely matched by the distinct list.
	progress.SetStatus("remove distinct")
	for _, item := range distinct {
		for k, v := range results {
			if wordset.Levenshtein(item, k) < v.EditDistance {
				exclude = append(exclude, k)
				delete(results, k)
			}
		}
	}

	// now remove any matches which are already aliased to the target tag.
	progress.SetStatus("remove aliases")
	aliases, err := storage.GetAliasesFor(start_tag, ctrl)
	if err != nil { log.Printf("Error when searching for aliases to %s: %s", start_tag, err.Error()) }
	for _, item := range aliases {
		delete(results, item.Name)
	}

	// filter only the general tags, removing any typed ones
	if only_general {
		progress.SetStatus("remove non-general")
		for k, v := range results {
			if v.Tag.Type != types.TCGeneral {
				delete(results, k)
			}
		}
	}

	// aaaaaand finally add any matches manually included.
	progress.SetStatus("merge included")
	ctrl.CreatePhantom = false
	for _, item := range include {
		t, _ := storage.GetTag(item, ctrl)
		if t != nil { results[item] = TagEditBox{Tag: *t, EditDistance: wordset.Levenshtein(t1.Name, t.Name)} }
	}

	progress.SetStatus("done.")

	total_posts := 0
	for _, v := range results {
		total_posts += v.Tag.ApparentCount(show_all_posts)
	}

	if len(results) > 50 {
		var buf bytes.Buffer
		buf.WriteString(fmt.Sprintf("Possible typos of %s: %d (%d estimated posts)\n", start_tag, len(results), total_posts))
		for _, v := range results {
			alert := " "
			if v.Tag.Type != types.TCGeneral { alert = "!" }
			buf.WriteString(fmt.Sprintf("%8d %s%s %s\n", v.Tag.ApparentCount(show_all_posts), v.Mode.Display(), alert, v.Tag.Name))
		}
		ctx.Bot.Remote.SendDocumentAsync(data.ODocument{
			SendData: data.SendData{
				TargetData: data.TargetData{ChatId: progress.Ctx.Msg.Chat.Id},
				ReplyToId: &progress.Ctx.Msg.Id,
			},
			MediaData: data.MediaData{
				FileName: "tag_duplicates.txt",
				File: buf.Bytes(),
			},
		}, nil)
		progress.AppendNotice(fmt.Sprintf("Possible typos of <code>%s</code>: %d (%d estimated posts)\n\nResults not shown, too many of them!\nSee attached text file.", html.EscapeString(start_tag), len(results), total_posts))
	} else {
		var buf bytes.Buffer
		buf.WriteString(fmt.Sprintf("Possible typos of <code>%s</code>: %d (%d estimated posts)\n<pre>", html.EscapeString(start_tag), len(results), total_posts))
		for _, v := range results {
			alert := " "
			if v.Tag.Type != types.TCGeneral { alert = "!" }
			buf.WriteString(fmt.Sprintf("%7d %s%s %s\n", v.Tag.ApparentCount(show_all_posts), v.Mode.Display(), alert, html.EscapeString(v.Tag.Name)))
		}
		buf.WriteString("</pre>")
		progress.AppendNotice(buf.String())
	}

	if fix {
		progress.AppendNotice("Fixing tags on ??? posts...")

		updated := 1
		diffs := make(map[int]tags.TagDiff)

		for _, v := range results {
			array, err := storage.PostsWithTag(v.Tag, ctrl)
			if err != nil {
				ctx.Bot.ErrorLog.Println("Error in FindTagTypos()/PostsWithTag():", err.Error())
				progress.SetStatus("error!")
				return
			}

			for _, post := range array {
				d := diffs[post.Id]
				d.Add(start_tag)
				d.Remove(v.Tag.Name)
				diffs[post.Id] = d
			}
		}

		// we now know for sure that exactly this many edits are required
		total_posts = len(diffs)
		progress.ReplaceNotice(fmt.Sprintf("Fixing tags on %d posts...", total_posts))

		for id, diff := range diffs {
			if diff.IsZero() { continue }

			reason := fmt.Sprintf("Bulk retag: %s (%s)", diff.APIString(), reason)
			newp, err := api.UpdatePost(user, api_key, id, diff, nil, nil, nil, nil, &reason)

			if err == api.PostIsDeleted {
				log.Printf("Post was deleted which we didn't know about? DB consistency? (%d)\n", id)
				err = storage.MarkPostDeleted(id, storage.UpdaterSettings{Transaction: ctrl.Transaction})
			}

			if err != nil {
				ctx.Bot.ErrorLog.Println("Error in FindTagTypos()/api.UpdatePost():", err.Error())
				progress.SetStatus("error!")
				break
			}

			if newp != nil {
				err = storage.UpdatePost(*newp, storage.UpdaterSettings{Transaction: ctrl.Transaction})
				if err != nil {
					ctx.Bot.ErrorLog.Println("Error in FindTagTypos()/storage.UpdatePost():", err.Error())
					progress.SetStatus("error!")
					break
				}
			}

			progress.SetStatus(fmt.Sprintf("(%d/%d %d: <code>%s</code>)", updated, total_posts, id, diff.APIString()))
			updated++
		}
		progress.SetStatus("done.")
	}

	if save {
		progress.AppendNotice(fmt.Sprintf("Saving %d identified typos...", len(results)))
		// everything in results goes into the database as either prompt or auto
		mode := storage.Prompt
		if autofix { mode = storage.AutoFix }
		for name, _ := range results {
			if _, already_exists := typos[name]; already_exists { continue }
			err := storage.AddTagTypo(start_tag, name, mode, storage.UpdaterSettings{Transaction: ctrl.Transaction})
			if err != nil {
				log.Printf("Error adding tag typo to database (%s -> %s): %s", name, start_tag, err.Error())
				return
			}
		}
		// everything in exclude goes in as ignore
		for _, name := range exclude {
			if _, already_exists := typos[name]; already_exists { continue }
			if _, already_exists := results[name]; already_exists { continue }
			err := storage.AddTagTypo(start_tag, name, storage.Ignore, storage.UpdaterSettings{Transaction: ctrl.Transaction})
			if err != nil {
				log.Printf("Error adding tag typo to database (%s -> %s): %s", name, start_tag, err.Error())
				return
			}
		}
		progress.SetStatus("done.")
	}

	ctrl.Transaction.MarkForCommit()
}

func RefetchDeletedPostsCommand(ctx *gogram.MessageCtx) {
	var err error
	settings := storage.UpdaterSettings{}
	settings.Transaction, err = storage.NewTxBox()
	if err != nil { log.Println(err.Error(), "newtxbox") }

	err = RefetchDeletedPosts(ctx, settings, nil, nil)
	if err == storage.ErrNoLogin {
		ctx.ReplyOrPMAsync(data.OMessage{SendData: data.SendData{Text: "You need to be logged in to " + api.ApiName + " to use this command (see <code>/help login</code>)", ParseMode: data.ParseHTML}}, nil)
		return
	} else if err != nil {
		ctx.Bot.Log.Println("Error occurred syncing deleted posts: %s", err.Error())
		return
	}

	settings.Transaction.MarkForCommit()
	settings.Transaction.Finalize(true)
}

func RefetchDeletedPosts(ctx *gogram.MessageCtx, settings storage.UpdaterSettings, msg, sfx chan string) (error) {
	user, api_key, janitor, err := storage.GetUserCreds(settings, ctx.Msg.From.Id)
	if err != nil || !janitor { return err }

	if msg == nil || sfx == nil {
		msg, sfx = ProgressMessage(ctx, "", "")
		defer close(msg)
		defer close(sfx)
	}

	return RefetchDeletedPostsInternal(user, api_key, settings, msg, sfx)
}

func RefetchDeletedPostsInternal(user, api_key string, settings storage.UpdaterSettings, msg, sfx chan string) (error) {
	message := func(x string) {
		if msg != nil {
			msg <- x
		}
	}
	suffix := func(x string) {
		if sfx != nil {
			sfx <- x
		}
	}

	message("Syncing deleted posts... ")

	fixed_posts := make(chan []int)

	limit := 10000
	consecutive_errors := 0
	latest_id := 0
	highest_id := storage.GetHighestPostID(settings)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		err := storage.PostDeleter(fixed_posts, settings)
		wg.Done()
		if err != nil { log.Println(err.Error()) }
	}()

	for {
		list, err := api.ListPosts(user, api_key, types.ListPostOptions{Limit: limit, SearchQuery: types.DeletedPostsAfterId(latest_id)})
		if err != nil {
			if consecutive_errors++; consecutive_errors == 10 {
				// transient API errors are okay, they might be because of network issues or whatever, but give up if they last too long.
				close(fixed_posts)
				return errors.New(fmt.Sprintf("Repeated failure while calling " + api.ApiName + " API (%s)", err.Error()))
			}
			time.Sleep(30 * time.Second)
			continue
		}

		consecutive_errors = 0

		if len(list) == 0 { break }

		// return results in ascending order, unlike many similar queries
		latest_id = list[len(list) - 1].Id
		var post_ids []int

		for _, p := range list {
			post_ids = append(post_ids, p.Id)
		}
		fixed_posts <- post_ids

		suffix(fmt.Sprintf("%.1f%%", float32(latest_id) * 100.0 / float32(highest_id)))
	}

	close(fixed_posts)
	wg.Wait()

	suffix(" done.")
	return nil
}

func Blits(ctx *gogram.MessageCtx) {
	txbox, err := storage.NewTxBox()
	if err != nil {
		ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: fmt.Sprintf("Error opening DB transaction: %s.", err.Error())}}, nil)
		return
	}

	ctrl := storage.EnumerateControl{
		Transaction: txbox,
		CreatePhantom: true,
		OrderByCount: true,
	}

	defer ctrl.Transaction.Finalize(true)

	_, _, janitor, err := storage.GetUserCreds(storage.UpdaterSettings{Transaction: txbox}, ctx.Msg.From.Id)
	if err != nil || !janitor { return }

	mode := MODE_READY
	include, exclude := make(map[string]bool), make(map[string]bool)

	for _, token := range ctx.Cmd.Args {
		token = strings.Replace(strings.ToLower(token), "\uFE0F", "", -1)
		if token == "--exclude" {
			mode = MODE_EXCLUDE
		} else if token == "--mark" {
			mode = MODE_INCLUDE
		} else if token == "--list" {
			mode = MODE_LIST
		} else if mode == MODE_EXCLUDE {
			exclude[token] = true
		} else if mode == MODE_INCLUDE {
			include[token] = true
		}
	}

	if mode == MODE_LIST {
		allblits, err := storage.GetMarkedAndUnmarkedBlits(ctrl)
		if err != nil {
			ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: "Whoops! " + err.Error(), ParseMode: data.ParseHTML}}, nil)
			return
		}

		var buf bytes.Buffer
		last_valid := true

		buf.WriteString("<b>Blit List</b>\n<pre>")
		for _, b := range allblits {
			if last_valid != b.Valid {
				buf.WriteString("</pre>\n\n<b>Ignore List</b>\n<pre>")
			}
			last_valid = b.Valid
			if len(b.Name) + 1 + buf.Len() > 4095 + 36 - 24 { break } // 4095 max, 36 chars of HTML tag, 24 characters of string literal
			buf.WriteString(html.EscapeString(b.Name))
			buf.WriteRune(' ')
		}
		if last_valid != false {
			buf.WriteString("</pre>\n\n<b>Ignore List</b>\n")
		} else {
			buf.WriteString("</pre>")
		}

		ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: buf.String(), ParseMode: data.ParseHTML}}, nil)
		return
	}

	tags, _ := storage.EnumerateAllTags(ctrl)
	var intermediate, blits types.TTagInfoArray
	for _, t := range tags {
		if utf8.RuneCountInString(t.Name) <= 2 {
			intermediate = append(intermediate, t)
		}

		if include[t.Name] {
			storage.MarkBlit(t.Id, true, ctrl)
		} else if exclude[t.Name] {
			storage.MarkBlit(t.Id, false, ctrl)
		}
	}

	allknownblits := make(map[int]bool)
	allblits, err := storage.GetMarkedAndUnmarkedBlits(ctrl)
	if err != nil {
		ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: "Whoops! " + err.Error(), ParseMode: data.ParseHTML}}, nil)
		return
	}

	for _, b := range allblits {
		allknownblits[b.Id] = true
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
		if t.Type == types.TCGeneral {
			tagtype = "GENERAL"
		} else if t.Type == types.TCSpecies {
			tagtype = "SPECIES"
		} else if t.Type == types.TCArtist {
			tagtype = "ARTIST "
		} else if t.Type == types.TCCopyright {
			tagtype = "CPYRIGT"
		} else if t.Type == types.TCCharacter {
			tagtype = "CHARCTR"
		} else if t.Type == types.TCInvalid {
			tagtype = "INVALID"
		} else if t.Type == types.TCLore {
			tagtype = "LORE   "
		} else if t.Type == types.TCMeta {
			tagtype = "META   "
		}
		newstr := fmt.Sprintf("%5d (%s) %s\n", t.Count, tagtype, t.Name)
		if len(newstr) + buf.Len() > 4096 - 12 { break }
		buf.WriteString(html.EscapeString(newstr))
	}
	ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: "<pre>" + buf.String() + "</pre>", ParseMode: data.ParseHTML}}, nil)
	ctrl.Transaction.MarkForCommit()
}

type Triplet struct {
	tag, subtag1, subtag2 types.TTagData
}

func Concatenations(ctx *gogram.MessageCtx) {
	txbox, err := storage.NewTxBox()
	if err != nil {
		ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: fmt.Sprintf("Error opening DB transaction: %s.", err.Error())}}, nil)
		return
	}

	ctrl := storage.EnumerateControl{
		Transaction: txbox,
		CreatePhantom: true,
		OrderByCount: true,
	}

	defer ctrl.Transaction.Finalize(true)

	user, api_key, janitor, err := storage.GetUserCreds(storage.UpdaterSettings{Transaction: txbox}, ctx.Msg.From.Id)
	if err != nil || !janitor { return }

	var cats []Triplet
	header := "Here are some random concatenated tags:"
	if ctx.Msg.ReplyToMessage != nil {
		text := ctx.Msg.ReplyToMessage.Text
		if text != nil {
			prev_cats := strings.Split(*text, "\n")
			if prev_cats[0] == header {
				prev_cats = prev_cats[1:]
				for _, line := range prev_cats {
					var t Triplet
					tokens := strings.Split(line, " ")
					if !(len(tokens) == 4 && tokens[2] == "+") { continue }
					t.subtag1.Name = tokens[1]
					t.subtag2.Name = tokens[3]
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
			storage.SetCatsException(tag, ctrl)
			message.WriteString(fmt.Sprintf("Adding to ignore list: <code>%s</code>\n", tag))
		}
		for _, tag := range manual_unignore {
			storage.ClearCatsException(tag, ctrl)
			message.WriteString(fmt.Sprintf("Removing from ignore list: <code>%s</code>\n", tag))
		}

		if manual_ignore != nil || manual_unignore != nil {
			ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: message.String(), ParseMode: data.ParseHTML}}, nil)
			return
		}

		tags, _ := storage.EnumerateAllTags(storage.EnumerateControl{Transaction: txbox})
		exceptions, _ := storage.EnumerateCatsExceptions(ctrl)

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
			if v.Type != types.TCGeneral { continue } // skip anything that's not a general tag.
			runes := []rune(k)
			for i := 1; i < len(runes) - 1; i++ {
				prefix, prefix_ok := tagmap[string(runes[:i])]
				suffix, suffix_ok := tagmap[string(runes[i:])]
				if prefix_ok && suffix_ok && ratio * v.Count < prefix.Count && ratio * v.Count < suffix.Count && v.Type == types.TCGeneral {
					candidates = append(candidates, Triplet{tag: v, subtag1: prefix, subtag2: suffix})
				}
			}
		}

		message.WriteString(header + "\n")

		for i := 0; i < 10; i++ {
			cats = append(cats, candidates[rand.Intn(len(candidates))])
		}

		for i, t := range cats {
			message.WriteString(fmt.Sprintf("%d: <code>%s</code> + <code>%s</code> (%d)\n", i, t.subtag1.Name, t.subtag2.Name, t.tag.Count))
		}

		ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: message.String(), ParseMode: data.ParseHTML}}, nil)
		return
	}

	msg, sfx := ProgressMessage(ctx, "", "")
	var message bytes.Buffer
	for _, i := range ignore_list {
		storage.SetCatsException(cats[i].tag.Name, ctrl)
		msg <- fmt.Sprintf("Adding %d to ignore list: <code>%s</code>\n", i, cats[i].tag.Name)
	}
	msg <- "\nUpdating posts which need fixing... "

	updated := 1
	for _, i := range fix_list {
		t, err := storage.GetTag(cats[i].tag.Name, storage.EnumerateControl{Transaction: txbox})
		cats[i].tag = *t
		posts, err := storage.LocalTagSearch(cats[i].tag, storage.EnumerateControl{Transaction: txbox})
		if err != nil {
			sfx <- fmt.Sprintf(" (error: %s)", err.Error())
			return
		}

		reason := fmt.Sprintf("Bulk retag: %s --> %s, %s (fixed concatenated tags)", cats[i].tag.Name, cats[i].subtag1.Name, cats[i].subtag2.Name)
		for _, p := range posts {
			var diff tags.TagDiff
			diff.Add(cats[i].subtag1.Name)
			diff.Add(cats[i].subtag2.Name)
			diff.Remove(cats[i].tag.Name)
			newp, err := api.UpdatePost(user, api_key, p.Id, diff, nil, nil, nil, nil, &reason)
			err = nil
			if err != nil {
				sfx <- fmt.Sprintf(" (error: %s)", err.Error())
				return
			}

			if newp != nil {
				err = storage.UpdatePost(*newp, storage.UpdaterSettings{Transaction: txbox})
				if err != nil {
					sfx <- fmt.Sprintf(" (error: %s)", err.Error())
					return
				}
			}

			sfx <- fmt.Sprintf(" (%d/%d %d: <code>%s</code> -> <code>%s</code>, <code>%s</code>)", updated, -1, p.Id, cats[i].tag.Name, cats[i].subtag1.Name, cats[i].subtag2.Name)

			updated++
		}
		sfx <- " done."
		message.WriteString(fmt.Sprintf("Fixing %d: <code>%s</code> -> <code>%s, %s</code>\n", i, cats[i].tag.Name, cats[i].subtag1.Name, cats[i].subtag2.Name))
	}

	ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: message.String(), ParseMode: data.ParseHTML}}, nil)
	ctrl.Transaction.MarkForCommit()
}

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
