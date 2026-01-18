package usecase

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math/big"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	domainCampaign "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/campaign"
	domainSend "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/send"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/whatsapp"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/validations"
)

// CampaignService implements ICampaignUsecase
type CampaignService struct {
	repo        domainCampaign.ICampaignRepository
	sendService domainSend.ISendUsecase
	basePath    string

	// Worker control
	workerCtx    context.Context
	workerCancel context.CancelFunc
	workerWg     sync.WaitGroup
	workerMu     sync.Mutex
}

// NewCampaignService creates a new campaign service
func NewCampaignService(repo domainCampaign.ICampaignRepository, sendService domainSend.ISendUsecase, basePath string) *CampaignService {
	return &CampaignService{
		repo:        repo,
		sendService: sendService,
		basePath:    basePath,
	}
}

// ============================================================================
// Customer Management
// ============================================================================

func (s *CampaignService) CreateCustomer(ctx context.Context, req domainCampaign.CreateCustomerRequest) (*domainCampaign.Customer, error) {
	// Validate phone starts with + and is in international format
	if !strings.HasPrefix(req.Phone, "+") {
		return nil, errors.New("phone must start with +")
	}
	// Validate phone format (no leading 0 after country code)
	phone := strings.TrimPrefix(req.Phone, "+")
	if len(phone) > 0 && phone[0] == '0' {
		return nil, errors.New("phone number must be in international format (should not start with 0 after +)")
	}

	// Check if customer already exists
	existing, err := s.repo.GetCustomerByPhone(ctx, req.DeviceID, req.Phone)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, errors.New("customer with this phone already exists")
	}

	customer := &domainCampaign.Customer{
		DeviceID:       req.DeviceID,
		Phone:          req.Phone,
		FullName:       req.FullName,
		Company:        req.Company,
		Country:        req.Country,
		Gender:         req.Gender,
		BirthYear:      req.BirthYear,
		PhoneValid:     domainCampaign.ValidationStatusPending,
		WhatsAppExists: domainCampaign.ValidationStatusPending,
	}

	if err := s.repo.CreateCustomer(ctx, customer); err != nil {
		return nil, err
	}

	logrus.WithFields(logrus.Fields{
		"device_id": req.DeviceID,
		"phone":     req.Phone,
		"id":        customer.ID,
	}).Info("Campaign: Customer created")

	return customer, nil
}

func (s *CampaignService) ImportCustomersFromCSV(ctx context.Context, deviceID string, csvData []byte) (imported int, errorsOut []string, err error) {
	reader := csv.NewReader(bytes.NewReader(csvData))

	// Read header
	header, err := reader.Read()
	if err != nil {
		return 0, nil, fmt.Errorf("failed to read CSV header: %w", err)
	}

	// Map column indices
	colMap := make(map[string]int)
	for i, col := range header {
		colMap[strings.ToLower(strings.TrimSpace(col))] = i
	}

	phoneIdx, ok := colMap["phone"]
	if !ok {
		return 0, nil, errors.New("CSV must have 'phone' column")
	}

	nameIdx := colMap["name"]
	fullNameIdx := colMap["full_name"]
	countryIdx := colMap["country"]
	genderIdx := colMap["gender"]
	birthYearIdx := colMap["birth_year"]

	// If 'name' column exists but not 'full_name', use 'name'
	if nameIdx > 0 && fullNameIdx == 0 {
		fullNameIdx = nameIdx
	}

	var customers []*domainCampaign.Customer
	lineNum := 1 // Start at 1 for header

	for {
		lineNum++
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			errorsOut = append(errorsOut, fmt.Sprintf("Line %d: %v", lineNum, err))
			continue
		}

		// Get phone
		if phoneIdx >= len(record) {
			errorsOut = append(errorsOut, fmt.Sprintf("Line %d: missing phone column", lineNum))
			continue
		}
		phone := strings.TrimSpace(record[phoneIdx])
		if phone == "" {
			errorsOut = append(errorsOut, fmt.Sprintf("Line %d: empty phone", lineNum))
			continue
		}
		if !strings.HasPrefix(phone, "+") {
			phone = "+" + phone
		}

		customer := &domainCampaign.Customer{
			DeviceID:       deviceID,
			Phone:          phone,
			PhoneValid:     domainCampaign.ValidationStatusPending,
			WhatsAppExists: domainCampaign.ValidationStatusPending,
		}

		// Optional fields
		if fullNameIdx > 0 && fullNameIdx < len(record) {
			name := strings.TrimSpace(record[fullNameIdx])
			if name != "" {
				customer.FullName = &name
			}
		}
		if countryIdx > 0 && countryIdx < len(record) {
			country := strings.TrimSpace(record[countryIdx])
			if country != "" {
				customer.Country = &country
			}
		}
		if genderIdx > 0 && genderIdx < len(record) {
			gender := strings.TrimSpace(record[genderIdx])
			if gender != "" {
				customer.Gender = &gender
			}
		}
		if birthYearIdx > 0 && birthYearIdx < len(record) {
			yearStr := strings.TrimSpace(record[birthYearIdx])
			if yearStr != "" {
				var year int
				if _, err := fmt.Sscanf(yearStr, "%d", &year); err == nil && year > 1900 && year < 2100 {
					customer.BirthYear = &year
				}
			}
		}

		customers = append(customers, customer)
	}

	if len(customers) == 0 {
		return 0, errorsOut, errors.New("no valid customers found in CSV")
	}

	imported, err = s.repo.BulkCreateCustomers(ctx, customers)
	return imported, errorsOut, err
}

func (s *CampaignService) GetCustomer(ctx context.Context, deviceID string, id uuid.UUID) (*domainCampaign.Customer, error) {
	return s.repo.GetCustomer(ctx, deviceID, id)
}

func (s *CampaignService) ListCustomers(ctx context.Context, deviceID string, page, pageSize int) (*domainCampaign.CustomerListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize
	customers, total, err := s.repo.ListCustomers(ctx, deviceID, pageSize, offset)
	if err != nil {
		return nil, err
	}

	totalPages := (total + pageSize - 1) / pageSize

	return &domainCampaign.CustomerListResponse{
		Customers:  customers,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

func (s *CampaignService) UpdateCustomer(ctx context.Context, req domainCampaign.UpdateCustomerRequest) (*domainCampaign.Customer, error) {
	if !strings.HasPrefix(req.Phone, "+") {
		return nil, errors.New("phone must start with +")
	}

	customer, err := s.repo.GetCustomer(ctx, req.DeviceID, req.ID)
	if err != nil {
		return nil, err
	}
	if customer == nil {
		return nil, errors.New("customer not found")
	}

	// If phone changed, reset validation status
	phoneChanged := customer.Phone != req.Phone

	customer.Phone = req.Phone
	customer.FullName = req.FullName
	customer.Company = req.Company
	customer.Country = req.Country
	customer.Gender = req.Gender
	customer.BirthYear = req.BirthYear

	if phoneChanged {
		customer.PhoneValid = domainCampaign.ValidationStatusPending
		customer.WhatsAppExists = domainCampaign.ValidationStatusPending
	}

	if err := s.repo.UpdateCustomer(ctx, customer); err != nil {
		return nil, err
	}

	return customer, nil
}

func (s *CampaignService) DeleteCustomer(ctx context.Context, deviceID string, id uuid.UUID) error {
	return s.repo.DeleteCustomer(ctx, deviceID, id)
}

// ============================================================================
// Group Management
// ============================================================================

func (s *CampaignService) CreateGroup(ctx context.Context, req domainCampaign.CreateGroupRequest) (*domainCampaign.Group, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, errors.New("group name is required")
	}

	group := &domainCampaign.Group{
		DeviceID:    req.DeviceID,
		Name:        req.Name,
		Description: req.Description,
	}

	if err := s.repo.CreateGroup(ctx, group); err != nil {
		return nil, err
	}

	return group, nil
}

func (s *CampaignService) GetGroup(ctx context.Context, deviceID string, id uuid.UUID) (*domainCampaign.Group, error) {
	group, err := s.repo.GetGroup(ctx, deviceID, id)
	if err != nil {
		return nil, err
	}
	if group == nil {
		return nil, nil
	}

	// Load customers
	customers, err := s.repo.GetGroupCustomers(ctx, id)
	if err != nil {
		return nil, err
	}
	group.Customers = make([]domainCampaign.Customer, len(customers))
	for i, c := range customers {
		group.Customers[i] = *c
	}

	return group, nil
}

func (s *CampaignService) ListGroups(ctx context.Context, deviceID string, page, pageSize int) (*domainCampaign.GroupListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize
	groups, total, err := s.repo.ListGroups(ctx, deviceID, pageSize, offset)
	if err != nil {
		return nil, err
	}

	totalPages := (total + pageSize - 1) / pageSize
	if total == 0 {
		totalPages = 0
	}

	return &domainCampaign.GroupListResponse{
		Groups:     groups,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

func (s *CampaignService) UpdateGroup(ctx context.Context, req domainCampaign.UpdateGroupRequest) (*domainCampaign.Group, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, errors.New("group name is required")
	}

	group, err := s.repo.GetGroup(ctx, req.DeviceID, req.ID)
	if err != nil {
		return nil, err
	}
	if group == nil {
		return nil, errors.New("group not found")
	}

	group.Name = req.Name
	group.Description = req.Description

	if err := s.repo.UpdateGroup(ctx, group); err != nil {
		return nil, err
	}

	return group, nil
}

func (s *CampaignService) DeleteGroup(ctx context.Context, deviceID string, id uuid.UUID) error {
	return s.repo.DeleteGroup(ctx, deviceID, id)
}

func (s *CampaignService) AddCustomersToGroup(ctx context.Context, deviceID string, groupID uuid.UUID, customerIDs []uuid.UUID) error {
	// Verify group exists
	group, err := s.repo.GetGroup(ctx, deviceID, groupID)
	if err != nil {
		return err
	}
	if group == nil {
		return errors.New("group not found")
	}

	return s.repo.AddCustomersToGroup(ctx, groupID, customerIDs)
}

func (s *CampaignService) RemoveCustomerFromGroup(ctx context.Context, deviceID string, groupID uuid.UUID, customerID uuid.UUID) error {
	return s.repo.RemoveCustomerFromGroup(ctx, groupID, customerID)
}

// ============================================================================
// Template Management
// ============================================================================

func (s *CampaignService) CreateTemplate(ctx context.Context, req domainCampaign.CreateTemplateRequest) (*domainCampaign.Template, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, errors.New("template name is required")
	}
	if strings.TrimSpace(req.Content) == "" {
		return nil, errors.New("template content is required")
	}

	template := &domainCampaign.Template{
		DeviceID: req.DeviceID,
		Name:     req.Name,
		Content:  req.Content,
	}

	if err := s.repo.CreateTemplate(ctx, template); err != nil {
		return nil, err
	}

	return template, nil
}

func (s *CampaignService) GetTemplate(ctx context.Context, deviceID string, id uuid.UUID) (*domainCampaign.Template, error) {
	return s.repo.GetTemplate(ctx, deviceID, id)
}

func (s *CampaignService) ListTemplates(ctx context.Context, deviceID string, page, pageSize int) (*domainCampaign.TemplateListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize
	templates, total, err := s.repo.ListTemplates(ctx, deviceID, pageSize, offset)
	if err != nil {
		return nil, err
	}

	totalPages := (total + pageSize - 1) / pageSize
	if total == 0 {
		totalPages = 0
	}

	return &domainCampaign.TemplateListResponse{
		Templates:  templates,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

func (s *CampaignService) UpdateTemplate(ctx context.Context, req domainCampaign.UpdateTemplateRequest) (*domainCampaign.Template, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, errors.New("template name is required")
	}
	if strings.TrimSpace(req.Content) == "" {
		return nil, errors.New("template content is required")
	}

	template, err := s.repo.GetTemplate(ctx, req.DeviceID, req.ID)
	if err != nil {
		return nil, err
	}
	if template == nil {
		return nil, errors.New("template not found")
	}

	template.Name = req.Name
	template.Content = req.Content

	if err := s.repo.UpdateTemplate(ctx, template); err != nil {
		return nil, err
	}

	return template, nil
}

func (s *CampaignService) DeleteTemplate(ctx context.Context, deviceID string, id uuid.UUID) error {
	return s.repo.DeleteTemplate(ctx, deviceID, id)
}

func (s *CampaignService) PreviewTemplate(ctx context.Context, content string, customer *domainCampaign.Customer) string {
	result := content

	// Replace placeholders
	if customer != nil {
		if customer.FullName != nil {
			result = strings.ReplaceAll(result, "[NAME]", *customer.FullName)
		} else {
			result = strings.ReplaceAll(result, "[NAME]", "")
		}
		result = strings.ReplaceAll(result, "[PHONE]", customer.Phone)
		if customer.Country != nil {
			result = strings.ReplaceAll(result, "[COUNTRY]", *customer.Country)
		} else {
			result = strings.ReplaceAll(result, "[COUNTRY]", "")
		}
		if customer.Company != nil {
			result = strings.ReplaceAll(result, "[COMPANY]", *customer.Company)
		} else {
			result = strings.ReplaceAll(result, "[COMPANY]", "")
		}
	}

	return result
}

// ============================================================================
// Campaign Management
// ============================================================================

func (s *CampaignService) CreateCampaign(ctx context.Context, req domainCampaign.CreateCampaignRequest) (*domainCampaign.Campaign, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, errors.New("campaign name is required")
	}

	// Verify template exists
	template, err := s.repo.GetTemplate(ctx, req.DeviceID, req.TemplateID)
	if err != nil {
		return nil, err
	}
	if template == nil {
		return nil, errors.New("template not found")
	}

	campaign := &domainCampaign.Campaign{
		DeviceID:    req.DeviceID,
		Name:        req.Name,
		TemplateID:  req.TemplateID,
		Status:      domainCampaign.CampaignStatusDraft,
		ScheduledAt: req.ScheduledAt,
	}

	if err := s.repo.CreateCampaign(ctx, campaign); err != nil {
		return nil, err
	}

	// Set targets
	if len(req.CustomerIDs) > 0 || len(req.GroupIDs) > 0 {
		if err := s.repo.SetCampaignTargets(ctx, campaign.ID, req.CustomerIDs, req.GroupIDs); err != nil {
			return nil, err
		}
	}

	return campaign, nil
}

func (s *CampaignService) GetCampaign(ctx context.Context, deviceID string, id uuid.UUID) (*domainCampaign.Campaign, error) {
	campaign, err := s.repo.GetCampaign(ctx, deviceID, id)
	if err != nil {
		return nil, err
	}
	if campaign == nil {
		return nil, nil
	}

	// Load template
	template, err := s.repo.GetTemplate(ctx, deviceID, campaign.TemplateID)
	if err == nil {
		campaign.Template = template
	}

	// Load stats
	stats, err := s.repo.GetCampaignStats(ctx, id)
	if err == nil {
		campaign.Stats = stats
	}

	// Load target IDs for editing
	customerIDs, groupIDs, err := s.repo.GetCampaignTargetIDs(ctx, id)
	if err == nil {
		campaign.CustomerIDs = customerIDs
		campaign.GroupIDs = groupIDs
	}

	return campaign, nil
}

func (s *CampaignService) ListCampaigns(ctx context.Context, deviceID string, page, pageSize int) (*domainCampaign.CampaignListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize
	campaigns, total, err := s.repo.ListCampaigns(ctx, deviceID, pageSize, offset)
	if err != nil {
		return nil, err
	}

	// Load stats for each campaign
	for _, campaign := range campaigns {
		stats, err := s.repo.GetCampaignStats(ctx, campaign.ID)
		if err == nil {
			campaign.Stats = stats
		}
	}

	totalPages := (total + pageSize - 1) / pageSize
	if total == 0 {
		totalPages = 0
	}

	return &domainCampaign.CampaignListResponse{
		Campaigns:  campaigns,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

func (s *CampaignService) UpdateCampaign(ctx context.Context, req domainCampaign.UpdateCampaignRequest) (*domainCampaign.Campaign, error) {
	campaign, err := s.repo.GetCampaign(ctx, req.DeviceID, req.ID)
	if err != nil {
		return nil, err
	}
	if campaign == nil {
		return nil, errors.New("campaign not found")
	}

	if campaign.Status == domainCampaign.CampaignStatusRunning {
		return nil, errors.New("cannot update running campaign, pause it first")
	}

	campaign.Name = req.Name
	campaign.TemplateID = req.TemplateID
	campaign.ScheduledAt = req.ScheduledAt

	if err := s.repo.UpdateCampaign(ctx, campaign); err != nil {
		return nil, err
	}

	// Update targets
	if err := s.repo.SetCampaignTargets(ctx, campaign.ID, req.CustomerIDs, req.GroupIDs); err != nil {
		return nil, err
	}

	return campaign, nil
}

func (s *CampaignService) DeleteCampaign(ctx context.Context, deviceID string, id uuid.UUID) error {
	campaign, err := s.repo.GetCampaign(ctx, deviceID, id)
	if err != nil {
		return err
	}
	if campaign == nil {
		return errors.New("campaign not found")
	}
	if campaign.Status == domainCampaign.CampaignStatusRunning {
		return errors.New("cannot delete running campaign, pause it first")
	}

	return s.repo.DeleteCampaign(ctx, deviceID, id)
}

func (s *CampaignService) StartCampaign(ctx context.Context, deviceID string, id uuid.UUID) error {
	logrus.WithFields(logrus.Fields{
		"campaign_id": id,
		"device_id":   deviceID,
	}).Info("Campaign: Starting campaign...")

	campaign, err := s.repo.GetCampaign(ctx, deviceID, id)
	if err != nil {
		return err
	}
	if campaign == nil {
		return errors.New("campaign not found")
	}

	logrus.WithFields(logrus.Fields{
		"campaign_name": campaign.Name,
		"status":        campaign.Status,
	}).Info("Campaign: Found campaign")

	if campaign.Status != domainCampaign.CampaignStatusDraft && campaign.Status != domainCampaign.CampaignStatusPaused {
		return errors.New("campaign is already running")
	}

	// Get template
	template, err := s.repo.GetTemplate(ctx, deviceID, campaign.TemplateID)
	if err != nil {
		return err
	}
	if template == nil {
		return errors.New("campaign template not found")
	}

	logrus.WithField("template_name", template.Name).Info("Campaign: Using template")

	// Get target customers
	customers, err := s.repo.GetCampaignTargetCustomers(ctx, id)
	if err != nil {
		return err
	}
	if len(customers) == 0 {
		return errors.New("no target customers for campaign")
	}

	logrus.WithField("target_customers", len(customers)).Info("Campaign: Found target customers")

	// Get customer groups for [GROUP] placeholder
	customerGroups := make(map[uuid.UUID]string)
	for _, customer := range customers {
		groups, err := s.repo.GetCustomerGroups(ctx, customer.ID)
		if err == nil && len(groups) > 0 {
			var groupNames []string
			for _, g := range groups {
				groupNames = append(groupNames, g.Name)
			}
			customerGroups[customer.ID] = strings.Join(groupNames, ", ")
		}
	}

	// Generate queue items
	var queueItems []*domainCampaign.QueueItem
	skippedAlreadyQueued := 0
	for _, customer := range customers {
		// Check if already queued
		queued, err := s.repo.IsMessageQueued(ctx, id, customer.ID)
		if err != nil {
			logrus.Warnf("Failed to check if message queued: %v", err)
			continue
		}
		if queued {
			skippedAlreadyQueued++
			continue
		}

		// Process template
		message := s.PreviewTemplate(ctx, template.Content, customer)

		// Replace [GROUP] placeholder
		if groupName, ok := customerGroups[customer.ID]; ok {
			message = strings.ReplaceAll(message, "[GROUP]", groupName)
		} else {
			message = strings.ReplaceAll(message, "[GROUP]", "")
		}

		// Shorten URLs
		message, err = s.ShortenURLsInText(ctx, deviceID, message)
		if err != nil {
			logrus.Warnf("Failed to shorten URLs: %v", err)
		}

		queueItems = append(queueItems, &domainCampaign.QueueItem{
			CampaignID: id,
			CustomerID: customer.ID,
			DeviceID:   deviceID,
			Phone:      customer.Phone,
			Message:    message,
		})
	}

	logrus.WithFields(logrus.Fields{
		"new_messages":    len(queueItems),
		"already_queued":  skippedAlreadyQueued,
		"total_customers": len(customers),
	}).Info("Campaign: Prepared messages for queue")

	// Enqueue messages
	if len(queueItems) > 0 {
		if err := s.repo.EnqueueMessages(ctx, queueItems); err != nil {
			return fmt.Errorf("failed to enqueue messages: %w", err)
		}
		logrus.WithField("count", len(queueItems)).Info("Campaign: Messages enqueued to database")
	} else {
		logrus.Info("Campaign: No new messages to queue (all already queued)")
	}

	// Update campaign status
	now := time.Now()
	campaign.Status = domainCampaign.CampaignStatusRunning
	if campaign.StartedAt == nil {
		campaign.StartedAt = &now
	}

	if err := s.repo.UpdateCampaign(ctx, campaign); err != nil {
		return err
	}

	logrus.WithFields(logrus.Fields{
		"campaign":    campaign.Name,
		"status":      campaign.Status,
		"queued_msgs": len(queueItems),
	}).Info("Campaign: Started successfully - queue worker will process messages")

	return nil
}

func (s *CampaignService) PauseCampaign(ctx context.Context, deviceID string, id uuid.UUID) error {
	campaign, err := s.repo.GetCampaign(ctx, deviceID, id)
	if err != nil {
		return err
	}
	if campaign == nil {
		return errors.New("campaign not found")
	}

	if campaign.Status != domainCampaign.CampaignStatusRunning {
		return errors.New("campaign is not running")
	}

	campaign.Status = domainCampaign.CampaignStatusPaused

	return s.repo.UpdateCampaign(ctx, campaign)
}

func (s *CampaignService) GetCampaignStats(ctx context.Context, deviceID string, id uuid.UUID) (*domainCampaign.CampaignStats, error) {
	campaign, err := s.repo.GetCampaign(ctx, deviceID, id)
	if err != nil {
		return nil, err
	}
	if campaign == nil {
		return nil, errors.New("campaign not found")
	}

	return s.repo.GetCampaignStats(ctx, id)
}

// ============================================================================
// Short URL
// ============================================================================

var urlRegex = regexp.MustCompile(`https?://[^\s]+`)

func (s *CampaignService) ShortenURLsInText(ctx context.Context, deviceID string, text string) (string, error) {
	if config.CampaignShortURLBase == "" {
		return text, nil // URL shortening disabled
	}

	urls := urlRegex.FindAllString(text, -1)
	if len(urls) == 0 {
		return text, nil
	}

	for _, url := range urls {
		code := s.generateShortCode()
		shortURL := &domainCampaign.ShortURL{
			DeviceID:    deviceID,
			Code:        code,
			OriginalURL: url,
		}

		if err := s.repo.CreateShortURL(ctx, shortURL); err != nil {
			logrus.Warnf("Failed to create short URL: %v", err)
			continue
		}

		shortLink := fmt.Sprintf("%s/s/%s", strings.TrimSuffix(config.CampaignShortURLBase, "/"), code)
		text = strings.Replace(text, url, shortLink, 1)
	}

	return text, nil
}

func (s *CampaignService) HandleShortURLRedirect(ctx context.Context, code string) (string, error) {
	shortURL, err := s.repo.GetShortURLByCode(ctx, code)
	if err != nil {
		return "", err
	}
	if shortURL == nil {
		return "", errors.New("short URL not found")
	}

	// Increment click count
	_ = s.repo.IncrementShortURLClicks(ctx, code)

	return shortURL.OriginalURL, nil
}

func (s *CampaignService) generateShortCode() string {
	b := make([]byte, 6)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)[:8]
}

// ============================================================================
// Queue Worker
// ============================================================================

func (s *CampaignService) StartQueueWorker(ctx context.Context) {
	s.workerMu.Lock()
	if s.workerCtx != nil {
		s.workerMu.Unlock()
		logrus.Info("Campaign queue worker already running")
		return // Already running
	}
	s.workerCtx, s.workerCancel = context.WithCancel(ctx)
	s.workerMu.Unlock()

	s.workerWg.Add(1)
	go s.runQueueWorker()

	logrus.Info("Campaign: Queue worker started")
}

func (s *CampaignService) StopQueueWorker() {
	s.workerMu.Lock()
	if s.workerCancel != nil {
		s.workerCancel()
	}
	s.workerMu.Unlock()

	s.workerWg.Wait()
	logrus.Info("Campaign: Queue worker stopped")
}

func (s *CampaignService) runQueueWorker() {
	defer s.workerWg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	logrus.Info("Campaign: Queue worker loop started")

	for {
		select {
		case <-s.workerCtx.Done():
			logrus.Info("Campaign: Queue worker context done, exiting")
			return
		case <-ticker.C:
			s.processQueueBatch()
		}
	}
}

func (s *CampaignService) processQueueBatch() {
	ctx := s.workerCtx

	// Get all devices that have pending messages
	deviceIDs, err := s.repo.GetActiveDeviceIDs(ctx)
	if err != nil {
		logrus.Errorf("Campaign: Failed to get active device IDs: %v", err)
		return
	}

	if len(deviceIDs) == 0 {
		// No devices with pending messages - this is normal
		return
	}

	logrus.WithField("devices", deviceIDs).Info("Campaign: Processing queue for devices")

	for _, deviceID := range deviceIDs {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Get one pending message for this device
		messages, err := s.repo.GetPendingMessages(ctx, deviceID, 1)
		if err != nil {
			logrus.Errorf("Campaign: Failed to get pending messages for device %s: %v", deviceID, err)
			continue
		}

		if len(messages) == 0 {
			logrus.WithField("device_id", deviceID).Debug("Campaign: No pending messages for device")
			continue
		}

		logrus.WithFields(logrus.Fields{
			"device_id": deviceID,
			"count":     len(messages),
		}).Info("Campaign: Found pending messages")

		for _, msg := range messages {
			select {
			case <-ctx.Done():
				return
			default:
			}

			s.sendMessage(ctx, msg)

			// Random delay between 30 seconds and 5 minutes
			delay := s.randomDelay(config.CampaignMinDelay, config.CampaignMaxDelay)
			logrus.WithFields(logrus.Fields{
				"device_id": deviceID,
				"delay":     delay,
			}).Info("Campaign: Waiting before next message")

			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
		}
	}
}

func (s *CampaignService) sendMessage(ctx context.Context, msg *domainCampaign.QueueItem) {
	logrus.WithFields(logrus.Fields{
		"message_id":  msg.ID,
		"campaign_id": msg.CampaignID,
		"phone":       msg.Phone,
		"device_id":   msg.DeviceID,
	}).Info("Campaign: Sending message")

	// Mark as sending
	if err := s.repo.UpdateMessageStatus(ctx, msg.ID, domainCampaign.MessageStatusSending, nil); err != nil {
		logrus.Errorf("Campaign: Failed to update message status to sending: %v", err)
	}

	// Get device instance for this message
	dm := whatsapp.GetDeviceManager()
	if dm == nil {
		errMsg := "Device manager not available"
		_ = s.repo.UpdateMessageStatus(ctx, msg.ID, domainCampaign.MessageStatusFailed, &errMsg)
		logrus.Warn("Campaign: " + errMsg)
		return
	}

	device, ok := dm.GetDevice(msg.DeviceID)
	if !ok || device == nil {
		errMsg := fmt.Sprintf("Device %s not found", msg.DeviceID)
		_ = s.repo.UpdateMessageStatus(ctx, msg.ID, domainCampaign.MessageStatusFailed, &errMsg)
		logrus.Warn("Campaign: " + errMsg)
		return
	}

	client := device.GetClient()
	if client == nil || !client.IsLoggedIn() {
		errMsg := fmt.Sprintf("Device %s not connected", msg.DeviceID)
		_ = s.repo.UpdateMessageStatus(ctx, msg.ID, domainCampaign.MessageStatusFailed, &errMsg)
		logrus.Warn("Campaign: " + errMsg)
		return
	}

	// Create context with device for the send service
	sendCtx := whatsapp.ContextWithDevice(ctx, device)

	// Format phone for WhatsApp (remove + prefix)
	phone := strings.TrimPrefix(msg.Phone, "+")

	// Send via WhatsApp
	req := domainSend.MessageRequest{
		BaseRequest: domainSend.BaseRequest{
			Phone: phone,
		},
		Message: msg.Message,
	}

	_, err := s.sendService.SendText(sendCtx, req)
	if err != nil {
		errMsg := err.Error()
		_ = s.repo.UpdateMessageStatus(ctx, msg.ID, domainCampaign.MessageStatusFailed, &errMsg)
		logrus.WithFields(logrus.Fields{
			"phone": msg.Phone,
			"error": err,
		}).Warn("Campaign: Failed to send message")
		return
	}

	_ = s.repo.UpdateMessageStatus(ctx, msg.ID, domainCampaign.MessageStatusSent, nil)
	logrus.WithFields(logrus.Fields{
		"phone":       msg.Phone,
		"campaign_id": msg.CampaignID,
	}).Info("Campaign: Message sent successfully")

	// Check if campaign is complete
	s.checkCampaignCompletion(ctx, msg.CampaignID, msg.DeviceID)
}

func (s *CampaignService) checkCampaignCompletion(ctx context.Context, campaignID uuid.UUID, deviceID string) {
	stats, err := s.repo.GetCampaignStats(ctx, campaignID)
	if err != nil {
		return
	}

	// Log when all current messages have been processed
	// Campaign stays running - new customers can be added anytime
	if stats.PendingMessages == 0 && stats.TotalMessages > 0 {
		campaign, err := s.repo.GetCampaign(ctx, deviceID, campaignID)
		if err != nil || campaign == nil {
			return
		}

		logrus.WithFields(logrus.Fields{
			"campaign": campaign.Name,
			"total":    stats.TotalMessages,
			"sent":     stats.SentMessages,
			"failed":   stats.FailedMessages,
		}).Info("Campaign: All current messages processed")
	}
}

func (s *CampaignService) randomDelay(minSeconds, maxSeconds int) time.Duration {
	if minSeconds >= maxSeconds {
		return time.Duration(minSeconds) * time.Second
	}
	diff := maxSeconds - minSeconds
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(diff)))
	return time.Duration(minSeconds+int(n.Int64())) * time.Second
}

// ============================================================================
// Validation Worker
// ============================================================================

func (s *CampaignService) ValidateCustomer(ctx context.Context, deviceID string, id uuid.UUID) error {
	customer, err := s.repo.GetCustomer(ctx, deviceID, id)
	if err != nil {
		return err
	}
	if customer == nil {
		return errors.New("customer not found")
	}

	// Validate phone format using the standard validation function
	phoneValid := domainCampaign.ValidationStatusValid
	if err := validations.ValidatePhoneNumber(customer.Phone); err != nil {
		phoneValid = domainCampaign.ValidationStatusInvalid
	}

	// Check if phone exists on WhatsApp
	whatsappExists := domainCampaign.ValidationStatusPending

	// Try to get WhatsApp client from context
	client := whatsapp.ClientFromContext(ctx)
	if client != nil && client.IsLoggedIn() && phoneValid == domainCampaign.ValidationStatusValid {
		// Use the IsOnWhatsapp check - remove + prefix for JID format
		phone := strings.TrimPrefix(customer.Phone, "+")
		jid := phone + "@s.whatsapp.net"
		if utils.IsOnWhatsapp(client, jid) {
			whatsappExists = domainCampaign.ValidationStatusValid
		} else {
			whatsappExists = domainCampaign.ValidationStatusInvalid
		}
	}

	// Update validation status
	if err := s.repo.UpdateCustomerValidation(ctx, id, phoneValid, whatsappExists); err != nil {
		return err
	}

	logrus.WithFields(logrus.Fields{
		"customer_id":     id,
		"phone":           customer.Phone,
		"phone_valid":     phoneValid,
		"whatsapp_exists": whatsappExists,
	}).Info("Campaign: Customer validated")

	return nil
}

func (s *CampaignService) ValidatePendingCustomers(ctx context.Context, deviceID string) error {
	// Get customers needing validation
	// Limit to 1000 for now to avoid timeout, user can click again if needed
	customers, err := s.repo.GetCustomersForValidation(ctx, deviceID, 1000)
	if err != nil {
		return err
	}

	if len(customers) == 0 {
		return nil
	}

	// Try to get WhatsApp client from context or device manager
	var client *whatsmeow.Client

	// Check context first
	client = whatsapp.ClientFromContext(ctx)

	// Fallback to device manager
	if client == nil {
		if dm := whatsapp.GetDeviceManager(); dm != nil {
			if device, ok := dm.GetDevice(deviceID); ok && device != nil {
				client = device.GetClient()
			}
		}
	}

	if client == nil || !client.IsLoggedIn() {
		return errors.New("whatsapp client not connected")
	}

	logrus.WithFields(logrus.Fields{
		"device_id": deviceID,
		"count":     len(customers),
	}).Info("Campaign: Starting bulk validation")

	for _, customer := range customers {
		// Validate phone format
		phoneValid := domainCampaign.ValidationStatusValid
		if err := validations.ValidatePhoneNumber(customer.Phone); err != nil {
			phoneValid = domainCampaign.ValidationStatusInvalid
		}

		// Check WhatsApp existence
		whatsappExists := domainCampaign.ValidationStatusPending
		if phoneValid == domainCampaign.ValidationStatusValid {
			phone := strings.TrimPrefix(customer.Phone, "+")
			jid := phone + "@s.whatsapp.net"
			if utils.IsOnWhatsapp(client, jid) {
				whatsappExists = domainCampaign.ValidationStatusValid
			} else {
				whatsappExists = domainCampaign.ValidationStatusInvalid
			}
		}

		// Update validation status
		if err := s.repo.UpdateCustomerValidation(ctx, customer.ID, phoneValid, whatsappExists); err != nil {
			logrus.Errorf("Campaign: Failed to update customer validation %s: %v", customer.ID, err)
		}

		// Small delay to avoid rate limiting
		time.Sleep(100 * time.Millisecond)
	}

	logrus.Info("Campaign: Bulk validation completed")
	return nil
}

func (s *CampaignService) StartValidationWorker(ctx context.Context) {
	// Validation worker runs less frequently than message worker
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		logrus.Info("Campaign: Validation worker started")

		for {
			select {
			case <-ctx.Done():
				logrus.Info("Campaign: Validation worker stopped")
				return
			case <-ticker.C:
				// Get devices with customers needing validation
				deviceIDs, err := s.repo.GetActiveDeviceIDs(ctx)
				if err != nil {
					continue
				}

				for _, deviceID := range deviceIDs {
					customers, err := s.repo.GetCustomersForValidation(ctx, deviceID, 10)
					if err != nil {
						continue
					}

					// Try to get WhatsApp client for this device
					var client *whatsmeow.Client
					if dm := whatsapp.GetDeviceManager(); dm != nil {
						if device, ok := dm.GetDevice(deviceID); ok && device != nil {
							client = device.GetClient()
						}
					}

					for _, customer := range customers {
						// Validate phone format using shared validation
						phoneValid := domainCampaign.ValidationStatusValid
						if err := validations.ValidatePhoneNumber(customer.Phone); err != nil {
							phoneValid = domainCampaign.ValidationStatusInvalid
						}

						// Check WhatsApp existence if client available and phone is valid
						whatsappExists := customer.WhatsAppExists
						if client != nil && client.IsLoggedIn() && phoneValid == domainCampaign.ValidationStatusValid {
							phone := strings.TrimPrefix(customer.Phone, "+")
							jid := phone + "@s.whatsapp.net"
							if utils.IsOnWhatsapp(client, jid) {
								whatsappExists = domainCampaign.ValidationStatusValid
							} else {
								whatsappExists = domainCampaign.ValidationStatusInvalid
							}
						}

						_ = s.repo.UpdateCustomerValidation(ctx, customer.ID, phoneValid, whatsappExists)
					}
				}
			}
		}
	}()
}
