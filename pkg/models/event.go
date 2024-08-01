package models

type Event struct {
	Action    string                 `json:"action"`
	TableName string                 `json:"table_name"`
	Data      map[string]interface{} `json:"data"`
}
