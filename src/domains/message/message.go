package message

import (
	"github.com/gofiber/fiber/v2"
)

type IMessageService interface {
	MarkAsRead(c *fiber.Ctx, request MarkAsReadRequest) (response GenericResponse, err error)
	ReactMessage(c *fiber.Ctx, request ReactionRequest) (response GenericResponse, err error)
	RevokeMessage(c *fiber.Ctx, request RevokeRequest) (response GenericResponse, err error)
	UpdateMessage(c *fiber.Ctx, request UpdateMessageRequest) (response GenericResponse, err error)
	DeleteMessage(c *fiber.Ctx, request DeleteRequest) (err error)
	StarMessage(c *fiber.Ctx, request StarRequest) (err error)
}

type GenericResponse struct {
	MessageID string `json:"message_id"`
	Status    string `json:"status"`
}

type RevokeRequest struct {
	MessageID string `json:"message_id" uri:"message_id"`
	Phone     string `json:"phone" form:"phone"`
}

type DeleteRequest struct {
	MessageID string `json:"message_id" uri:"message_id"`
	Phone     string `json:"phone" form:"phone"`
}

type ReactionRequest struct {
	MessageID string `json:"message_id" form:"message_id"`
	Phone     string `json:"phone" form:"phone"`
	Emoji     string `json:"emoji" form:"emoji"`
}

type UpdateMessageRequest struct {
	MessageID string `json:"message_id" uri:"message_id"`
	Message   string `json:"message" form:"message"`
	Phone     string `json:"phone" form:"phone"`
}

type MarkAsReadRequest struct {
	MessageID string `json:"message_id" uri:"message_id"`
	Phone     string `json:"phone" form:"phone"`
}

type StarRequest struct {
	MessageID string `json:"message_id" uri:"message_id"`
	Phone     string `json:"phone" form:"phone"`
	IsStarred bool   `json:"is_starred"`
}
