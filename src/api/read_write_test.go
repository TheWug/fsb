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

func TestListPosts(t *testing.T) {
	examiner := apiMock.Examine()
	var wg sync.WaitGroup

	var tuples = []struct{
		user, apikey string
		options types.ListPostOptions
		response http.Response
		err error
		expectedRequest reqtify.RequestImpl
		expectedOutput types.TPostInfoArray
		expectedError error
	}{
		{"testuser", "testpassword", types.ListPostOptions{SearchQuery: "tag -othertag", Page: types.Page(10), Limit: 100},
			http.Response{Status: "200 Testing", StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader(`{"posts":[]}`))},
			nil,
			reqtify.RequestImpl{URLPath: "/posts.json", Verb: reqtify.GET,
				QueryParams: url.Values{"tags":[]string{"tag -othertag"}, "page":[]string{"10"}, "limit":[]string{"100"}},
				BasicUser: "testuser", BasicPassword: "testpassword",
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil)}},
			nil,
			nil},
		{"testuser2", "testpassword2", types.ListPostOptions{SearchQuery: "tag -othertag anotherthing", Page: types.After(10), Limit: 50},
			http.Response{Status: "200 Testing Different", StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader(`{"posts":[` + samplePostJson + `]}`))},
			nil,
			reqtify.RequestImpl{URLPath: "/posts.json", Verb: reqtify.GET,
				QueryParams: url.Values{"tags":[]string{"tag -othertag anotherthing"}, "page":[]string{"a10"}, "limit":[]string{"50"}},
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
			posts, err = ListPosts(x.user, x.apikey, x.options)
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

func TestListTags(t *testing.T) {
	examiner := apiMock.Examine()
	var wg sync.WaitGroup
	species := types.TCSpecies
	f := false

	var tuples = []struct{
		user, apikey string
		options types.ListTagsOptions
		response http.Response
		err error
		expectedRequest reqtify.RequestImpl
		expectedOutput types.TTagInfoArray
		expectedError error
	}{
		{"testuser", "testpassword", types.ListTagsOptions{Page: types.Page(5), Limit: 100, MatchTags: "*_(artwork)", Category: &species, Order: types.TSOCount, HideEmpty: true, HasWiki: &f, HasArtist: &f},
			http.Response{Status: "200 Testing", StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader(`{"tags":[]}`))},
			nil,
			reqtify.RequestImpl{URLPath: "/tags.json", Verb: reqtify.GET,
				QueryParams: url.Values{"page":[]string{"5"}, "limit":[]string{"100"}, "search[name_matches]":[]string{"*_(artwork)"}, "search[order]":[]string{string(types.TSOCount)}, "search[category]":[]string{"5"}, "search[hide_empty]":[]string{"true"}, "search[has_wiki]":[]string{"false"}, "search[has_artist]":[]string{"false"}},
				BasicUser: "testuser", BasicPassword: "testpassword",
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil)}},
			types.TTagInfoArray{},
			nil},
		{"testuser2", "testpassword2", types.ListTagsOptions{Page: types.Page(3)},
			http.Response{Status: "200 Testing", StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader(`[{"id":123,"name":"kel","post_count":121}]`))},
			nil,
			reqtify.RequestImpl{URLPath: "/tags.json", Verb: reqtify.GET,
				QueryParams: url.Values{"page":[]string{"3"}, "search[hide_empty]":[]string{"false"}},
				BasicUser: "testuser2", BasicPassword: "testpassword2",
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil)}},
			types.TTagInfoArray{types.TTagData{Id: 123, Name: "kel", Count: 121}},
			nil},
	}

	for _, x := range tuples {
		var taginfo types.TTagInfoArray
		var err error

		wg.Add(1)
		go func() {
			taginfo, err = ListTags(x.user, x.apikey, x.options)
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
		if !(len(taginfo) == 0 && len(x.expectedOutput) == 0 || reflect.DeepEqual(taginfo, x.expectedOutput)) {
			t.Errorf("Discrepancy in tags!\nActual: %+v\nExpected: %+v\n", taginfo, x.expectedOutput)
		}
		if !reflect.DeepEqual(err, x.expectedError) {
			t.Errorf("Discrepancy in error!\nActual: %+v\nExpected: %+v\n", err, x.expectedError)
		}
	}
}

func TestListTagAliases(t *testing.T) {
	examiner := apiMock.Examine()
	var wg sync.WaitGroup

	var tuples = []struct{
		user, apikey string
		options types.ListTagAliasOptions
		response http.Response
		err error
		expectedRequest reqtify.RequestImpl
		expectedOutput types.TAliasInfoArray
		expectedError error
	}{
		{"testuser", "testpassword", types.ListTagAliasOptions{Page: types.Before(100), Limit: 75, MatchAliases: "abc*", Status: types.ASActive, Order: types.ASOUpdated},
			http.Response{Status: "200 Testing", StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader(`{"posts":[]}`))},
			nil,
			reqtify.RequestImpl{URLPath: "/tag_aliases.json", Verb: reqtify.GET,
				QueryParams: url.Values{"page":[]string{"b100"}, "limit":[]string{"75"}, "search[name_matches]":[]string{"abc*"}, "search[status]":[]string{string(types.ASActive)}, "search[order]":[]string{string(types.ASOUpdated)}},
				BasicUser: "testuser", BasicPassword: "testpassword",
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil)}},
			nil,
			nil},
		{"testuser2", "testpassword2", types.ListTagAliasOptions{Page: types.After(100), Limit: 99, MatchAliases: "def*", Status: types.ASRetired, Order: types.ASOName},
			http.Response{Status: "200 Testing", StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader(`[{"id":46006,"antecedent_name":"champion_(pokemon)","consequent_name":"pokémon_champion","post_count":0}]`))},
			nil,
			reqtify.RequestImpl{URLPath: "/tag_aliases.json", Verb: reqtify.GET,
				QueryParams: url.Values{"page":[]string{"a100"}, "limit":[]string{"99"}, "search[name_matches]":[]string{"def*"}, "search[status]":[]string{string(types.ASRetired)}, "search[order]":[]string{string(types.ASOName)}},
				BasicUser: "testuser2", BasicPassword: "testpassword2",
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil)}},
			types.TAliasInfoArray{types.TAliasData{Id: 46006, Name: "pokémon_champion", Alias: "champion_(pokemon)"}},
			nil},
	}

	for _, x := range tuples {
		var aliases types.TAliasInfoArray
		var err error

		wg.Add(1)
		go func() {
			aliases, err = ListTagAliases(x.user, x.apikey, x.options)
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
		if !(len(aliases) == 0 && len(x.expectedOutput) == 0 || reflect.DeepEqual(aliases, x.expectedOutput)) {
			t.Errorf("Discrepancy in aliases!\nActual: %+v\nExpected: %+v\n", aliases, x.expectedOutput)
		}
		if !reflect.DeepEqual(err, x.expectedError) {
			t.Errorf("Discrepancy in error!\nActual: %+v\nExpected: %+v\n", err, x.expectedError)
		}
	}
}

func TestFetchOnePost(t *testing.T) {
	examiner := apiMock.Examine()
	var wg sync.WaitGroup

	var tuples = []struct{
		user, apikey string
		id int
		response http.Response
		err error
		expectedRequest reqtify.RequestImpl
		expectedOutput *types.TPostInfo
		expectedError error
	}{
		{"testuser", "testpassword", 123,
			http.Response{Status: "200 Testing", StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader(`{"post":` + samplePostJson + `}`))},
			nil,
			reqtify.RequestImpl{URLPath: "/posts/123.json", Verb: reqtify.GET,
				BasicUser: "testuser", BasicPassword: "testpassword",
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil)}},
			&samplePost,
			nil},
		{"testuser", "testpassword", 456,
			http.Response{Status: "404 Testing", StatusCode: 404, Body: ioutil.NopCloser(strings.NewReader(`{"success":false,"reason":"not found"}`))},
			nil,
			reqtify.RequestImpl{URLPath: "/posts/456.json", Verb: reqtify.GET,
				BasicUser: "testuser", BasicPassword: "testpassword",
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil)}},
			nil,
			nil},
	}

	for _, x := range tuples {
		var post *types.TPostInfo
		var err error

		wg.Add(1)
		go func() {
			post, err = FetchOnePost(x.user, x.apikey, x.id)
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
		if !reflect.DeepEqual(post, x.expectedOutput) {
			t.Errorf("Discrepancy in post!\nActual: %+v\nExpected: %+v\n", post, x.expectedOutput)
		}
		if !reflect.DeepEqual(err, x.expectedError) {
			t.Errorf("Discrepancy in error!\nActual: %+v\nExpected: %+v\n", err, x.expectedError)
		}
	}
}

func TestGetTagData(t *testing.T) {
	examiner := apiMock.Examine()
	var wg sync.WaitGroup

	var tuples = []struct{
		user, apikey string
		id int
		response http.Response
		err error
		expectedRequest reqtify.RequestImpl
		expectedOutput *types.TTagData
		expectedError error
	}{
		{"testuser", "testpassword", 123,
			http.Response{Status: "200 Testing", StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader(`{"id":123,"name":"kel","post_count":121}`))},
			nil,
			reqtify.RequestImpl{URLPath: "/tags/123.json", Verb: reqtify.GET,
				BasicUser: "testuser", BasicPassword: "testpassword",
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil)}},
			&types.TTagData{Id: 123, Name: "kel", Count: 121},
			nil},
	}

	for _, x := range tuples {
		var tag *types.TTagData
		var err error

		wg.Add(1)
		go func() {
			tag, err = GetTagData(x.user, x.apikey, x.id)
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
		if !reflect.DeepEqual(tag, x.expectedOutput) {
			t.Errorf("Discrepancy in tag!\nActual: %+v\nExpected: %+v\n", tag, x.expectedOutput)
		}
		if !reflect.DeepEqual(err, x.expectedError) {
			t.Errorf("Discrepancy in error!\nActual: %+v\nExpected: %+v\n", err, x.expectedError)
		}
	}
}

func TestFetchUser(t *testing.T) {
	examiner := apiMock.Examine()
	var wg sync.WaitGroup

	var tuples = []struct{
		user, apikey string
		response http.Response
		err error
		expectedRequest reqtify.RequestImpl
		expectedOutput *types.TUserInfo
		expectedError error
	}{
		{"Snergal", "testpassword",
			http.Response{Status: "200 Testing", StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader(`[{"id":292290,"name":"Snergal","email":"this is an email"}]`))},
			nil,
			reqtify.RequestImpl{URLPath: "/users.json", Verb: reqtify.GET,
				AutoParams: url.Values{"search[name_matches]":[]string{"Snergal"}},
				BasicUser: "Snergal", BasicPassword: "testpassword",
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil), reqtify.FromJSON(nil)}},
			&types.TUserInfo{Id: 292290, Name: "Snergal", Email: "this is an email"},
			nil},
		{"Snergal", "testpassword",
			http.Response{Status: "200 Testing", StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader(`[{"id":292290,"name":"Snergal","email":"this is an email"}]`))},
			errors.New("endpoint failed"),
			reqtify.RequestImpl{URLPath: "/users.json", Verb: reqtify.GET,
				AutoParams: url.Values{"search[name_matches]":[]string{"Snergal"}},
				BasicUser: "Snergal", BasicPassword: "testpassword",
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil), reqtify.FromJSON(nil)}},
			nil,
			errors.New("endpoint failed")},
		{"Snergal", "testpassword",
			http.Response{Status: "200 Testing", StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader(`[]`))},
			nil,
			reqtify.RequestImpl{URLPath: "/users.json", Verb: reqtify.GET,
				AutoParams: url.Values{"search[name_matches]":[]string{"Snergal"}},
				BasicUser: "Snergal", BasicPassword: "testpassword",
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil), reqtify.FromJSON(nil)}},
			nil,
			nil},
		{"Snergal", "testpassword",
			http.Response{Status: "200 Testing", StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader(`[{"id":292290,"name":"Snergal"}]`))},
			nil,
			reqtify.RequestImpl{URLPath: "/users.json", Verb: reqtify.GET,
				AutoParams: url.Values{"search[name_matches]":[]string{"Snergal"}},
				BasicUser: "Snergal", BasicPassword: "testpassword",
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil), reqtify.FromJSON(nil)}},
			&types.TUserInfo{Id: 292290, Name: "Snergal"},
			nil},
		{"Snergal", "testpassword",
			http.Response{Status: "401 Testing", StatusCode: 401, Body: ioutil.NopCloser(strings.NewReader(`[{"id":292290,"name":"Snerg","email":"this is an email"}]`))},
			nil,
			reqtify.RequestImpl{URLPath: "/users.json", Verb: reqtify.GET,
				AutoParams: url.Values{"search[name_matches]":[]string{"Snergal"}},
				BasicUser: "Snergal", BasicPassword: "testpassword",
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil), reqtify.FromJSON(nil)}},
			nil,
			errors.New("Got non-matching user?")},
		{"Snergal", "testpassword",
			http.Response{Status: "401 Testing", StatusCode: 401, Body: ioutil.NopCloser(strings.NewReader(`[{"id":292290,"name":"Snergal","email":"this is an email"}, {"id":292291,"name":"Snergle"}]`))},
			nil,
			reqtify.RequestImpl{URLPath: "/users.json", Verb: reqtify.GET,
				AutoParams: url.Values{"search[name_matches]":[]string{"Snergal"}},
				BasicUser: "Snergal", BasicPassword: "testpassword",
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil), reqtify.FromJSON(nil)}},
			nil,
			errors.New("Got wrong number of users?")},
		{"Snergal", "testpassword",
			http.Response{Status: "401 Testing", StatusCode: 401, Body: ioutil.NopCloser(strings.NewReader(`{"success": false,"message": "SessionLoader::AuthenticationFailure","code": null}`))},
			nil,
			reqtify.RequestImpl{URLPath: "/users.json", Verb: reqtify.GET,
				AutoParams: url.Values{"search[name_matches]":[]string{"Snergal"}},
				BasicUser: "Snergal", BasicPassword: "testpassword",
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil), reqtify.FromJSON(nil)}},
			nil,
			nil},
	}

	for _, x := range tuples {
		var user *types.TUserInfo
		var err error

		wg.Add(1)
		go func() {
			user, err = FetchUser(x.user, x.apikey)
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
		if !reflect.DeepEqual(user, x.expectedOutput) {
			t.Errorf("Discrepancy in tag!\nActual: %+v\nExpected: %+v\n", user, x.expectedOutput)
		}
		if !reflect.DeepEqual(err, x.expectedError) {
			t.Errorf("Discrepancy in error!\nActual: %+v\nExpected: %+v\n", err, x.expectedError)
		}
	}
}
