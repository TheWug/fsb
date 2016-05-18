package telegram

import (
	"encoding/json"
)

type TUser struct {
        Id         int    `json:"id"`
        First_name string `json:"first_name"`
        Last_name  string `json:"last_name"`
        Username   string `json:"username"`
}

type TInlineQuery struct {
        Id     string `json:"id"`
        From   TUser  `json:"from"`
        Query  string `json:"query"`
        Offset string `json:"offset"`
}

type TChosenInlineResult struct {
	Result_id          string    `json:"result_id"`
	From               TUser     `json:"from"`
//	Location          *TLocation `json:"location,omitempty"`
	Inline_message_id *string    `json:"inline_message_id,omitempty"`
	Query              string    `json:"query"`
}

type TUpdate struct {
        Update_id             int                 `json:"update_id"`
//	Message              *TMessage            `json:"message,omitempty"`
        Inline_query         *TInlineQuery        `json:"inline_query,omitempty"`
	Chosen_inline_result *TChosenInlineResult `json:"chosen_inline_result,omitempty"`
//	Callback_query       *TCallbackQuery      `json:"callback_query,omitempty"`
}

type TGenericResponse struct {
        Ok          bool             `json:"ok"`
	Error_code  *int             `json:"error_code,omitempty"`
	Description *string          `json:"description,omitempty"`
	Result      *json.RawMessage `json:"result,omitempty"`
}

type TInlineQueryResultPhoto struct {
	Type                   string `json:"type"`
	Id                     string `json:"id"`
	Photo_url              string `json:"photo_url"`
	Thumb_url              string `json:"thumb_url"`
	Photo_width           *int    `json:"photo_width,omitempty"`
	Photo_height          *int    `json:"photo_height,omitempty"`
	Title                 *string `json:"title,omitempty"`
	Description           *string `json:"description,omitempty"`
	Caption               *string `json:"caption,omitempty"`
	Reply_markup          *string `json:"reply_markup,omitempty"`
	Input_message_content *string `json:"input_message_content,omitempty"`
}

type TInlineQueryResultGif struct {
	Type                   string `json:"type"`
	Id                     string `json:"id"`
	Gif_url                string `json:"gif_url"`
	Gif_width             *int    `json:"gif_width,omitempty"`
	Gif_height            *int    `json:"gif_height,omitempty"`
	Thumb_url              string `json:"thumb_url"`
	Title                 *string `json:"title,omitempty"`
	Caption               *string `json:"caption,omitempty"`
	Reply_markup          *string `json:"reply_markup,omitempty"`
	Input_message_content *string `json:"input_message_content,omitempty"`
}
