type Response struct {
	User *struct {
		Id string `json:"id"`
	} `json:"user"`
}

type role string

const (
	studentRole role = "STUDENT"
	teacherRole role = "TEACHER"
)

type userQueryInput struct {
	Email *string `json:"email"`
	Name  *string `json:"name"`
	Id    *string `json:"id"`
	Role  *role   `json:"role"`
}