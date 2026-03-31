package dto

type WebhookResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}
