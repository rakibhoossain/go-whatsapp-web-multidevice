package services

import (
	domainNewsletter "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/newsletter"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/auth"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/whatsapp"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/validations"
	"github.com/gofiber/fiber/v2"
)

type newsletterService struct {
	Clients *map[string]*whatsapp.WhatsAppTenantClient
}

func NewNewsletterService(clients *map[string]*whatsapp.WhatsAppTenantClient) domainNewsletter.INewsletterService {
	return &newsletterService{
		Clients: clients,
	}
}

func (service newsletterService) Unfollow(c *fiber.Ctx, request domainNewsletter.UnfollowRequest) (err error) {

	authPayload, err := auth.AuthPayload(c)
	if err != nil {
		return err
	}

	tenantClient, err := whatsapp.GetWhatsappTenantClient(service.Clients, authPayload.User)
	if err != nil {
		return err
	}

	if err = validations.ValidateUnfollowNewsletter(c.UserContext(), request); err != nil {
		return err
	}

	JID, err := whatsapp.ValidateJidWithLogin(tenantClient.Conn, request.NewsletterID)
	if err != nil {
		return err
	}

	return tenantClient.Conn.UnfollowNewsletter(JID)
}
