type ProfileData struct {
	Name   *string  `json:"name"`
	Emails []string `json:"emails"`
}

type Response struct {
	User *struct {
		Id string `json:"id"`
		ProfileData
	} `json:"user"`
}