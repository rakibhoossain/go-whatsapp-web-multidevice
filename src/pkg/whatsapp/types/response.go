package types


type ClientStatusResponse struct {
	ID         int64  `json:"id"`
	ClientName string `json:"client_name"`
	UUID       string `json:"uuid"`
	WebhookURL string `json:"webhook_url"`
	SecretKey  string `json:"secret_key"`
	Status     string `json:"status"`
	StatusCode int    `json:"status_code"`
	UserCount  int    `json:"user_count"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}