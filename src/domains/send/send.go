package send

import (
	"github.com/gofiber/fiber/v2"
)

type ISendService interface {
	SendText(c *fiber.Ctx, request MessageRequest) (response GenericResponse, err error)
	SendImage(c *fiber.Ctx, request ImageRequest) (response GenericResponse, err error)
	SendFile(c *fiber.Ctx, request FileRequest) (response GenericResponse, err error)
	SendVideo(c *fiber.Ctx, request VideoRequest) (response GenericResponse, err error)
	SendContact(c *fiber.Ctx, request ContactRequest) (response GenericResponse, err error)
	SendLink(c *fiber.Ctx, request LinkRequest) (response GenericResponse, err error)
	SendLocation(c *fiber.Ctx, request LocationRequest) (response GenericResponse, err error)
	SendAudio(c *fiber.Ctx, request AudioRequest) (response GenericResponse, err error)
	SendPoll(c *fiber.Ctx, request PollRequest) (response GenericResponse, err error)
	SendPresence(c *fiber.Ctx, request PresenceRequest) (response GenericResponse, err error)
}

type GenericResponse struct {
	MessageID string `json:"message_id"`
	Status    string `json:"status"`
}
