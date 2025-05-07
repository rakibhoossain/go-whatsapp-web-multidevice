package services

import (
	"context"
	"fmt"
	"time"

	domainMessage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/message"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/auth"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/whatsapp"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/validations"
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/proto/waSyncAction"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

type serviceMessage struct {
	Clients *map[string]*whatsapp.WhatsAppTenantClient
}

func NewMessageService(clients *map[string]*whatsapp.WhatsAppTenantClient) domainMessage.IMessageService {
	return &serviceMessage{
		Clients: clients,
	}
}

func (service serviceMessage) MarkAsRead(c *fiber.Ctx, request domainMessage.MarkAsReadRequest) (response domainMessage.GenericResponse, err error) {

	authPayload, err := auth.AuthPayload(c)
	if err != nil {
		return response, err
	}

	tenantClient, err := whatsapp.GetWhatsappTenantClient(service.Clients, authPayload.User)
	if err != nil {
		return response, err
	}

	if err = validations.ValidateMarkAsRead(c.UserContext(), request); err != nil {
		return response, err
	}
	dataWaRecipient, err := whatsapp.ValidateJidWithLogin(tenantClient.Conn, request.Phone)
	if err != nil {
		return response, err
	}

	ids := []types.MessageID{request.MessageID}
	if err = tenantClient.Conn.MarkRead(ids, time.Now(), dataWaRecipient, *tenantClient.Conn.Store.ID); err != nil {
		return response, err
	}

	logrus.Info(map[string]interface{}{
		"phone":      request.Phone,
		"message_id": request.MessageID,
		"chat":       dataWaRecipient.String(),
		"sender":     tenantClient.Conn.Store.ID.String(),
	})

	response.MessageID = request.MessageID
	response.Status = fmt.Sprintf("Mark as read success %s", request.MessageID)
	return response, nil
}

func (service serviceMessage) GetAllChatMessage(c *fiber.Ctx, request domainMessage.ChatMessageRequest) (response domainMessage.ChatMessageResponse, err error) {
	// Authentication
	authPayload, err := auth.AuthPayload(c)
	if err != nil {
		return response, fmt.Errorf("authentication failed: %w", err)
	}

	_, err = whatsapp.GetWhatsappTenantClient(service.Clients, authPayload.User)
	if err != nil {
		return response, fmt.Errorf("whatsapp client error: %w", err)
	}

	// Build query with pagination
	query := `
        SELECT 
            m.id, 
            m.sender_jid, 
            m.timestamp, 
            m.content,
			m.is_from_me
        FROM messages m
        WHERE m.chat_jid = $1
        ORDER BY m.timestamp DESC
        LIMIT $2 OFFSET $3
    `

	// Set default pagination values if not provided
	if request.Limit <= 0 {
		request.Limit = 50 // Default page size
	}
	if request.Offset < 0 {
		request.Offset = 0
	}

	rows, err := whatsapp.Db.Query(query, request.ChatID, request.Limit, request.Offset)
	if err != nil {
		return response, fmt.Errorf("database query failed: %w", err)
	}
	defer rows.Close()

	messages := []domainMessage.Message{}
	for rows.Next() {
		var msg domainMessage.Message

		err := rows.Scan(
			&msg.ID,
			&msg.SenderJID,
			&msg.Timestamp,
			&msg.Content,
			&msg.IsFromMe,
		)
		if err != nil {
			fmt.Printf("Failed to scan message row: %v", err)
			continue
		}

		messages = append(messages, msg)
	}

	// Check for any iteration errors
	if err = rows.Err(); err != nil {
		return response, fmt.Errorf("row iteration error: %w", err)
	}

	response.Data = messages
	response.ChatID = request.ChatID

	return response, nil
}

func (service serviceMessage) ReactMessage(c *fiber.Ctx, request domainMessage.ReactionRequest) (response domainMessage.GenericResponse, err error) {

	authPayload, err := auth.AuthPayload(c)
	if err != nil {
		return response, err
	}

	tenantClient, err := whatsapp.GetWhatsappTenantClient(service.Clients, authPayload.User)
	if err != nil {
		return response, err
	}

	if err = validations.ValidateReactMessage(c.UserContext(), request); err != nil {
		return response, err
	}
	dataWaRecipient, err := whatsapp.ValidateJidWithLogin(tenantClient.Conn, request.Phone)
	if err != nil {
		return response, err
	}

	msg := &waE2E.Message{
		ReactionMessage: &waE2E.ReactionMessage{
			Key: &waCommon.MessageKey{
				FromMe:    proto.Bool(true),
				ID:        proto.String(request.MessageID),
				RemoteJID: proto.String(dataWaRecipient.String()),
			},
			Text:              proto.String(request.Emoji),
			SenderTimestampMS: proto.Int64(time.Now().UnixMilli()),
		},
	}
	ts, err := tenantClient.Conn.SendMessage(c.UserContext(), dataWaRecipient, msg)
	if err != nil {
		return response, err
	}

	response.MessageID = ts.ID
	response.Status = fmt.Sprintf("Reaction sent to %s (server timestamp: %s)", request.Phone, ts.Timestamp)
	return response, nil
}

func (service serviceMessage) RevokeMessage(c *fiber.Ctx, request domainMessage.RevokeRequest) (response domainMessage.GenericResponse, err error) {

	authPayload, err := auth.AuthPayload(c)
	if err != nil {
		return response, err
	}

	tenantClient, err := whatsapp.GetWhatsappTenantClient(service.Clients, authPayload.User)
	if err != nil {
		return response, err
	}

	if err = validations.ValidateRevokeMessage(c.UserContext(), request); err != nil {
		return response, err
	}
	dataWaRecipient, err := whatsapp.ValidateJidWithLogin(tenantClient.Conn, request.Phone)
	if err != nil {
		return response, err
	}

	ts, err := tenantClient.Conn.SendMessage(context.Background(), dataWaRecipient, tenantClient.Conn.BuildRevoke(dataWaRecipient, types.EmptyJID, request.MessageID))
	if err != nil {
		return response, err
	}

	response.MessageID = ts.ID
	response.Status = fmt.Sprintf("Revoke success %s (server timestamp: %s)", request.Phone, ts.Timestamp)
	return response, nil
}

func (service serviceMessage) DeleteMessage(c *fiber.Ctx, request domainMessage.DeleteRequest) (err error) {

	authPayload, err := auth.AuthPayload(c)
	if err != nil {
		return err
	}

	tenantClient, err := whatsapp.GetWhatsappTenantClient(service.Clients, authPayload.User)
	if err != nil {
		return err
	}

	if err = validations.ValidateDeleteMessage(c.UserContext(), request); err != nil {
		return err
	}
	dataWaRecipient, err := whatsapp.ValidateJidWithLogin(tenantClient.Conn, request.Phone)
	if err != nil {
		return err
	}

	isFromMe := "1"
	if len(request.MessageID) > 22 {
		isFromMe = "0"
	}

	patchInfo := appstate.PatchInfo{
		Timestamp: time.Now(),
		Type:      appstate.WAPatchRegularHigh,
		Mutations: []appstate.MutationInfo{{
			Index: []string{appstate.IndexDeleteMessageForMe, dataWaRecipient.String(), request.MessageID, isFromMe, tenantClient.Conn.Store.ID.String()},
			Value: &waSyncAction.SyncActionValue{
				DeleteMessageForMeAction: &waSyncAction.DeleteMessageForMeAction{
					DeleteMedia:      proto.Bool(true),
					MessageTimestamp: proto.Int64(time.Now().UnixMilli()),
				},
			},
		}},
	}

	if err = tenantClient.Conn.SendAppState(patchInfo); err != nil {
		return err
	}
	return nil
}

func (service serviceMessage) UpdateMessage(c *fiber.Ctx, request domainMessage.UpdateMessageRequest) (response domainMessage.GenericResponse, err error) {

	authPayload, err := auth.AuthPayload(c)
	if err != nil {
		return response, err
	}

	tenantClient, err := whatsapp.GetWhatsappTenantClient(service.Clients, authPayload.User)
	if err != nil {
		return response, err
	}

	if err = validations.ValidateUpdateMessage(c.UserContext(), request); err != nil {
		return response, err
	}

	dataWaRecipient, err := whatsapp.ValidateJidWithLogin(tenantClient.Conn, request.Phone)
	if err != nil {
		return response, err
	}

	msg := &waE2E.Message{Conversation: proto.String(request.Message)}
	ts, err := tenantClient.Conn.SendMessage(context.Background(), dataWaRecipient, tenantClient.Conn.BuildEdit(dataWaRecipient, request.MessageID, msg))
	if err != nil {
		return response, err
	}

	response.MessageID = ts.ID
	response.Status = fmt.Sprintf("Update message success %s (server timestamp: %s)", request.Phone, ts.Timestamp)
	return response, nil
}

// StarMessage implements message.IMessageService.
func (service serviceMessage) StarMessage(c *fiber.Ctx, request domainMessage.StarRequest) (err error) {

	authPayload, err := auth.AuthPayload(c)
	if err != nil {
		return err
	}

	tenantClient, err := whatsapp.GetWhatsappTenantClient(service.Clients, authPayload.User)
	if err != nil {
		return err
	}

	if err = validations.ValidateStarMessage(c.UserContext(), request); err != nil {
		return err
	}

	dataWaRecipient, err := whatsapp.ValidateJidWithLogin(tenantClient.Conn, request.Phone)
	if err != nil {
		return err
	}

	isFromMe := true
	if len(request.MessageID) > 22 {
		isFromMe = false
	}

	patchInfo := appstate.BuildStar(dataWaRecipient.ToNonAD(), *tenantClient.Conn.Store.ID, request.MessageID, isFromMe, request.IsStarred)

	if err = tenantClient.Conn.SendAppState(patchInfo); err != nil {
		return err
	}
	return nil
}
