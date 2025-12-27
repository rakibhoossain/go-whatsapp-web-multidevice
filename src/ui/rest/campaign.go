package rest

import (
	"io"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	domainCampaign "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/campaign"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/whatsapp"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
)

// Campaign handles campaign REST endpoints
type Campaign struct {
	Service domainCampaign.ICampaignUsecase
}

// InitRestCampaign registers all campaign routes
func InitRestCampaign(app fiber.Router, service domainCampaign.ICampaignUsecase) Campaign {
	rest := Campaign{Service: service}

	campaign := app.Group("/campaign")

	// Customers
	campaign.Get("/customers", rest.ListCustomers)
	campaign.Post("/customers", rest.CreateCustomer)
	campaign.Post("/customers/import", rest.ImportCustomers)
	campaign.Get("/customers/:id", rest.GetCustomer)
	campaign.Put("/customers/:id", rest.UpdateCustomer)
	campaign.Delete("/customers/:id", rest.DeleteCustomer)
	campaign.Post("/customers/:id/validate", rest.ValidateCustomer)

	// Groups
	campaign.Get("/groups", rest.ListGroups)
	campaign.Post("/groups", rest.CreateGroup)
	campaign.Get("/groups/:id", rest.GetGroup)
	campaign.Put("/groups/:id", rest.UpdateGroup)
	campaign.Delete("/groups/:id", rest.DeleteGroup)
	campaign.Post("/groups/:id/members", rest.AddGroupMembers)
	campaign.Delete("/groups/:id/members/:customerId", rest.RemoveGroupMember)

	// Templates
	campaign.Get("/templates", rest.ListTemplates)
	campaign.Post("/templates", rest.CreateTemplate)
	campaign.Get("/templates/:id", rest.GetTemplate)
	campaign.Put("/templates/:id", rest.UpdateTemplate)
	campaign.Delete("/templates/:id", rest.DeleteTemplate)
	campaign.Post("/templates/preview", rest.PreviewTemplate)

	// Campaigns
	campaign.Get("/campaigns", rest.ListCampaigns)
	campaign.Post("/campaigns", rest.CreateCampaign)
	campaign.Get("/campaigns/:id", rest.GetCampaign)
	campaign.Put("/campaigns/:id", rest.UpdateCampaign)
	campaign.Delete("/campaigns/:id", rest.DeleteCampaign)
	campaign.Post("/campaigns/:id/start", rest.StartCampaign)
	campaign.Post("/campaigns/:id/pause", rest.PauseCampaign)
	campaign.Get("/campaigns/:id/stats", rest.GetCampaignStats)

	// Short URL redirect (at app level, not under /campaign)
	app.Get("/s/:code", rest.ShortURLRedirect)

	return rest
}

// Helper to get device ID from context (set by middleware)
// Always use device.ID() (internal ID like "pantone") not JID (WhatsApp number)
// The internal ID remains stable before and after QR login
func getDeviceID(c *fiber.Ctx) string {
	if device := getDeviceFromCtx(c); device != nil {
		return device.ID()
	}
	return ""
}

// checkDeviceConnected checks if a WhatsApp device is connected and returns error response if not
func (h *Campaign) checkDeviceConnected(c *fiber.Ctx) (string, error) {
	deviceID := getDeviceID(c)
	if deviceID == "" {
		return "", c.Status(401).JSON(utils.ResponseData{
			Status:  401,
			Code:    "NOT_CONNECTED",
			Message: "Please connect your WhatsApp device first. Go to App menu and scan QR code.",
		})
	}
	return deviceID, nil
}

// ============================================================================
// Customer Endpoints
// ============================================================================

func (h *Campaign) ListCustomers(c *fiber.Ctx) error {
	deviceID, err := h.checkDeviceConnected(c)
	if err != nil {
		return err
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("page_size", "20"))

	result, err := h.Service.ListCustomers(c.UserContext(), deviceID, page, pageSize)
	if err != nil {
		return c.Status(500).JSON(utils.ResponseData{Status: 500, Code: "ERROR", Message: err.Error()})
	}

	return c.JSON(utils.ResponseData{Status: 200, Code: "SUCCESS", Message: "Customers retrieved", Results: result})
}

func (h *Campaign) CreateCustomer(c *fiber.Ctx) error {
	deviceID, err := h.checkDeviceConnected(c)
	if err != nil {
		return err
	}

	var req domainCampaign.CreateCustomerRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: "Invalid request body"})
	}
	req.DeviceID = deviceID

	customer, err := h.Service.CreateCustomer(c.UserContext(), req)
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: err.Error()})
	}

	return c.JSON(utils.ResponseData{Status: 200, Code: "SUCCESS", Message: "Customer created", Results: customer})
}

func (h *Campaign) ImportCustomers(c *fiber.Ctx) error {
	deviceID, err := h.checkDeviceConnected(c)
	if err != nil {
		return err
	}

	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: "CSV file is required"})
	}

	f, err := file.Open()
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: "Failed to open file"})
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: "Failed to read file"})
	}

	imported, errors, err := h.Service.ImportCustomersFromCSV(c.UserContext(), deviceID, data)
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: err.Error()})
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "Import completed",
		Results: fiber.Map{"imported": imported, "errors": errors},
	})
}

func (h *Campaign) GetCustomer(c *fiber.Ctx) error {
	deviceID, err := h.checkDeviceConnected(c)
	if err != nil {
		return err
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: "Invalid customer ID"})
	}

	customer, err := h.Service.GetCustomer(c.UserContext(), deviceID, id)
	if err != nil {
		return c.Status(500).JSON(utils.ResponseData{Status: 500, Code: "ERROR", Message: err.Error()})
	}
	if customer == nil {
		return c.Status(404).JSON(utils.ResponseData{Status: 404, Code: "NOT_FOUND", Message: "Customer not found"})
	}

	return c.JSON(utils.ResponseData{Status: 200, Code: "SUCCESS", Message: "Customer retrieved", Results: customer})
}

func (h *Campaign) UpdateCustomer(c *fiber.Ctx) error {
	deviceID, err := h.checkDeviceConnected(c)
	if err != nil {
		return err
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: "Invalid customer ID"})
	}

	var req domainCampaign.UpdateCustomerRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: "Invalid request body"})
	}
	req.DeviceID = deviceID
	req.ID = id

	customer, err := h.Service.UpdateCustomer(c.UserContext(), req)
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: err.Error()})
	}

	return c.JSON(utils.ResponseData{Status: 200, Code: "SUCCESS", Message: "Customer updated", Results: customer})
}

func (h *Campaign) DeleteCustomer(c *fiber.Ctx) error {
	deviceID, err := h.checkDeviceConnected(c)
	if err != nil {
		return err
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: "Invalid customer ID"})
	}

	if err := h.Service.DeleteCustomer(c.UserContext(), deviceID, id); err != nil {
		return c.Status(500).JSON(utils.ResponseData{Status: 500, Code: "ERROR", Message: err.Error()})
	}

	return c.JSON(utils.ResponseData{Status: 200, Code: "SUCCESS", Message: "Customer deleted"})
}

func (h *Campaign) ValidateCustomer(c *fiber.Ctx) error {
	deviceID, err := h.checkDeviceConnected(c)
	if err != nil {
		return err
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: "Invalid customer ID"})
	}

	// Pass device context so usecase can access WhatsApp client for validation
	ctx := whatsapp.ContextWithDevice(c.UserContext(), getDeviceFromCtx(c))
	if err := h.Service.ValidateCustomer(ctx, deviceID, id); err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: err.Error()})
	}

	return c.JSON(utils.ResponseData{Status: 200, Code: "SUCCESS", Message: "Customer validated"})
}

// ============================================================================
// Group Endpoints
// ============================================================================

func (h *Campaign) ListGroups(c *fiber.Ctx) error {
	deviceID, err := h.checkDeviceConnected(c)
	if err != nil {
		return err
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("page_size", "20"))

	result, err := h.Service.ListGroups(c.UserContext(), deviceID, page, pageSize)
	if err != nil {
		return c.Status(500).JSON(utils.ResponseData{Status: 500, Code: "ERROR", Message: err.Error()})
	}

	return c.JSON(utils.ResponseData{Status: 200, Code: "SUCCESS", Message: "Groups retrieved", Results: result})
}

func (h *Campaign) CreateGroup(c *fiber.Ctx) error {
	deviceID, err := h.checkDeviceConnected(c)
	if err != nil {
		return err
	}

	var req domainCampaign.CreateGroupRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: "Invalid request body"})
	}
	req.DeviceID = deviceID

	group, err := h.Service.CreateGroup(c.UserContext(), req)
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: err.Error()})
	}

	return c.JSON(utils.ResponseData{Status: 200, Code: "SUCCESS", Message: "Group created", Results: group})
}

func (h *Campaign) GetGroup(c *fiber.Ctx) error {
	deviceID, err := h.checkDeviceConnected(c)
	if err != nil {
		return err
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: "Invalid group ID"})
	}

	group, err := h.Service.GetGroup(c.UserContext(), deviceID, id)
	if err != nil {
		return c.Status(500).JSON(utils.ResponseData{Status: 500, Code: "ERROR", Message: err.Error()})
	}
	if group == nil {
		return c.Status(404).JSON(utils.ResponseData{Status: 404, Code: "NOT_FOUND", Message: "Group not found"})
	}

	return c.JSON(utils.ResponseData{Status: 200, Code: "SUCCESS", Message: "Group retrieved", Results: group})
}

func (h *Campaign) UpdateGroup(c *fiber.Ctx) error {
	deviceID, err := h.checkDeviceConnected(c)
	if err != nil {
		return err
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: "Invalid group ID"})
	}

	var req domainCampaign.UpdateGroupRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: "Invalid request body"})
	}
	req.DeviceID = deviceID
	req.ID = id

	group, err := h.Service.UpdateGroup(c.UserContext(), req)
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: err.Error()})
	}

	return c.JSON(utils.ResponseData{Status: 200, Code: "SUCCESS", Message: "Group updated", Results: group})
}

func (h *Campaign) DeleteGroup(c *fiber.Ctx) error {
	deviceID, err := h.checkDeviceConnected(c)
	if err != nil {
		return err
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: "Invalid group ID"})
	}

	if err := h.Service.DeleteGroup(c.UserContext(), deviceID, id); err != nil {
		return c.Status(500).JSON(utils.ResponseData{Status: 500, Code: "ERROR", Message: err.Error()})
	}

	return c.JSON(utils.ResponseData{Status: 200, Code: "SUCCESS", Message: "Group deleted"})
}

func (h *Campaign) AddGroupMembers(c *fiber.Ctx) error {
	deviceID, err := h.checkDeviceConnected(c)
	if err != nil {
		return err
	}

	groupID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: "Invalid group ID"})
	}

	var req domainCampaign.AddCustomersToGroupRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: "Invalid request body"})
	}

	if err := h.Service.AddCustomersToGroup(c.UserContext(), deviceID, groupID, req.CustomerIDs); err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: err.Error()})
	}

	return c.JSON(utils.ResponseData{Status: 200, Code: "SUCCESS", Message: "Members added to group"})
}

func (h *Campaign) RemoveGroupMember(c *fiber.Ctx) error {
	deviceID, err := h.checkDeviceConnected(c)
	if err != nil {
		return err
	}

	groupID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: "Invalid group ID"})
	}

	customerID, err := uuid.Parse(c.Params("customerId"))
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: "Invalid customer ID"})
	}

	if err := h.Service.RemoveCustomerFromGroup(c.UserContext(), deviceID, groupID, customerID); err != nil {
		return c.Status(500).JSON(utils.ResponseData{Status: 500, Code: "ERROR", Message: err.Error()})
	}

	return c.JSON(utils.ResponseData{Status: 200, Code: "SUCCESS", Message: "Member removed from group"})
}

// ============================================================================
// Template Endpoints
// ============================================================================

func (h *Campaign) ListTemplates(c *fiber.Ctx) error {
	deviceID, err := h.checkDeviceConnected(c)
	if err != nil {
		return err
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("page_size", "20"))

	result, err := h.Service.ListTemplates(c.UserContext(), deviceID, page, pageSize)
	if err != nil {
		return c.Status(500).JSON(utils.ResponseData{Status: 500, Code: "ERROR", Message: err.Error()})
	}

	return c.JSON(utils.ResponseData{Status: 200, Code: "SUCCESS", Message: "Templates retrieved", Results: result})
}

func (h *Campaign) CreateTemplate(c *fiber.Ctx) error {
	deviceID, err := h.checkDeviceConnected(c)
	if err != nil {
		return err
	}

	var req domainCampaign.CreateTemplateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: "Invalid request body"})
	}
	req.DeviceID = deviceID

	template, err := h.Service.CreateTemplate(c.UserContext(), req)
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: err.Error()})
	}

	return c.JSON(utils.ResponseData{Status: 200, Code: "SUCCESS", Message: "Template created", Results: template})
}

func (h *Campaign) GetTemplate(c *fiber.Ctx) error {
	deviceID, err := h.checkDeviceConnected(c)
	if err != nil {
		return err
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: "Invalid template ID"})
	}

	template, err := h.Service.GetTemplate(c.UserContext(), deviceID, id)
	if err != nil {
		return c.Status(500).JSON(utils.ResponseData{Status: 500, Code: "ERROR", Message: err.Error()})
	}
	if template == nil {
		return c.Status(404).JSON(utils.ResponseData{Status: 404, Code: "NOT_FOUND", Message: "Template not found"})
	}

	return c.JSON(utils.ResponseData{Status: 200, Code: "SUCCESS", Message: "Template retrieved", Results: template})
}

func (h *Campaign) UpdateTemplate(c *fiber.Ctx) error {
	deviceID, err := h.checkDeviceConnected(c)
	if err != nil {
		return err
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: "Invalid template ID"})
	}

	var req domainCampaign.UpdateTemplateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: "Invalid request body"})
	}
	req.DeviceID = deviceID
	req.ID = id

	template, err := h.Service.UpdateTemplate(c.UserContext(), req)
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: err.Error()})
	}

	return c.JSON(utils.ResponseData{Status: 200, Code: "SUCCESS", Message: "Template updated", Results: template})
}

func (h *Campaign) DeleteTemplate(c *fiber.Ctx) error {
	deviceID, err := h.checkDeviceConnected(c)
	if err != nil {
		return err
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: "Invalid template ID"})
	}

	if err := h.Service.DeleteTemplate(c.UserContext(), deviceID, id); err != nil {
		return c.Status(500).JSON(utils.ResponseData{Status: 500, Code: "ERROR", Message: err.Error()})
	}

	return c.JSON(utils.ResponseData{Status: 200, Code: "SUCCESS", Message: "Template deleted"})
}

func (h *Campaign) PreviewTemplate(c *fiber.Ctx) error {
	var req struct {
		Content  string                   `json:"content"`
		Customer *domainCampaign.Customer `json:"customer"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: "Invalid request body"})
	}

	preview := h.Service.PreviewTemplate(c.UserContext(), req.Content, req.Customer)

	return c.JSON(utils.ResponseData{Status: 200, Code: "SUCCESS", Message: "Preview generated", Results: fiber.Map{"preview": preview}})
}

// ============================================================================
// Campaign Endpoints
// ============================================================================

func (h *Campaign) ListCampaigns(c *fiber.Ctx) error {
	deviceID, err := h.checkDeviceConnected(c)
	if err != nil {
		return err
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("page_size", "20"))

	result, err := h.Service.ListCampaigns(c.UserContext(), deviceID, page, pageSize)
	if err != nil {
		return c.Status(500).JSON(utils.ResponseData{Status: 500, Code: "ERROR", Message: err.Error()})
	}

	return c.JSON(utils.ResponseData{Status: 200, Code: "SUCCESS", Message: "Campaigns retrieved", Results: result})
}

func (h *Campaign) CreateCampaign(c *fiber.Ctx) error {
	deviceID, err := h.checkDeviceConnected(c)
	if err != nil {
		return err
	}

	var req domainCampaign.CreateCampaignRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: "Invalid request body"})
	}
	req.DeviceID = deviceID

	campaign, err := h.Service.CreateCampaign(c.UserContext(), req)
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: err.Error()})
	}

	return c.JSON(utils.ResponseData{Status: 200, Code: "SUCCESS", Message: "Campaign created", Results: campaign})
}

func (h *Campaign) GetCampaign(c *fiber.Ctx) error {
	deviceID, err := h.checkDeviceConnected(c)
	if err != nil {
		return err
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: "Invalid campaign ID"})
	}

	campaign, err := h.Service.GetCampaign(c.UserContext(), deviceID, id)
	if err != nil {
		return c.Status(500).JSON(utils.ResponseData{Status: 500, Code: "ERROR", Message: err.Error()})
	}
	if campaign == nil {
		return c.Status(404).JSON(utils.ResponseData{Status: 404, Code: "NOT_FOUND", Message: "Campaign not found"})
	}

	return c.JSON(utils.ResponseData{Status: 200, Code: "SUCCESS", Message: "Campaign retrieved", Results: campaign})
}

func (h *Campaign) UpdateCampaign(c *fiber.Ctx) error {
	deviceID, err := h.checkDeviceConnected(c)
	if err != nil {
		return err
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: "Invalid campaign ID"})
	}

	var req domainCampaign.UpdateCampaignRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: "Invalid request body"})
	}
	req.DeviceID = deviceID
	req.ID = id

	campaign, err := h.Service.UpdateCampaign(c.UserContext(), req)
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: err.Error()})
	}

	return c.JSON(utils.ResponseData{Status: 200, Code: "SUCCESS", Message: "Campaign updated", Results: campaign})
}

func (h *Campaign) DeleteCampaign(c *fiber.Ctx) error {
	deviceID, err := h.checkDeviceConnected(c)
	if err != nil {
		return err
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: "Invalid campaign ID"})
	}

	if err := h.Service.DeleteCampaign(c.UserContext(), deviceID, id); err != nil {
		return c.Status(500).JSON(utils.ResponseData{Status: 500, Code: "ERROR", Message: err.Error()})
	}

	return c.JSON(utils.ResponseData{Status: 200, Code: "SUCCESS", Message: "Campaign deleted"})
}

func (h *Campaign) StartCampaign(c *fiber.Ctx) error {
	deviceID, err := h.checkDeviceConnected(c)
	if err != nil {
		return err
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: "Invalid campaign ID"})
	}

	if err := h.Service.StartCampaign(c.UserContext(), deviceID, id); err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: err.Error()})
	}

	return c.JSON(utils.ResponseData{Status: 200, Code: "SUCCESS", Message: "Campaign started"})
}

func (h *Campaign) PauseCampaign(c *fiber.Ctx) error {
	deviceID, err := h.checkDeviceConnected(c)
	if err != nil {
		return err
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: "Invalid campaign ID"})
	}

	if err := h.Service.PauseCampaign(c.UserContext(), deviceID, id); err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: err.Error()})
	}

	return c.JSON(utils.ResponseData{Status: 200, Code: "SUCCESS", Message: "Campaign paused"})
}

func (h *Campaign) GetCampaignStats(c *fiber.Ctx) error {
	deviceID, err := h.checkDeviceConnected(c)
	if err != nil {
		return err
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: "Invalid campaign ID"})
	}

	stats, err := h.Service.GetCampaignStats(c.UserContext(), deviceID, id)
	if err != nil {
		return c.Status(400).JSON(utils.ResponseData{Status: 400, Code: "ERROR", Message: err.Error()})
	}

	return c.JSON(utils.ResponseData{Status: 200, Code: "SUCCESS", Message: "Stats retrieved", Results: stats})
}

// ============================================================================
// Short URL Redirect
// ============================================================================

func (h *Campaign) ShortURLRedirect(c *fiber.Ctx) error {
	code := c.Params("code")
	if code == "" {
		return c.Status(400).SendString("Invalid short URL")
	}

	originalURL, err := h.Service.HandleShortURLRedirect(c.UserContext(), code)
	if err != nil {
		return c.Status(404).SendString("URL not found")
	}

	return c.Redirect(originalURL, 302)
}
