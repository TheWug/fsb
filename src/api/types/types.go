package types

// tag info
const (
	General = 0
	Artist = 1
	Unused = 2
	Copyright = 3
	Character = 4
	Species = 5
	Invalid = 6
	Meta = 7
	Lore = 8
)

type TTagData struct {
	Id int `json:"id"`
	Name string `json:"name"`
	Count int `json:"post_count"`
	Type int `json:"category"`
	Locked *bool `json:"is_locked"`

	// created_at
	// updated_at
	// related_tags
	// related_tags_updated_at
}

type TTagInfoArray []TTagData

type TAliasData struct {
	Id int `json:"id"`
	Name string `json:"consequent_name"`
	Alias int `json:"antecedent_name"`

	// reason
	// creator_id
	// created_at
	// updated_at
	// forum_post_id
	// forum_topic_id
}

type TAliasInfoArray []TAliasData

type TTagHistory struct {
	Id int `json:"id"`
	Post_id int `json:"post_id"`
	Tags string `json:"tags"`
	// there's other fields, but I don't care about them and all they'll do is waste memory.
}

type THistoryArray []TTagHistory

type TSearchResult struct {
	Id int `json:"id"`
	Tags string `json:"tags"`
	Description string `json:"description"`
//	Created_at JSONTime `json:"created_at"`
	Creator_id int `json:"creator_id"`
	Author string `json:"author"`
	Change int `json:"change"`
	Source string `json:"source"`
	Score int `json:"score"`
	Fav_count int `json:"fav_count"`
	Md5 string `json:"md5"`
	File_size int `json:"file_size"`
	File_url string `json:"file_url"`
	File_ext string `json:"file_ext"`
	Preview_url string `json:"preview_url"`
	Preview_width int `json:"preview_width"`
	Preview_height int `json:"preview_height"`
	Sample_url string `json:"sample_url"`
	Sample_width int `json:"sample_width"`
	Sample_height int `json:"sample_height"`
	Rating string `json:"rating"`
	Status string `json:"status"`
	Width int `json:"width"`
	Height int `json:"height"`
	Has_comments bool `json:"has_comments"`
	Has_notes bool `json:"has_notes"`
	Has_children bool `json:"has_children"`
//	Children string `json:"children,omitempty"`
	Parent_id int `json:"parent_id,omitempty"`
	Artist []string `json:"artist,omitempty"`
	Sources []string `json:"sources,omitempty"`
}

type TResultArray []TSearchResult
