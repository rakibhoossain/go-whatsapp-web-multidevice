package app

import (
	"github.com/gofiber/fiber/v2"
)

type IAppService interface {
	Login(c *fiber.Ctx) (response LoginResponse, err error)
	LoginWithCode(c *fiber.Ctx, phoneNumber string) (loginCode string, err error)
	Logout(c *fiber.Ctx) (err error)
	Reconnect(c *fiber.Ctx) (err error)
	FirstDevice(c *fiber.Ctx) (response DevicesResponse, err error)
	FetchDevices(c *fiber.Ctx) (response []DevicesResponse, err error)
}

type DevicesResponse struct {
	Name   string `json:"name"`
	Device string `json:"device"`
}

type LoginResponse struct {
	Duration int    `json:"duration"`
	Code     string `json:"code"`
}
