package types

type CreateClientRequest struct {
	Name       string `form:"name" validate:"required,min=3,max=50"`
	WebhookURL string `form:"webhook_url" validate:"omitempty,url"`
}

type UpdateClientStatusRequest struct {
	StatusCode string `form:"status_code" validate:"required,oneof=0 1"`
}
