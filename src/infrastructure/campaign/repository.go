package campaign

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	domainCampaign "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/campaign"
)

// Repository implements ICampaignRepository using SQL database
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new campaign repository
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// InitializeSchema runs campaign migrations
func (r *Repository) InitializeSchema() error {
	migrations := r.getMigrations()
	for i, migration := range migrations {
		if _, err := r.db.Exec(migration); err != nil {
			// Ignore "already exists" errors for idempotent migrations
			if !strings.Contains(err.Error(), "already exists") {
				return fmt.Errorf("migration %d failed: %w", i+1, err)
			}
		}
	}
	return nil
}

// getMigrations returns campaign-specific migrations
func (r *Repository) getMigrations() []string {
	return []string{
		// Migration 1: Campaign customers table
		`CREATE TABLE IF NOT EXISTS campaign_customers (
			id VARCHAR(36) PRIMARY KEY,
			device_id VARCHAR(255) NOT NULL,
			phone VARCHAR(50) NOT NULL,
			full_name VARCHAR(255),
			company VARCHAR(255),
			country VARCHAR(100),
			gender VARCHAR(20),
			birth_year INTEGER,
			phone_valid VARCHAR(20) DEFAULT 'pending',
			whatsapp_exists VARCHAR(20) DEFAULT 'pending',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(device_id, phone)
		)`,

		// Migration 2: Campaign groups table
		`CREATE TABLE IF NOT EXISTS campaign_groups (
			id VARCHAR(36) PRIMARY KEY,
			device_id VARCHAR(255) NOT NULL,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(device_id, name)
		)`,

		// Migration 3: Group members junction table
		`CREATE TABLE IF NOT EXISTS campaign_group_members (
			group_id VARCHAR(36) NOT NULL,
			customer_id VARCHAR(36) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (group_id, customer_id)
		)`,

		// Migration 4: Campaign templates table
		`CREATE TABLE IF NOT EXISTS campaign_templates (
			id VARCHAR(36) PRIMARY KEY,
			device_id VARCHAR(255) NOT NULL,
			name VARCHAR(255) NOT NULL,
			content TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(device_id, name)
		)`,

		// Migration 5: Campaigns table
		`CREATE TABLE IF NOT EXISTS campaigns (
			id VARCHAR(36) PRIMARY KEY,
			device_id VARCHAR(255) NOT NULL,
			name VARCHAR(255) NOT NULL,
			template_id VARCHAR(36) NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'draft',
			scheduled_at TIMESTAMP,
			started_at TIMESTAMP,
			completed_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Migration 6: Campaign targets (customers)
		`CREATE TABLE IF NOT EXISTS campaign_target_customers (
			campaign_id VARCHAR(36) NOT NULL,
			customer_id VARCHAR(36) NOT NULL,
			PRIMARY KEY (campaign_id, customer_id)
		)`,

		// Migration 7: Campaign targets (groups)
		`CREATE TABLE IF NOT EXISTS campaign_target_groups (
			campaign_id VARCHAR(36) NOT NULL,
			group_id VARCHAR(36) NOT NULL,
			PRIMARY KEY (campaign_id, group_id)
		)`,

		// Migration 8: Message queue table
		`CREATE TABLE IF NOT EXISTS campaign_messages (
			id VARCHAR(36) PRIMARY KEY,
			campaign_id VARCHAR(36) NOT NULL,
			customer_id VARCHAR(36) NOT NULL,
			device_id VARCHAR(255) NOT NULL,
			phone VARCHAR(50) NOT NULL,
			message TEXT NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			error TEXT,
			sent_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(campaign_id, customer_id)
		)`,

		// Migration 9: Short URLs table
		`CREATE TABLE IF NOT EXISTS campaign_short_urls (
			id VARCHAR(36) PRIMARY KEY,
			device_id VARCHAR(255) NOT NULL,
			code VARCHAR(20) NOT NULL UNIQUE,
			original_url TEXT NOT NULL,
			clicks INTEGER DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Migration 10: Indexes for performance
		`CREATE INDEX IF NOT EXISTS idx_campaign_customers_device ON campaign_customers(device_id)`,
		`CREATE INDEX IF NOT EXISTS idx_campaign_customers_phone ON campaign_customers(phone)`,
		`CREATE INDEX IF NOT EXISTS idx_campaign_groups_device ON campaign_groups(device_id)`,
		`CREATE INDEX IF NOT EXISTS idx_campaign_templates_device ON campaign_templates(device_id)`,
		`CREATE INDEX IF NOT EXISTS idx_campaigns_device ON campaigns(device_id)`,
		`CREATE INDEX IF NOT EXISTS idx_campaigns_status ON campaigns(status)`,
		`CREATE INDEX IF NOT EXISTS idx_campaign_messages_device ON campaign_messages(device_id)`,
		`CREATE INDEX IF NOT EXISTS idx_campaign_messages_status ON campaign_messages(status)`,
		`CREATE INDEX IF NOT EXISTS idx_campaign_messages_campaign ON campaign_messages(campaign_id)`,
	}
}

// ============================================================================
// Customer Operations
// ============================================================================

func (r *Repository) CreateCustomer(ctx context.Context, customer *domainCampaign.Customer) error {
	customer.ID = uuid.New()
	customer.CreatedAt = time.Now()
	customer.UpdatedAt = time.Now()
	customer.PhoneValid = domainCampaign.ValidationStatusPending
	customer.WhatsAppExists = domainCampaign.ValidationStatusPending

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO campaign_customers (id, device_id, phone, full_name, company, country, gender, birth_year, phone_valid, whatsapp_exists, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, customer.ID.String(), customer.DeviceID, customer.Phone, customer.FullName, customer.Company, customer.Country,
		customer.Gender, customer.BirthYear, string(customer.PhoneValid), string(customer.WhatsAppExists),
		customer.CreatedAt, customer.UpdatedAt)
	return err
}

func (r *Repository) GetCustomer(ctx context.Context, deviceID string, id uuid.UUID) (*domainCampaign.Customer, error) {
	customer := &domainCampaign.Customer{}
	var idStr string
	var phoneValid, whatsappExists string
	err := r.db.QueryRowContext(ctx, `
		SELECT id, device_id, phone, full_name, company, country, gender, birth_year, phone_valid, whatsapp_exists, created_at, updated_at
		FROM campaign_customers WHERE id = ? AND device_id = ?
	`, id.String(), deviceID).Scan(&idStr, &customer.DeviceID, &customer.Phone, &customer.FullName,
		&customer.Company, &customer.Country, &customer.Gender, &customer.BirthYear,
		&phoneValid, &whatsappExists, &customer.CreatedAt, &customer.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	customer.ID, _ = uuid.Parse(idStr)
	customer.PhoneValid = domainCampaign.ValidationStatus(phoneValid)
	customer.WhatsAppExists = domainCampaign.ValidationStatus(whatsappExists)
	customer.IsReady = customer.PhoneValid == domainCampaign.ValidationStatusValid &&
		customer.WhatsAppExists == domainCampaign.ValidationStatusValid
	return customer, nil
}

func (r *Repository) GetCustomerByPhone(ctx context.Context, deviceID string, phone string) (*domainCampaign.Customer, error) {
	customer := &domainCampaign.Customer{}
	var idStr string
	var phoneValid, whatsappExists string
	err := r.db.QueryRowContext(ctx, `
		SELECT id, device_id, phone, full_name, company, country, gender, birth_year, phone_valid, whatsapp_exists, created_at, updated_at
		FROM campaign_customers WHERE phone = ? AND device_id = ?
	`, phone, deviceID).Scan(&idStr, &customer.DeviceID, &customer.Phone, &customer.FullName,
		&customer.Company, &customer.Country, &customer.Gender, &customer.BirthYear,
		&phoneValid, &whatsappExists, &customer.CreatedAt, &customer.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	customer.ID, _ = uuid.Parse(idStr)
	customer.PhoneValid = domainCampaign.ValidationStatus(phoneValid)
	customer.WhatsAppExists = domainCampaign.ValidationStatus(whatsappExists)
	customer.IsReady = customer.PhoneValid == domainCampaign.ValidationStatusValid &&
		customer.WhatsAppExists == domainCampaign.ValidationStatusValid
	return customer, nil
}

func (r *Repository) ListCustomers(ctx context.Context, deviceID string, limit, offset int) ([]*domainCampaign.Customer, int, error) {
	// Get total count
	var total int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM campaign_customers WHERE device_id = ?
	`, deviceID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get customers
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, device_id, phone, full_name, company, country, gender, birth_year, phone_valid, whatsapp_exists, created_at, updated_at
		FROM campaign_customers WHERE device_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?
	`, deviceID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var customers []*domainCampaign.Customer
	for rows.Next() {
		customer := &domainCampaign.Customer{}
		var idStr string
		var phoneValid, whatsappExists string
		if err := rows.Scan(&idStr, &customer.DeviceID, &customer.Phone, &customer.FullName,
			&customer.Company, &customer.Country, &customer.Gender, &customer.BirthYear,
			&phoneValid, &whatsappExists, &customer.CreatedAt, &customer.UpdatedAt); err != nil {
			return nil, 0, err
		}
		customer.ID, _ = uuid.Parse(idStr)
		customer.PhoneValid = domainCampaign.ValidationStatus(phoneValid)
		customer.WhatsAppExists = domainCampaign.ValidationStatus(whatsappExists)
		customer.IsReady = customer.PhoneValid == domainCampaign.ValidationStatusValid &&
			customer.WhatsAppExists == domainCampaign.ValidationStatusValid
		customers = append(customers, customer)
	}
	return customers, total, rows.Err()
}

func (r *Repository) UpdateCustomer(ctx context.Context, customer *domainCampaign.Customer) error {
	customer.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx, `
		UPDATE campaign_customers SET phone = ?, full_name = ?, company = ?, country = ?, gender = ?, birth_year = ?, 
			phone_valid = ?, whatsapp_exists = ?, updated_at = ?
		WHERE id = ? AND device_id = ?
	`, customer.Phone, customer.FullName, customer.Company, customer.Country, customer.Gender, customer.BirthYear,
		string(customer.PhoneValid), string(customer.WhatsAppExists),
		customer.UpdatedAt, customer.ID.String(), customer.DeviceID)
	return err
}

func (r *Repository) DeleteCustomer(ctx context.Context, deviceID string, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM campaign_customers WHERE id = ? AND device_id = ?`, id.String(), deviceID)
	return err
}

func (r *Repository) BulkCreateCustomers(ctx context.Context, customers []*domainCampaign.Customer) (int, error) {
	if len(customers) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO campaign_customers (id, device_id, phone, full_name, company, country, gender, birth_year, phone_valid, whatsapp_exists, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(device_id, phone) DO UPDATE SET
			full_name = excluded.full_name,
			company = excluded.company,
			country = excluded.country,
			gender = excluded.gender,
			birth_year = excluded.birth_year,
			updated_at = excluded.updated_at
	`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	now := time.Now()
	imported := 0
	for _, customer := range customers {
		customer.ID = uuid.New()
		customer.CreatedAt = now
		customer.UpdatedAt = now
		customer.PhoneValid = domainCampaign.ValidationStatusPending
		customer.WhatsAppExists = domainCampaign.ValidationStatusPending
		_, err := stmt.ExecContext(ctx, customer.ID.String(), customer.DeviceID, customer.Phone,
			customer.FullName, customer.Company, customer.Country, customer.Gender, customer.BirthYear,
			string(customer.PhoneValid), string(customer.WhatsAppExists),
			customer.CreatedAt, customer.UpdatedAt)
		if err != nil {
			continue // Skip errors for individual rows
		}
		imported++
	}

	return imported, tx.Commit()
}

// ============================================================================
// Group Operations
// ============================================================================

func (r *Repository) CreateGroup(ctx context.Context, group *domainCampaign.Group) error {
	group.ID = uuid.New()
	group.CreatedAt = time.Now()
	group.UpdatedAt = time.Now()

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO campaign_groups (id, device_id, name, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, group.ID.String(), group.DeviceID, group.Name, group.Description, group.CreatedAt, group.UpdatedAt)
	return err
}

func (r *Repository) GetGroup(ctx context.Context, deviceID string, id uuid.UUID) (*domainCampaign.Group, error) {
	group := &domainCampaign.Group{}
	var idStr string
	err := r.db.QueryRowContext(ctx, `
		SELECT g.id, g.device_id, g.name, g.description, g.created_at, g.updated_at,
			(SELECT COUNT(*) FROM campaign_group_members WHERE group_id = g.id) as customer_count
		FROM campaign_groups g WHERE g.id = ? AND g.device_id = ?
	`, id.String(), deviceID).Scan(&idStr, &group.DeviceID, &group.Name, &group.Description,
		&group.CreatedAt, &group.UpdatedAt, &group.CustomerCount)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	group.ID, _ = uuid.Parse(idStr)
	return group, nil
}

func (r *Repository) ListGroups(ctx context.Context, deviceID string, limit, offset int) ([]*domainCampaign.Group, int, error) {
	var total int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM campaign_groups WHERE device_id = ?", deviceID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT g.id, g.device_id, g.name, g.description, g.created_at, g.updated_at,
			(SELECT COUNT(*) FROM campaign_group_members WHERE group_id = g.id) as customer_count
		FROM campaign_groups g WHERE g.device_id = ? ORDER BY g.name ASC LIMIT ? OFFSET ?
	`, deviceID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var groups []*domainCampaign.Group
	for rows.Next() {
		group := &domainCampaign.Group{}
		var idStr string
		if err := rows.Scan(&idStr, &group.DeviceID, &group.Name, &group.Description,
			&group.CreatedAt, &group.UpdatedAt, &group.CustomerCount); err != nil {
			return nil, 0, err
		}
		group.ID, _ = uuid.Parse(idStr)
		groups = append(groups, group)
	}
	return groups, total, rows.Err()
}

func (r *Repository) UpdateGroup(ctx context.Context, group *domainCampaign.Group) error {
	group.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx, `
		UPDATE campaign_groups SET name = ?, description = ?, updated_at = ?
		WHERE id = ? AND device_id = ?
	`, group.Name, group.Description, group.UpdatedAt, group.ID.String(), group.DeviceID)
	return err
}

func (r *Repository) DeleteGroup(ctx context.Context, deviceID string, id uuid.UUID) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete group members first
	_, err = tx.ExecContext(ctx, `DELETE FROM campaign_group_members WHERE group_id = ?`, id.String())
	if err != nil {
		return err
	}

	// Delete group
	_, err = tx.ExecContext(ctx, `DELETE FROM campaign_groups WHERE id = ? AND device_id = ?`, id.String(), deviceID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *Repository) AddCustomersToGroup(ctx context.Context, groupID uuid.UUID, customerIDs []uuid.UUID) error {
	if len(customerIDs) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO campaign_group_members (group_id, customer_id, created_at)
		VALUES (?, ?, ?) ON CONFLICT DO NOTHING
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now()
	for _, customerID := range customerIDs {
		_, err := stmt.ExecContext(ctx, groupID.String(), customerID.String(), now)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *Repository) RemoveCustomerFromGroup(ctx context.Context, groupID uuid.UUID, customerID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM campaign_group_members WHERE group_id = ? AND customer_id = ?
	`, groupID.String(), customerID.String())
	return err
}

func (r *Repository) GetGroupCustomers(ctx context.Context, groupID uuid.UUID) ([]*domainCampaign.Customer, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.device_id, c.phone, c.full_name, c.country, c.gender, c.birth_year, c.created_at, c.updated_at
		FROM campaign_customers c
		INNER JOIN campaign_group_members gm ON c.id = gm.customer_id
		WHERE gm.group_id = ?
	`, groupID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var customers []*domainCampaign.Customer
	for rows.Next() {
		customer := &domainCampaign.Customer{}
		var idStr string
		if err := rows.Scan(&idStr, &customer.DeviceID, &customer.Phone, &customer.FullName,
			&customer.Country, &customer.Gender, &customer.BirthYear, &customer.CreatedAt, &customer.UpdatedAt); err != nil {
			return nil, err
		}
		customer.ID, _ = uuid.Parse(idStr)
		customers = append(customers, customer)
	}
	return customers, rows.Err()
}

func (r *Repository) GetCustomerGroups(ctx context.Context, customerID uuid.UUID) ([]*domainCampaign.Group, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT g.id, g.device_id, g.name, g.description, g.created_at, g.updated_at
		FROM campaign_groups g
		INNER JOIN campaign_group_members gm ON g.id = gm.group_id
		WHERE gm.customer_id = ?
	`, customerID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []*domainCampaign.Group
	for rows.Next() {
		group := &domainCampaign.Group{}
		var idStr string
		if err := rows.Scan(&idStr, &group.DeviceID, &group.Name, &group.Description,
			&group.CreatedAt, &group.UpdatedAt); err != nil {
			return nil, err
		}
		group.ID, _ = uuid.Parse(idStr)
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

// ============================================================================
// Template Operations
// ============================================================================

func (r *Repository) CreateTemplate(ctx context.Context, template *domainCampaign.Template) error {
	template.ID = uuid.New()
	template.CreatedAt = time.Now()
	template.UpdatedAt = time.Now()

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO campaign_templates (id, device_id, name, content, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, template.ID.String(), template.DeviceID, template.Name, template.Content, template.CreatedAt, template.UpdatedAt)
	return err
}

func (r *Repository) GetTemplate(ctx context.Context, deviceID string, id uuid.UUID) (*domainCampaign.Template, error) {
	template := &domainCampaign.Template{}
	var idStr string
	err := r.db.QueryRowContext(ctx, `
		SELECT id, device_id, name, content, created_at, updated_at
		FROM campaign_templates WHERE id = ? AND device_id = ?
	`, id.String(), deviceID).Scan(&idStr, &template.DeviceID, &template.Name, &template.Content,
		&template.CreatedAt, &template.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	template.ID, _ = uuid.Parse(idStr)
	return template, nil
}

func (r *Repository) ListTemplates(ctx context.Context, deviceID string, limit, offset int) ([]*domainCampaign.Template, int, error) {
	var total int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM campaign_templates WHERE device_id = ?", deviceID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, device_id, name, content, created_at, updated_at
		FROM campaign_templates WHERE device_id = ? ORDER BY name ASC LIMIT ? OFFSET ?
	`, deviceID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var templates []*domainCampaign.Template
	for rows.Next() {
		template := &domainCampaign.Template{}
		var idStr string
		if err := rows.Scan(&idStr, &template.DeviceID, &template.Name, &template.Content,
			&template.CreatedAt, &template.UpdatedAt); err != nil {
			return nil, 0, err
		}
		template.ID, _ = uuid.Parse(idStr)
		templates = append(templates, template)
	}
	return templates, total, rows.Err()
}

func (r *Repository) UpdateTemplate(ctx context.Context, template *domainCampaign.Template) error {
	template.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx, `
		UPDATE campaign_templates SET name = ?, content = ?, updated_at = ?
		WHERE id = ? AND device_id = ?
	`, template.Name, template.Content, template.UpdatedAt, template.ID.String(), template.DeviceID)
	return err
}

func (r *Repository) DeleteTemplate(ctx context.Context, deviceID string, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM campaign_templates WHERE id = ? AND device_id = ?`, id.String(), deviceID)
	return err
}

// ============================================================================
// Campaign Operations
// ============================================================================

func (r *Repository) CreateCampaign(ctx context.Context, campaign *domainCampaign.Campaign) error {
	campaign.ID = uuid.New()
	campaign.CreatedAt = time.Now()
	campaign.UpdatedAt = time.Now()
	if campaign.Status == "" {
		campaign.Status = domainCampaign.CampaignStatusDraft
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO campaigns (id, device_id, name, template_id, status, scheduled_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, campaign.ID.String(), campaign.DeviceID, campaign.Name, campaign.TemplateID.String(),
		string(campaign.Status), campaign.ScheduledAt, campaign.CreatedAt, campaign.UpdatedAt)
	return err
}

func (r *Repository) GetCampaign(ctx context.Context, deviceID string, id uuid.UUID) (*domainCampaign.Campaign, error) {
	campaign := &domainCampaign.Campaign{}
	var idStr, templateIDStr, status string
	err := r.db.QueryRowContext(ctx, `
		SELECT id, device_id, name, template_id, status, scheduled_at, started_at, completed_at, created_at, updated_at
		FROM campaigns WHERE id = ? AND device_id = ?
	`, id.String(), deviceID).Scan(&idStr, &campaign.DeviceID, &campaign.Name, &templateIDStr,
		&status, &campaign.ScheduledAt, &campaign.StartedAt, &campaign.CompletedAt,
		&campaign.CreatedAt, &campaign.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	campaign.ID, _ = uuid.Parse(idStr)
	campaign.TemplateID, _ = uuid.Parse(templateIDStr)
	campaign.Status = domainCampaign.CampaignStatus(status)
	return campaign, nil
}

func (r *Repository) ListCampaigns(ctx context.Context, deviceID string, limit, offset int) ([]*domainCampaign.Campaign, int, error) {
	var total int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM campaigns WHERE device_id = ?", deviceID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, device_id, name, template_id, status, scheduled_at, started_at, completed_at, created_at, updated_at
		FROM campaigns WHERE device_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?
	`, deviceID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var campaigns []*domainCampaign.Campaign
	for rows.Next() {
		campaign := &domainCampaign.Campaign{}
		var idStr, templateIDStr, status string
		if err := rows.Scan(&idStr, &campaign.DeviceID, &campaign.Name, &templateIDStr,
			&status, &campaign.ScheduledAt, &campaign.StartedAt, &campaign.CompletedAt,
			&campaign.CreatedAt, &campaign.UpdatedAt); err != nil {
			return nil, 0, err
		}
		campaign.ID, _ = uuid.Parse(idStr)
		campaign.TemplateID, _ = uuid.Parse(templateIDStr)
		campaign.Status = domainCampaign.CampaignStatus(status)
		campaigns = append(campaigns, campaign)
	}
	return campaigns, total, rows.Err()
}

func (r *Repository) UpdateCampaign(ctx context.Context, campaign *domainCampaign.Campaign) error {
	campaign.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx, `
		UPDATE campaigns SET name = ?, template_id = ?, status = ?, scheduled_at = ?,
			started_at = ?, completed_at = ?, updated_at = ?
		WHERE id = ? AND device_id = ?
	`, campaign.Name, campaign.TemplateID.String(), string(campaign.Status), campaign.ScheduledAt,
		campaign.StartedAt, campaign.CompletedAt, campaign.UpdatedAt, campaign.ID.String(), campaign.DeviceID)
	return err
}

func (r *Repository) DeleteCampaign(ctx context.Context, deviceID string, id uuid.UUID) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete targets and messages
	_, _ = tx.ExecContext(ctx, `DELETE FROM campaign_target_customers WHERE campaign_id = ?`, id.String())
	_, _ = tx.ExecContext(ctx, `DELETE FROM campaign_target_groups WHERE campaign_id = ?`, id.String())
	_, _ = tx.ExecContext(ctx, `DELETE FROM campaign_messages WHERE campaign_id = ?`, id.String())
	_, err = tx.ExecContext(ctx, `DELETE FROM campaigns WHERE id = ? AND device_id = ?`, id.String(), deviceID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *Repository) SetCampaignTargets(ctx context.Context, campaignID uuid.UUID, customerIDs, groupIDs []uuid.UUID) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Clear existing targets
	_, err = tx.ExecContext(ctx, `DELETE FROM campaign_target_customers WHERE campaign_id = ?`, campaignID.String())
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM campaign_target_groups WHERE campaign_id = ?`, campaignID.String())
	if err != nil {
		return err
	}

	// Add customer targets
	if len(customerIDs) > 0 {
		stmt, err := tx.PrepareContext(ctx, `INSERT INTO campaign_target_customers (campaign_id, customer_id) VALUES (?, ?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, customerID := range customerIDs {
			_, _ = stmt.ExecContext(ctx, campaignID.String(), customerID.String())
		}
	}

	// Add group targets
	if len(groupIDs) > 0 {
		stmt, err := tx.PrepareContext(ctx, `INSERT INTO campaign_target_groups (campaign_id, group_id) VALUES (?, ?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, groupID := range groupIDs {
			_, _ = stmt.ExecContext(ctx, campaignID.String(), groupID.String())
		}
	}

	return tx.Commit()
}

func (r *Repository) GetCampaignTargetIDs(ctx context.Context, campaignID uuid.UUID) (customerIDs, groupIDs []uuid.UUID, err error) {
	// Get customer IDs
	rows, err := r.db.QueryContext(ctx, `SELECT customer_id FROM campaign_target_customers WHERE campaign_id = ?`, campaignID.String())
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var idStr string
		if err := rows.Scan(&idStr); err != nil {
			continue
		}
		if id, err := uuid.Parse(idStr); err == nil {
			customerIDs = append(customerIDs, id)
		}
	}

	// Get group IDs
	rows2, err := r.db.QueryContext(ctx, `SELECT group_id FROM campaign_target_groups WHERE campaign_id = ?`, campaignID.String())
	if err != nil {
		return customerIDs, nil, err
	}
	defer rows2.Close()

	for rows2.Next() {
		var idStr string
		if err := rows2.Scan(&idStr); err != nil {
			continue
		}
		if id, err := uuid.Parse(idStr); err == nil {
			groupIDs = append(groupIDs, id)
		}
	}

	return customerIDs, groupIDs, nil
}

func (r *Repository) GetCampaignTargetCustomers(ctx context.Context, campaignID uuid.UUID) ([]*domainCampaign.Customer, error) {
	// Get directly targeted customers + customers from targeted groups
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT c.id, c.device_id, c.phone, c.full_name, c.country, c.gender, c.birth_year, c.created_at, c.updated_at
		FROM campaign_customers c
		WHERE c.id IN (
			SELECT customer_id FROM campaign_target_customers WHERE campaign_id = ?
			UNION
			SELECT gm.customer_id FROM campaign_group_members gm
			INNER JOIN campaign_target_groups tg ON gm.group_id = tg.group_id
			WHERE tg.campaign_id = ?
		)
	`, campaignID.String(), campaignID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var customers []*domainCampaign.Customer
	for rows.Next() {
		customer := &domainCampaign.Customer{}
		var idStr string
		if err := rows.Scan(&idStr, &customer.DeviceID, &customer.Phone, &customer.FullName,
			&customer.Country, &customer.Gender, &customer.BirthYear, &customer.CreatedAt, &customer.UpdatedAt); err != nil {
			return nil, err
		}
		customer.ID, _ = uuid.Parse(idStr)
		customers = append(customers, customer)
	}
	return customers, rows.Err()
}

func (r *Repository) GetCampaignStats(ctx context.Context, campaignID uuid.UUID) (*domainCampaign.CampaignStats, error) {
	stats := &domainCampaign.CampaignStats{}
	err := r.db.QueryRowContext(ctx, `
		SELECT 
			COUNT(*) as total,
			SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END) as pending,
			SUM(CASE WHEN status = 'sent' THEN 1 ELSE 0 END) as sent,
			SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed
		FROM campaign_messages WHERE campaign_id = ?
	`, campaignID.String()).Scan(&stats.TotalMessages, &stats.PendingMessages, &stats.SentMessages, &stats.FailedMessages)
	if err != nil {
		return nil, err
	}
	return stats, nil
}

// ============================================================================
// Queue Operations
// ============================================================================

func (r *Repository) EnqueueMessages(ctx context.Context, items []*domainCampaign.QueueItem) error {
	if len(items) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO campaign_messages (id, campaign_id, customer_id, device_id, phone, message, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(campaign_id, customer_id) DO NOTHING
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now()
	for _, item := range items {
		item.ID = uuid.New()
		item.Status = domainCampaign.MessageStatusPending
		item.CreatedAt = now
		item.UpdatedAt = now
		_, _ = stmt.ExecContext(ctx, item.ID.String(), item.CampaignID.String(), item.CustomerID.String(),
			item.DeviceID, item.Phone, item.Message, string(item.Status), item.CreatedAt, item.UpdatedAt)
	}

	return tx.Commit()
}

func (r *Repository) GetPendingMessages(ctx context.Context, deviceID string, limit int) ([]*domainCampaign.QueueItem, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT m.id, m.campaign_id, m.customer_id, m.device_id, m.phone, m.message, m.status, m.error, m.sent_at, m.created_at, m.updated_at
		FROM campaign_messages m
		INNER JOIN campaigns c ON m.campaign_id = c.id
		WHERE m.device_id = ? AND m.status = 'pending' AND c.status = 'running'
		ORDER BY m.created_at ASC
		LIMIT ?
	`, deviceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*domainCampaign.QueueItem
	for rows.Next() {
		item := &domainCampaign.QueueItem{}
		var idStr, campaignIDStr, customerIDStr, status string
		if err := rows.Scan(&idStr, &campaignIDStr, &customerIDStr, &item.DeviceID, &item.Phone,
			&item.Message, &status, &item.Error, &item.SentAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.ID, _ = uuid.Parse(idStr)
		item.CampaignID, _ = uuid.Parse(campaignIDStr)
		item.CustomerID, _ = uuid.Parse(customerIDStr)
		item.Status = domainCampaign.MessageStatus(status)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) UpdateMessageStatus(ctx context.Context, id uuid.UUID, status domainCampaign.MessageStatus, errorMsg *string) error {
	now := time.Now()
	var sentAt *time.Time
	if status == domainCampaign.MessageStatusSent {
		sentAt = &now
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE campaign_messages SET status = ?, error = ?, sent_at = ?, updated_at = ?
		WHERE id = ?
	`, string(status), errorMsg, sentAt, now, id.String())
	return err
}

func (r *Repository) IsMessageQueued(ctx context.Context, campaignID, customerID uuid.UUID) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM campaign_messages WHERE campaign_id = ? AND customer_id = ?
	`, campaignID.String(), customerID.String()).Scan(&count)
	return count > 0, err
}

// ============================================================================
// Short URL Operations
// ============================================================================

func (r *Repository) CreateShortURL(ctx context.Context, shortURL *domainCampaign.ShortURL) error {
	shortURL.ID = uuid.New()
	shortURL.CreatedAt = time.Now()
	shortURL.Clicks = 0

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO campaign_short_urls (id, device_id, code, original_url, clicks, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, shortURL.ID.String(), shortURL.DeviceID, shortURL.Code, shortURL.OriginalURL, shortURL.Clicks, shortURL.CreatedAt)
	return err
}

func (r *Repository) GetShortURLByCode(ctx context.Context, code string) (*domainCampaign.ShortURL, error) {
	shortURL := &domainCampaign.ShortURL{}
	var idStr string
	err := r.db.QueryRowContext(ctx, `
		SELECT id, device_id, code, original_url, clicks, created_at
		FROM campaign_short_urls WHERE code = ?
	`, code).Scan(&idStr, &shortURL.DeviceID, &shortURL.Code, &shortURL.OriginalURL, &shortURL.Clicks, &shortURL.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	shortURL.ID, _ = uuid.Parse(idStr)
	return shortURL, nil
}

func (r *Repository) IncrementShortURLClicks(ctx context.Context, code string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE campaign_short_urls SET clicks = clicks + 1 WHERE code = ?`, code)
	return err
}

// ============================================================================
// Validation Operations
// ============================================================================

func (r *Repository) GetCustomersForValidation(ctx context.Context, deviceID string, limit int) ([]*domainCampaign.Customer, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, device_id, phone, full_name, company, country, gender, birth_year, 
			   phone_valid, whatsapp_exists, created_at, updated_at
		FROM campaign_customers 
		WHERE device_id = ? AND (phone_valid = 'pending' OR whatsapp_exists = 'pending')
		ORDER BY created_at ASC
		LIMIT ?
	`, deviceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var customers []*domainCampaign.Customer
	for rows.Next() {
		customer := &domainCampaign.Customer{}
		var idStr string
		var phoneValid, whatsappExists string
		if err := rows.Scan(&idStr, &customer.DeviceID, &customer.Phone, &customer.FullName,
			&customer.Company, &customer.Country, &customer.Gender, &customer.BirthYear,
			&phoneValid, &whatsappExists, &customer.CreatedAt, &customer.UpdatedAt); err != nil {
			return nil, err
		}
		customer.ID, _ = uuid.Parse(idStr)
		customer.PhoneValid = domainCampaign.ValidationStatus(phoneValid)
		customer.WhatsAppExists = domainCampaign.ValidationStatus(whatsappExists)
		customer.IsReady = customer.PhoneValid == domainCampaign.ValidationStatusValid &&
			customer.WhatsAppExists == domainCampaign.ValidationStatusValid
		customers = append(customers, customer)
	}
	return customers, rows.Err()
}

func (r *Repository) UpdateCustomerValidation(ctx context.Context, id uuid.UUID, phoneValid, whatsappExists domainCampaign.ValidationStatus) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx, `
		UPDATE campaign_customers SET phone_valid = ?, whatsapp_exists = ?, updated_at = ?
		WHERE id = ?
	`, string(phoneValid), string(whatsappExists), now, id.String())
	return err
}

func (r *Repository) GetActiveDeviceIDs(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT device_id FROM campaign_messages WHERE status = 'pending'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deviceIDs []string
	for rows.Next() {
		var deviceID string
		if err := rows.Scan(&deviceID); err != nil {
			return nil, err
		}
		deviceIDs = append(deviceIDs, deviceID)
	}
	return deviceIDs, rows.Err()
}
