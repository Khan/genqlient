type Response struct {
	User *struct {
		Emails                []string  `json:"emails"`
		EmailsOrNull          []string  `json:"emailsOrNull"`
		EmailsWithNulls       []*string `json:"emailsWithNulls"`
		EmailsWithNullsOrNull []*string `json:"emailsWithNullsOrNull"`
	} `json:"user"`
}