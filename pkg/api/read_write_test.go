package api

import (
	"github.com/thewug/reqtify"
	"github.com/thewug/reqtify/mock"
	"testing"
	"os"
	"io"
	"flag"
	"fmt"
	"strings"
	"sync"
	"net/http"
	"net/url"
	"reflect"
	"io/ioutil"
	"errors"

	"github.com/thewug/fsb/pkg/api/types"
	"github.com/thewug/fsb/pkg/api/tags"
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

func CompareRequests(t *testing.T, req1, req2 reqtify.RequestImpl) bool {
	urleq := req1.URLPath == req2.URLPath
	verbeq := req1.Verb == req2.Verb
	usereq := req1.BasicUser == req2.BasicUser
	pwdeq :=req1.BasicPassword == req2.BasicPassword
	queryeq := (len(req1.QueryParams) == 0 && len(req2.QueryParams) == 0 || reflect.DeepEqual(req1.QueryParams, req2.QueryParams))
	formeq := (len(req1.FormParams) == 0 && len(req2.FormParams) == 0 || reflect.DeepEqual(req1.FormParams, req2.FormParams))
	autoeq := (len(req1.AutoParams) == 0 && len(req2.AutoParams) == 0 || reflect.DeepEqual(req1.AutoParams, req2.AutoParams))
	headeq :=(len(req1.Headers) == 0 && len(req2.Headers) == 0 || reflect.DeepEqual(req1.Headers, req2.Headers))
	cookieeq :=(len(req1.Cookies) == 0 && len(req2.Cookies) == 0 || reflect.DeepEqual(req1.Cookies, req2.Cookies))
	
	if !urleq { t.Errorf("Request URL not equal") }
	if !verbeq { t.Errorf("Request Verb not equal") }
	if !usereq { t.Errorf("Request User not equal") }
	if !pwdeq { t.Errorf("Request Password not equal") }
	if !queryeq { t.Errorf("Request Query not equal") }
	if !formeq { t.Errorf("Request Form not equal") }
	if !autoeq { t.Errorf("Request Auto not equal") }
	if !headeq { t.Errorf("Request Headers not equal") }
	if !cookieeq { t.Errorf("Request Cookies not equal") }
	
	if !urleq || !verbeq || !usereq || !pwdeq || !queryeq || !formeq || !autoeq || !headeq || !cookieeq {
		return false
	}

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
	if !reflect.DeepEqual(resp1, resp2) {
		t.Errorf("Request Response Object Types not similar")
		return false
	}

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
	if !reflect.DeepEqual(file1, file2) {
		t.Errorf("Request Form Files not similar")
		return false
	}
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
		if !CompareRequests(t, req.RequestImpl, x.expectedRequest) {
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
		expectedSelf *types.TUserInfo
	}{
		{"Snergal", "testpassword",
			http.Response{Status: "200 Testing", StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader(`[{"id":292290,"name":"Snergal","email":"this is an email"}]`))},
			nil,
			reqtify.RequestImpl{URLPath: "/users.json", Verb: reqtify.GET,
				AutoParams: url.Values{"search[name_matches]":[]string{"Snergal"}},
				BasicUser: "Snergal", BasicPassword: "testpassword",
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil), reqtify.FromJSON(nil)}},
			true,
			nil,
			&types.TUserInfo{Id: 292290, Name: "Snergal", Email: "this is an email"}},
		{"Snergal", "testpassword",
			http.Response{Status: "200 Testing", StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader(`[{"id":292290,"name":"Snergal","email":"this is an email"}]`))},
			errors.New("endpoint failed"),
			reqtify.RequestImpl{URLPath: "/users.json", Verb: reqtify.GET,
				AutoParams: url.Values{"search[name_matches]":[]string{"Snergal"}},
				BasicUser: "Snergal", BasicPassword: "testpassword",
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil), reqtify.FromJSON(nil)}},
			false,
			errors.New("endpoint failed"),
			nil},
		{"Snergal", "testpassword",
			http.Response{Status: "200 Testing", StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader(`[]`))},
			nil,
			reqtify.RequestImpl{URLPath: "/users.json", Verb: reqtify.GET,
				AutoParams: url.Values{"search[name_matches]":[]string{"Snergal"}},
				BasicUser: "Snergal", BasicPassword: "testpassword",
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil), reqtify.FromJSON(nil)}},
			false,
			nil,
			nil},
		{"Snergal", "testpassword",
			http.Response{Status: "200 Testing", StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader(`[{"id":292290,"name":"Snergal"}]`))},
			nil,
			reqtify.RequestImpl{URLPath: "/users.json", Verb: reqtify.GET,
				AutoParams: url.Values{"search[name_matches]":[]string{"Snergal"}},
				BasicUser: "Snergal", BasicPassword: "testpassword",
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil), reqtify.FromJSON(nil)}},
			false,
			nil,
			&types.TUserInfo{Id: 292290, Name: "Snergal"}},
		{"Snergal", "testpassword",
			http.Response{Status: "401 Testing", StatusCode: 401, Body: ioutil.NopCloser(strings.NewReader(`[{"id":292290,"name":"Snerg","email":"this is an email"}]`))},
			nil,
			reqtify.RequestImpl{URLPath: "/users.json", Verb: reqtify.GET,
				AutoParams: url.Values{"search[name_matches]":[]string{"Snergal"}},
				BasicUser: "Snergal", BasicPassword: "testpassword",
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil), reqtify.FromJSON(nil)}},
			false,
			errors.New("Got non-matching user?"),
			nil},
		{"Snergal", "testpassword",
			http.Response{Status: "401 Testing", StatusCode: 401, Body: ioutil.NopCloser(strings.NewReader(`[{"id":292290,"name":"Snergal","email":"this is an email"}, {"id":292291,"name":"Snergle"}]`))},
			nil,
			reqtify.RequestImpl{URLPath: "/users.json", Verb: reqtify.GET,
				AutoParams: url.Values{"search[name_matches]":[]string{"Snergal"}},
				BasicUser: "Snergal", BasicPassword: "testpassword",
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil), reqtify.FromJSON(nil)}},
			false,
			errors.New("Got wrong number of users?"),
			nil},
		{"Snergal", "testpassword",
			http.Response{Status: "401 Testing", StatusCode: 401, Body: ioutil.NopCloser(strings.NewReader(`{"success": false,"message": "SessionLoader::AuthenticationFailure","code": null}`))},
			nil,
			reqtify.RequestImpl{URLPath: "/users.json", Verb: reqtify.GET,
				AutoParams: url.Values{"search[name_matches]":[]string{"Snergal"}},
				BasicUser: "Snergal", BasicPassword: "testpassword",
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil), reqtify.FromJSON(nil)}},
			false,
			nil,
			nil},
	}

	for _, x := range tuples {
		var loggedIn bool
		var err error
		var self *types.TUserInfo

		wg.Add(1)
		go func() {
			self, loggedIn, err = TestLogin(x.user, x.apikey)
			wg.Done()
		}()

		req := <- examiner.Requests
		if !CompareRequests(t, req.RequestImpl, x.expectedRequest) {
			t.Errorf("Discrepancy in request!\nActual: %+v\nExpected: %+v\n", req.RequestImpl, x.expectedRequest)
		}
		examiner.Responses <- mock.ResponseAndError{
			Response: &x.response,
			Error: x.err,
		}

		wg.Wait()
		if !reflect.DeepEqual(err, x.expectedError) {
			t.Errorf("Discrepancy in error!\nActual: %+v\nExpected: %+v\n", err, x.expectedError)
		}
		if loggedIn != x.expectedOutput {
			t.Errorf("Discrepancy in loggedIn!\nActual: %+v\nExpected: %+v\n", loggedIn, x.expectedOutput)
		}
		if !reflect.DeepEqual(self, x.expectedSelf) {
			t.Errorf("Discrepancy in self!\nActual: %+v\nExpected: %+v\n", self, x.expectedSelf)
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
		if !CompareRequests(t, req.RequestImpl, x.expectedRequest) {
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
		if !CompareRequests(t, req.RequestImpl, x.expectedRequest) {
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
		if !CompareRequests(t, req.RequestImpl, x.expectedRequest) {
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
		if !CompareRequests(t, req.RequestImpl, x.expectedRequest) {
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
		if !CompareRequests(t, req.RequestImpl, x.expectedRequest) {
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

func sptr(s string) *string {
	return &s
}

func TestUploadFile(t *testing.T) {
	examiner := apiMock.Examine()
	var wg sync.WaitGroup

	var tuples = []struct{
		file_data io.Reader
		upload_url string
		tags tags.TagSet
		rating, source, description string
		parent *int
		user, apikey string
		response *http.Response
		err error
		expectedRequest reqtify.RequestImpl
		expectedOutput *UploadCallResult
		expectedError error
	}{
		{strings.NewReader("file data"), "", tags.TagSet{tags.StringSet{map[string]bool{"afoo":true, "bar":true}}}, "e", "source", "description", nil,
			"testuser", "testpassword",
			&http.Response{Status: "200 Testing", StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader(`{"success":true,"location":"url"}`))},
			nil,
			reqtify.RequestImpl{URLPath: "/uploads.json", Verb: reqtify.POST,
				BasicUser: "testuser", BasicPassword: "testpassword",
				FormParams: url.Values{"upload[source]":[]string{"source"}, "upload[description]":[]string{"description"}, "upload[tag_string]":[]string{"afoo bar"}, "upload[rating]":[]string{"e"}},
				FormFiles: map[string][]reqtify.FormFile{"upload[file]": []reqtify.FormFile{reqtify.FormFile{Name: "post.file", Data: strings.NewReader("file data")}}},
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil)}},
			&UploadCallResult{Success: true, Location: sptr("url"), Status: "200 Testing", StatusCode: 200},
			nil},
		{nil, "upload from url", tags.TagSet{tags.StringSet{map[string]bool{"afoo":true, "bar":true}}}, "e", "source", "description", new(int),
			"testuser", "testpassword",
			&http.Response{Status: "200 Testing", StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader(`{"success":true,"location":"url"}`))},
			nil,
			reqtify.RequestImpl{URLPath: "/uploads.json", Verb: reqtify.POST,
				BasicUser: "testuser", BasicPassword: "testpassword",
				FormParams: url.Values{"upload[source]":[]string{"source"}, "upload[direct_url]":[]string{"upload from url"}, "upload[description]":[]string{"description"}, "upload[tag_string]":[]string{"afoo bar"}, "upload[rating]":[]string{"e"}, "upload[parent_id]":[]string{"0"}},
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil)}},
			&UploadCallResult{Success: true, Location: sptr("url"), Status: "200 Testing", StatusCode: 200},
			nil},
		{nil, "upload from url", tags.TagSet{tags.StringSet{map[string]bool{"afoo":true, "bar":true}}}, "e", "source", "description", new(int),
			"testuser", "testpassword",
			nil,
			errors.New("failure"),
			reqtify.RequestImpl{URLPath: "/uploads.json", Verb: reqtify.POST,
				BasicUser: "testuser", BasicPassword: "testpassword",
				FormParams: url.Values{"upload[source]":[]string{"source"}, "upload[direct_url]":[]string{"upload from url"}, "upload[description]":[]string{"description"}, "upload[tag_string]":[]string{"afoo bar"}, "upload[rating]":[]string{"e"}, "upload[parent_id]":[]string{"0"}},
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil)}},
			&UploadCallResult{},
			errors.New("failure")},
		{nil, "upload from url", tags.TagSet{tags.StringSet{map[string]bool{"afoo":true, "bar":true}}}, "e", "source", "description", new(int),
			"testuser", "testpassword",
			&http.Response{Status: "401 Testing", StatusCode: 401, Body: ioutil.NopCloser(strings.NewReader(`{"success":false,"reason":"fail"}`))},
			nil,
			reqtify.RequestImpl{URLPath: "/uploads.json", Verb: reqtify.POST,
				BasicUser: "testuser", BasicPassword: "testpassword",
				FormParams: url.Values{"upload[source]":[]string{"source"}, "upload[direct_url]":[]string{"upload from url"}, "upload[description]":[]string{"description"}, "upload[tag_string]":[]string{"afoo bar"}, "upload[rating]":[]string{"e"}, "upload[parent_id]":[]string{"0"}},
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil)}},
			&UploadCallResult{Success: false, Reason: sptr("fail"), Status: "401 Testing", StatusCode: 401},
			nil},
		{nil, "", tags.TagSet{tags.StringSet{map[string]bool{"afoo":true, "bar":true}}}, "e", "source", "description", new(int),
			"testuser", "testpassword",
			nil,
			nil,
			reqtify.RequestImpl{},
			nil,
			MissingArguments},
	}

	for _, x := range tuples {
		var status *UploadCallResult
		var err error

		wg.Add(1)
		go func() {
			status, err = UploadFile(x.file_data, x.upload_url, x.tags, x.rating, x.source, x.description, x.parent, x.user, x.apikey)
			wg.Done()
		}()

		if x.err != nil || x.response != nil {
			req := <- examiner.Requests
			if !CompareRequests(t, req.RequestImpl, x.expectedRequest) {
				t.Errorf("Discrepancy in request!\nActual: %+v\nExpected: %+v\n", req.RequestImpl, x.expectedRequest)
			}
			examiner.Responses <- mock.ResponseAndError{
				Response: x.response,
				Error: x.err,
			}
		}

		wg.Wait()
		if !reflect.DeepEqual(status, x.expectedOutput) {
			t.Errorf("Discrepancy in status!\nActual: %+v\nExpected: %+v\n", status, x.expectedOutput)
		}
		if !reflect.DeepEqual(err, x.expectedError) {
			t.Errorf("Discrepancy in error!\nActual: %+v\nExpected: %+v\n", err, x.expectedError)
		}
	}
}

func iptr(i int) *int { return &i }

func TestUpdatePost(t *testing.T) {
	examiner := apiMock.Examine()
	var wg sync.WaitGroup
	
	tagstring := "afoo -bar"

	testcases := map[string]struct{
		tags tags.TagDiff
		rating, description, reason *string
		parent *int
		sourcediff []string
		user, apikey string
		httpResponse *http.Response
		httpErr error
		expectedHttpRequest reqtify.RequestImpl
		expectedFuncOutput *types.TPostInfo
		expectedFuncError error
	}{
		"normal": {tags.TagDiffFromString(tagstring), sptr("new_rating"), sptr("new_description"), sptr("edit_reason"),
			iptr(1000), []string{"-source1", "source2"},
			"testuser", "testapikey",
			&http.Response{Status: "200 Testing", StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader(`{"id":9999, "tags":{"general":["tag1", "afoo"]}, "sources":["source0", "source2"], "relationships":{"parent_id":1000}, "rating":"new_rating", "description":"new_description"}`))},
			nil,
			reqtify.RequestImpl{URLPath: "/posts/9999.json", Verb: reqtify.PATCH,
				BasicUser: "testuser", BasicPassword: "testapikey",
				FormParams: url.Values{"post[source_diff]":[]string{"-source1\nsource2"}, "post[description]":[]string{"new_description"}, "post[tag_string_diff]":[]string{tagstring}, "post[rating]":[]string{"new_rating"}, "post[parent_id]":[]string{"1000"}, "post[edit_reason]": []string{"edit_reason"}},
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil), reqtify.FromJSON(nil)}},
			&types.TPostInfo{Id: 9999, Description: "new_description", Rating: "new_rating", TPostTags: types.TPostTags{General: []string{"tag1", "afoo"}}, TPostRelationships: types.TPostRelationships{Parent_id: 1000}, Sources: []string{"source0", "source2"}},
			nil},
		"unset-parent": {tags.TagDiff{}, nil, nil, sptr("edit_reason"),
			iptr(-1), nil,
			"testuser", "testapikey",
			&http.Response{Status: "200 Testing", StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader(`{"id":9999, "tags":{"general":["tag1", "bar"]}, "sources":["source0", "source1"], "rating":"old_rating", "description":"old_description"}`))},
			nil,
			reqtify.RequestImpl{URLPath: "/posts/9999.json", Verb: reqtify.PATCH,
				BasicUser: "testuser", BasicPassword: "testapikey",
				FormParams: url.Values{"post[parent_id]":[]string{""}, "post[edit_reason]": []string{"edit_reason"}},
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil), reqtify.FromJSON(nil)}},
			&types.TPostInfo{Id: 9999, Description: "old_description", Rating: "old_rating", TPostTags: types.TPostTags{General: []string{"tag1", "bar"}}, Sources: []string{"source0", "source1"}},
			nil},
		"noop": {tags.TagDiff{}, nil, nil, sptr("edit_reason"),
			nil, nil,
			"testuser", "testapikey",
			&http.Response{Status: "200 Testing", StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader(`{"id":9999, "tags":{"general":["tag1", "bar"]}, "sources":["source0", "source1"], "rating":"old_rating", "description":"old_description"}`))},
			nil,
			reqtify.RequestImpl{URLPath: "/posts/9999.json", Verb: reqtify.PATCH,
				BasicUser: "testuser", BasicPassword: "testapikey",
				FormParams: url.Values{"post[edit_reason]": []string{"edit_reason"}},
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil), reqtify.FromJSON(nil)}},
			&types.TPostInfo{Id: 9999, Description: "old_description", Rating: "old_rating", TPostTags: types.TPostTags{General: []string{"tag1", "bar"}}, Sources: []string{"source0", "source1"}},
			nil},
		"update-deleted": {tags.TagDiff{}, nil, nil, sptr("edit_reason"),
			nil, nil,
			"testuser", "testapikey",
			&http.Response{Status: "403 Testing", StatusCode: 403, Body: ioutil.NopCloser(strings.NewReader(`{"success":false, "reason":"Access Denied: Post not visible to you"}`))},
			nil,
			reqtify.RequestImpl{URLPath: "/posts/9999.json", Verb: reqtify.PATCH,
				BasicUser: "testuser", BasicPassword: "testapikey",
				FormParams: url.Values{"post[edit_reason]": []string{"edit_reason"}},
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil), reqtify.FromJSON(nil)}},
			nil,
			PostIsDeleted},
		"parse-error": {tags.TagDiff{}, nil, nil, sptr("edit_reason"),
			nil, nil,
			"testuser", "testapikey",
			&http.Response{Status: "200 OK", StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader(`<html>hi</html>`))},
			errors.New("Whoa, an error!"),
			reqtify.RequestImpl{URLPath: "/posts/9999.json", Verb: reqtify.PATCH,
				BasicUser: "testuser", BasicPassword: "testapikey",
				FormParams: url.Values{"post[edit_reason]": []string{"edit_reason"}},
				Response: []reqtify.ResponseUnmarshaller{reqtify.FromJSON(nil), reqtify.FromJSON(nil)}},
			nil,
			errors.New("Whoa, an error!")},
	}

	for k, v := range testcases {
		t.Run(k, func(t *testing.T) {
			var post *types.TPostInfo
			var err error

			wg.Add(1)
			go func() {
				post, err = UpdatePost(v.user, v.apikey, 9999, v.tags, v.rating, v.parent, v.sourcediff, v.description, v.reason)
				wg.Done()
			}()

			req := <- examiner.Requests
			if !CompareRequests(t, req.RequestImpl, v.expectedHttpRequest) {
				t.Errorf("Discrepancy in request!\nActual:   %+v\nExpected: %+v\n", req.RequestImpl, v.expectedHttpRequest)
			}
			examiner.Responses <- mock.ResponseAndError{
				Response: v.httpResponse,
				Error: v.httpErr,
			}

			wg.Wait()
			if !reflect.DeepEqual(post, v.expectedFuncOutput) {
				t.Errorf("Discrepancy in status!\nActual:   %+v\nExpected: %+v\n", post, v.expectedFuncOutput)
			}
			if !reflect.DeepEqual(err, v.expectedFuncError) {
				t.Errorf("Discrepancy in error!\nActual:   %+v\nExpected: %+v\n", err, v.expectedFuncError)
			}
		})
	}
}
