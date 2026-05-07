package models

type User struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	// omitempty means if field is empty, it will be omitted from the JSON response.
	Avatar    string `json:"avatar,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}