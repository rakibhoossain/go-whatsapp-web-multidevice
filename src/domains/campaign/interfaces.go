package campaign

import (
	"context"

	"github.com/google/uuid"
)

// ICampaignRepository defines database operations for campaign entities
type ICampaignRepository interface {
	// Customer operations
	CreateCustomer(ctx context.Context, customer *Customer) error
	GetCustomer(ctx context.Context, deviceID string, id uuid.UUID) (*Customer, error)
	GetCustomerByPhone(ctx context.Context, deviceID string, phone string) (*Customer, error)
	ListCustomers(ctx context.Context, deviceID string, limit, offset int, search string) ([]*Customer, int, error)
	UpdateCustomer(ctx context.Context, customer *Customer) error
	DeleteCustomer(ctx context.Context, deviceID string, id uuid.UUID) error
	DeleteCustomers(ctx context.Context, deviceID string, ids []uuid.UUID) error
	BulkCreateCustomers(ctx context.Context, customers []*Customer) (int, error)
	GetCustomersForValidation(ctx context.Context, deviceID string, limit int) ([]*Customer, error)
	UpdateCustomerValidation(ctx context.Context, id uuid.UUID, phoneValid, whatsappExists ValidationStatus) error

	// Group operations
	CreateGroup(ctx context.Context, group *Group) error
	GetGroup(ctx context.Context, deviceID string, id uuid.UUID) (*Group, error)
	ListGroups(ctx context.Context, deviceID string, limit, offset int) ([]*Group, int, error)
	UpdateGroup(ctx context.Context, group *Group) error
	DeleteGroup(ctx context.Context, deviceID string, id uuid.UUID) error
	AddCustomersToGroup(ctx context.Context, groupID uuid.UUID, customerIDs []uuid.UUID) error
	RemoveCustomerFromGroup(ctx context.Context, groupID uuid.UUID, customerID uuid.UUID) error
	GetGroupCustomers(ctx context.Context, groupID uuid.UUID) ([]*Customer, error)
	GetCustomerGroups(ctx context.Context, customerID uuid.UUID) ([]*Group, error)

	// Template operations
	CreateTemplate(ctx context.Context, template *Template) error
	GetTemplate(ctx context.Context, deviceID string, id uuid.UUID) (*Template, error)
	ListTemplates(ctx context.Context, deviceID string, limit, offset int) ([]*Template, int, error)
	UpdateTemplate(ctx context.Context, template *Template) error
	DeleteTemplate(ctx context.Context, deviceID string, id uuid.UUID) error

	// Campaign operations
	CreateCampaign(ctx context.Context, campaign *Campaign) error
	GetCampaign(ctx context.Context, deviceID string, id uuid.UUID) (*Campaign, error)
	ListCampaigns(ctx context.Context, deviceID string, limit, offset int) ([]*Campaign, int, error)
	UpdateCampaign(ctx context.Context, campaign *Campaign) error
	DeleteCampaign(ctx context.Context, deviceID string, id uuid.UUID) error
	SetCampaignTargets(ctx context.Context, campaignID uuid.UUID, customerIDs, groupIDs []uuid.UUID) error
	GetCampaignTargetIDs(ctx context.Context, campaignID uuid.UUID) (customerIDs, groupIDs []uuid.UUID, err error)
	GetCampaignTargetCustomers(ctx context.Context, campaignID uuid.UUID) ([]*Customer, error)
	GetCampaignStats(ctx context.Context, campaignID uuid.UUID) (*CampaignStats, error)

	// Queue operations
	EnqueueMessages(ctx context.Context, items []*QueueItem) error
	GetPendingMessages(ctx context.Context, deviceID string, limit int) ([]*QueueItem, error)
	UpdateMessageStatus(ctx context.Context, id uuid.UUID, status MessageStatus, errorMsg *string) error
	IsMessageQueued(ctx context.Context, campaignID, customerID uuid.UUID) (bool, error)

	// Short URL operations
	CreateShortURL(ctx context.Context, shortURL *ShortURL) error
	GetShortURLByCode(ctx context.Context, code string) (*ShortURL, error)
	IncrementShortURLClicks(ctx context.Context, code string) error

	// Device operations for queue worker
	GetActiveDeviceIDs(ctx context.Context) ([]string, error)

	// Schema
	InitializeSchema() error
}

// ICampaignUsecase defines business logic for campaign management
type ICampaignUsecase interface {
	// Customer management
	CreateCustomer(ctx context.Context, req CreateCustomerRequest) (*Customer, error)
	ImportCustomersFromCSV(ctx context.Context, deviceID string, csvData []byte) (imported int, errors []string, err error)
	GetCustomer(ctx context.Context, deviceID string, id uuid.UUID) (*Customer, error)
	ListCustomers(ctx context.Context, deviceID string, page, pageSize int, search string) (*CustomerListResponse, error)
	UpdateCustomer(ctx context.Context, req UpdateCustomerRequest) (*Customer, error)
	DeleteCustomer(ctx context.Context, deviceID string, id uuid.UUID) error
	DeleteCustomers(ctx context.Context, deviceID string, ids []uuid.UUID) error
	ValidateCustomer(ctx context.Context, deviceID string, id uuid.UUID) error // Manual validation trigger
	ValidateCustomers(ctx context.Context, deviceID string, ids []uuid.UUID) error
	ValidatePendingCustomers(ctx context.Context, deviceID string) error // Bulk manual validation trigger
	StartValidationWorker(ctx context.Context)                           // Background validation

	// Group management
	CreateGroup(ctx context.Context, req CreateGroupRequest) (*Group, error)
	GetGroup(ctx context.Context, deviceID string, id uuid.UUID) (*Group, error)
	ListGroups(ctx context.Context, deviceID string, page, pageSize int) (*GroupListResponse, error)
	UpdateGroup(ctx context.Context, req UpdateGroupRequest) (*Group, error)
	DeleteGroup(ctx context.Context, deviceID string, id uuid.UUID) error
	AddCustomersToGroup(ctx context.Context, deviceID string, groupID uuid.UUID, customerIDs []uuid.UUID) error
	RemoveCustomerFromGroup(ctx context.Context, deviceID string, groupID uuid.UUID, customerID uuid.UUID) error

	// Template management
	CreateTemplate(ctx context.Context, req CreateTemplateRequest) (*Template, error)
	GetTemplate(ctx context.Context, deviceID string, id uuid.UUID) (*Template, error)
	ListTemplates(ctx context.Context, deviceID string, page, pageSize int) (*TemplateListResponse, error)
	UpdateTemplate(ctx context.Context, req UpdateTemplateRequest) (*Template, error)
	DeleteTemplate(ctx context.Context, deviceID string, id uuid.UUID) error
	PreviewTemplate(ctx context.Context, content string, customer *Customer) string

	// Campaign management
	CreateCampaign(ctx context.Context, req CreateCampaignRequest) (*Campaign, error)
	GetCampaign(ctx context.Context, deviceID string, id uuid.UUID) (*Campaign, error)
	ListCampaigns(ctx context.Context, deviceID string, page, pageSize int) (*CampaignListResponse, error)
	UpdateCampaign(ctx context.Context, req UpdateCampaignRequest) (*Campaign, error)
	DeleteCampaign(ctx context.Context, deviceID string, id uuid.UUID) error
	StartCampaign(ctx context.Context, deviceID string, id uuid.UUID) error
	PauseCampaign(ctx context.Context, deviceID string, id uuid.UUID) error
	GetCampaignStats(ctx context.Context, deviceID string, id uuid.UUID) (*CampaignStats, error)

	// Short URL
	ShortenURLsInText(ctx context.Context, deviceID string, text string) (string, error)
	HandleShortURLRedirect(ctx context.Context, code string) (string, error)

	// Queue worker
	StartQueueWorker(ctx context.Context)
	StopQueueWorker()
}
