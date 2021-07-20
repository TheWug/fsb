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
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
	"sort"
	"database/sql"
)

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
	InitialMessage data.OMessage
	Bot *gogram.TelegramBot

	// public shared fields which are initialized after the first push.
	Ctx *gogram.MessageCtx

	// internally accessible fields
	target, actual string
	updater <- chan time.Time
}

func (this *ProgMessage) Respin(previous, notice, status string) string {
	if this == nil { return "" }
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
			this.InitialMessage.Text = this.target
			msg, err := this.Bot.Remote.SendMessage(this.InitialMessage)
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
					ParseMode: this.InitialMessage.ParseMode,
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
	if this == nil { return nil }
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
	if this == nil { return nil }
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
	if this == nil { return nil }
	this.notice = text
	this.active = this.Respin(this.previous, this.notice, this.status)
	return this.Push(this.active)
}

func (this *ProgMessage) SetStatus(text string) (error) {
	if this == nil { return nil }
	this.status = text
	this.active = this.Respin(this.previous, this.notice, this.status)
	return this.Push(this.active)
}

func (this *ProgMessage) SetMessage(text string) (error) {
	if this == nil { return nil }
	this.active = text
	this.previous = ""
	this.status = ""
	this.notice = text
	return this.Push(this.active)
}

func (this *ProgMessage) Active() string {
	if this == nil { return "" }
	return this.active
}

func (this *ProgMessage) Close() {
	if this == nil { return }
	if this.running {
		close(this.text_updates)
		this.running = false
	}
}

func ProgressMessage2(initial_message data.OMessage,
		      initial_text string,
		      interval time.Duration,
		      bot *gogram.TelegramBot) (*ProgMessage, error) {
	x := ProgMessage{
		UpdateInterval: interval,
		InitialMessage: initial_message,
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
	err := ResyncList(ctx, storage.UpdaterSettings{}, nil)
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

func ResyncList(ctx *gogram.MessageCtx, settings storage.UpdaterSettings, progress *ProgMessage) (error) {
	creds, err := storage.GetUserCreds(settings, ctx.Msg.From.Id)
	if err != nil || !creds.Janitor { return err }

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

	if progress == nil {
		progress, _ = ProgressMessage2(data.OMessage{SendData: data.SendData{ReplyToId: &ctx.Msg.Id, ParseMode: data.ParseHTML}, DisableWebPagePreview: true},
	                                       "", 3 * time.Second, ctx.Bot)
		defer progress.Close()
	}

	return ResyncListInternal(creds.User, creds.ApiKey, settings, file_data, progress)
}


func ResyncListInternal(user, api_key string, settings storage.UpdaterSettings, file_data io.Reader, progress *ProgMessage) (error) {
	progress.AppendNotice("Updating posts from list...")

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

	progress.SetStatus("done.")
	return nil
}

func SyncTagsCommand(ctx *gogram.MessageCtx) {
	full := false
	for _, token := range ctx.Cmd.Args {
		if token == "--full" {
			full = true
		}
	}

	err := SyncTags(ctx, storage.UpdaterSettings{Full: full}, nil)
	if err == storage.ErrNoLogin {
		ctx.ReplyOrPMAsync(data.OMessage{SendData: data.SendData{Text: "You need to be logged in to " + api.ApiName + " to use this command (see <code>/help login</code>)", ParseMode: data.ParseHTML}}, nil)
		return
	} else if err != nil {
		ctx.Bot.Log.Println("Error occurred syncing tags: %s", err.Error())
	}
}

func SyncTags(ctx *gogram.MessageCtx, settings storage.UpdaterSettings, progress *ProgMessage) (error) {
	creds, err := storage.GetUserCreds(storage.UpdaterSettings{}, ctx.Msg.From.Id)
	if err != nil || !creds.Janitor { return err }

	if progress == nil {
		progress, _ = ProgressMessage2(data.OMessage{SendData: data.SendData{ReplyToId: &ctx.Msg.Id, ParseMode: data.ParseHTML}, DisableWebPagePreview: true},
	                                       "", 3 * time.Second, ctx.Bot)
		defer progress.Close()
	}

	return SyncTagsInternal(creds.User, creds.ApiKey, settings, progress)
}


func SyncTagsInternal(user, api_key string, settings storage.UpdaterSettings, progress *ProgMessage) (error) {
	m := "Syncing tag database..."
	if settings.Full {
		m = "Full syncing tag database..."
	}

	progress.AppendNotice(m)

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

	progress.SetStatus("done.")
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

	err := RecountTags(ctx, storage.UpdaterSettings{}, nil, real_counts, alias_counts)
	if err == storage.ErrNoLogin {
		ctx.ReplyOrPMAsync(data.OMessage{SendData: data.SendData{Text: "You need to be logged in to " + api.ApiName + " to use this command (see <code>/help login</code>)", ParseMode: data.ParseHTML}}, nil)
		return
	} else if err != nil {
		ctx.Bot.Log.Println("Error occurred syncing tags: %s", err.Error())
	}
}

func RecountTags(ctx *gogram.MessageCtx, settings storage.UpdaterSettings, progress *ProgMessage, real_counts, alias_counts bool) (error) {
	creds, err := storage.GetUserCreds(storage.UpdaterSettings{}, ctx.Msg.From.Id)
	if err != nil { return err }
	if !creds.Janitor { return errors.New("You need to be a janitor to use this command.") }

	if progress == nil {
		progress, _ = ProgressMessage2(data.OMessage{SendData: data.SendData{ReplyToId: &ctx.Msg.Id, ParseMode: data.ParseHTML}, DisableWebPagePreview: true},
	                                       "", 3 * time.Second, ctx.Bot)
		defer progress.Close()
	}

	if real_counts {
		err = RecountTagsInternal(settings, progress)
		if err != nil { return err }
	}

	if alias_counts {
		err = CalculateAliasedCountsInternal(settings, progress)
		if err != nil { return err }
	}

	return nil
}

func RecountTagsInternal(settings storage.UpdaterSettings, progress *ProgMessage) (error) {
	progress.AppendNotice("Recounting tags...")

	var err error
	sfx := make(chan string)
	go func() {
		err = storage.CountTags(settings, sfx)
		if err != nil {
			progress.SetStatus(fmt.Sprintf("(error: %s)", html.EscapeString(err.Error())))
		} else {
			progress.SetStatus("done.")
		}
		close(sfx)
	}()

	for str := range sfx {
		progress.SetStatus(str)
	}

	return err
}

func CalculateAliasedCountsInternal(settings storage.UpdaterSettings, progress *ProgMessage) (error) {
	progress.AppendNotice("Mapping counts between aliased tags...")

	err := storage.RecalculateAliasedCounts(settings)
	if err != nil {
		progress.SetStatus(fmt.Sprintf("(error: %s)", html.EscapeString(err.Error())))
	} else {
		progress.SetStatus("done.")
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

	err = SyncPosts(ctx, settings, aliases, recount, nil)
	if err == storage.ErrNoLogin {
		ctx.ReplyOrPMAsync(data.OMessage{SendData: data.SendData{Text: "You need to be logged in to " + api.ApiName + " to use this command (see <code>/help login</code>)", ParseMode: data.ParseHTML}}, nil)
		return
	} else if err != nil {
		ctx.Bot.ErrorLog.Println("Error occurred syncing posts: %s", err.Error())
		return
	}

	settings.Transaction.MarkForCommit()
	settings.Transaction.Finalize(true)
}

func SyncPosts(ctx *gogram.MessageCtx, settings storage.UpdaterSettings, aliases_too, recount_too bool, progress *ProgMessage) (error) {
	creds, err := storage.GetUserCreds(settings, ctx.Msg.From.Id)
	if err != nil || !creds.Janitor { return err }

	if progress == nil {
		progress, _ = ProgressMessage2(data.OMessage{SendData: data.SendData{ReplyToId: &ctx.Msg.Id, ParseMode: data.ParseHTML}, DisableWebPagePreview: true},
	                                       "", 3 * time.Second, ctx.Bot)
		defer progress.Close()
	}

	return SyncPostsInternal(creds.User, creds.ApiKey, settings, aliases_too, recount_too, progress, nil)
}

func SyncOnlyPostsInternal(user, api_key string, settings storage.UpdaterSettings, progress *ProgMessage, post_updates chan []types.TPostInfo) (error) {
	update := func(p []types.TPostInfo) {
		if post_updates != nil {
			post_updates <- p
		}
	}

	progress.AppendNotice("Syncing posts... ")

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

	progress.SetStatus(" done.")
	return nil
}

func SyncPostsInternal(user, api_key string, settings storage.UpdaterSettings, aliases_too, recount_too bool, progress *ProgMessage, post_updates chan []types.TPostInfo) (error) {
	progress.AppendNotice("Syncing activity... ")

	if err := SyncOnlyPostsInternal(user, api_key, settings, progress, post_updates); err != nil { return err }
	if err := SyncTagsInternal(user, api_key, settings, progress); err != nil { return err }

	if aliases_too {
		if err := SyncAliasesInternal(user, api_key, settings, progress); err != nil { return err }
	}

	progress.AppendNotice("Resolving post tags...")

	var err error
	sfx := make(chan string)
	go func() {
		err = storage.ImportPostTagsFromNameToID(settings, sfx)
		close(sfx)
	}()

	for str := range sfx {
		progress.SetStatus(str)
	}

	if err != nil { return err }

	if recount_too {
		if err := RecountTagsInternal(settings, progress); err != nil { return err }
		if err := CalculateAliasedCountsInternal(settings, progress); err != nil { return err }
	}

	progress.SetStatus("done.")

	return nil
}

func SyncAliasesCommand(ctx *gogram.MessageCtx) {
	err := SyncAliases(ctx, storage.UpdaterSettings{}, nil)
	if err == storage.ErrNoLogin {
		ctx.ReplyOrPMAsync(data.OMessage{SendData: data.SendData{Text: "You need to be logged in to " + api.ApiName + " to use this command (see <code>/help login</code>)", ParseMode: data.ParseHTML}}, nil)
		return
	} else if err != nil {
		ctx.Bot.Log.Println("Error occurred syncing tags: %s", err.Error())
	}
}

func SyncAliases(ctx *gogram.MessageCtx, settings storage.UpdaterSettings, progress *ProgMessage) (error) {
	creds, err := storage.GetUserCreds(settings, ctx.Msg.From.Id)
	if err != nil || !creds.Janitor { return err }

	if progress == nil {
		progress, _ = ProgressMessage2(data.OMessage{SendData: data.SendData{ReplyToId: &ctx.Msg.Id, ParseMode: data.ParseHTML}, DisableWebPagePreview: true},
	                                       "", 3 * time.Second, ctx.Bot)
		defer progress.Close()
	}

	return SyncAliasesInternal(creds.User, creds.ApiKey, settings, progress)
}

func SyncAliasesInternal(user, api_key string, settings storage.UpdaterSettings, progress *ProgMessage) (error) {
	progress.AppendNotice("Syncing alias list...")

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

	progress.SetStatus("done.")
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
	MODE_DISTINCT
	MODE_EXCLUDE
	MODE_INCLUDE
	MODE_LIST
	MODE_FREQ_RATIO
	MODE_IGNORE
	MODE_UNIGNORE
	MODE_FIX
	MODE_REASON
	MODE_MARK
	MODE_DELETE
	MODE_INSPECT
	MODE_ENTRY
	MODE_PROMPT
	MODE_AUTOFIX
	MODE_SELECT_1
	MODE_SELECT_2
	MODE_SELECT_CAT
	MODE_THRESHOLD
	MODE_SELECT
	MODE_SKIP
	MODE_ALIAS
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

type Pair struct {
	tag, fixed *types.TTagData
}

func (p Pair) TypoData() storage.TypoData2 {
	return storage.TypoData2{Tag: *p.tag, Fix: p.fixed}
}

func (p Pair) String() string {
	names := map[types.TagCategory]rune{
		types.TCGeneral: ' ',
		types.TCArtist: 'A',
		types.TCCopyright: 'P',
		types.TCCharacter: 'C',
		types.TCSpecies: 'S',
		types.TCInvalid: 'I',
		types.TCMeta: 'M',
		types.TCLore: 'L',
	}

	r, ok := names[p.tag.Type]
	if !ok {
		r = '?'
	}

	return fmt.Sprintf("%8d %s %s", p.tag.Count, string(r), p.tag.Name)
}

func max(a, b int) int {
	if a > b { return a }
	return b
}

func min(a, b int) int {
	if a < b { return a }
	return b
}

func abs(a int) int {
	if a < 0 { return 0 - a }
	return a
}

type TyposControl struct {
	alias    []string
	distinct []string
	include  []string
	exclude  []string

	mode      int
	threshold int

	fix        bool
	del        bool
	autofix    bool
	register   bool
	unregister bool

	show_blits   bool
	show_zero    bool
	only_general bool
	no_auto      bool

	start_tag string
	reason    string

	list_settings ListSettings
}

func Typos(ctx *gogram.MessageCtx) {
	var err error
	var creds storage.UserCreds
	storage.DefaultTransact(func(tx *sql.Tx) error {
		creds, err = storage.GetUserCreds(storage.UpdaterSettings{Transaction: storage.Wrap(tx)}, ctx.Msg.From.Id)
		return err
	})
	if err != nil || !creds.Janitor { return }

	var control TyposControl

	control.mode = MODE_READY
	control.threshold = -1
	control.reason = "supervised tag replacement"
	control.list_settings = ListSettings{wild: true}

	for _, token := range ctx.Cmd.Args {
		ltoken := strings.Replace(strings.ToLower(token), "\uFE0F", "", -1)
		switch token {
		case "--list-wild", "-w":  // show unconfirmed possible typos
			control.list_settings.Apply(ListSettings{wild: true})
		case "--list-yes", "-y":   // show confirmed typos
			control.list_settings.Apply(ListSettings{yes: true})
		case "--list-no", "-n":    // show confirmed non-typos
			control.list_settings.Apply(ListSettings{no: true})
		case "--list", "-l":       // show confirmed typos and non-typos
			control.list_settings.Apply(ListSettings{yes: true, no: true})
		case "--show-blits", "-b": // typos which are blits
			control.show_blits = true
		case "--show-zero", "-z":  // typos which have zero posts
			control.show_zero = true
		case "--no-auto", "-x":    // don't automatically count start tag as an alias
			control.no_auto = true
		case "--general", "-g":    // show tags which are general tags
			control.only_general = true
		case "--threshold", "-t":  // set the edit distance threshold
			control.mode = MODE_THRESHOLD
		case "--reason", "-r":     // set the edit reason
			control.mode = MODE_REASON
		case "--select", "-s":     // select a specific tag
			control.mode = MODE_SELECT
		case "--skip", "-k":       // deselect a specific tag
			control.mode = MODE_SKIP
		case "--alias", "-a":      // select all tags similar to a specific tag
			control.mode = MODE_ALIAS
		case "--distinct", "-d":   // deselect all tags more similar to a specific tag
			control.mode = MODE_DISTINCT
		case "--exclude", "-E":     // mark all selected tags as non-typos
			control.mode = MODE_EXCLUDE
		case "--include", "-I":     // mark all selected tags as typos
			control.mode = MODE_INCLUDE
		case "--delete", "-D":      // forget all selected tags completely.
			control.mode = MODE_DELETE
		case "--autofix", "-A":     // mark all selected tags for automatic fixes
			control.mode = MODE_AUTOFIX
		case "--fix", "-F":         // fix all matching posts immediately
			control.mode = MODE_FIX
		default:
			switch control.mode {
			case MODE_READY:
				control.start_tag = ltoken
			case MODE_THRESHOLD:
				t, err := strconv.Atoi(token)
				if err == nil { control.threshold = t }
			case MODE_SELECT:
				control.include = append(control.include, ltoken)
			case MODE_SKIP:
				control.exclude = append(control.exclude, ltoken)
			case MODE_DISTINCT:
				control.distinct = append(control.distinct, ltoken)
			case MODE_ALIAS:
				control.alias = append(control.alias, ltoken)
			case MODE_REASON:
				control.reason = token
			}
			control.mode = MODE_READY
		}

		switch control.mode {
		case MODE_READY, MODE_THRESHOLD, MODE_SELECT, MODE_SKIP, MODE_DISTINCT, MODE_ALIAS, MODE_REASON: // any mode which attempts to read a parameter skips everything past this point.
			continue
		case MODE_FIX:
		default:
			control.register = false
			control.unregister = false
			control.del = false
			control.autofix = false
		}

		switch control.mode {
		case MODE_FIX:
			control.fix = true
		case MODE_INCLUDE:
			control.register = true
		case MODE_EXCLUDE:
			control.unregister = true
		case MODE_DELETE:
			control.del = true
		case MODE_AUTOFIX:
			control.register = true
			control.autofix = true
		}
	}

	switch control.mode {
	case MODE_READY, MODE_LIST:
	default:
		err = fmt.Errorf("missing required argument (%d)", control.mode)
	}

	if err != nil {
		ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: fmt.Sprintf("Bad arguments: %s.", err.Error())}}, nil)
		return
	}

	progress, err := ProgressMessage2(data.OMessage{SendData: data.SendData{TargetData: data.TargetData{ChatId: ctx.Msg.Chat.Id}, ReplyToId: &ctx.Msg.Id, ParseMode: data.ParseHTML}, DisableWebPagePreview: true},
	                                  "Checking for typos...", 3 * time.Second, ctx.Bot)

	err = storage.DefaultTransact(func(tx *sql.Tx) error { return TyposInternal(tx, control, creds, progress) })
	if err != nil {
		progress.SetMessage(fmt.Sprintf("Error: %s", html.EscapeString(err.Error())))
	}
}

func TyposInternal(tx *sql.Tx, control TyposControl, creds storage.UserCreds, progress *ProgMessage) error {
	results := make(map[string]Pair)

	if control.start_tag == "" { return errors.New("You must specify a tag.") }

	get_threshold := func(length int) int {
		switch {
		case control.threshold > 0:
			return control.threshold
		case length < 8:
			return 1
		case length < 16:
			return 2
		case length < 32:
			return 3
		default:
			return 4
		}
	}

	target, err := storage.GetTag(control.start_tag, storage.EnumerateControl{Transaction: storage.Wrap(tx)})
	if err != nil { log.Printf("Error occurred when looking up tag: %s", err.Error()) }
	if target == nil { return errors.New(fmt.Sprintf("Tag doesn't exist: %s", control.start_tag)) }

	alltags, err := storage.EnumerateAllTags(storage.EnumerateControl{Transaction: storage.Wrap(tx)})
	if err != nil { return err }
	blits, err := storage.EnumerateAllBlits(storage.EnumerateControl{Transaction: storage.Wrap(tx)}) // XXX make this return yes and wild blits
	if err != nil { return err }
	typos, err := storage.GetTagTypos(control.start_tag, storage.EnumerateControl{Transaction: storage.Wrap(tx)})
	if err != nil { return err }

	if !control.no_auto {
		control.alias = append(control.alias, control.start_tag)
	}

	var alias_wordsets []wordset.WordSet
	var alias_thresholds []int
	var alias_lengths []int
	for _, tag := range control.alias {
		alias_wordsets = append(alias_wordsets, wordset.MakeWordSet(tag))
		alias_lengths = append(alias_lengths, utf8.RuneCountInString(tag))
		alias_thresholds = append(alias_thresholds, get_threshold(alias_lengths[len(alias_lengths) - 1]))
	}

	show_all_posts := false

	tags_by_name := make(map[string]*types.TTagData)
	for x, tag := range alltags {
		tags_by_name[tag.Name] = &alltags[x]
		if _, blit := blits[tag.Name]; blit && !control.show_blits { continue }
		if zero := tag.ApparentCount(show_all_posts) <= 0; zero && !control.show_zero { continue }
		if tag.Type != types.TCGeneral && !control.only_general { continue }
		if typo, is_typo := typos[tag.Name]; is_typo {
			// if it's already a registered or deregistered typo, only show it if we're
			// in the correct list mode.
			if !control.list_settings.no && typo.Mode == storage.Ignore { continue }
			if !control.list_settings.yes && typo.Mode > storage.Ignore { continue }
			results[tag.Name] = Pair{fixed: target, tag: &alltags[x]}
			continue
		}

		// if it's not a registered or deregistered typo, and we're not showing wild typos, skip it.
		if !control.list_settings.wild { continue }

		var tag_wordset_real *wordset.WordSet
		tag_wordset := func() *wordset.WordSet {
			// populate tag_wordset_real only if it's actually needed, as building a map is expensive
			// and a heuristic may rule the tag out first.
			if tag_wordset_real == nil {
				w := wordset.MakeWordSet(tag.Name)
				tag_wordset_real = &w
			}
			return tag_wordset_real
		}

		tag_len := utf8.RuneCountInString(tag.Name)

		for i, alias_tag := range control.alias {
			// two cheap checks, which establish lower bounds on the edit distance, skip if it's too high
			if abs(tag_len - alias_lengths[i]) > alias_thresholds[i] { continue }
			add, remove, _ := tag_wordset().DifferenceMagnitudes(alias_wordsets[i])
			if max(add, remove) > alias_thresholds[i] { continue }

			// calculate the actual edit distance, which is expensive, and skip this tag if it's too high
			distance := wordset.Levenshtein(tag.Name, alias_tag)
			if distance > alias_thresholds[i] { continue }

			// check if it's closer/equal to something we are excluding, and skip if it is
			for _, item := range control.exclude {
				if tag.Name == item { continue }
			}

			for _, item := range control.distinct {
				if wordset.Levenshtein(tag.Name, item) < distance { continue }
			}

			// these tags are similar!
			results[tag.Name] = Pair{fixed: target, tag: &alltags[x]}
		}
	}

	// now remove any matches which are already aliased to the target tag.
	aliases, err := storage.GetAliasesFor(control.start_tag, storage.EnumerateControl{Transaction: storage.Wrap(tx)})
	if err != nil { log.Printf("Error when searching for aliases to %s: %s", control.start_tag, err.Error()) }
	for _, item := range aliases {
		delete(results, item.Name)
	}

	// aaaaaand finally add any matches manually included.
	for _, item := range control.include {
		if tag, ok := tags_by_name[item]; ok {
			results[item] = Pair{fixed: target, tag: tag}
		}
	}

	var results_ordered []Pair

	total_posts := 0
	for _, v := range results {
		total_posts += v.tag.ApparentCount(show_all_posts)
		results_ordered = append(results_ordered, v)
	}

	sort.Slice(results_ordered, func(i, j int) bool {
		return results_ordered[i].tag.Count > results_ordered[j].tag.Count ||
		results_ordered[i].tag.Count == results_ordered[j].tag.Count && (
		results_ordered[i].tag.Name < results_ordered[j].tag.Name)
	})

	var buf bytes.Buffer
	buf.WriteString("Possible typos:\n<code>")
	for _, p := range results_ordered {
		buf.WriteString(p.String())
		buf.WriteString("\n")
	}
	buf.WriteString("</code>")

	// progress.something
	// ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: buf.String(), ParseMode: data.ParseHTML}}, nil)

	if control.fix {
		updated := 1
		diffs := make(map[int]tags.TagDiff)

		for _, v := range results {
			array, err := storage.PostsWithTag(*v.tag, storage.EnumerateControl{Transaction: storage.Wrap(tx)})
			if err != nil { return err }

			for _, post := range array {
				d := diffs[post.Id]
				d.Add(v.fixed.Name)
				d.Remove(v.tag.Name)
				diffs[post.Id] = d
			}
		}

		// we now know for sure that exactly this many edits are required
		total_posts = len(diffs)

		for id, diff := range diffs {
			if diff.IsZero() { continue }

			reason := fmt.Sprintf("Bulk retag: %s (%s)", diff.APIString(), control.reason)
			newp, err := api.UpdatePost(creds.User, creds.ApiKey, id, diff, nil, nil, nil, nil, &reason)

			if err == api.PostIsDeleted {
				log.Printf("Post was deleted which we didn't know about? DB consistency? (%d)\n", id)
				err = storage.MarkPostDeleted(id, storage.UpdaterSettings{Transaction: storage.Wrap(tx)})
			}

			if err != nil { return err }

			if newp != nil {
				err = storage.UpdatePost(*newp, storage.UpdaterSettings{Transaction: storage.Wrap(tx)})
				if err != nil { return err }
			}

			progress.SetStatus(fmt.Sprintf("(%d/%d %d: <code>%s</code>)", updated, total_posts, id, diff.APIString()))
			updated++
		}
	}

	if control.del {
		for _, action := range results {
			err = storage.DelTagTypoByTag(tx, action.TypoData())
			if err != nil { return err }
		}
	}

	if control.register || control.unregister || control.autofix {
		for _, action := range results {
			err = storage.SetTagTypoByTag(tx, action.TypoData(), control.register || control.autofix, control.autofix)
			if err != nil { return err }
		}
	}

	return nil
}

func RefetchDeletedPostsCommand(ctx *gogram.MessageCtx) {
	var err error
	settings := storage.UpdaterSettings{}
	settings.Transaction, err = storage.NewTxBox()
	if err != nil { log.Println(err.Error(), "newtxbox") }

	err = RefetchDeletedPosts(ctx, settings, nil)
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

func RefetchDeletedPosts(ctx *gogram.MessageCtx, settings storage.UpdaterSettings, progress *ProgMessage) (error) {
	creds, err := storage.GetUserCreds(settings, ctx.Msg.From.Id)
	if err != nil || !creds.Janitor { return err }

	if progress == nil {
		progress, _ = ProgressMessage2(data.OMessage{SendData: data.SendData{ReplyToId: &ctx.Msg.Id, ParseMode: data.ParseHTML}, DisableWebPagePreview: true},
	                                       "", 3 * time.Second, ctx.Bot)
		defer progress.Close()
	}

	return RefetchDeletedPostsInternal(creds.User, creds.ApiKey, settings, progress)
}

func RefetchDeletedPostsInternal(user, api_key string, settings storage.UpdaterSettings, progress *ProgMessage) (error) {
	progress.AppendNotice("Syncing deleted posts...")

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

		progress.SetStatus(fmt.Sprintf("%.1f%%", float32(latest_id) * 100.0 / float32(highest_id)))
	}

	close(fixed_posts)
	wg.Wait()

	progress.SetStatus("done.")
	return nil
}

type ListSettings struct {
	overridden, wild, yes, no bool
}

func (ls *ListSettings) Apply(other ListSettings) {
	// the first change overwrites all of the defaults.
	// subsequent ones are cumulative
	if !ls.overridden {
		*ls = other
		ls.overridden = true
	} else {
		ls.wild = ls.wild || other.wild
		ls.yes = ls.yes || other.yes
		ls.no = ls.no || other.no
	}
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

	creds, err := storage.GetUserCreds(storage.UpdaterSettings{Transaction: txbox}, ctx.Msg.From.Id)
	if err != nil || !creds.Janitor { return }

	mode := MODE_LIST
	include, exclude, to_delete := make(map[string]bool), make(map[string]bool), make(map[string]bool)

	list_settings := ListSettings{wild: true}

	for _, token := range ctx.Cmd.Args {
		ltoken := strings.Replace(strings.ToLower(token), "\uFE0F", "", -1)
		if mode == MODE_EXCLUDE {
			exclude[ltoken] = true
		} else if mode == MODE_INCLUDE {
			include[ltoken] = true
		} else if mode == MODE_DELETE {
			to_delete[ltoken] = true
		} else if token == "--include" || token == "-I" {
			mode = MODE_INCLUDE
		} else if token == "--exclude" || token == "-E" {
			mode = MODE_EXCLUDE
		} else if token == "--delete" || token == "-D" {
			mode = MODE_DELETE
		} else if token == "--list-wild" || token == "-w" {
			list_settings.Apply(ListSettings{wild: true})
		} else if token == "--list-yes" || token == "-y" {
			list_settings.Apply(ListSettings{yes: true})
		} else if token == "--list-no" || token == "-n" {
			list_settings.Apply(ListSettings{no: true})
		} else if token == "--list" || token == "-l" {
			list_settings.Apply(ListSettings{yes: true, no: true})
		}
	}

	if mode == MODE_LIST {
		yesblits, noblits, wildblits, err := storage.GetBlits(list_settings.yes, list_settings.no, list_settings.wild, ctrl)
		if err != nil {
			ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: "Whoops! " + html.EscapeString(err.Error()), ParseMode: data.ParseHTML}}, nil)
			return
		}

		var buf bytes.Buffer

		buf.WriteString("== Blit List ==\n")
		for _, b := range yesblits {
			buf.WriteString(fmt.Sprintf("%v\n", b))
		}
		buf.WriteString("\n== Marked Non-Blit List ==\n")
		for _, b := range noblits {
			buf.WriteString(fmt.Sprintf("%v\n", b))
		}
		buf.WriteString("\n== Wild Blit List ==\n")
		for _, b := range wildblits {
			buf.WriteString(fmt.Sprintf("%v\n", b))
		}

		ctx.Bot.Remote.SendDocumentAsync(data.ODocument{SendData: data.SendData{Text: "Blit List", ReplyToId: &ctx.Msg.Id, TargetData: data.TargetData{ChatId: ctx.Msg.Chat.Id}}, MediaData: data.MediaData{File: ioutil.NopCloser(&buf), FileName: "blitlist.txt"}}, nil)
		return
	}

	var bad_tags []string
	for tag, _ := range include {
		err := storage.MarkBlitByName(tag, true, ctrl)
		if err == storage.ErrNoTag {
			bad_tags = append(bad_tags, tag)
		} else if err != nil {
			ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: "Whoops! " + html.EscapeString(err.Error()), ParseMode: data.ParseHTML}}, nil)
			return
		}
	}
	for tag, _ := range exclude {
		err := storage.MarkBlitByName(tag, false, ctrl)
		if err == storage.ErrNoTag {
			bad_tags = append(bad_tags, tag)
		} else if err != nil {
			ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: "Whoops! " + html.EscapeString(err.Error()), ParseMode: data.ParseHTML}}, nil)
			return
		}
	}
	for tag, _ := range to_delete {
		err := storage.DeleteBlitByName(tag, ctrl)
		if err != nil {
			ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: "Whoops! " + html.EscapeString(err.Error()), ParseMode: data.ParseHTML}}, nil)
			return
		}
	}

	if bad_tags == nil {
		ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: "All blit changes completed successfully.", ParseMode: data.ParseHTML}}, nil)
	} else {
		ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: fmt.Sprintf("The following blits failed to update properly (perhaps they correspond to tags which do not exist?)\n%s", strings.Join(bad_tags, " ")), ParseMode: data.ParseHTML}}, nil)
	}

	ctrl.Transaction.MarkForCommit()
}

func GetAllWildCats(tagMap map[string]*types.TTagData, blitMap map[string]*storage.BlitData, ratio int, with_empty, with_typed bool) []Triplet { // also needs blits
	var candidates []Triplet

	for k, v := range tagMap {
		if !with_empty && v.Count == 0 { continue } // skip anything with no posts.
		if !with_typed && v.Type != types.TCGeneral { continue } // skip anything that's not a general tag.
		runes := []rune(k)
		for i := 1; i < len(runes); i++ {
			prefix, prefix_ok := tagMap[string(runes[:i])]
			suffix, suffix_ok := tagMap[string(runes[i:])]
			_, ok1 := blitMap[string(runes[:i])]
			_, ok2 := blitMap[string(runes[i:])]
			if ok1 || ok2 { continue }
			if prefix_ok && suffix_ok && ratio * v.Count < prefix.Count && ratio * v.Count < suffix.Count && v.Type == types.TCGeneral {
				candidates = append(candidates, Triplet{tag: v, subtag1: prefix, subtag2: suffix})
			}
		}
	}

	return candidates
}

func GetSpecificWildCats(tagMap map[string]*types.TTagData, blitMap map[string]*storage.BlitData, search string, prefixes, suffixes bool, ratio int, with_empty, with_typed bool) []Triplet {
	var candidates []Triplet

	// `search` isn't a tag, and therefore no cat can possibly consist of it.
	tag, ok := tagMap[search]
	if !ok {
		return nil
	}

	for k, v := range tagMap {
		if !with_empty && v.Count <= 0 { continue } // skip anything with no posts.
		if !with_typed && v.Type != types.TCGeneral { continue } // skip anything that's not a general tag.
		if len(k) < len(search) + 1 { continue } // skip anything that's too short to hold the search string.
		if prefixes {
			func() {
				suffixString := strings.TrimPrefix(k, search)
				if _, ok := blitMap[suffixString]; ok { return }
				if len(suffixString) != len(k) {
					suffix, suffix_ok := tagMap[suffixString]
					if suffix_ok && ratio * v.Count < tag.Count && ratio * v.Count < suffix.Count {
						candidates = append(candidates, Triplet{tag: v, subtag1: tag, subtag2: suffix})
					}
				}
			}()
		}
		if suffixes {
			func() {
				prefixString := strings.TrimSuffix(k, search)
				if _, ok := blitMap[prefixString]; ok { return }
				if len(prefixString) != len(k) {
					prefix, prefix_ok := tagMap[prefixString]
					if prefix_ok && ratio * v.Count < tag.Count && ratio * v.Count < prefix.Count {
						candidates = append(candidates, Triplet{tag: v, subtag1: prefix, subtag2: tag})
					}
				}
			}()
		}
	}

	return candidates
}

type Triplet struct {
	tag, subtag1, subtag2 *types.TTagData
}

func (t Triplet) String() string {
	if t.subtag1 != nil && t.subtag2 != nil {
		return fmt.Sprintf("%-32s %s + %s", t.tag.Name, t.subtag1.Name, t.subtag2.Name)
	} else {
		return t.tag.Name
	}
}

func (t Triplet) CatData() storage.CatData {
	return storage.CatData{Merged: *t.tag, First: t.subtag1, Second: t.subtag2}
}

type CatsControl struct {
	cats            []Triplet

	fix_list        []Triplet
	exclude_list    []Triplet
	prompt_list     []Triplet
	autofix_list    []Triplet
	delete_list     []Triplet

	mode              int
	ratio             int

	inspect_tag       string

	prefix_only       bool
	suffix_only       bool
	with_blits        bool
	with_empty        bool
	with_typed        bool
	needs_empty       bool
	needs_empty_fixed bool

	list_settings     ListSettings
}

func Concatenations(ctx *gogram.MessageCtx) {
	var err error
	var creds storage.UserCreds
	storage.DefaultTransact(func(tx *sql.Tx) error {
		creds, err = storage.GetUserCreds(storage.UpdaterSettings{Transaction: storage.Wrap(tx)}, ctx.Msg.From.Id)
		return err
	})
	if err != nil || !creds.Janitor { return }

	var control CatsControl

	var current_list []Triplet
	var select_first   string

	control.ratio = 10
	control.mode = MODE_LIST
	control.prefix_only = true
	control.suffix_only = true
	control.list_settings = ListSettings{wild: true}

	// read candidate cats from the replied message, if there is one, so -e can select them
	header := "Here are some random concatenated tags:"
	if ctx.Msg.ReplyToMessage != nil {
		text := ctx.Msg.ReplyToMessage.Text
		if text != nil {
			prev_cats := strings.Split(*text, "\n")
			if prev_cats[0] == header {
				prev_cats = prev_cats[1:]
				for _, line := range prev_cats {
					t := Triplet{&types.TTagData{}, &types.TTagData{}, &types.TTagData{}}
					tokens := strings.Split(line, " ")
					if !(len(tokens) == 4 && tokens[2] == "+") { continue }
					t.subtag1.Name = tokens[1]
					t.subtag2.Name = tokens[3]
					t.tag.Name = t.subtag1.Name + t.subtag2.Name
					control.cats = append(control.cats, t)
				}
			}
		}
	}

	for _, token := range ctx.Cmd.Args {
		ltoken := strings.Replace(strings.ToLower(token), "\uFE0F", "", -1)
		switch token {
		case "--list-wild", "-w":
			control.list_settings.Apply(ListSettings{wild: true})
		case "--list-yes", "-y":
			control.list_settings.Apply(ListSettings{yes: true})
		case "--list-no", "-n":
			control.list_settings.Apply(ListSettings{no: true})
		case "--list", "-l":
			control.list_settings.Apply(ListSettings{yes: true, no: true})

		case "--inspect", "-i":
			control.mode = MODE_INSPECT
		case "--first", "-1":
			control.prefix_only, control.suffix_only = true, false
		case "--second", "-2":
			control.prefix_only, control.suffix_only = false, true
		case "--ratio", "-r":
			control.mode = MODE_FREQ_RATIO
		case "--with-blits", "-b":
			control.with_blits = true
		case "--with-empty", "-0":
			control.with_empty = true
		case "--with-typed", "-t":
			control.with_typed = true

		case "--entry", "-e":
			control.mode = MODE_ENTRY
		case "--select", "-s":
			control.mode = MODE_SELECT_1
		case "--cat-name", "-c":
			control.mode = MODE_SELECT_CAT

		case "--exclude", "-E":
			control.mode = MODE_EXCLUDE
		case "--prompt", "-P":
			control.mode = MODE_PROMPT
		case "--autofix", "-A":
			control.mode = MODE_AUTOFIX
		case "--delete", "-D":
			control.mode = MODE_DELETE
		case "--fix", "-F":
			control.mode = MODE_FIX
		default: // we failed to parse a mode changing token, so try to parse an argument for the current mode.
			switch control.mode {
			case MODE_ENTRY, MODE_SELECT_2, MODE_SELECT_CAT: // all cases which write to current_list
				if control.needs_empty || control.needs_empty_fixed {
					current_list = current_list[0:0]
					control.needs_empty = false
					control.needs_empty_fixed = false
				}
			}

			switch control.mode {
			case MODE_INSPECT:
				control.inspect_tag = ltoken
				control.mode = MODE_LIST
			case MODE_FREQ_RATIO:
				var temp int
				temp, err = strconv.Atoi(ltoken)
				if err == nil {
					control.ratio = temp
				} else {
					break
				}
				control.mode = MODE_LIST
			case MODE_ENTRY:
				var temp int
				temp, err = strconv.Atoi(ltoken)
				if err == nil && temp >= 0 && temp < len(control.cats) {
					current_list = append(current_list, control.cats[temp])
				} else {
					break
				}
				control.mode = MODE_READY
			case MODE_SELECT_1:
				select_first = ltoken
				control.mode = MODE_SELECT_2
			case MODE_SELECT_2:
				current_list = append(current_list, Triplet{&types.TTagData{Name: select_first + ltoken}, &types.TTagData{Name: select_first}, &types.TTagData{Name: ltoken}})
				control.mode = MODE_READY
			case MODE_SELECT_CAT:
				current_list = append(current_list, Triplet{tag: &types.TTagData{Name: ltoken}})
				control.mode = MODE_READY
			}
		}

		switch control.mode {
		case MODE_READY, MODE_LIST, MODE_INSPECT, MODE_FREQ_RATIO, MODE_ENTRY, MODE_SELECT_1, MODE_SELECT_2, MODE_SELECT_CAT: // any mode which attempts to read a parameter skips everything past this point.
			continue
		case MODE_FIX: // fix is special because it should be read once but not mutually exclusively with the others so handle it here.
			control.mode = MODE_READY
			if control.needs_empty_fixed { continue }
			control.fix_list = append(control.fix_list, current_list...)
			control.needs_empty_fixed = true
			continue
		default: // for any other edit control mode, continue if we've already pulled the list but not added anything to it.
			if control.needs_empty {
				control.mode = MODE_READY
				continue
			}
		}

		switch control.mode {
		case MODE_EXCLUDE:
			control.exclude_list = append(control.exclude_list, current_list...)
		case MODE_PROMPT:
			control.prompt_list = append(control.prompt_list, current_list...)
		case MODE_AUTOFIX:
			control.autofix_list = append(control.autofix_list, current_list...)
		case MODE_DELETE:
			control.delete_list = append(control.delete_list, current_list...)
		}

		control.needs_empty = true
		control.mode = MODE_READY
	}

	switch control.mode {
	case MODE_READY, MODE_LIST:
	default:
		err = fmt.Errorf("missing required argument (%d)", control.mode)
	}

	// If there was an error processing command line arguments, report the error and bail.
	if err != nil {
		ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: "Error while processing command: " + html.EscapeString(err.Error()), ParseMode: data.ParseHTML}}, nil)
		return
	}

	progress, err := ProgressMessage2(data.OMessage{SendData: data.SendData{TargetData: data.TargetData{ChatId: ctx.Msg.Chat.Id}, ReplyToId: &ctx.Msg.Id, ParseMode: data.ParseHTML}, DisableWebPagePreview: true},
	                                  "", 3 * time.Second, ctx.Bot)

	if err != nil {
		ctx.Bot.ErrorLog.Println("Failed to create ProgMessage")
		return
	}

	defer progress.Close()

	err = storage.DefaultTransact(func(tx *sql.Tx) error { return CatsInternal(tx, control, creds, progress) })
	if err != nil {
		progress.SetMessage(fmt.Sprintf("Whoops! An error occurred: %s", html.EscapeString(err.Error())))
	}
}

func CatsInternal(tx *sql.Tx, control CatsControl, creds storage.UserCreds, progress *ProgMessage) error {
	var err error

	if control.mode == MODE_LIST {
		exceptions_yes, exceptions_no, err := storage.GetCats(tx, control.list_settings.yes, control.list_settings.no)
		if err != nil { return err }

		var wildCandidates []Triplet
		var yesCandidates, noCandidates []storage.CatData

		if control.list_settings.wild {
			tags, err := storage.EnumerateAllTags(storage.EnumerateControl{Transaction: storage.Wrap(tx)})
			if err != nil { return err }

			// fetch blits the same way
			var blits_yes, blits_wild []storage.BlitData
			if !control.with_blits {
				blits_yes, _, blits_wild, err = storage.GetBlits(true, false, true, storage.EnumerateControl{Transaction: storage.Wrap(tx)})
				if err != nil { return err }
			}

			tagMap := make(map[string]*types.TTagData, len(tags))
			tagMapById := make(map[int]*types.TTagData, len(tags))
			blitsMap := make(map[string]*storage.BlitData, len(blits_yes) + len(blits_wild))
			exceptionMap := make(map[int]bool, len(exceptions_yes) + len(exceptions_no))

			for _, t := range exceptions_yes {
				exceptionMap[t.Merged.Id] = true
			}
			for _, t := range exceptions_no {
				exceptionMap[t.Merged.Id] = true
			}

			for i, _ := range blits_yes {
				blitsMap[blits_yes[i].Name] = &blits_yes[i]
			}
			for i, _ := range blits_wild {
				blitsMap[blits_wild[i].Name] = &blits_wild[i]
			}

			for i := range tags {
				tagMapById[tags[i].Id] = &tags[i]

				if !exceptionMap[tags[i].Id] {
					tagMap[tags[i].Name] = &tags[i]
				}
			}

			if control.inspect_tag == "" {
				wildCandidates = GetAllWildCats(tagMap, blitsMap, control.ratio, control.with_empty, control.with_typed)
			} else {
				wildCandidates = GetSpecificWildCats(tagMap, blitsMap, control.inspect_tag, control.prefix_only, control.suffix_only, control.ratio, control.with_empty, control.with_typed)
			}
		}
		if control.list_settings.yes {
			if control.inspect_tag == "" {
				yesCandidates = exceptions_yes
			} else {
				tag, err := storage.GetTag(control.inspect_tag, storage.EnumerateControl{Transaction: storage.Wrap(tx)})
				if err != nil { return err }

				for _, v := range exceptions_yes {
					if control.prefix_only && v.First.Id == tag.Id {
						yesCandidates = append(yesCandidates, v)
					}
					if control.suffix_only && v.Second.Id == tag.Id {
						yesCandidates = append(yesCandidates, v)
					}
				}
			}
		}
		if control.list_settings.no {
			if control.inspect_tag == "" {
				noCandidates = exceptions_no
			} else {
				for _, v := range exceptions_no {
					if control.prefix_only && strings.HasPrefix(v.Merged.Name, control.inspect_tag) {
						noCandidates = append(noCandidates, v)
					}
					if control.suffix_only && strings.HasSuffix(v.Merged.Name, control.inspect_tag) {
						noCandidates = append(noCandidates, v)
					}
				}
			}
		}

		var buf bytes.Buffer

		buf.WriteString("== Confirmed Cat List ==\n")
		for _, c := range yesCandidates {
			buf.WriteString(fmt.Sprintf("%v\n", c))
		}
		buf.WriteString("\n== Confirmed Non-Cat List ==\n")
		for _, c := range noCandidates {
			buf.WriteString(fmt.Sprintf("%v\n", c))
		}
		buf.WriteString("\n== Wild Cat List ==\n")
		for _, c := range wildCandidates {
			buf.WriteString(fmt.Sprintf("%v\n", c))
		}

		// teach it to write a format it can understand with -e

		// ctx.Bot.Remote.SendDocumentAsync(data.ODocument{SendData: data.SendData{Text: "Cat List", ReplyToId: &ctx.Msg.Id, TargetData: data.TargetData{ChatId: ctx.Msg.Chat.Id}}, MediaData: data.MediaData{File: ioutil.NopCloser(&buf), FileName: "catlist.txt"}}, nil)
		return nil
	}

	work := []struct {
		list []Triplet
		mark, autofix bool
	}{
		{control.exclude_list, false, false},
		{control.prompt_list, true, false},
		{control.autofix_list, true, true},
		{control.fix_list, false, false},
		{control.delete_list, false, false},
	}

	for _, job := range work[1:4] { // only do subtags check for prompt, autofix, and fix lists
		for _, triplet := range job.list {
			if triplet.subtag1 == nil || triplet.subtag2 == nil { return errors.New("tried to add or update a cat without specifying subtags") }
		}
	}

	for _, job := range work[0:3] { // process normal additions and removals from the exclude, prompt, and autofix lists
		for _, triplet := range job.list {
			err = storage.SetCatByTagNames(tx, triplet.CatData(), job.mark, job.autofix)
			if err != nil { return err }
		}
	}

	for _, triplet := range control.delete_list {
		err = storage.DeleteCatByTagNames(tx, triplet.CatData())
		if err != nil { return err }
	}

	if len(control.fix_list) != 0 {
		var updated int
		for _, triplet := range control.fix_list {
			triplet.tag, err = storage.GetTag(triplet.tag.Name, storage.EnumerateControl{Transaction: storage.Wrap(tx)})
			if triplet.tag == nil { continue }

			posts, err := storage.PostsWithTag(*triplet.tag, storage.EnumerateControl{Transaction: storage.Wrap(tx)})
			if err != nil {
				return err
			}

			reason := fmt.Sprintf("Bulk retag: %s --> %s, %s (fixed concatenated tags)", triplet.tag.Name, triplet.subtag1.Name, triplet.subtag2.Name)
			for _, p := range posts {
				updated++
				var diff tags.TagDiff
				diff.Add(triplet.subtag1.Name)
				diff.Add(triplet.subtag2.Name)
				diff.Remove(triplet.tag.Name)
				newp, err := api.UpdatePost(creds.User, creds.ApiKey, p.Id, diff, nil, nil, nil, nil, &reason)
				if err != nil { return err }

				if newp != nil {
					err = storage.UpdatePost(*newp, storage.UpdaterSettings{Transaction: storage.Wrap(tx)})
					if err != nil { return err }
				}
			}
		}
	}

	return nil
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
