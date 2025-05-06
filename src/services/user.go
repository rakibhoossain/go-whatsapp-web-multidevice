package services

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"time"

	domainUser "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/user"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/auth"
	pkgError "github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/error"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/whatsapp"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/validations"
	"github.com/disintegration/imaging"
	"github.com/gofiber/fiber/v2"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/types"
)

type userService struct {
	Clients *map[string]*whatsapp.WhatsAppTenantClient
}

func NewUserService(clients *map[string]*whatsapp.WhatsAppTenantClient) domainUser.IUserService {
	return &userService{
		Clients: clients,
	}
}

func (service userService) Info(c *fiber.Ctx, request domainUser.InfoRequest) (response domainUser.InfoResponse, err error) {
	
	
	authPayload, err := auth.AuthPayload(c)
	if err != nil {
		return response, err
	}

	tenantClient, err := whatsapp.GetWhatsappTenantClient(service.Clients, authPayload.User)
	if err != nil {
		return response, err
	}
	
	
	err = validations.ValidateUserInfo(c.UserContext(), request)
	if err != nil {
		return response, err
	}
	var jids []types.JID
	dataWaRecipient, err := whatsapp.ValidateJidWithLogin(tenantClient.Conn, request.Phone)
	if err != nil {
		return response, err
	}

	jids = append(jids, dataWaRecipient)
	resp, err := tenantClient.Conn.GetUserInfo(jids)
	if err != nil {
		return response, err
	}

	for _, userInfo := range resp {
		var device []domainUser.InfoResponseDataDevice
		for _, j := range userInfo.Devices {
			device = append(device, domainUser.InfoResponseDataDevice{
				User:   j.User,
				Agent:  j.RawAgent,
				Device: whatsapp.GetPlatformName(int(j.Device)),
				Server: j.Server,
				AD:     j.ADString(),
			})
		}

		data := domainUser.InfoResponseData{
			Status:    userInfo.Status,
			PictureID: userInfo.PictureID,
			Devices:   device,
		}
		if userInfo.VerifiedName != nil {
			data.VerifiedName = fmt.Sprintf("%v", *userInfo.VerifiedName)
		}
		response.Data = append(response.Data, data)
	}

	return response, nil
}

func (service userService) Avatar(c *fiber.Ctx, request domainUser.AvatarRequest) (response domainUser.AvatarResponse, err error) {

	authPayload, err := auth.AuthPayload(c)
	if err != nil {
		return response, err
	}

	tenantClient, err := whatsapp.GetWhatsappTenantClient(service.Clients, authPayload.User)
	if err != nil {
		return response, err
	}


	chanResp := make(chan domainUser.AvatarResponse)
	chanErr := make(chan error)
	waktu := time.Now()

	go func() {
		err = validations.ValidateUserAvatar(c.UserContext(), request)
		if err != nil {
			chanErr <- err
		}
		dataWaRecipient, err := whatsapp.ValidateJidWithLogin(tenantClient.Conn, request.Phone)
		if err != nil {
			chanErr <- err
		}
		pic, err := tenantClient.Conn.GetProfilePictureInfo(dataWaRecipient, &whatsmeow.GetProfilePictureParams{
			Preview:     request.IsPreview,
			IsCommunity: request.IsCommunity,
		})
		if err != nil {
			chanErr <- err
		} else if pic == nil {
			chanErr <- errors.New("no avatar found")
		} else {
			response.URL = pic.URL
			response.ID = pic.ID
			response.Type = pic.Type

			chanResp <- response
		}
	}()

	for {
		select {
		case err := <-chanErr:
			return response, err
		case response := <-chanResp:
			return response, nil
		default:
			if waktu.Add(2 * time.Second).Before(time.Now()) {
				return response, pkgError.ContextError("Error timeout get avatar !")
			}
		}
	}

}

func (service userService) MyListGroups(c *fiber.Ctx) (response domainUser.MyListGroupsResponse, err error) {
	
	authPayload, err := auth.AuthPayload(c)
	if err != nil {
		return response, err
	}

	tenantClient, err := whatsapp.GetWhatsappTenantClient(service.Clients, authPayload.User)
	if err != nil {
		return response, err
	}
	
	
	whatsapp.MustLogin(tenantClient.Conn)

	groups, err := tenantClient.Conn.GetJoinedGroups()
	if err != nil {
		return
	}
	fmt.Printf("%+v\n", groups)
	for _, group := range groups {
		response.Data = append(response.Data, *group)
	}
	return response, nil
}

func (service userService) MyListNewsletter(c *fiber.Ctx) (response domainUser.MyListNewsletterResponse, err error) {
	
	authPayload, err := auth.AuthPayload(c)
	if err != nil {
		return response, err
	}

	tenantClient, err := whatsapp.GetWhatsappTenantClient(service.Clients, authPayload.User)
	if err != nil {
		return response, err
	}
	
	
	whatsapp.MustLogin(tenantClient.Conn)

	datas, err := tenantClient.Conn.GetSubscribedNewsletters()
	if err != nil {
		return
	}
	fmt.Printf("%+v\n", datas)
	for _, data := range datas {
		response.Data = append(response.Data, *data)
	}
	return response, nil
}

func (service userService) MyPrivacySetting(c *fiber.Ctx) (response domainUser.MyPrivacySettingResponse, err error) {
	
	
	authPayload, err := auth.AuthPayload(c)
	if err != nil {
		return response, err
	}

	tenantClient, err := whatsapp.GetWhatsappTenantClient(service.Clients, authPayload.User)
	if err != nil {
		return response, err
	}
	
	whatsapp.MustLogin(tenantClient.Conn)

	resp, err := tenantClient.Conn.TryFetchPrivacySettings(true)
	if err != nil {
		return
	}

	response.GroupAdd = string(resp.GroupAdd)
	response.Status = string(resp.Status)
	response.ReadReceipts = string(resp.ReadReceipts)
	response.Profile = string(resp.Profile)
	return response, nil
}

func (service userService) MyListContacts(c *fiber.Ctx) (response domainUser.MyListContactsResponse, err error) {
	
	
	authPayload, err := auth.AuthPayload(c)
	if err != nil {
		return response, err
	}

	tenantClient, err := whatsapp.GetWhatsappTenantClient(service.Clients, authPayload.User)
	if err != nil {
		return response, err
	}
	
	
	whatsapp.MustLogin(tenantClient.Conn)

	contacts, err := tenantClient.Conn.Store.Contacts.GetAllContacts()
	if err != nil {
		return
	}

	for jid, contact := range contacts {
		response.Data = append(response.Data, domainUser.MyListContactsResponseData{
			JID:  jid,
			Name: contact.FullName,
		})
	}

	return response, nil
}

func (service userService) ChangeAvatar(c *fiber.Ctx, request domainUser.ChangeAvatarRequest) (err error) {
	
	
	authPayload, err := auth.AuthPayload(c)
	if err != nil {
		return err
	}

	tenantClient, err := whatsapp.GetWhatsappTenantClient(service.Clients, authPayload.User)
	if err != nil {
		return err
	}
	
	
	whatsapp.MustLogin(tenantClient.Conn)

	file, err := request.Avatar.Open()
	if err != nil {
		return err
	}
	defer file.Close()

	// Read original image
	srcImage, err := imaging.Decode(file)
	if err != nil {
		return fmt.Errorf("failed to decode image: %v", err)
	}

	// Get original dimensions
	bounds := srcImage.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Calculate new dimensions for 1:1 aspect ratio
	size := width
	if height < width {
		size = height
	}
	if size > 640 {
		size = 640
	}

	// Create a square crop from the center
	left := (width - size) / 2
	top := (height - size) / 2
	croppedImage := imaging.Crop(srcImage, image.Rect(left, top, left+size, top+size))

	// Resize if needed
	if size > 640 {
		croppedImage = imaging.Resize(croppedImage, 640, 640, imaging.Lanczos)
	}

	// Convert to bytes
	var buf bytes.Buffer
	err = imaging.Encode(&buf, croppedImage, imaging.JPEG, imaging.JPEGQuality(80))
	if err != nil {
		return fmt.Errorf("failed to encode image: %v", err)
	}

	_, err = tenantClient.Conn.SetGroupPhoto(types.JID{}, buf.Bytes())
	if err != nil {
		return err
	}

	return nil
}

func (service userService) ChangePushName(c *fiber.Ctx, request domainUser.ChangePushNameRequest) (err error) {
	
	authPayload, err := auth.AuthPayload(c)
	if err != nil {
		return err
	}

	tenantClient, err := whatsapp.GetWhatsappTenantClient(service.Clients, authPayload.User)
	if err != nil {
		return err
	}
	
	
	whatsapp.MustLogin(tenantClient.Conn)

	err = tenantClient.Conn.SendAppState(appstate.BuildSettingPushName(request.PushName))
	if err != nil {
		return err
	}
	return nil
}
