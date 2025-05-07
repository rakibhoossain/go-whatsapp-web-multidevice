package whatsapp

import (
	"fmt"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types/events"
)

type MediaInfo struct {
	URL       *string
	MimeType  *string
	Size      *uint64
	Duration  *uint32
	Caption   *string
	Thumbnail string
}

func extractMessageContent(msg *waE2E.Message) (string, string, MediaInfo) {
	switch {
	case msg.Conversation != nil:
		return "text", *msg.Conversation, MediaInfo{}
	case msg.ExtendedTextMessage != nil:
		extended := msg.ExtendedTextMessage
		if extended.Text == nil {
			return "text", "", MediaInfo{}
		}
		return "text", *extended.Text, MediaInfo{}
	case msg.ImageMessage != nil:
		img := msg.ImageMessage
		return "image", "", MediaInfo{
			URL:       img.URL,
			MimeType:  img.Mimetype,
			Size:      img.FileLength,
			Caption:   img.Caption,
			Thumbnail: string(img.JPEGThumbnail),
		}
	// Handle other message types (video, audio, document, etc.)
	default:
		return "unknown", "", MediaInfo{}
	}
}

func StoreMessageOnDatabase(user *WhatsAppTenantUser, evt *events.Message) {

	messageType, content, mediaInfo := extractMessageContent(evt.Message)

	_, err := Db.Exec(`
		INSERT INTO messages (
			user_reference_id, chat_jid, sender_jid, message_id, timestamp, 
			message_type, content, media_url, media_mime_type,
			media_size, media_duration, media_caption, media_thumbnail,
			status, is_from_me, is_forwarded, is_ephemeral, is_view_once
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		ON CONFLICT (message_id) DO UPDATE SET
			status = EXCLUDED.status,
			is_deleted = EXCLUDED.is_deleted
	
		`,
		user.ID,
		evt.Info.Chat.User,
		evt.Info.Sender.String(),
		evt.Info.ID,
		evt.Info.Timestamp,
		messageType,
		content,
		mediaInfo.URL,
		mediaInfo.MimeType,
		mediaInfo.Size,
		mediaInfo.Duration,
		mediaInfo.Caption,
		mediaInfo.Thumbnail,
		"delivered",
		evt.Info.IsFromMe,
		buildForwarded(evt),
		evt.IsEphemeral,
		evt.IsViewOnce,
	)

	if err != nil {
		fmt.Printf("Failed to update chat last message: %v", err)
	}
}
