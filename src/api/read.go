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

func TagSearch(user, apitoken string, tags string, page int, limit int) (types.TResultArray, error) {
	temp := struct {
		Posts types.TResultArray `json:"posts"`
	}{}

	url := "/posts.json"
	r, e := api.New(url).
			BasicAuthentication(user, apitoken).
			URLArg("tags", tags).
			URLArg("page", strconv.Itoa(page)).
			URLArg("limit", strconv.Itoa(limit)).
			Into(&temp).
			Do()

	caller := "unauthenticated"
	if user != "" {
		caller = fmt.Sprintf("as %s", user)
	}

	if e != nil {
		log.Printf("[api     ] API call: %s [%s] (ERROR: %s)\n", url, caller, e.Error())
		return nil, e
	} else if r != nil {
		log.Printf("[api     ] API call: %s [%s] (%s, %d results)\n", url, caller, r.Status, len(temp.Posts))
	}

	return temp.Posts, e
}

func TestLogin(user, apitoken string) (bool, error) {
	url := "/dmails.json"
	var canary interface{}

	r, e := api.New(url).
			BasicAuthentication(user, apitoken).
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

func ListTagHistory(user, apitoken string, limit int, before, after *int) (types.THistoryArray, error) { // moved to post_versions, requires rework
	url := "/post_tag_history/index.json"

	var hist types.THistoryArray

	req := api.New(url).
			BasicAuthentication(user, apitoken).
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
	url := "/tags.json"

	var results types.TTagInfoArray

	r, e := api.New(url).
			BasicAuthentication(user, apitoken).
			URLArg("limit", "10000").
			URLArg("page", strconv.Itoa(page)).
			URLArg("[search]order", "date").
			URLArg("[search]hide_empty", "no").
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
	url := "/tag_aliases.json"

	var aliases types.TAliasInfoArray

	r, e := api.New(url).
			BasicAuthentication(user, apitoken).
			URLArg("limit", "10000").
			URLArg("page", strconv.Itoa(page)).
			URLArg("[search]order", "date").
			URLArg("[search]status", "approved").
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
	url := "/posts.json"

	var posts types.TResultArray

	req := api.New(url).
			BasicAuthentication(user, apitoken).
			URLArg("limit", "10000").
			Into(&posts)
	if before > 0 { req.URLArg("page", fmt.Sprintf("b%d", before)) }
	r, e := req.Do()

	log.Printf("[api     ] API call: %s [as %s] (%s)\n", url, user, r.Status)

	if e != nil {
		return posts, true, before, e
	}

	if len(posts) != 0 { before = posts[len(posts) - 1].Id }
	return posts, len(posts) != 0, before, nil
}


func FetchOnePost(user, apitoken string, id int) (*types.TSearchResult, error) {
	url := fmt.Sprintf("/posts/%d.json", id)

	var post types.TSearchResult

	r, e := api.New(url).
			BasicAuthentication(user, apitoken).
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
	url := "/posts.json"

	var posts types.TResultArray

	r, e := api.New(url).
			BasicAuthentication(user, apitoken).
			URLArg("limit", "10000").
			URLArg("page", strconv.Itoa(page)).
			URLArg("tags", "status:deleted").
			Into(&posts).
			Do()

	log.Printf("[api     ] API call: %s [as %s] (%s)\n", url, user, r.Status)

	if e != nil {
		return posts, true, page, e
	}

	return posts, len(posts) != 0, page + 1, nil
}

func GetTagData(user, apitoken string, id int) (*types.TTagData, error) {
	url := fmt.Sprintf("/tags/%d.json", id)

	var tag types.TTagData

	r, e := api.New(url).
			BasicAuthentication(user, apitoken).
			Into(&tag).
			Do()

	log.Printf("[api     ] API call: %s [as %s] (%s)\n", url, user, r.Status)

	if e != nil {
		return nil, e
	}

	return &tag, nil
}
