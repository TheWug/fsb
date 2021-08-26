package api

import (
	"net/http"
	"strconv"
	"bytes"
	"errors"
	"io/ioutil"
	"mime/multipart"
	"encoding/json"
	"log"
)

var MissingArguments error = errors.New("Missing file or upload_url")

type UploadCallResult struct {
	Success    bool   `json:"success"`
	Reason    *string `json:"reason"`
	Location  *string `json:"location"`
	StatusCode int
	Status     string
}

func UploadFile(file_contents []byte, upload_url, tags, rating, source, description string, parent int, user, apikey string) (*UploadCallResult, error) {
	post_url := apiEndpoint + "post/create.json"

	var content_buffer bytes.Buffer

	//post_url += "&post[upload_url]=" + url.QueryEscape(upload_url)
	w := multipart.NewWriter(&content_buffer)
	/*
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="post[file]"; filename="file.png"`)
	h.Set("Content-Type", "image/png")
	fw, err := w.CreatePart(h)
	if err != nil {
		return "", err
	}

	if _, err := fw.Write(file_contents); err != nil {
		return "", err
	}
	*/

	if upload_url == "" && file_contents != nil {
		wl, err := w.CreateFormFile("post[file]", "postfile")
		if err != nil { return nil, err }
		_, err = wl.Write(file_contents)
		if err != nil { return nil, err }
	} else if upload_url != "" && file_contents == nil {
		w.WriteField("post[upload_url]", "")
	} else { return nil, MissingArguments }

	parent_str := ""
	if parent != 0 { parent_str = strconv.Itoa(parent) }

	w.WriteField("post[tags]", tags)
	w.WriteField("post[source]", source)
	w.WriteField("post[description]", description)
	w.WriteField("post[parent_id]", parent_str)
	w.WriteField("post[rating]", rating)

	w.WriteField("login", user)
	w.WriteField("password_hash", apikey)

	w.Close()

	b2 := make([]byte, content_buffer.Len())
	copy(b2, content_buffer.Bytes())

	// Now that you have a form, you can submit it to your handler.
	req, e := http.NewRequest("POST", post_url, &content_buffer)

	if e != nil {
		return nil, e
	}

	req.Header.Set("Content-Type", w.FormDataContentType())
	r, e := apiDo(req)
	log.Printf("[api     ] API call: %s (%s)\n", post_url, r.Status)

	if r != nil {
		defer r.Body.Close()
	}

	out := UploadCallResult{Status: r.Status, StatusCode: r.StatusCode}

	if e != nil {
		return &out, e
	}

	b, e := ioutil.ReadAll(r.Body)
	if e != nil {
		return &out, e
	}

	e = json.Unmarshal(b, &out)

	return &out, e
}
