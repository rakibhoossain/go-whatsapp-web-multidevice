package campaign

import (
	"time"

	"github.com/google/uuid"
)

// CreateCustomerRequest is the request to create a new customer
type CreateCustomerRequest struct {
	DeviceID  string  `json:"-"` // Set from context
	Phone     string  `json:"phone" form:"phone"`
	FullName  *string `json:"full_name" form:"full_name"`
	Company   *string `json:"company" form:"company"`
	Country   *string `json:"country" form:"country"`
	Gender    *string `json:"gender" form:"gender"`
	BirthYear *int    `json:"birth_year" form:"birth_year"`
}

// UpdateCustomerRequest is the request to update a customer
type UpdateCustomerRequest struct {
	DeviceID  string    `json:"-"`
	ID        uuid.UUID `json:"-"`
	Phone     string    `json:"phone" form:"phone"`
	FullName  *string   `json:"full_name" form:"full_name"`
	Company   *string   `json:"company" form:"company"`
	Country   *string   `json:"country" form:"country"`
	Gender    *string   `json:"gender" form:"gender"`
	BirthYear *int      `json:"birth_year" form:"birth_year"`
}

// CustomerListResponse is the paginated response for customer list
type CustomerListResponse struct {
	Customers  []*Customer `json:"customers"`
	Total      int         `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
}

// CreateGroupRequest is the request to create a new group
type CreateGroupRequest struct {
	DeviceID    string  `json:"-"`
	Name        string  `json:"name" form:"name"`
	Description *string `json:"description" form:"description"`
}

// UpdateGroupRequest is the request to update a group
type UpdateGroupRequest struct {
	DeviceID    string    `json:"-"`
	ID          uuid.UUID `json:"-"`
	Name        string    `json:"name" form:"name"`
	Description *string   `json:"description" form:"description"`
}

// AddCustomersToGroupRequest is the request to add customers to a group
type AddCustomersToGroupRequest struct {
	CustomerIDs []uuid.UUID `json:"customer_ids" form:"customer_ids"`
}

// CreateTemplateRequest is the request to create a new template
type CreateTemplateRequest struct {
	DeviceID string `json:"-"`
	Name     string `json:"name" form:"name"`
	Content  string `json:"content" form:"content"`
}

// UpdateTemplateRequest is the request to update a template
type UpdateTemplateRequest struct {
	DeviceID string    `json:"-"`
	ID       uuid.UUID `json:"-"`
	Name     string    `json:"name" form:"name"`
	Content  string    `json:"content" form:"content"`
}

// CreateCampaignRequest is the request to create a new campaign
type CreateCampaignRequest struct {
	DeviceID    string      `json:"-"`
	Name        string      `json:"name" form:"name"`
	TemplateID  uuid.UUID   `json:"template_id" form:"template_id"`
	CustomerIDs []uuid.UUID `json:"customer_ids" form:"customer_ids"`
	GroupIDs    []uuid.UUID `json:"group_ids" form:"group_ids"`
	ScheduledAt *time.Time  `json:"scheduled_at" form:"scheduled_at"`
}

// UpdateCampaignRequest is the request to update a campaign
type UpdateCampaignRequest struct {
	DeviceID    string      `json:"-"`
	ID          uuid.UUID   `json:"-"`
	Name        string      `json:"name" form:"name"`
	TemplateID  uuid.UUID   `json:"template_id" form:"template_id"`
	CustomerIDs []uuid.UUID `json:"customer_ids" form:"customer_ids"`
	GroupIDs    []uuid.UUID `json:"group_ids" form:"group_ids"`
	ScheduledAt *time.Time  `json:"scheduled_at" form:"scheduled_at"`
}
