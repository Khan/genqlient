type Response struct {
	User *struct {
		Typename *string `json:"__typename"`
		Id       string  `json:"id"`
	} `json:"user"`
}