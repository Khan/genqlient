type Response struct {
	User *struct {
		ID     string
		AlsoID string
	} `json:"user"`
}