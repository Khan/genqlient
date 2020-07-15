type Response struct {
	Root struct {
		Id       string    `json:"id"`
		Name     string    `json:"name"`
		Children []content `json:"children"`
	} `json:"root"`
}

type content struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}