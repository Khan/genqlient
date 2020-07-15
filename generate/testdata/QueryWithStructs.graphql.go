type Response struct {
	User *struct {
		AuthMethods []struct {
			Provider *string `json:"provider"`
			Email    *string `json:"email"`
		} `json:"authMethods"`
	} `json:"user"`
}