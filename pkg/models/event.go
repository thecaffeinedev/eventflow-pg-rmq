package models

type Event struct {
	Action    string                 `json:"action"`
	TableName string                 `json:"table_name"`
	Data      map[string]interface{} `json:"data"`
}

type User struct {
	ID        int    `json:"id,omitempty"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	CreatedAt string `json:"created_at,omitempty"`
}
