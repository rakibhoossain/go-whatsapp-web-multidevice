package services

import (
	"fmt"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	domainGroup "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/group"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/auth"
	pkgError "github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/error"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/whatsapp"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/validations"
	"github.com/gofiber/fiber/v2"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

type groupService struct {
	Clients *map[string]*whatsapp.WhatsAppTenantClient
}

func NewGroupService(clients *map[string]*whatsapp.WhatsAppTenantClient) domainGroup.IGroupService {
	return &groupService{
		Clients: clients,
	}
}

func (service groupService) JoinGroupWithLink(c *fiber.Ctx, request domainGroup.JoinGroupWithLinkRequest) (groupID string, err error) {

	authPayload, err := auth.AuthPayload(c)
	if err != nil {
		return groupID, err
	}

	tenantClient, err := whatsapp.GetWhatsappTenantClient(service.Clients, authPayload.User)
	if err != nil {
		return groupID, err
	}

	if err = validations.ValidateJoinGroupWithLink(c.UserContext(), request); err != nil {
		return groupID, err
	}

	whatsapp.MustLogin(tenantClient.Conn)

	jid, err := tenantClient.Conn.JoinGroupWithLink(request.Link)
	if err != nil {
		return
	}
	return jid.String(), nil
}

func (service groupService) LeaveGroup(c *fiber.Ctx, request domainGroup.LeaveGroupRequest) (err error) {

	authPayload, err := auth.AuthPayload(c)
	if err != nil {
		return err
	}

	tenantClient, err := whatsapp.GetWhatsappTenantClient(service.Clients, authPayload.User)
	if err != nil {
		return err
	}

	if err = validations.ValidateLeaveGroup(c.UserContext(), request); err != nil {
		return err
	}

	JID, err := whatsapp.ValidateJidWithLogin(tenantClient.Conn, request.GroupID)
	if err != nil {
		return err
	}

	return tenantClient.Conn.LeaveGroup(JID)
}

func (service groupService) CreateGroup(c *fiber.Ctx, request domainGroup.CreateGroupRequest) (groupID string, err error) {

	authPayload, err := auth.AuthPayload(c)
	if err != nil {
		return groupID, err
	}

	tenantClient, err := whatsapp.GetWhatsappTenantClient(service.Clients, authPayload.User)
	if err != nil {
		return groupID, err
	}

	if err = validations.ValidateCreateGroup(c.UserContext(), request); err != nil {
		return groupID, err
	}
	whatsapp.MustLogin(tenantClient.Conn)

	participantsJID, err := service.participantToJID(tenantClient.Conn, request.Participants)
	if err != nil {
		return
	}

	groupConfig := whatsmeow.ReqCreateGroup{
		Name:              request.Title,
		Participants:      participantsJID,
		GroupParent:       types.GroupParent{},
		GroupLinkedParent: types.GroupLinkedParent{},
	}

	groupInfo, err := tenantClient.Conn.CreateGroup(groupConfig)
	if err != nil {
		return
	}

	return groupInfo.JID.String(), nil
}

func (service groupService) ManageParticipant(c *fiber.Ctx, request domainGroup.ParticipantRequest) (result []domainGroup.ParticipantStatus, err error) {

	authPayload, err := auth.AuthPayload(c)
	if err != nil {
		return result, err
	}

	tenantClient, err := whatsapp.GetWhatsappTenantClient(service.Clients, authPayload.User)
	if err != nil {
		return result, err
	}

	if err = validations.ValidateParticipant(c.UserContext(), request); err != nil {
		return result, err
	}
	whatsapp.MustLogin(tenantClient.Conn)

	groupJID, err := whatsapp.ValidateJidWithLogin(tenantClient.Conn, request.GroupID)
	if err != nil {
		return result, err
	}

	participantsJID, err := service.participantToJID(tenantClient.Conn, request.Participants)
	if err != nil {
		return result, err
	}

	participants, err := tenantClient.Conn.UpdateGroupParticipants(groupJID, participantsJID, request.Action)
	if err != nil {
		return result, err
	}

	for _, participant := range participants {
		if participant.Error == 403 && participant.AddRequest != nil {
			result = append(result, domainGroup.ParticipantStatus{
				Participant: participant.JID.String(),
				Status:      "error",
				Message:     "Failed to add participant",
			})
		} else {
			result = append(result, domainGroup.ParticipantStatus{
				Participant: participant.JID.String(),
				Status:      "success",
				Message:     "Action success",
			})
		}
	}

	return result, nil
}

func (service groupService) GetGroupRequestParticipants(c *fiber.Ctx, request domainGroup.GetGroupRequestParticipantsRequest) (result []domainGroup.GetGroupRequestParticipantsResponse, err error) {

	authPayload, err := auth.AuthPayload(c)
	if err != nil {
		return result, err
	}

	tenantClient, err := whatsapp.GetWhatsappTenantClient(service.Clients, authPayload.User)
	if err != nil {
		return result, err
	}

	if err = validations.ValidateGetGroupRequestParticipants(c.UserContext(), request); err != nil {
		return result, err
	}

	groupJID, err := whatsapp.ValidateJidWithLogin(tenantClient.Conn, request.GroupID)
	if err != nil {
		return result, err
	}

	participants, err := tenantClient.Conn.GetGroupRequestParticipants(groupJID)
	if err != nil {
		return result, err
	}

	for _, participant := range participants {
		result = append(result, domainGroup.GetGroupRequestParticipantsResponse{
			JID:         participant.JID.String(),
			RequestedAt: participant.RequestedAt,
		})
	}

	return result, nil
}

func (service groupService) ManageGroupRequestParticipants(c *fiber.Ctx, request domainGroup.GroupRequestParticipantsRequest) (result []domainGroup.ParticipantStatus, err error) {

	authPayload, err := auth.AuthPayload(c)
	if err != nil {
		return result, err
	}

	tenantClient, err := whatsapp.GetWhatsappTenantClient(service.Clients, authPayload.User)
	if err != nil {
		return result, err
	}

	if err = validations.ValidateManageGroupRequestParticipants(c.UserContext(), request); err != nil {
		return result, err
	}

	groupJID, err := whatsapp.ValidateJidWithLogin(tenantClient.Conn, request.GroupID)
	if err != nil {
		return result, err
	}

	participantsJID, err := service.participantToJID(tenantClient.Conn, request.Participants)
	if err != nil {
		return result, err
	}

	participants, err := tenantClient.Conn.UpdateGroupRequestParticipants(groupJID, participantsJID, request.Action)
	if err != nil {
		return result, err
	}

	for _, participant := range participants {
		if participant.Error != 0 {
			result = append(result, domainGroup.ParticipantStatus{
				Participant: participant.JID.String(),
				Status:      "error",
				Message:     fmt.Sprintf("Action %s failed (code %d)", request.Action, participant.Error),
			})
		} else {
			result = append(result, domainGroup.ParticipantStatus{
				Participant: participant.JID.String(),
				Status:      "success",
				Message:     fmt.Sprintf("Action %s success", request.Action),
			})
		}
	}

	return result, nil
}

func (service groupService) participantToJID(waCli *whatsmeow.Client, participants []string) ([]types.JID, error) {

	var participantsJID []types.JID
	for _, participant := range participants {
		formattedParticipant := participant + config.WhatsappTypeUser

		if !whatsapp.IsOnWhatsapp(waCli, formattedParticipant) {
			return nil, pkgError.ErrUserNotRegistered
		}

		if participantJID, err := types.ParseJID(formattedParticipant); err == nil {
			participantsJID = append(participantsJID, participantJID)
		}
	}
	return participantsJID, nil
}
