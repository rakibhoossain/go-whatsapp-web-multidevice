package message

import (
	"github.com/gofiber/fiber/v2"
)

type IMessageService interface {
	GetAllChatMessage(c *fiber.Ctx, request ChatMessageRequest) (response ChatMessageResponse, err error)
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

type ChatMessageRequest struct {
	ChatID string `json:"chat_id" uri:"chat_id"`
	Limit  int    `json:"limit" uri:"limit"`
	Offset int    `json:"offset" uri:"offset"`
}

type ChatMessageResponse struct {
	ChatID string `json:"chat_id"`
	Data   any    `json:"data"`
}

type Message struct {
	ID        int    `json:"id"`
	SenderJID string `json:"sender_jid"`
	Timestamp string `json:"timestamp"`
	Content   string `json:"content"`
	IsFromMe  bool   `json:"is_from_me"`
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
