package api

import (
	"api/types"
	"strconv"
	"errors"
	"io/ioutil"
	"github.com/thewug/reqtify"
	"io"
	"fmt"
)

var MissingArguments error = errors.New("Missing file or upload_url")

type UploadCallResult struct {
	Success    bool   `json:"success"`
	Reason    *string `json:"reason"`
	Location  *string `json:"location"`
	StatusCode int
	Status     string
}

func UploadFile(file_data io.Reader, upload_url, tags, rating, source, description string, parent *int, user, apitoken string) (*UploadCallResult, error) {
	url := "/uploads.json"

	out := UploadCallResult{}

	req := api.New(url).
			Method(reqtify.POST).
			BasicAuthentication(user, apitoken).
			FormArg("upload[source]", source).
			FormArg("upload[description]", description).
			FormArg("upload[tag_string]", tags).
			FormArg("upload[rating]", rating).
			JSONInto(&out).
			Multipart()
	if parent != nil { req.FormArg("upload[parent_id]", strconv.Itoa(*parent)) }

	if upload_url == "" && file_data != nil {
		req.FileArg("upload[file]", "post.file", file_data)
	} else if upload_url != "" && file_data == nil {
		req.FormArg("upload[direct_url]", upload_url)
	} else { return nil, MissingArguments }

	r, e := req.Do()
	APILog(url, user, -1, r, e)

	if r != nil {
		out.Status, out.StatusCode = r.Status, r.StatusCode
	}

	return &out, e
}

var PostIsDeleted error = errors.New("This post has been deleted.")

func UpdatePost(user, apitoken string,
		id int,
		tagdiff types.TagDiff,				// empty to leave tags unchanged.
		rating *string,					// nil to leave rating unchanged.
		parent *int,					// nil to leave parent unchanged, -1 to UNSET parent
		source *string,					// nil to leave source unchanged
		description *string,				// nil to leave description unchanged
		reason *string) (*types.TPostInfo, error) {
	url := fmt.Sprintf("/posts/%d.json", id)

	var post struct {
		types.TPostEditInfo
		types.TApiStatus
	}

	post.Success = true

	req := api.New(url).
			Method(reqtify.PATCH).
			BasicAuthentication(user, apitoken).
			JSONInto(&post)
	if !tagdiff.IsZero() { req.FormArgDefault("post[tag_string_diff]", tagdiff.APIString(), "") }
	if rating != nil { req.FormArg("post[rating]", *rating) }
	if parent != nil && *parent == -1 { req.FormArg("post[parent_id]", "") }
	if parent != nil && *parent != -1 { req.FormArg("post[parent_id]", strconv.Itoa(*parent)) }
	if source != nil { req.FormArg("post[source]", *source) }
	if description != nil { req.FormArg("post[description]", *description) }
	if reason != nil { req.FormArg("post[edit_reason]", *reason) }
	r, e := req.Do()

	APILog(url, user, -1, r, e)

	if e != nil {
		return nil, e
	}

	if post.Reason == "Access Denied: Post not visible to you" {
		return nil, PostIsDeleted
	}

	return post.TPostEditInfo.TPostInfo(), e
}

func VotePost(user, apitoken string,
              id int,
              vote types.PostVote,
              no_unvote bool) (*types.TPostScore, error) {
	url := fmt.Sprintf("/posts/%d/votes.json", id)

	var score types.TPostScore

	r, e := api.New(url).
			Method(reqtify.POST).
			BasicAuthentication(user, apitoken).
			FormArg("score", vote.Value()).
			FormArgDefault("no_unvote", no_unvote, false).
			JSONInto(&score).
			Do()

	APILog(url, user, -1, r, e)

	// this returns HTML, but 200, if you pick an ID which doesn't exist, so ??? i guess

	if e != nil {
		return nil, e
	}

	return &score, e
}

func UnvotePost(user, apitoken string,
		id int) (error) {
	url := fmt.Sprintf("/posts/%d/votes.json", id)

	r, e := api.New(url).
			Method(reqtify.DELETE).
			BasicAuthentication(user, apitoken).
			Do()

	APILog(url, user, -1, r, e)

	// this returns HTML, but 200, if you pick an ID which doesn't exist, so ??? i guess

	if e != nil {
		return e
	}

	bytes, e := ioutil.ReadAll(r.Body)
	r.Body.Close()
	if len(bytes) != 0 {
		return errors.New("Got a response when none was expected (nonexistent post id?)")
	}

	return nil
}

// you shouldn't depend on this to return anything useful, as it will return nil if you favorite the same post twice
func FavoritePost(user, apitoken string,
		id int) (*types.TPostInfo, error) {
	url := "/favorites.json"

	post := struct {
		Post    types.TPostInfo `json:"post"`
		Success bool            `json:"success"`
		Message string          `json:"message"`
	}{Success: true}

	r, e := api.New(url).
		Method(reqtify.POST).
		BasicAuthentication(user, apitoken).
		FormArg("post_id", id).
		JSONInto(&post).
		DebugPrint().
		Do()

	APILog(url, user, -1, r, e)

	// this means the post was already favorited, which the api treats as an error, but we want to treat it as OK
	if post.Success == false && post.Message == "You have already favorited this post" {
		return nil, nil
	} else if e != nil {
		return nil, e
	}

	return &post.Post, e
}

func UnfavoritePost(user, apitoken string,
		id int) (error) {
	// i know this isn't the same as the other one, i promise it's correct right now though
	url := fmt.Sprintf("/favorites/%d.json", id)

	r, e := api.New(url).
		Method(reqtify.DELETE).
		BasicAuthentication(user, apitoken).
		DebugPrint().
		Do()

	APILog(url, user, -1, r, e)

	// this returns HTML, but 200, if you pick an ID which doesn't exist, so ??? i guess

	if e != nil {
		return e
	}

	bytes, e := ioutil.ReadAll(r.Body)
	if len(bytes) != 0 {
		return errors.New("Got a response when none was expected (nonexistent post id?)")
	}

	return e
}

// this is a little trick given to me by kiranoot. sometimes the post count for a tag will get fudged up, and it can be fixed by
// searching for that single tag and trying to view a page of results past the end. there is a limit of 750 pages of results though
// and pagination doesn't work automatically when enumerating results using the before_id mechanism, so this can only work for tags
// with less than 320 * 750 = 240000 results (which is all but the 40 or so most popular tags).
// after calling it, the count will be reset to the number of non-deleted posts with the tag.

// with next-gen, do we even need to keep this?
func FixPostcountForTag(user, apitoken, tag string) (error) {
	_, e := TagSearch(user, apitoken, tag, 750, 320)
	return e
}
