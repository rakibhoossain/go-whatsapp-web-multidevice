package campaign

import (
	"time"

	"github.com/google/uuid"
)

// CampaignStatus represents the status of a campaign
type CampaignStatus string

const (
	CampaignStatusDraft   CampaignStatus = "draft"
	CampaignStatusRunning CampaignStatus = "running"
	CampaignStatusPaused  CampaignStatus = "paused"
)

// MessageStatus represents the status of a queued message
type MessageStatus string

const (
	MessageStatusPending MessageStatus = "pending"
	MessageStatusSending MessageStatus = "sending"
	MessageStatusSent    MessageStatus = "sent"
	MessageStatusFailed  MessageStatus = "failed"
)

// ValidationStatus for phone/whatsapp checks
type ValidationStatus string

const (
	ValidationStatusPending ValidationStatus = "pending"
	ValidationStatusValid   ValidationStatus = "valid"
	ValidationStatusInvalid ValidationStatus = "invalid"
)

// Customer represents a campaign recipient
type Customer struct {
	ID             uuid.UUID        `json:"id"`
	DeviceID       string           `json:"device_id"`
	Phone          string           `json:"phone"`           // Must start with +
	FullName       *string          `json:"full_name"`       // Nullable
	Company        *string          `json:"company"`         // Nullable - for [COMPANY] placeholder
	Country        *string          `json:"country"`         // Nullable
	Gender         *string          `json:"gender"`          // Nullable
	BirthYear      *int             `json:"birth_year"`      // Nullable
	PhoneValid     ValidationStatus `json:"phone_valid"`     // Phone format validation status
	WhatsAppExists ValidationStatus `json:"whatsapp_exists"` // WhatsApp account validation status
	IsReady        bool             `json:"is_ready"`        // Computed: true if phone_valid=valid AND whatsapp_exists=valid
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

// Group represents a customer group for targeting
type Group struct {
	ID          uuid.UUID `json:"id"`
	DeviceID    string    `json:"device_id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	// Populated on demand
	CustomerCount int        `json:"customer_count,omitempty"`
	Customers     []Customer `json:"customers,omitempty"`
}

// Template represents a campaign message template
type Template struct {
	ID        uuid.UUID `json:"id"`
	DeviceID  string    `json:"device_id"`
	Name      string    `json:"name"`
	Content   string    `json:"content"` // Supports placeholders: [NAME], [PHONE], [COUNTRY], [GROUP], [COMPANY]
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Campaign represents a message campaign
type Campaign struct {
	ID          uuid.UUID      `json:"id"`
	DeviceID    string         `json:"device_id"`
	Name        string         `json:"name"`
	TemplateID  uuid.UUID      `json:"template_id"`
	Status      CampaignStatus `json:"status"`
	ScheduledAt *time.Time     `json:"scheduled_at,omitempty"`
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	// Populated on demand
	Template    *Template      `json:"template,omitempty"`
	Stats       *CampaignStats `json:"stats,omitempty"`
	CustomerIDs []uuid.UUID    `json:"customer_ids,omitempty"`
	GroupIDs    []uuid.UUID    `json:"group_ids,omitempty"`
}

// CampaignStats holds campaign execution statistics
type CampaignStats struct {
	TotalMessages   int `json:"total_messages"`
	PendingMessages int `json:"pending_messages"`
	SentMessages    int `json:"sent_messages"`
	FailedMessages  int `json:"failed_messages"`
}

// QueueItem represents a message in the send queue
type QueueItem struct {
	ID         uuid.UUID     `json:"id"`
	CampaignID uuid.UUID     `json:"campaign_id"`
	CustomerID uuid.UUID     `json:"customer_id"`
	DeviceID   string        `json:"device_id"`
	Phone      string        `json:"phone"`   // Denormalized for quick access
	Message    string        `json:"message"` // Processed message with placeholders replaced
	Status     MessageStatus `json:"status"`
	Error      *string       `json:"error,omitempty"`
	SentAt     *time.Time    `json:"sent_at,omitempty"`
	CreatedAt  time.Time     `json:"created_at"`
	UpdatedAt  time.Time     `json:"updated_at"`
}

// ShortURL represents a shortened URL for tracking
type ShortURL struct {
	ID          uuid.UUID `json:"id"`
	DeviceID    string    `json:"device_id"`
	Code        string    `json:"code"`         // Short code (e.g., "abc123")
	OriginalURL string    `json:"original_url"` // Full URL
	Clicks      int       `json:"clicks"`
	CreatedAt   time.Time `json:"created_at"`
}

// GroupListResponse for pagination
type GroupListResponse struct {
	Groups     []*Group `json:"groups"`
	Total      int      `json:"total"`
	Page       int      `json:"page"`
	PageSize   int      `json:"page_size"`
	TotalPages int      `json:"total_pages"`
}

// TemplateListResponse for pagination
type TemplateListResponse struct {
	Templates  []*Template `json:"templates"`
	Total      int         `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
}

// CampaignListResponse for pagination
type CampaignListResponse struct {
	Campaigns  []*Campaign `json:"campaigns"`
	Total      int         `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
}
