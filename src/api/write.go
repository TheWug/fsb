package api

import (
	"api/types"
	"strconv"
	"errors"
	"log"
	"github.com/thewug/reqtify"
	"io"
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
	url := "/post/create.json"

	out := UploadCallResult{}

	req := api.New(url).
			Method(reqtify.POST).
			FormArg("login", user).
			FormArg("password_hash", apitoken).

			FormArg("post[tags]", tags).
			FormArg("post[source]", source).
			FormArg("post[description]", description).
			FormArg("post[tags]", tags).
			FormArg("post[rating]", rating).
			Into(&out).
			Multipart()
	if parent != nil { req.FormArg("post[parent_id]", strconv.Itoa(*parent)) }

	if upload_url == "" && file_data != nil {
		req.FileArg("post[file]", "postfile", file_data)
	} else if upload_url != "" && file_data == nil {
		req.FormArg("post[upload_url]", upload_url)
	} else { return nil, MissingArguments }

	r, e := req.Do()
	out.Status = r.Status
	out.StatusCode = r.StatusCode
	log.Printf("[api     ] API call: %s [as %s] (%s)\n", url, user, r.Status)

	if e != nil {
		return &out, e
	}

	return &out, e
}


func UpdatePost(user, apitoken string,
		id int,
		oldtags, newtags *string,			// nil to leave tags unchanged.
		rating *string,					// nil to leave rating unchanged.
		parent *int,					// nil to leave parent unchanged, -1 to UNSET parent
		source *string,					// nil to leave source unchanged
		description *string,				// nil to leave description unchanged
		reason *string) (*types.TSearchResult, error) {
	url := "/post/update.json"

	var post types.TSearchResult

	req := api.New(url).
			Method(reqtify.POST).
			FormArg("login", user).
			FormArg("password_hash", apitoken).
			FormArg("id", strconv.Itoa(id)).
			Into(&post)
	if oldtags != nil { req.FormArg("post[old_tags]", *oldtags) }
	if newtags != nil { req.FormArg("post[tags]", *newtags) }
	if rating != nil { req.FormArg("post[rating]", *rating) }
	if parent != nil && *parent == -1 { req.FormArg("post[parent_id]", "") }
	if parent != nil && *parent != -1 { req.FormArg("post[parent_id]", strconv.Itoa(*parent)) }
	if source != nil { req.FormArg("post[source]", *source) }
	if description != nil { req.FormArg("post[description]", *description) }
	if reason != nil { req.FormArg("reason", *reason) }
	r, e := req.Do()

	log.Printf("[api     ] API call: %s [as %s] (%s)\n", url, user, r.Status)

	if e != nil {
		return nil, e
	}

	return &post, e
}

// this is a little trick given to me by kiranoot. sometimes the post count for a tag will get fudged up, and it can be fixed by
// searching for that single tag and trying to view a page of results past the end. there is a limit of 750 pages of results though
// and pagination doesn't work automatically when enumerating results using the before_id mechanism, so this can only work for tags
// with less than 320 * 750 = 240000 results (which is all but the 40 or so most popular tags).
// after calling it, the count will be reset to the number of non-deleted posts with the tag.
func FixPostcountForTag(user, apitoken, tag string) (error) {
	url := "/post/index.json"

	var posts types.TResultArray

	r, e := api.New(url).
			URLArg("login", user).
			URLArg("password_hash", apitoken).
			URLArg("limit", "320").
			URLArg("page", "750").
			URLArg("tags", tag).
			Into(&posts).
			Do()

	log.Printf("[api     ] API call: %s [as %s] (%s)\n", url, user, r.Status)
	return e
}
