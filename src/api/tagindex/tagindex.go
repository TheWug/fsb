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
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
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
	MODE_MARK = iota
	MODE_DELETE = iota
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

	creds, err := storage.GetUserCreds(storage.UpdaterSettings{Transaction: ctrl.Transaction}, ctx.Msg.From.Id)
	if (err != nil || !creds.Janitor) && fix {
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

	progress, err := ProgressMessage2(data.OMessage{SendData: data.SendData{TargetData: data.TargetData{ChatId: ctx.Msg.Chat.Id}, ReplyToId: &ctx.Msg.Id, ParseMode: data.ParseHTML}, DisableWebPagePreview: true},
	                                  "Checking for typos...", 3 * time.Second, ctx.Bot)
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
			newp, err := api.UpdatePost(creds.User, creds.ApiKey, id, diff, nil, nil, nil, nil, &reason)

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

	creds, err := storage.GetUserCreds(storage.UpdaterSettings{Transaction: txbox}, ctx.Msg.From.Id)
	if err != nil || !creds.Janitor { return }

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
			message.WriteString(fmt.Sprintf("Adding to ignore list: <code>%s</code>\n", html.EscapeString(tag)))
		}
		for _, tag := range manual_unignore {
			storage.ClearCatsException(tag, ctrl)
			message.WriteString(fmt.Sprintf("Removing from ignore list: <code>%s</code>\n", html.EscapeString(tag)))
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
			message.WriteString(fmt.Sprintf("%d: <code>%s</code> + <code>%s</code> (%d)\n", i, html.EscapeString(t.subtag1.Name), html.EscapeString(t.subtag2.Name), t.tag.Count))
		}

		ctx.ReplyAsync(data.OMessage{SendData: data.SendData{Text: message.String(), ParseMode: data.ParseHTML}}, nil)
		return
	}

	progress, _ := ProgressMessage2(data.OMessage{SendData: data.SendData{ReplyToId: &ctx.Msg.Id, ParseMode: data.ParseHTML}, DisableWebPagePreview: true},
                                       "", 3 * time.Second, ctx.Bot)
	defer progress.Close()

	var message bytes.Buffer
	for _, i := range ignore_list {
		storage.SetCatsException(cats[i].tag.Name, ctrl)
		progress.AppendNotice(fmt.Sprintf("Adding %d to ignore list: <code>%s</code>\n", i, html.EscapeString(cats[i].tag.Name)))
	}
	progress.AppendNotice("\nUpdating posts which need fixing...")

	updated := 1
	for _, i := range fix_list {
		t, err := storage.GetTag(cats[i].tag.Name, storage.EnumerateControl{Transaction: txbox})
		cats[i].tag = *t
		posts, err := storage.LocalTagSearch(cats[i].tag, storage.EnumerateControl{Transaction: txbox})
		if err != nil {
			progress.SetStatus(fmt.Sprintf(" (error: %s)", html.EscapeString(err.Error())))
			return
		}

		reason := fmt.Sprintf("Bulk retag: %s --> %s, %s (fixed concatenated tags)", cats[i].tag.Name, cats[i].subtag1.Name, cats[i].subtag2.Name)
		for _, p := range posts {
			var diff tags.TagDiff
			diff.Add(cats[i].subtag1.Name)
			diff.Add(cats[i].subtag2.Name)
			diff.Remove(cats[i].tag.Name)
			newp, err := api.UpdatePost(creds.User, creds.ApiKey, p.Id, diff, nil, nil, nil, nil, &reason)
			err = nil
			if err != nil {
				progress.SetStatus(fmt.Sprintf(" (error: %s)", html.EscapeString(err.Error())))
				return
			}

			if newp != nil {
				err = storage.UpdatePost(*newp, storage.UpdaterSettings{Transaction: txbox})
				if err != nil {
					progress.SetStatus(fmt.Sprintf(" (error: %s)", html.EscapeString(err.Error())))
					return
				}
			}

			progress.SetStatus(fmt.Sprintf(" (%d/%d %d: <code>%s</code> -> <code>%s</code>, <code>%s</code>)", updated, -1, p.Id, html.EscapeString(cats[i].tag.Name), html.EscapeString(cats[i].subtag1.Name), html.EscapeString(cats[i].subtag2.Name)))

			updated++
		}
		progress.SetStatus("done.")
		message.WriteString(fmt.Sprintf("Fixing %d: <code>%s</code> -> <code>%s, %s</code>\n", i, html.EscapeString(cats[i].tag.Name), html.EscapeString(cats[i].subtag1.Name), html.EscapeString(cats[i].subtag2.Name)))
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
