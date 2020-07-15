type Response struct {
	Me *struct {
		Roles []role `json:"roles"`
	}
	OtherUser *struct {
		Roles []role `json:"roles"`
	}
}

type role string

const (
	studentRole role = "STUDENT"
	teacherRole role = "TEACHER"
)