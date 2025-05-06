package user

import (
	"github.com/gofiber/fiber/v2"
)

type IUserService interface {
	Info(c *fiber.Ctx, request InfoRequest) (response InfoResponse, err error)
	Avatar(c *fiber.Ctx, request AvatarRequest) (response AvatarResponse, err error)
	ChangeAvatar(c *fiber.Ctx, request ChangeAvatarRequest) (err error)
	ChangePushName(c *fiber.Ctx, request ChangePushNameRequest) (err error)
	MyListGroups(c *fiber.Ctx) (response MyListGroupsResponse, err error)
	MyListNewsletter(c *fiber.Ctx) (response MyListNewsletterResponse, err error)
	MyPrivacySetting(c *fiber.Ctx) (response MyPrivacySettingResponse, err error)
	MyListContacts(c *fiber.Ctx) (response MyListContactsResponse, err error)
}
