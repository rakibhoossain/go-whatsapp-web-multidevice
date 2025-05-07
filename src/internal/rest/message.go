package rest

import (
	domainMessage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/message"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/auth"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/whatsapp"
	"github.com/gofiber/fiber/v2"
)

type Message struct {
	Service domainMessage.IMessageService
}

func InitRestMessage(app *fiber.App, service domainMessage.IMessageService) Message {
	rest := Message{Service: service}
	app.Get("/message/chat/:chat_id/messages", auth.BasicAuth(), rest.GetAllChatMessage)
	app.Post("/message/:message_id/reaction", auth.BasicAuth(), rest.ReactMessage)
	app.Post("/message/:message_id/revoke", auth.BasicAuth(), rest.RevokeMessage)
	app.Post("/message/:message_id/delete", auth.BasicAuth(), rest.DeleteMessage)
	app.Post("/message/:message_id/update", auth.BasicAuth(), rest.UpdateMessage)
	app.Post("/message/:message_id/read", auth.BasicAuth(), rest.MarkAsRead)
	app.Post("/message/:message_id/star", auth.BasicAuth(), rest.StarMessage)
	app.Post("/message/:message_id/unstar", auth.BasicAuth(), rest.UnstarMessage)
	return rest
}

func (controller *Message) RevokeMessage(c *fiber.Ctx) error {
	var request domainMessage.RevokeRequest
	err := c.BodyParser(&request)
	utils.PanicIfNeeded(err)

	request.MessageID = c.Params("message_id")
	whatsapp.SanitizePhone(&request.Phone)

	response, err := controller.Service.RevokeMessage(c, request)
	utils.PanicIfNeeded(err)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: response.Status,
		Results: response,
	})
}

func (controller *Message) DeleteMessage(c *fiber.Ctx) error {
	var request domainMessage.DeleteRequest
	err := c.BodyParser(&request)
	utils.PanicIfNeeded(err)

	request.MessageID = c.Params("message_id")
	whatsapp.SanitizePhone(&request.Phone)

	err = controller.Service.DeleteMessage(c, request)
	utils.PanicIfNeeded(err)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Message deleted successfully",
		Results: nil,
	})
}

func (controller *Message) UpdateMessage(c *fiber.Ctx) error {
	var request domainMessage.UpdateMessageRequest
	err := c.BodyParser(&request)
	utils.PanicIfNeeded(err)

	request.MessageID = c.Params("message_id")
	whatsapp.SanitizePhone(&request.Phone)

	response, err := controller.Service.UpdateMessage(c, request)
	utils.PanicIfNeeded(err)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: response.Status,
		Results: response,
	})
}

func (controller *Message) GetAllChatMessage(c *fiber.Ctx) error {
	var request domainMessage.ChatMessageRequest
	request.ChatID = c.Params("chat_id")

	response, err := controller.Service.GetAllChatMessage(c, request)
	utils.PanicIfNeeded(err)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Fetch messages",
		Results: response,
	})
}

func (controller *Message) ReactMessage(c *fiber.Ctx) error {
	var request domainMessage.ReactionRequest
	err := c.BodyParser(&request)
	utils.PanicIfNeeded(err)

	request.MessageID = c.Params("message_id")
	whatsapp.SanitizePhone(&request.Phone)

	response, err := controller.Service.ReactMessage(c, request)
	utils.PanicIfNeeded(err)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: response.Status,
		Results: response,
	})
}

func (controller *Message) MarkAsRead(c *fiber.Ctx) error {
	var request domainMessage.MarkAsReadRequest
	err := c.BodyParser(&request)
	utils.PanicIfNeeded(err)

	request.MessageID = c.Params("message_id")
	whatsapp.SanitizePhone(&request.Phone)

	response, err := controller.Service.MarkAsRead(c, request)
	utils.PanicIfNeeded(err)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: response.Status,
		Results: response,
	})
}

func (controller *Message) StarMessage(c *fiber.Ctx) error {
	var request domainMessage.StarRequest
	err := c.BodyParser(&request)
	utils.PanicIfNeeded(err)

	request.MessageID = c.Params("message_id")
	whatsapp.SanitizePhone(&request.Phone)
	request.IsStarred = true

	err = controller.Service.StarMessage(c, request)
	utils.PanicIfNeeded(err)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Starred message successfully",
		Results: nil,
	})
}

func (controller *Message) UnstarMessage(c *fiber.Ctx) error {
	var request domainMessage.StarRequest
	err := c.BodyParser(&request)
	utils.PanicIfNeeded(err)

	request.MessageID = c.Params("message_id")
	whatsapp.SanitizePhone(&request.Phone)
	request.IsStarred = false
	err = controller.Service.StarMessage(c, request)
	utils.PanicIfNeeded(err)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Unstarred message successfully",
		Results: nil,
	})
}
