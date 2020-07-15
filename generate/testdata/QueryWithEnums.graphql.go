type Response struct {
	User *struct {
		Roles []role `json:"roles"`
	} `json:"user"`
}

type role string

const (
	studentRole role = "STUDENT"
	teacherRole role = "TEACHER"
)