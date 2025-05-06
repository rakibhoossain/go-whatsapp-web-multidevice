package services

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/domains/app"
	domainSend "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/send"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/internal/rest/helpers"
	pkgError "github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/error"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/whatsapp"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/validations"
	"github.com/disintegration/imaging"
	"github.com/gofiber/fiber/v2"
	fiberUtils "github.com/gofiber/fiber/v2/utils"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/auth"
)

type serviceSend struct {
	Clients    *map[string]*whatsapp.WhatsAppTenantClient
	appService app.IAppService
}

func NewSendService(clients *map[string]*whatsapp.WhatsAppTenantClient, appService app.IAppService) domainSend.ISendService {
	return &serviceSend{
		Clients:    clients,
		appService: appService,
	}
}

// wrapSendMessage wraps the message sending process with message ID saving
func (service serviceSend) wrapSendMessage(c *fiber.Ctx, recipient types.JID, msg *waE2E.Message, content string) (whatsmeow.SendResponse, error) {

	authPayload, err := auth.AuthPayload(c)
	if err != nil {
		return whatsmeow.SendResponse{}, err
	}

	tenantClient, err := whatsapp.GetWhatsappTenantClient(service.Clients, authPayload.User)
	if err != nil {
		return whatsmeow.SendResponse{}, err
	}

	ts, err := tenantClient.Conn.SendMessage(c.UserContext(), recipient, msg)
	if err != nil {
		return whatsmeow.SendResponse{}, err
	}

	utils.RecordMessage(ts.ID, tenantClient.Conn.Store.ID.String(), content)

	return ts, nil
}

func (service serviceSend) SendText(c *fiber.Ctx, request domainSend.MessageRequest) (response domainSend.GenericResponse, err error) {
	
	authPayload, err := auth.AuthPayload(c)
	if err != nil {
		return response, err
	}

	tenantClient, err := whatsapp.GetWhatsappTenantClient(service.Clients, authPayload.User)
	if err != nil {
		return response, err
	}

	err = validations.ValidateSendMessage(c.UserContext(), request)
	if err != nil {
		return response, err
	}
	dataWaRecipient, err := whatsapp.ValidateJidWithLogin(tenantClient.Conn, request.Phone)
	if err != nil {
		return response, err
	}

	// Create base message
	msg := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text:        proto.String(request.Message),
			ContextInfo: &waE2E.ContextInfo{},
		},
	}

	// Add forwarding context if IsForwarded is true
	if request.IsForwarded {
		msg.ExtendedTextMessage.ContextInfo.IsForwarded = proto.Bool(true)
		msg.ExtendedTextMessage.ContextInfo.ForwardingScore = proto.Uint32(100)
	}

	parsedMentions := service.getMentionFromText(c, request.Message)
	if len(parsedMentions) > 0 {
		msg.ExtendedTextMessage.ContextInfo.MentionedJID = parsedMentions
	}

	// Reply message
	if request.ReplyMessageID != nil && *request.ReplyMessageID != "" {
		record, err := utils.FindRecordFromStorage(*request.ReplyMessageID)
		if err == nil { // Only set reply context if we found the message ID
			msg.ExtendedTextMessage = &waE2E.ExtendedTextMessage{
				Text: proto.String(request.Message),
				ContextInfo: &waE2E.ContextInfo{
					StanzaID:    request.ReplyMessageID,
					Participant: proto.String(record.JID),
					QuotedMessage: &waE2E.Message{
						Conversation: proto.String(record.MessageContent),
					},
				},
			}

			if len(parsedMentions) > 0 {
				msg.ExtendedTextMessage.ContextInfo.MentionedJID = parsedMentions
			}
		} else {
			logrus.Warnf("Reply message ID %s not found in storage, continuing without reply context", *request.ReplyMessageID)
		}
	}

	ts, err := service.wrapSendMessage(c, dataWaRecipient, msg, request.Message)
	if err != nil {
		return response, err
	}

	response.MessageID = ts.ID
	response.Status = fmt.Sprintf("Message sent to %s (server timestamp: %s)", request.Phone, ts.Timestamp.String())
	return response, nil
}

func (service serviceSend) SendImage(c *fiber.Ctx, request domainSend.ImageRequest) (response domainSend.GenericResponse, err error) {
	
	authPayload, err := auth.AuthPayload(c)
	if err != nil {
		return response, err
	}

	tenantClient, err := whatsapp.GetWhatsappTenantClient(service.Clients, authPayload.User)
	if err != nil {
		return response, err
	}
	
	err = validations.ValidateSendImage(c.UserContext(), request)
	if err != nil {
		return response, err
	}
	dataWaRecipient, err := whatsapp.ValidateJidWithLogin(tenantClient.Conn, request.Phone)
	if err != nil {
		return response, err
	}

	var (
		imagePath      string
		imageThumbnail string
		imageName      string
		deletedItems   []string
		oriImagePath   string
	)

	if request.ImageURL != nil && *request.ImageURL != "" {
		// Download image from URL
		imageData, fileName, err := utils.DownloadImageFromURL(*request.ImageURL)
		oriImagePath = fmt.Sprintf("%s/%s", config.PathSendItems, fileName)
		if err != nil {
			return response, pkgError.InternalServerError(fmt.Sprintf("failed to download image from URL %v", err))
		}
		imageName = fileName
		err = os.WriteFile(oriImagePath, imageData, 0644)
		if err != nil {
			return response, pkgError.InternalServerError(fmt.Sprintf("failed to save downloaded image %v", err))
		}
	} else if request.Image != nil {
		// Save image to server
		oriImagePath = fmt.Sprintf("%s/%s", config.PathSendItems, request.Image.Filename)
		err = fasthttp.SaveMultipartFile(request.Image, oriImagePath)
		if err != nil {
			return response, err
		}
		imageName = request.Image.Filename
	}
	deletedItems = append(deletedItems, oriImagePath)

	/* Generate thumbnail with smalled image size */
	srcImage, err := imaging.Open(oriImagePath)
	if err != nil {
		return response, pkgError.InternalServerError(fmt.Sprintf("failed to open image %v", err))
	}

	// Resize Thumbnail
	resizedImage := imaging.Resize(srcImage, 100, 0, imaging.Lanczos)
	imageThumbnail = fmt.Sprintf("%s/thumbnails-%s", config.PathSendItems, imageName)
	if err = imaging.Save(resizedImage, imageThumbnail); err != nil {
		return response, pkgError.InternalServerError(fmt.Sprintf("failed to save thumbnail %v", err))
	}
	deletedItems = append(deletedItems, imageThumbnail)

	if request.Compress {
		// Resize image
		openImageBuffer, err := imaging.Open(oriImagePath)
		if err != nil {
			return response, pkgError.InternalServerError(fmt.Sprintf("failed to open image %v", err))
		}
		newImage := imaging.Resize(openImageBuffer, 600, 0, imaging.Lanczos)
		newImagePath := fmt.Sprintf("%s/new-%s", config.PathSendItems, imageName)
		if err = imaging.Save(newImage, newImagePath); err != nil {
			return response, pkgError.InternalServerError(fmt.Sprintf("failed to save image %v", err))
		}
		deletedItems = append(deletedItems, newImagePath)
		imagePath = newImagePath
	} else {
		imagePath = oriImagePath
	}

	// Send to WA server
	dataWaCaption := request.Caption
	dataWaImage, err := os.ReadFile(imagePath)
	if err != nil {
		return response, err
	}
	uploadedImage, err := service.uploadMedia(c, whatsmeow.MediaImage, dataWaImage, dataWaRecipient)
	if err != nil {
		fmt.Printf("failed to upload file: %v", err)
		return response, err
	}
	dataWaThumbnail, err := os.ReadFile(imageThumbnail)
	if err != nil {
		return response, pkgError.InternalServerError(fmt.Sprintf("failed to read thumbnail %v", err))
	}

	msg := &waE2E.Message{ImageMessage: &waE2E.ImageMessage{
		JPEGThumbnail: dataWaThumbnail,
		Caption:       proto.String(dataWaCaption),
		URL:           proto.String(uploadedImage.URL),
		DirectPath:    proto.String(uploadedImage.DirectPath),
		MediaKey:      uploadedImage.MediaKey,
		Mimetype:      proto.String(http.DetectContentType(dataWaImage)),
		FileEncSHA256: uploadedImage.FileEncSHA256,
		FileSHA256:    uploadedImage.FileSHA256,
		FileLength:    proto.Uint64(uint64(len(dataWaImage))),
		ViewOnce:      proto.Bool(request.ViewOnce),
	}}

	if request.IsForwarded {
		msg.ImageMessage.ContextInfo = &waE2E.ContextInfo{
			IsForwarded:     proto.Bool(true),
			ForwardingScore: proto.Uint32(100),
		}
	}

	caption := "🖼️ Image"
	if request.Caption != "" {
		caption = "🖼️ " + request.Caption
	}
	ts, err := service.wrapSendMessage(c, dataWaRecipient, msg, caption)
	go func() {
		errDelete := utils.RemoveFile(0, deletedItems...)
		if errDelete != nil {
			fmt.Println("error when deleting picture: ", errDelete)
		}
	}()
	if err != nil {
		return response, err
	}

	response.MessageID = ts.ID
	response.Status = fmt.Sprintf("Message sent to %s (server timestamp: %s)", request.Phone, ts.Timestamp.String())
	return response, nil
}

func (service serviceSend) SendFile(c *fiber.Ctx, request domainSend.FileRequest) (response domainSend.GenericResponse, err error) {
	

	authPayload, err := auth.AuthPayload(c)
	if err != nil {
		return response, err
	}

	tenantClient, err := whatsapp.GetWhatsappTenantClient(service.Clients, authPayload.User)
	if err != nil {
		return response, err
	}
	
	err = validations.ValidateSendFile(c.UserContext(), request)
	if err != nil {
		return response, err
	}
	dataWaRecipient, err := whatsapp.ValidateJidWithLogin(tenantClient.Conn, request.Phone)
	if err != nil {
		return response, err
	}

	fileBytes := helpers.MultipartFormFileHeaderToBytes(request.File)
	fileMimeType := http.DetectContentType(fileBytes)

	// Send to WA server
	uploadedFile, err := service.uploadMedia(c, whatsmeow.MediaDocument, fileBytes, dataWaRecipient)
	if err != nil {
		fmt.Printf("Failed to upload file: %v", err)
		return response, err
	}

	msg := &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{
		URL:           proto.String(uploadedFile.URL),
		Mimetype:      proto.String(fileMimeType),
		Title:         proto.String(request.File.Filename),
		FileSHA256:    uploadedFile.FileSHA256,
		FileLength:    proto.Uint64(uploadedFile.FileLength),
		MediaKey:      uploadedFile.MediaKey,
		FileName:      proto.String(request.File.Filename),
		FileEncSHA256: uploadedFile.FileEncSHA256,
		DirectPath:    proto.String(uploadedFile.DirectPath),
		Caption:       proto.String(request.Caption),
	}}

	if request.IsForwarded {
		msg.DocumentMessage.ContextInfo = &waE2E.ContextInfo{
			IsForwarded:     proto.Bool(true),
			ForwardingScore: proto.Uint32(100),
		}
	}

	caption := "📄 Document"
	if request.Caption != "" {
		caption = "📄 " + request.Caption
	}
	ts, err := service.wrapSendMessage(c, dataWaRecipient, msg, caption)
	if err != nil {
		return response, err
	}

	response.MessageID = ts.ID
	response.Status = fmt.Sprintf("Document sent to %s (server timestamp: %s)", request.Phone, ts.Timestamp.String())
	return response, nil
}

func (service serviceSend) SendVideo(c *fiber.Ctx, request domainSend.VideoRequest) (response domainSend.GenericResponse, err error) {
	

	authPayload, err := auth.AuthPayload(c)
	if err != nil {
		return response, err
	}

	tenantClient, err := whatsapp.GetWhatsappTenantClient(service.Clients, authPayload.User)
	if err != nil {
		return response, err
	}
	
	err = validations.ValidateSendVideo(c.UserContext(), request)
	if err != nil {
		return response, err
	}
	dataWaRecipient, err := whatsapp.ValidateJidWithLogin(tenantClient.Conn, request.Phone)
	if err != nil {
		return response, err
	}

	var (
		videoPath      string
		videoThumbnail string
		deletedItems   []string
	)

	generateUUID := fiberUtils.UUIDv4()
	// Save video to server
	oriVideoPath := fmt.Sprintf("%s/%s", config.PathSendItems, generateUUID+request.Video.Filename)
	err = fasthttp.SaveMultipartFile(request.Video, oriVideoPath)
	if err != nil {
		return response, pkgError.InternalServerError(fmt.Sprintf("failed to store video in server %v", err))
	}

	// Check if ffmpeg is installed
	_, err = exec.LookPath("ffmpeg")
	if err != nil {
		return response, pkgError.InternalServerError("ffmpeg not installed")
	}

	// Get thumbnail video with ffmpeg
	thumbnailVideoPath := fmt.Sprintf("%s/%s", config.PathSendItems, generateUUID+".png")
	cmdThumbnail := exec.Command("ffmpeg", "-i", oriVideoPath, "-ss", "00:00:01.000", "-vframes", "1", thumbnailVideoPath)
	err = cmdThumbnail.Run()
	if err != nil {
		return response, pkgError.InternalServerError(fmt.Sprintf("failed to create thumbnail %v", err))
	}

	// Resize Thumbnail
	srcImage, err := imaging.Open(thumbnailVideoPath)
	if err != nil {
		return response, pkgError.InternalServerError(fmt.Sprintf("failed to open image %v", err))
	}
	resizedImage := imaging.Resize(srcImage, 100, 0, imaging.Lanczos)
	thumbnailResizeVideoPath := fmt.Sprintf("%s/thumbnails-%s", config.PathSendItems, generateUUID+".png")
	if err = imaging.Save(resizedImage, thumbnailResizeVideoPath); err != nil {
		return response, pkgError.InternalServerError(fmt.Sprintf("failed to save thumbnail %v", err))
	}

	deletedItems = append(deletedItems, thumbnailVideoPath)
	deletedItems = append(deletedItems, thumbnailResizeVideoPath)
	videoThumbnail = thumbnailResizeVideoPath

	if request.Compress {
		compresVideoPath := fmt.Sprintf("%s/%s", config.PathSendItems, generateUUID+".mp4")

		cmdCompress := exec.Command("ffmpeg", "-i", oriVideoPath, "-strict", "-2", compresVideoPath)
		err = cmdCompress.Run()
		if err != nil {
			return response, pkgError.InternalServerError("failed to compress video")
		}

		videoPath = compresVideoPath
		deletedItems = append(deletedItems, compresVideoPath)
	} else {
		videoPath = oriVideoPath
		deletedItems = append(deletedItems, oriVideoPath)
	}

	//Send to WA server
	dataWaVideo, err := os.ReadFile(videoPath)
	if err != nil {
		return response, err
	}
	uploaded, err := service.uploadMedia(c, whatsmeow.MediaVideo, dataWaVideo, dataWaRecipient)
	if err != nil {
		return response, pkgError.InternalServerError(fmt.Sprintf("Failed to upload file: %v", err))
	}
	dataWaThumbnail, err := os.ReadFile(videoThumbnail)
	if err != nil {
		return response, err
	}

	msg := &waE2E.Message{VideoMessage: &waE2E.VideoMessage{
		URL:                 proto.String(uploaded.URL),
		Mimetype:            proto.String(http.DetectContentType(dataWaVideo)),
		Caption:             proto.String(request.Caption),
		FileLength:          proto.Uint64(uploaded.FileLength),
		FileSHA256:          uploaded.FileSHA256,
		FileEncSHA256:       uploaded.FileEncSHA256,
		MediaKey:            uploaded.MediaKey,
		DirectPath:          proto.String(uploaded.DirectPath),
		ViewOnce:            proto.Bool(request.ViewOnce),
		JPEGThumbnail:       dataWaThumbnail,
		ThumbnailEncSHA256:  dataWaThumbnail,
		ThumbnailSHA256:     dataWaThumbnail,
		ThumbnailDirectPath: proto.String(uploaded.DirectPath),
	}}

	if request.IsForwarded {
		msg.VideoMessage.ContextInfo = &waE2E.ContextInfo{
			IsForwarded:     proto.Bool(true),
			ForwardingScore: proto.Uint32(100),
		}
	}

	caption := "🎥 Video"
	if request.Caption != "" {
		caption = "🎥 " + request.Caption
	}
	ts, err := service.wrapSendMessage(c, dataWaRecipient, msg, caption)
	go func() {
		errDelete := utils.RemoveFile(1, deletedItems...)
		if errDelete != nil {
			logrus.Infof("error when deleting picture: %v", errDelete)
		}
	}()
	if err != nil {
		return response, err
	}

	response.MessageID = ts.ID
	response.Status = fmt.Sprintf("Video sent to %s (server timestamp: %s)", request.Phone, ts.Timestamp.String())
	return response, nil
}

func (service serviceSend) SendContact(c *fiber.Ctx, request domainSend.ContactRequest) (response domainSend.GenericResponse, err error) {
	

	authPayload, err := auth.AuthPayload(c)
	if err != nil {
		return response, err
	}

	tenantClient, err := whatsapp.GetWhatsappTenantClient(service.Clients, authPayload.User)
	if err != nil {
		return response, err
	}
	
	err = validations.ValidateSendContact(c.UserContext(), request)
	if err != nil {
		return response, err
	}
	dataWaRecipient, err := whatsapp.ValidateJidWithLogin(tenantClient.Conn, request.Phone)
	if err != nil {
		return response, err
	}

	msgVCard := fmt.Sprintf("BEGIN:VCARD\nVERSION:3.0\nN:;%v;;;\nFN:%v\nTEL;type=CELL;waid=%v:+%v\nEND:VCARD",
		request.ContactName, request.ContactName, request.ContactPhone, request.ContactPhone)
	msg := &waE2E.Message{ContactMessage: &waE2E.ContactMessage{
		DisplayName: proto.String(request.ContactName),
		Vcard:       proto.String(msgVCard),
	}}

	if request.IsForwarded {
		msg.ContactMessage.ContextInfo = &waE2E.ContextInfo{
			IsForwarded:     proto.Bool(true),
			ForwardingScore: proto.Uint32(100),
		}
	}

	content := "👤 " + request.ContactName

	ts, err := service.wrapSendMessage(c, dataWaRecipient, msg, content)
	if err != nil {
		return response, err
	}

	response.MessageID = ts.ID
	response.Status = fmt.Sprintf("Contact sent to %s (server timestamp: %s)", request.Phone, ts.Timestamp.String())
	return response, nil
}

func (service serviceSend) SendLink(c *fiber.Ctx, request domainSend.LinkRequest) (response domainSend.GenericResponse, err error) {
	
	
	authPayload, err := auth.AuthPayload(c)
	if err != nil {
		return response, err
	}

	tenantClient, err := whatsapp.GetWhatsappTenantClient(service.Clients, authPayload.User)
	if err != nil {
		return response, err
	}	
	
	err = validations.ValidateSendLink(c.UserContext(), request)
	if err != nil {
		return response, err
	}
	dataWaRecipient, err := whatsapp.ValidateJidWithLogin(tenantClient.Conn, request.Phone)
	if err != nil {
		return response, err
	}

	metadata, err := utils.GetMetaDataFromURL(request.Link)
	if err != nil {
		return response, err
	}

	// Log image dimensions if available, otherwise note it's a square image or dimensions not available
	if metadata.Width != nil && metadata.Height != nil {
		logrus.Debugf("Image dimensions: %dx%d", *metadata.Width, *metadata.Height)
	} else {
		logrus.Debugf("Image dimensions: Square image or dimensions not available")
	}

	// Create the message
	msg := &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{
		Text:          proto.String(fmt.Sprintf("%s\n%s", request.Caption, request.Link)),
		Title:         proto.String(metadata.Title),
		MatchedText:   proto.String(request.Link),
		Description:   proto.String(metadata.Description),
		JPEGThumbnail: metadata.ImageThumb,
	}}

	if request.IsForwarded {
		msg.ExtendedTextMessage.ContextInfo = &waE2E.ContextInfo{
			IsForwarded:     proto.Bool(true),
			ForwardingScore: proto.Uint32(100),
		}
	}

	// If we have a thumbnail image, upload it to WhatsApp's servers
	if len(metadata.ImageThumb) > 0 && metadata.Height != nil && metadata.Width != nil {
		uploadedThumb, err := service.uploadMedia(c, whatsmeow.MediaLinkThumbnail, metadata.ImageThumb, dataWaRecipient)
		if err == nil {
			// Update the message with the uploaded thumbnail information
			msg.ExtendedTextMessage.ThumbnailDirectPath = proto.String(uploadedThumb.DirectPath)
			msg.ExtendedTextMessage.ThumbnailSHA256 = uploadedThumb.FileSHA256
			msg.ExtendedTextMessage.ThumbnailEncSHA256 = uploadedThumb.FileEncSHA256
			msg.ExtendedTextMessage.MediaKey = uploadedThumb.MediaKey
			msg.ExtendedTextMessage.ThumbnailHeight = metadata.Height
			msg.ExtendedTextMessage.ThumbnailWidth = metadata.Width
		} else {
			logrus.Warnf("Failed to upload thumbnail: %v, continue without uploaded thumbnail", err)
		}
	}

	content := "🔗 " + request.Link
	if request.Caption != "" {
		content = "🔗 " + request.Caption
	}
	ts, err := service.wrapSendMessage(c, dataWaRecipient, msg, content)
	if err != nil {
		return response, err
	}

	response.MessageID = ts.ID
	response.Status = fmt.Sprintf("Link sent to %s (server timestamp: %s)", request.Phone, ts.Timestamp.String())
	return response, nil
}

func (service serviceSend) SendLocation(c *fiber.Ctx, request domainSend.LocationRequest) (response domainSend.GenericResponse, err error) {
	

	authPayload, err := auth.AuthPayload(c)
	if err != nil {
		return response, err
	}

	tenantClient, err := whatsapp.GetWhatsappTenantClient(service.Clients, authPayload.User)
	if err != nil {
		return response, err
	}
	
	err = validations.ValidateSendLocation(c.UserContext(), request)
	if err != nil {
		return response, err
	}
	dataWaRecipient, err := whatsapp.ValidateJidWithLogin(tenantClient.Conn, request.Phone)
	if err != nil {
		return response, err
	}

	// Compose WhatsApp Proto
	msg := &waE2E.Message{
		LocationMessage: &waE2E.LocationMessage{
			DegreesLatitude:  proto.Float64(utils.StrToFloat64(request.Latitude)),
			DegreesLongitude: proto.Float64(utils.StrToFloat64(request.Longitude)),
		},
	}

	if request.IsForwarded {
		msg.LocationMessage.ContextInfo = &waE2E.ContextInfo{
			IsForwarded:     proto.Bool(true),
			ForwardingScore: proto.Uint32(100),
		}
	}

	content := "📍 " + request.Latitude + ", " + request.Longitude

	// Send WhatsApp Message Proto
	ts, err := service.wrapSendMessage(c, dataWaRecipient, msg, content)
	if err != nil {
		return response, err
	}

	response.MessageID = ts.ID
	response.Status = fmt.Sprintf("Send location success %s (server timestamp: %s)", request.Phone, ts.Timestamp.String())
	return response, nil
}

func (service serviceSend) SendAudio(c *fiber.Ctx, request domainSend.AudioRequest) (response domainSend.GenericResponse, err error) {
	

	authPayload, err := auth.AuthPayload(c)
	if err != nil {
		return response, err
	}

	tenantClient, err := whatsapp.GetWhatsappTenantClient(service.Clients, authPayload.User)
	if err != nil {
		return response, err
	}
	
	err = validations.ValidateSendAudio(c.UserContext(), request)
	if err != nil {
		return response, err
	}
	dataWaRecipient, err := whatsapp.ValidateJidWithLogin(tenantClient.Conn, request.Phone)
	if err != nil {
		return response, err
	}

	autioBytes := helpers.MultipartFormFileHeaderToBytes(request.Audio)
	audioMimeType := http.DetectContentType(autioBytes)

	audioUploaded, err := service.uploadMedia(c, whatsmeow.MediaAudio, autioBytes, dataWaRecipient)
	if err != nil {
		err = pkgError.WaUploadMediaError(fmt.Sprintf("Failed to upload audio: %v", err))
		return response, err
	}

	msg := &waE2E.Message{
		AudioMessage: &waE2E.AudioMessage{
			URL:           proto.String(audioUploaded.URL),
			DirectPath:    proto.String(audioUploaded.DirectPath),
			Mimetype:      proto.String(audioMimeType),
			FileLength:    proto.Uint64(audioUploaded.FileLength),
			FileSHA256:    audioUploaded.FileSHA256,
			FileEncSHA256: audioUploaded.FileEncSHA256,
			MediaKey:      audioUploaded.MediaKey,
		},
	}

	if request.IsForwarded {
		msg.AudioMessage.ContextInfo = &waE2E.ContextInfo{
			IsForwarded:     proto.Bool(true),
			ForwardingScore: proto.Uint32(100),
		}
	}

	content := "🎵 Audio"

	ts, err := service.wrapSendMessage(c, dataWaRecipient, msg, content)
	if err != nil {
		return response, err
	}

	response.MessageID = ts.ID
	response.Status = fmt.Sprintf("Send audio success %s (server timestamp: %s)", request.Phone, ts.Timestamp.String())
	return response, nil
}

func (service serviceSend) SendPoll(c *fiber.Ctx, request domainSend.PollRequest) (response domainSend.GenericResponse, err error) {
	
	authPayload, err := auth.AuthPayload(c)
	if err != nil {
		return response, err
	}

	tenantClient, err := whatsapp.GetWhatsappTenantClient(service.Clients, authPayload.User)
	if err != nil {
		return response, err
	}
	
	err = validations.ValidateSendPoll(c.UserContext(), request)
	if err != nil {
		return response, err
	}
	dataWaRecipient, err := whatsapp.ValidateJidWithLogin(tenantClient.Conn, request.Phone)
	if err != nil {
		return response, err
	}

	content := "📊 " + request.Question

	msg := tenantClient.Conn.BuildPollCreation(request.Question, request.Options, request.MaxAnswer)

	ts, err := service.wrapSendMessage(c, dataWaRecipient, msg, content)
	if err != nil {
		return response, err
	}

	response.MessageID = ts.ID
	response.Status = fmt.Sprintf("Send poll success %s (server timestamp: %s)", request.Phone, ts.Timestamp.String())
	return response, nil
}

func (service serviceSend) SendPresence(c *fiber.Ctx, request domainSend.PresenceRequest) (response domainSend.GenericResponse, err error) {
	
	authPayload, err := auth.AuthPayload(c)
	if err != nil {
		return response, err
	}

	tenantClient, err := whatsapp.GetWhatsappTenantClient(service.Clients, authPayload.User)
	if err != nil {
		return response, err
	}
	
	err = validations.ValidateSendPresence(c.UserContext(), request)
	if err != nil {
		return response, err
	}

	err = tenantClient.Conn.SendPresence(types.Presence(request.Type))
	if err != nil {
		return response, err
	}

	response.MessageID = "presence"
	response.Status = fmt.Sprintf("Send presence success %s", request.Type)
	return response, nil
}

func (service serviceSend) getMentionFromText(c *fiber.Ctx, messages string) (result []string) {
	
	authPayload, err := auth.AuthPayload(c)
	if err != nil {
		return result
	}

	tenantClient, err := whatsapp.GetWhatsappTenantClient(service.Clients, authPayload.User)
	if err != nil {
		return result
	}
	
	mentions := utils.ContainsMention(messages)
	for _, mention := range mentions {
		// Get JID from phone number
		if dataWaRecipient, err := whatsapp.ValidateJidWithLogin(tenantClient.Conn, mention); err == nil {
			result = append(result, dataWaRecipient.String())
		}
	}
	return result
}

func (service serviceSend) uploadMedia(c *fiber.Ctx, mediaType whatsmeow.MediaType, media []byte, recipient types.JID) (uploaded whatsmeow.UploadResponse, err error) {
	

	authPayload, err := auth.AuthPayload(c)
	if err != nil {
		return uploaded, err
	}

	tenantClient, err := whatsapp.GetWhatsappTenantClient(service.Clients, authPayload.User)
	if err != nil {
		return uploaded, err
	}
	
	if recipient.Server == types.NewsletterServer {
		uploaded, err = tenantClient.Conn.UploadNewsletter(c.UserContext(), media, mediaType)
	} else {
		uploaded, err = tenantClient.Conn.Upload(c.UserContext(), media, mediaType)
	}
	return uploaded, err
}
