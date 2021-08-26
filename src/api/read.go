package api

import (
	"api/types"
	"strconv"
	"log"
	"fmt"
)

type FailedCall struct {
	Success bool `json:"success"`
}

func TagSearch(user, apitoken string, tags string, page int, limit int) (results types.TResultArray, e error) {
	url := "/post/index.json"
	r, e := api.New(url).
			URLArgDefault("login", user, "").
			URLArgDefault("password_hash", apitoken, "").
			URLArg("tags", tags).
			URLArg("page", strconv.Itoa(page)).
			URLArg("limit", strconv.Itoa(limit)).
			Into(&results).
			Do()

	caller := "unauthenticated"
	if user != "" {
		caller = fmt.Sprintf("as %s", user)
	}

	if e != nil {
		log.Printf("[api     ] API call: %s [%s] (ERROR: %s)\n", url, caller, e.Error())
		return
	} else if r != nil {
		log.Printf("[api     ] API call: %s [%s] (%s, %d results)\n", url, caller, r.Status, len(results))
	}

	return
}

func TestLogin(user, apitoken string) (bool, error) {
	url := "/dmail/inbox.json"
	var canary interface{}

	r, e := api.New(url).
			URLArg("login", user).
			URLArg("password_hash", apitoken).
			Into(&canary).
			Do()

	log.Printf("[api     ] API call: %s [as %s] (%s)\n", url, user, r.Status)

	if e != nil {
		return false, e
	}

	switch v := canary.(type) {
	case map[string]interface{}:
		switch w := v["success"].(type) {
		case bool:
			return w, nil
		default:
			return true, nil
		} 
	default:
		return true, nil
	}
}

func ListTagHistory(user, apitoken string, limit int, before, after *int) (types.THistoryArray, error) {
	url := "/post_tag_history/index.json"

	var hist types.THistoryArray

	req := api.New(url).
			URLArg("login", user).
			URLArg("password_hash", apitoken).
			URLArg("limit", strconv.Itoa(limit)).
			Into(&hist)
	if before != nil { req.URLArg("before", strconv.Itoa(*before)) }
	if after != nil { req.URLArg("after", strconv.Itoa(*after)) }
	r, e := req.Do()

	log.Printf("[api     ] API call: %s [as %s] (%s)\n", url, user, r.Status)

	if e != nil {
		return nil, e
	}
	return hist, nil
}

func ListOnePageOfTags(user, apitoken string, page int, list types.TTagInfoArray) (types.TTagInfoArray, bool, int, error) {
	url := "/tag/index.json"

	var results types.TTagInfoArray

	r, e := api.New(url).
			URLArg("login", user).
			URLArg("password_hash", apitoken).
			URLArg("limit", "10000").
			URLArg("order", "date").
			URLArg("show_empty_tags", "true").
			URLArg("page", strconv.Itoa(page)).
			Into(&results).
			Do()

	log.Printf("[api     ] API call: %s [as %s] (%s)\n", url, user, r.Status)

	if e != nil {
		return list, true, page, e
	}

	list = append(list, results...)
	return list, len(results) != 0, page + 1, nil
}

func ListOnePageOfAliases(user, apitoken string, page int, list types.TAliasInfoArray) (types.TAliasInfoArray, bool, int, error) {
	url := "/tag_alias/index.json"

	var aliases types.TAliasInfoArray

	r, e := api.New(url).
			URLArg("login", user).
			URLArg("password_hash", apitoken).
			URLArg("limit", "10000").
			URLArg("order", "date").
			URLArg("approved", "true").
			URLArg("page", strconv.Itoa(page)).
			Into(&aliases).
			Do()

	log.Printf("[api     ] API call: %s [as %s] (%s)\n", url, user, r.Status)

	if e != nil {
		return list, true, page, e
	}

	list = append(list, aliases...)
	return list, len(aliases) != 0, page + 1, nil
}

func ListOnePageOfPosts(user, apitoken string, before int) (types.TResultArray, bool, int, error) {
	url := "/post/index.json"

	var posts types.TResultArray

	req := api.New(url).
			URLArg("login", user).
			URLArg("password_hash", apitoken).
			URLArg("limit", "10000").
			Into(&posts)
	if before > 0 { req.URLArg("before_id", strconv.Itoa(before)) }
	r, e := req.Do()

	log.Printf("[api     ] API call: %s [as %s] (%s)\n", url, user, r.Status)

	if e != nil {
		return posts, true, before, e
	}

	if len(posts) != 0 { before = posts[len(posts) - 1].Id }
	return posts, len(posts) != 0, before, nil
}


func FetchOnePost(user, apitoken string, id int) (*types.TSearchResult, error) {
	url := "/post/show.json"

	var post types.TSearchResult

	r, e := api.New(url).
			URLArg("login", user).
			URLArg("password_hash", apitoken).
			URLArg("id", strconv.Itoa(id)).
			Into(&post).
			Do()

	log.Printf("[api     ] API call: %s [as %s] (%s)\n", url, user, r.Status)

	if e != nil {
		return nil, e
	}

	if post.Id != 0 { return &post, nil }
	return nil, nil
}

func ListOnePageOfDeletedPosts(user, apitoken string, page int) (types.TResultArray, bool, int, error) {
	url := "/post/deleted_index.json"

	var posts types.TResultArray

	r, e := api.New(url).
			URLArg("login", user).
			URLArg("password_hash", apitoken).
			URLArg("limit", "10000").
			URLArg("page", strconv.Itoa(page)).
			Into(&posts).
			Do()

	log.Printf("[api     ] API call: %s [as %s] (%s)\n", url, user, r.Status)

	if e != nil {
		return posts, true, page, e
	}

	return posts, len(posts) != 0, page + 1, nil
}

func GetTagData(user, apitoken string, id int) (*types.TTagData, error) {
	url := "/tag/show.json"

	var tag types.TTagData

	r, e := api.New(url).
			URLArg("login", user).
			URLArg("password_hash", apitoken).
			URLArg("id", strconv.Itoa(id)).
			Into(&tag).
			Do()

	log.Printf("[api     ] API call: %s [as %s] (%s)\n", url, user, r.Status)

	if e != nil {
		return nil, e
	}

	return &tag, nil
}
