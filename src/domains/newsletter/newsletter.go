package newsletter

import (
	"github.com/gofiber/fiber/v2"
)

type INewsletterService interface {
	Unfollow(c *fiber.Ctx, request UnfollowRequest) (err error)
}

type UnfollowRequest struct {
	NewsletterID string `json:"newsletter_id" form:"newsletter_id"`
}
