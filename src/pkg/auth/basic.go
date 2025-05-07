package auth

import (
	"encoding/base64"
	"strings"

	"errors"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/whatsapp"
	"github.com/gofiber/fiber/v2"
)

type AuthBasicPayload struct {
	User *whatsapp.WhatsAppTenantUser
}

func BasicAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Parse HTTP Header Authorization
		authHeader := strings.SplitN(c.Get("Authorization"), " ", 2)

		// Validate Authorization header
		if len(authHeader) != 2 || authHeader[0] != "Basic" {
			return utils.ResponseAuthenticate(c)
		}

		// Decode credentials from base64
		authPayload, err := base64.StdEncoding.DecodeString(authHeader[1])
		if err != nil {
			return utils.ResponseBadRequest(c, "invalid base64 credentials")
		}

		authCredentials := strings.SplitN(string(authPayload), ":", 2)
		if len(authCredentials) != 2 {
			return utils.ResponseBadRequest(c, "invalid auth payload")
		}

		username, password := authCredentials[0], authCredentials[1]

		user, err := whatsapp.GetWhatsAppUserWithToken(username, password)
		if err != nil {
			return utils.ResponseBadRequest(c, err.Error())
		}

		if user == nil {
			return utils.ResponseBadRequest(c, "bad user")
		}

		// Set user to context locals
		c.Locals("User", user)

		// Call next handler
		return c.Next()
	}
}

func AuthPayload(c *fiber.Ctx) (AuthBasicPayload, error) {
	// Get the raw value from context
	value := c.Locals("User")
	if value == nil {
		return AuthBasicPayload{}, errors.New("user not found in context")
	}

	// Type assertion with check
	user, ok := value.(*whatsapp.WhatsAppTenantUser)
	if !ok {
		return AuthBasicPayload{}, errors.New("invalid user data")
	}

	return AuthBasicPayload{User: user}, nil
}
