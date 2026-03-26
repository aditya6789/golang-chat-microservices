package model

type UserProfile struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
	Online   bool   `json:"online"`
}

