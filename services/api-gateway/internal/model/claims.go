package model

type Claims struct {
	Sub string `json:"sub"`
	Exp int64  `json:"exp"`
}

