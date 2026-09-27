package model

type Node struct {
	Resource string  `json:"resource"`
	Title    string  `json:"title"`
	Links    []*Node `json:"links"`
}

type Page struct {
	Title string
	Links []string
}
