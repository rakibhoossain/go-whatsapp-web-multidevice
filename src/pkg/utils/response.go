package utils

import (
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type ResponseData struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Results any    `json:"results,omitempty"`
}

func ResponseBadRequest(c *fiber.Ctx, message string) error {
	code := http.StatusBadRequest

	if strings.TrimSpace(message) == "" {
		message = http.StatusText(code)
	}

	response := ResponseData{
		Status:  code,
		Code:    http.StatusText(code),
		Message: message,
	}

	return c.Status(code).JSON(response)
}

func ResponseUnauthorized(c *fiber.Ctx, message string) error {
	code := http.StatusBadRequest

	if strings.TrimSpace(message) == "" {
		message = http.StatusText(code)
	}

	response := ResponseData{
		Status:  code,
		Code:    http.StatusText(code),
		Message: message,
	}

	return c.Status(code).JSON(response)
}

func ResponseAuthenticate(c *fiber.Ctx) error {
	c.Set("WWW-Authenticate", `Basic realm="Authentication Required"`)
	return ResponseUnauthorized(c, "")
}

func ResponseSuccessWithData(c *fiber.Ctx, message string, data interface{}) error {
	return c.JSON(ResponseData{
		Status:  http.StatusOK,
		Code:    "SUCCESS",
		Message: message,
		Results: data,
	})
}

func ResponseNotFound(c *fiber.Ctx, message string) error {

	code := http.StatusNotFound

	if strings.TrimSpace(message) == "" {
		message = http.StatusText(code)
	}

	response := ResponseData{
		Status:  code,
		Code:    http.StatusText(code),
		Message: message,
	}

	return c.Status(code).JSON(response)
}

func ResponseSuccess(c *fiber.Ctx, message string) error {
	return c.JSON(ResponseData{
		Status:  http.StatusOK,
		Code:    "SUCCESS",
		Message: message,
	})
}
