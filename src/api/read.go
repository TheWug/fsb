package api

import (
	"api/types"
	"strconv"
	"errors"
	"strings"
	"fmt"
)

type FailedCall struct {
	Success bool `json:"success"`
}

func TagSearch(user, apitoken string, tags string, page int, limit int) (types.TPostInfoArray, error) {
	temp := struct {
		Posts types.TPostInfoArray `json:"posts"`
	}{}

	url := "/posts.json"
	r, e := api.New(url).
			BasicAuthentication(user, apitoken).
			URLArg("tags", tags).
			URLArg("page", strconv.Itoa(page)).
			URLArg("limit", strconv.Itoa(limit)).
			Into(&temp).
			Do()

	APILog(url, user, len(temp.Posts), r, e)

	return temp.Posts, e
}

func TestLogin(user, apitoken string) (bool, error) {
	u, err := FetchUser(user, apitoken)
	if err != nil { return false, err }
	// email is only populated if we are logged into the account we are querying.
	return (u != nil && u.Email != ""), nil
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

	APILog(url, user, len(hist), r, e)

	if e != nil {
		return nil, e
	}
	return hist, nil
}

func ListTags(user, apitoken string, options types.ListTagsOptions) (types.TTagInfoArray, error) {
	url := "/tags.json"

	var results types.TTagListing

	r, e := api.New(url).
			BasicAuthentication(user, apitoken).
			URLArgDefault("page", options.Page, "").
			URLArgDefault("limit", options.Limit, 0).
			URLArgDefault("search[name_matches]", options.MatchTags, "").
			URLArgDefault("search[order]", options.Order.String(), "").
			URLArg("search[category]", (*int)(options.Category)).
			URLArg("search[hide_empty]", options.HideEmpty).
			URLArg("search[has_wiki]", options.HasWiki).
			URLArg("search[has_artist]", options.HasArtist).
			Into(&results).
			Do()

	APILog(url, user, len(results.Tags), r, e)

	if e != nil {
		return nil, e
	}

	return results.Tags, nil
}

func ListTagAliases(user, apitoken string, options types.ListTagAliasOptions) (types.TAliasInfoArray, error) {
	url := "/tag_aliases.json"

	var results types.TAliasListing

	r, e := api.New(url).
			BasicAuthentication(user, apitoken).
			URLArgDefault("page", options.Page, "").
			URLArgDefault("limit", options.Limit, 0).
			URLArgDefault("search[name_matches]", options.MatchAliases, "").
			URLArgDefault("search[status]", options.Status, "").
			URLArgDefault("search[order]", options.Order, "").
			Into(&results).
			Do()

	APILog(url, user, len(results.Aliases), r, e)

	if e != nil {
		return nil, e
	}

	return results.Aliases, nil
}

func ListPosts(user, apitoken string, options types.ListPostOptions) (types.TPostInfoArray, error) {
	url := "/posts.json"

	var results types.TPostListing

	r, e := api.New(url).
			BasicAuthentication(user, apitoken).
			URLArgDefault("tags", options.SearchQuery, "").
			URLArgDefault("limit", options.Limit, "0").
			URLArgDefault("page", options.Page, "").
			Into(&results).
			Do()

	APILog(url, user, len(results.Posts), r, e)

	if e != nil {
		return nil, e
	}

	return results.Posts, nil
}


func FetchOnePost(user, apitoken string, id int) (*types.TPostInfo, error) {
	url := fmt.Sprintf("/posts/%d.json", id)

	var post struct {
		Post types.TPostInfo `json:"post"`
	}

	r, e := api.New(url).
			BasicAuthentication(user, apitoken).
			Into(&post).
			Do()

	APILog(url, user, -1, r, e)

	if e != nil {
		return nil, e
	}

	if post.Post.Id != 0 { return &post.Post, nil }
	return nil, nil
}

func ListOnePageOfDeletedPosts(user, apitoken string, page int) (types.TPostInfoArray, bool, int, error) {
	posts, err := TagSearch(user, apitoken, "status:deleted", page, 10000)

	if err != nil {
		return posts, true, page, err
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

	APILog(url, user, -1, r, e)

	if e != nil {
		return nil, e
	}

	return &tag, nil
}

func FetchUser(username, api_key string) (*types.TUserInfo, error) {
	url := "/users.json"

	var user types.TUserInfoArray
	var status types.TApiStatus = types.TApiStatus{Success: true}

	req := api.New(url).
			Arg("search[name_matches]", username).
			Into(&user).
			Into(&status)

	if api_key != "" {
		req.BasicAuthentication(username, api_key)
	}

	r, e := req.Do()

	APILog(url, username, -1, r, e)

	if e != nil {
		return nil, e
	} else if !status.Success {
		return nil, nil
	} else if len(user) == 0 {
		return nil, nil 
	} else if len(user) > 1 {
		return nil, errors.New("Got wrong number of users?")
	} else if strings.ToLower(user[0].Name) != strings.ToLower(username) {
		return nil, errors.New("Got non-matching user?")
	}

	return &user[0], e
}
