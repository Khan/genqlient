type Response struct {
	User *struct {
		Id string `json:"id"`
		profileData
	} `json:"user"`
}

type profileData struct {
	Name   *string  `json:"name"`
	Emails []string `json:"emails"`
}