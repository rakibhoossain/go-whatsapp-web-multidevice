package rest

import (
	domainSend "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/send"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/auth"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/whatsapp"
	"github.com/gofiber/fiber/v2"
)

type Send struct {
	Service domainSend.ISendService
}

func InitRestSend(app *fiber.App, service domainSend.ISendService) Send {
	rest := Send{Service: service}
	app.Post("/send/message", auth.BasicAuth(), rest.SendText)
	app.Post("/send/image", auth.BasicAuth(), rest.SendImage)
	app.Post("/send/file", auth.BasicAuth(), rest.SendFile)
	app.Post("/send/video", auth.BasicAuth(), rest.SendVideo)
	app.Post("/send/contact", auth.BasicAuth(), rest.SendContact)
	app.Post("/send/link", auth.BasicAuth(), rest.SendLink)
	app.Post("/send/location", auth.BasicAuth(), rest.SendLocation)
	app.Post("/send/audio", auth.BasicAuth(), rest.SendAudio)
	app.Post("/send/poll", auth.BasicAuth(), rest.SendPoll)
	app.Post("/send/presence", auth.BasicAuth(), rest.SendPresence)
	return rest
}

func (controller *Send) SendText(c *fiber.Ctx) error {
	var request domainSend.MessageRequest
	err := c.BodyParser(&request)
	utils.PanicIfNeeded(err)

	whatsapp.SanitizePhone(&request.Phone)

	response, err := controller.Service.SendText(c, request)
	utils.PanicIfNeeded(err)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: response.Status,
		Results: response,
	})
}

func (controller *Send) SendImage(c *fiber.Ctx) error {
	var request domainSend.ImageRequest
	request.Compress = true

	err := c.BodyParser(&request)
	utils.PanicIfNeeded(err)

	file, err := c.FormFile("image")
	if err == nil {
		request.Image = file
	}

	whatsapp.SanitizePhone(&request.Phone)

	response, err := controller.Service.SendImage(c, request)
	utils.PanicIfNeeded(err)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: response.Status,
		Results: response,
	})
}

func (controller *Send) SendFile(c *fiber.Ctx) error {
	var request domainSend.FileRequest
	err := c.BodyParser(&request)
	utils.PanicIfNeeded(err)

	file, err := c.FormFile("file")
	utils.PanicIfNeeded(err)

	request.File = file
	whatsapp.SanitizePhone(&request.Phone)

	response, err := controller.Service.SendFile(c, request)
	utils.PanicIfNeeded(err)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: response.Status,
		Results: response,
	})
}

func (controller *Send) SendVideo(c *fiber.Ctx) error {
	var request domainSend.VideoRequest
	err := c.BodyParser(&request)
	utils.PanicIfNeeded(err)

	video, err := c.FormFile("video")
	utils.PanicIfNeeded(err)

	request.Video = video
	whatsapp.SanitizePhone(&request.Phone)

	response, err := controller.Service.SendVideo(c, request)
	utils.PanicIfNeeded(err)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: response.Status,
		Results: response,
	})
}

func (controller *Send) SendContact(c *fiber.Ctx) error {
	var request domainSend.ContactRequest
	err := c.BodyParser(&request)
	utils.PanicIfNeeded(err)

	whatsapp.SanitizePhone(&request.Phone)

	response, err := controller.Service.SendContact(c, request)
	utils.PanicIfNeeded(err)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: response.Status,
		Results: response,
	})
}

func (controller *Send) SendLink(c *fiber.Ctx) error {
	var request domainSend.LinkRequest
	err := c.BodyParser(&request)
	utils.PanicIfNeeded(err)

	whatsapp.SanitizePhone(&request.Phone)

	response, err := controller.Service.SendLink(c, request)
	utils.PanicIfNeeded(err)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: response.Status,
		Results: response,
	})
}

func (controller *Send) SendLocation(c *fiber.Ctx) error {
	var request domainSend.LocationRequest
	err := c.BodyParser(&request)
	utils.PanicIfNeeded(err)

	whatsapp.SanitizePhone(&request.Phone)

	response, err := controller.Service.SendLocation(c, request)
	utils.PanicIfNeeded(err)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: response.Status,
		Results: response,
	})
}

func (controller *Send) SendAudio(c *fiber.Ctx) error {
	var request domainSend.AudioRequest
	err := c.BodyParser(&request)
	utils.PanicIfNeeded(err)

	audio, err := c.FormFile("audio")
	utils.PanicIfNeeded(err)

	request.Audio = audio
	whatsapp.SanitizePhone(&request.Phone)

	response, err := controller.Service.SendAudio(c, request)
	utils.PanicIfNeeded(err)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: response.Status,
		Results: response,
	})
}

func (controller *Send) SendPoll(c *fiber.Ctx) error {
	var request domainSend.PollRequest
	err := c.BodyParser(&request)
	utils.PanicIfNeeded(err)

	whatsapp.SanitizePhone(&request.Phone)

	response, err := controller.Service.SendPoll(c, request)
	utils.PanicIfNeeded(err)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: response.Status,
		Results: response,
	})
}

func (controller *Send) SendPresence(c *fiber.Ctx) error {
	var request domainSend.PresenceRequest
	err := c.BodyParser(&request)
	utils.PanicIfNeeded(err)

	response, err := controller.Service.SendPresence(c, request)
	utils.PanicIfNeeded(err)

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: response.Status,
		Results: response,
	})
}
