package group

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"go.mau.fi/whatsmeow"
)

type IGroupService interface {
	JoinGroupWithLink(c *fiber.Ctx, request JoinGroupWithLinkRequest) (groupID string, err error)
	LeaveGroup(c *fiber.Ctx, request LeaveGroupRequest) (err error)
	CreateGroup(c *fiber.Ctx, request CreateGroupRequest) (groupID string, err error)
	ManageParticipant(c *fiber.Ctx, request ParticipantRequest) (result []ParticipantStatus, err error)
	GetGroupRequestParticipants(c *fiber.Ctx, request GetGroupRequestParticipantsRequest) (result []GetGroupRequestParticipantsResponse, err error)
	ManageGroupRequestParticipants(c *fiber.Ctx, request GroupRequestParticipantsRequest) (result []ParticipantStatus, err error)
}

type JoinGroupWithLinkRequest struct {
	Link string `json:"link" form:"link"`
}

type LeaveGroupRequest struct {
	GroupID string `json:"group_id" form:"group_id"`
}

type CreateGroupRequest struct {
	Title        string   `json:"title" form:"title"`
	Participants []string `json:"participants" form:"participants"`
}

type ParticipantRequest struct {
	GroupID      string                      `json:"group_id" form:"group_id"`
	Participants []string                    `json:"participants" form:"participants"`
	Action       whatsmeow.ParticipantChange `json:"action" form:"action"`
}

type ParticipantStatus struct {
	Participant string `json:"participant"`
	Status      string `json:"status"`
	Message     string `json:"message"`
}

type GetGroupRequestParticipantsRequest struct {
	GroupID string `json:"group_id" query:"group_id"`
}

type GetGroupRequestParticipantsResponse struct {
	JID         string    `json:"jid"`
	RequestedAt time.Time `json:"requested_at"`
}

type GroupRequestParticipantsRequest struct {
	GroupID      string                             `json:"group_id" form:"group_id"`
	Participants []string                           `json:"participants" form:"participants"`
	Action       whatsmeow.ParticipantRequestChange `json:"action" form:"action"`
}
