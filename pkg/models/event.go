package models

type Event struct {
	TableName string                 `json:"table_name"`
	Old       map[string]interface{} `json:"old"`
	New       map[string]interface{} `json:"new"`
	ID        int                    `json:"id"`
	Diff      map[string]interface{} `json:"diff"`
	Action    string                 `json:"action"`
}

type User struct {
	ID        int    `json:"id,omitempty"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	CreatedAt string `json:"created_at,omitempty"`
}
