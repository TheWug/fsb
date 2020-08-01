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
	"errors"
)

var apiMock *mock.ReqtifierMock

var samplePost types.TPostInfo = types.TPostInfo{
	TPostScore: types.TPostScore{Upvotes:158, Downvotes:-3, Score:161, OurScore:0},
	TPostFile: types.TPostFile{Width:635, Height:860, File_ext:"jpg", File_size:414628, File_url:"https://api.static.endpoint/data/27/f6/27f64791b434b92622f97eafbb75d321.jpg", Md5:"27f64791b434b92622f97eafbb75d321"},
	TPostPreview: types.TPostPreview{Preview_width:110, Preview_height:150, Preview_url:"https://api.static.endpoint/data/preview/27/f6/27f64791b434b92622f97eafbb75d321.jpg"},
	TPostSample: types.TPostSample{Sample_width:635, Sample_height:860, Sample_url:"https://api.static.endpoint/data/27/f6/27f64791b434b92622f97eafbb75d321.jpg", Has_sample:false},
	TPostFlags: types.TPostFlags{Pending:false, Flagged:false, Locked_notes:false, Locked_status:false, Locked_rating:false, Deleted:false},
	TPostRelationships: types.TPostRelationships{Parent_id:0, Has_children:true, Has_active_children:false, Children:[]int{2207557, 2234052}},
	TPostTags: types.TPostTags{
		General: []string{"alley","amazing_background","bicycle","building","dappled_light","day","detailed_background","female","hair","house","light","memory","outside","scenery","shadow","sky","solo","standing","street","sunlight","tree","wood","young"},
		Species: []string{"animal_humanoid","cat_humanoid","felid","felid_humanoid","feline","feline_humanoid","humanoid","mammal","mammal_humanoid"},
		Character: []string{},
		Copyright: []string{"by-nc-nd","creative_commons"},
		Artist: []string{"tysontan"},
		Meta: []string{"signature"},
		Invalid: []string{},
		Lore: []string{},
	}, Id:12345, Description:"", Creator_id:46, Change:28668882, Fav_count:196, Rating:"s", Comment_count:21, Sources: []string{"https://tysontan.deviantart.com/art/Alley-in-the-Memory-66157556"},
}

var samplePostJson string = `{"id":12345,"created_at":"2007-10-07T17:25:15.019-04:00","updated_at":"2020-07-29T00:57:30.518-04:00","file":{"width":635,"height":860,"ext":"jpg","size":414628,"md5":"27f64791b434b92622f97eafbb75d321","url":"https://api.static.endpoint/data/27/f6/27f64791b434b92622f97eafbb75d321.jpg"},"preview":{"width":110,"height":150,"url":"https://api.static.endpoint/data/preview/27/f6/27f64791b434b92622f97eafbb75d321.jpg"},"sample":{"has":false,"height":860,"width":635,"url":"https://api.static.endpoint/data/27/f6/27f64791b434b92622f97eafbb75d321.jpg"},"score":{"up":158,"down":-3,"total":161},"tags":{"general":["alley","amazing_background","bicycle","building","dappled_light","day","detailed_background","female","hair","house","light","memory","outside","scenery","shadow","sky","solo","standing","street","sunlight","tree","wood","young"],"species":["animal_humanoid","cat_humanoid","felid","felid_humanoid","feline","feline_humanoid","humanoid","mammal","mammal_humanoid"],"character":[],"copyright":["by-nc-nd","creative_commons"],"artist":["tysontan"],"invalid":[],"lore":[],"meta":["signature"]},"locked_tags":[],"change_seq":28668882,"flags":{"pending":false,"flagged":false,"note_locked":false,"status_locked":false,"rating_locked":false,"deleted":false},"rating":"s","fav_count":196,"sources":["https://tysontan.deviantart.com/art/Alley-in-the-Memory-66157556"],"pools":[],"relationships":{"parent_id":null,"has_children":true,"has_active_children":false,"children":[2207557,2234052]},"approver_id":null,"uploader_id":46,"description":"","comment_count":21,"is_favorited":false,"has_notes":false}`

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
			http.Response{Status: "200 Testing Different", StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader(`{"posts":[` + samplePostJson + `]}`))},
			nil,
			reqtify.RequestImpl{URLPath: "/posts.json", Verb: reqtify.GET,
				QueryParams: url.Values{"tags":[]string{"+tag -othertag anotherthing"}, "page":[]string{"2"}, "limit":[]string{"200"}},
				BasicUser: "testuser2", BasicPassword: "testpassword2",
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil)}},
			types.TPostInfoArray{samplePost},
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

func TestTestLogin(t *testing.T) {
	examiner := apiMock.Examine()
	var wg sync.WaitGroup

	var tuples = []struct{
		user, apikey string
		response http.Response
		err error
		expectedRequest reqtify.RequestImpl
		expectedOutput bool
		expectedError error
	}{
		{"Snergal", "testpassword",
			http.Response{Status: "200 Testing", StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader(`[{"id":292290,"name":"Snergal","email":"this is an email"}]`))},
			nil,
			reqtify.RequestImpl{URLPath: "/users.json", Verb: reqtify.GET,
				AutoParams: url.Values{"search[name_matches]":[]string{"Snergal"}},
				BasicUser: "Snergal", BasicPassword: "testpassword",
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil), reqtify.FromJSON(nil)}},
			true,
			nil},
		{"Snergal", "testpassword",
			http.Response{Status: "200 Testing", StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader(`[{"id":292290,"name":"Snergal","email":"this is an email"}]`))},
			errors.New("endpoint failed"),
			reqtify.RequestImpl{URLPath: "/users.json", Verb: reqtify.GET,
				AutoParams: url.Values{"search[name_matches]":[]string{"Snergal"}},
				BasicUser: "Snergal", BasicPassword: "testpassword",
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil), reqtify.FromJSON(nil)}},
			false,
			errors.New("endpoint failed")},
		{"Snergal", "testpassword",
			http.Response{Status: "200 Testing", StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader(`[]`))},
			nil,
			reqtify.RequestImpl{URLPath: "/users.json", Verb: reqtify.GET,
				AutoParams: url.Values{"search[name_matches]":[]string{"Snergal"}},
				BasicUser: "Snergal", BasicPassword: "testpassword",
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil), reqtify.FromJSON(nil)}},
			false,
			nil},
		{"Snergal", "testpassword",
			http.Response{Status: "200 Testing", StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader(`[{"id":292290,"name":"Snergal"}]`))},
			nil,
			reqtify.RequestImpl{URLPath: "/users.json", Verb: reqtify.GET,
				AutoParams: url.Values{"search[name_matches]":[]string{"Snergal"}},
				BasicUser: "Snergal", BasicPassword: "testpassword",
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil), reqtify.FromJSON(nil)}},
			false,
			nil},
		{"Snergal", "testpassword",
			http.Response{Status: "401 Testing", StatusCode: 401, Body: ioutil.NopCloser(strings.NewReader(`[{"id":292290,"name":"Snerg","email":"this is an email"}]`))},
			nil,
			reqtify.RequestImpl{URLPath: "/users.json", Verb: reqtify.GET,
				AutoParams: url.Values{"search[name_matches]":[]string{"Snergal"}},
				BasicUser: "Snergal", BasicPassword: "testpassword",
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil), reqtify.FromJSON(nil)}},
			false,
			errors.New("Got non-matching user?")},
		{"Snergal", "testpassword",
			http.Response{Status: "401 Testing", StatusCode: 401, Body: ioutil.NopCloser(strings.NewReader(`[{"id":292290,"name":"Snergal","email":"this is an email"}, {"id":292291,"name":"Snergle"}]`))},
			nil,
			reqtify.RequestImpl{URLPath: "/users.json", Verb: reqtify.GET,
				AutoParams: url.Values{"search[name_matches]":[]string{"Snergal"}},
				BasicUser: "Snergal", BasicPassword: "testpassword",
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil), reqtify.FromJSON(nil)}},
			false,
			errors.New("Got wrong number of users?")},
		{"Snergal", "testpassword",
			http.Response{Status: "401 Testing", StatusCode: 401, Body: ioutil.NopCloser(strings.NewReader(`{"success": false,"message": "SessionLoader::AuthenticationFailure","code": null}`))},
			nil,
			reqtify.RequestImpl{URLPath: "/users.json", Verb: reqtify.GET,
				AutoParams: url.Values{"search[name_matches]":[]string{"Snergal"}},
				BasicUser: "Snergal", BasicPassword: "testpassword",
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil), reqtify.FromJSON(nil)}},
			false,
			nil},
	}

	for _, x := range tuples {
		var loggedIn bool
		var err error

		wg.Add(1)
		go func() {
			loggedIn, err = TestLogin(x.user, x.apikey)
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
		if loggedIn != x.expectedOutput {
			t.Errorf("Discrepancy in loggedIn!\nActual: %+v\nExpected: %+v\n", loggedIn, x.expectedOutput)
		}
		if !reflect.DeepEqual(err, x.expectedError) {
			t.Errorf("Discrepancy in error!\nActual: %+v\nExpected: %+v\n", err, x.expectedError)
		}
	}
}