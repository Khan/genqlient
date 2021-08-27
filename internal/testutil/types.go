package testutil

type ID string

type Pokemon struct {
	Species string `json:"species"`
	Level   int    `json:"level"`
}

func (p Pokemon) Battle(q Pokemon) bool {
	return p.Level > q.Level
}
