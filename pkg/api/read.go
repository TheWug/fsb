package api

import (
	"github.com/thewug/fsb/pkg/api/types"

	"errors"
	"fmt"
	"strings"
)

type FailedCall struct {
	Success bool `json:"success"`
}

func TestLogin(user, apitoken string) (*types.TUserInfo, bool, error) {
	u, err := FetchUser(user, apitoken)
	if err != nil { return nil, false, err }
	// email is only populated if we are logged into the account we are querying.
	return u, (u != nil && u.Email != ""), nil
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
			JSONInto(&results).
			Do()

	APILog(url, user, len(results.Tags), r, e)

	if e != nil {
		return nil, e
	}

	if r.StatusCode != 200 {
		return nil, errors.New(r.Status)
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
			JSONInto(&results).
			Do()

	APILog(url, user, len(results.Aliases), r, e)

	if e != nil {
		return nil, e
	}

	if r.StatusCode != 200 {
		return nil, errors.New(r.Status)
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
			JSONInto(&results).
			Do()

	APILog(url, user, len(results.Posts), r, e)

	if e != nil {
		return nil, e
	}

	if r.StatusCode != 200 {
		return nil, errors.New(r.Status)
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
			JSONInto(&post).
			Do()

	APILog(url, user, -1, r, e)

	if e != nil {
		return nil, e
	}

	if r.StatusCode != 200 {
		return nil, errors.New(r.Status)
	}

	if post.Post.Id != 0 { return &post.Post, nil }
	return nil, nil
}

func GetTagData(user, apitoken string, id int) (*types.TTagData, error) {
	url := fmt.Sprintf("/tags/%d.json", id)

	var tag types.TTagData

	r, e := api.New(url).
			BasicAuthentication(user, apitoken).
			JSONInto(&tag).
			Do()

	APILog(url, user, -1, r, e)

	if e != nil {
		return nil, e
	}

	if r.StatusCode != 200 {
		return nil, errors.New(r.Status)
	}

	return &tag, nil
}

func FetchUser(username, api_key string) (*types.TUserInfo, error) {
	url := "/users.json"

	var user types.TUserInfoArray
	var status types.TApiStatus = types.TApiStatus{Success: true}

	req := api.New(url).
			Arg("search[name_matches]", username).
			JSONInto(&user).
			JSONInto(&status)

	if api_key != "" {
		req.BasicAuthentication(username, api_key)
	}

	r, e := req.Do()

	APILog(url, username, -1, r, e)

	if e != nil {
		return nil, e
	} else if r.StatusCode != 200 {
		return nil, errors.New(r.Status)
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
