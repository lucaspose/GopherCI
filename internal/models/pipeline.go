package models

type Step struct {
	Name string `json:"name"`
	Cmd []string `json:"cmd"`
}