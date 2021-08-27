package api

import (
	"github.com/thewug/reqtify"
	"github.com/thewug/reqtify/mock"
	"testing"
	"os"
	"flag"
	"fmt"
	"strings"
	"sync"
	"net/http"
	"net/url"
	"api/types"
	"reflect"
	"io/ioutil"
)

var apiMock *mock.ReqtifierMock

func TestMain(m *testing.M) {
	ApiName = "website name"
	Endpoint = "website"
	FilteredEndpoint = "filteredwebsite"
	StaticPrefix = "static."
	api = reqtify.New(fmt.Sprintf("https://%s", Endpoint), nil, nil, nil, "this is a test user agent")

	flag.Parse()

	apiMock = &mock.ReqtifierMock{
		FakeReqtifier: api.(*reqtify.ReqtifierImpl),
	}
	api = apiMock

	os.Exit(m.Run())
}

func CompareRequests(req1, req2 reqtify.RequestImpl) bool {
	b :=	req1.URLPath == req2.URLPath &&
		req1.Verb == req2.Verb &&
		req1.BasicUser == req2.BasicUser &&
		req1.BasicPassword == req2.BasicPassword &&
		(len(req1.QueryParams) == 0 && len(req2.QueryParams) == 0 || reflect.DeepEqual(req1.QueryParams, req2.QueryParams)) &&
		(len(req1.FormParams) == 0 && len(req2.FormParams) == 0 || reflect.DeepEqual(req1.FormParams, req2.FormParams)) &&
		(len(req1.AutoParams) == 0 && len(req2.AutoParams) == 0 || reflect.DeepEqual(req1.AutoParams, req2.AutoParams)) &&
		(len(req1.Headers) == 0 && len(req2.Headers) == 0 || reflect.DeepEqual(req1.Headers, req2.Headers)) &&
		(len(req1.Cookies) == 0 && len(req2.Cookies) == 0 || reflect.DeepEqual(req1.Cookies, req2.Cookies))

	if !b { return false }

	// okay, b is true, so now do the complicated comparisons. read the contents of the file readers and compare them, and
	// compare the types of the response handlers (since there's no way for the objects to be the same, as long as they are
	// the same type that's good enough

	var resp1, resp2 []reflect.Type
	for _, x1 := range req1.Response {
		resp1 = append(resp1, reflect.TypeOf(x1))
	}
	for _, x2 := range req2.Response {
		resp2 = append(resp2, reflect.TypeOf(x2))
	}
	if !reflect.DeepEqual(resp1, resp2) { return false }

	var file1, file2 []struct{
		key, name string
		data []byte
	}
	for k, v := range req1.FormFiles {
		for _, x1 := range v {
			file1 = append(file1, struct{key, name string; data []byte}{key: k, name: x1.Name, data: func(b[]byte, e error) []byte { return b }(ioutil.ReadAll(x1.Data))})
		}
	}
	for k, v := range req2.FormFiles {
		for _, x2 := range v {
			file2 = append(file2, struct{key, name string; data []byte}{key: k, name: x2.Name, data: func(b[]byte, e error) []byte { return b }(ioutil.ReadAll(x2.Data))})
		}
	}
	if !reflect.DeepEqual(file1, file2) { return false}
	return true
}

func TestTagSearch(t *testing.T) {
	examiner := apiMock.Examine()
	var wg sync.WaitGroup

	var tuples = []struct{
		user, apikey string
		tags string
		page, limit int
		response http.Response
		err error
		expectedRequest reqtify.RequestImpl
		expectedOutput types.TPostInfoArray
		expectedError error
	}{
		{"testuser", "testpassword", "+tag -othertag", 1, 100,
			http.Response{Status: "200 Testing", StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader(`{"posts":[]}`))},
			nil,
			reqtify.RequestImpl{URLPath: "/posts.json", Verb: reqtify.GET,
				QueryParams: url.Values{"tags":[]string{"+tag -othertag"}, "page":[]string{"1"}, "limit":[]string{"100"}},
				BasicUser: "testuser", BasicPassword: "testpassword",
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil)}},
			nil,
			nil},
		{"testuser2", "testpassword2", "+tag -othertag anotherthing", 2, 200,
			http.Response{Status: "200 Testing Different", StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader(`{"posts":[], "extra crap":""}`))},
			nil,
			reqtify.RequestImpl{URLPath: "/posts.json", Verb: reqtify.GET,
				QueryParams: url.Values{"tags":[]string{"+tag -othertag anotherthing"}, "page":[]string{"2"}, "limit":[]string{"200"}},
				BasicUser: "testuser2", BasicPassword: "testpassword2",
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil)}},
			nil,
			nil},
	}

	for _, x := range tuples {
		var posts types.TPostInfoArray
		var err error

		wg.Add(1)
		go func() {
			posts, err = TagSearch(x.user, x.apikey, x.tags, x.page, x.limit)
			wg.Done()
		}()

		req := <- examiner.Requests
		if !CompareRequests(req.RequestImpl, x.expectedRequest) {
			t.Errorf("Discrepancy in request!\nActual: %+v\nExpected: %+v\n", req.RequestImpl, x.expectedRequest)
		}
		examiner.Responses <- mock.ResponseAndError{
			Response: &x.response,
			Error: x.err,
		}

		wg.Wait()
		if !(len(posts) == 0 && len(x.expectedOutput) == 0 || reflect.DeepEqual(posts, x.expectedOutput)) {
			t.Errorf("Discrepancy in posts!\nActual: %+v\nExpected: %+v\n", posts, x.expectedOutput)
		}
		if !reflect.DeepEqual(err, x.expectedError) {
			t.Errorf("Discrepancy in error!\nActual: %+v\nExpected: %+v\n", err, x.expectedError)
		}
	}
}
