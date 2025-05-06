package services

import (
	domainApp "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/app"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/auth"
	pkgError "github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/error"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/whatsapp"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/validations"
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow/store/sqlstore"
)

type serviceApp struct {
	Clients *map[string]*whatsapp.WhatsAppTenantClient
	db      *sqlstore.Container
}

func NewAppService(clients *map[string]*whatsapp.WhatsAppTenantClient, db *sqlstore.Container) domainApp.IAppService {
	return &serviceApp{
		Clients: clients,
		db:      db,
	}
}

func (service serviceApp) Login(c *fiber.Ctx) (response domainApp.LoginResponse, err error) {
	authPayload, err := auth.AuthPayload(c)
	if err != nil {
		return response, err
	}

	// Initialize WhatsApp Client
	whatsapp.WhatsAppInitClient(nil, authPayload.User)

	// Get WhatsApp QR Code Image
	qrCodeImage, qrCodeTimeout, err := whatsapp.WhatsAppLogin(authPayload.User)
	if err != nil {
		return response, err
	}

	// If Return is Not QR Code But Reconnected
	if qrCodeImage == "WhatsApp Client is Reconnected" {
		return response, pkgError.ErrAlreadyLoggedIn
	}

	response.Code = qrCodeImage
	response.Duration = qrCodeTimeout

	return response, nil
}

func (service serviceApp) LoginWithCode(c *fiber.Ctx, phoneNumber string) (loginCode string, err error) {
	authPayload, err := auth.AuthPayload(c)
	if err != nil {
		return loginCode, err
	}

	if err = validations.ValidateLoginWithCode(c.UserContext(), phoneNumber); err != nil {
		logrus.Errorf("Error when validate login with code: %s", err.Error())
		return loginCode, err
	}

	// Initialize WhatsApp Client
	whatsapp.WhatsAppInitClient(nil, authPayload.User)

	// Get WhatsApp pairing Code text
	pairCode, err := whatsapp.WhatsAppLoginPair(authPayload.User)
	if err != nil {
		return loginCode, err
	}

	// If Return is not pairing code but Reconnected
	// Then Return OK With Reconnected Status
	if pairCode == "WhatsApp Client is Reconnected" {
		return pairCode, nil
	}

	return loginCode, nil
}

func (service serviceApp) Logout(c *fiber.Ctx) (err error) {

	authPayload, err := auth.AuthPayload(c)
	if err != nil {
		return err
	}

	err = whatsapp.WhatsAppLogout(authPayload.User)
	return

	// delete history
	// files, err := filepath.Glob(fmt.Sprintf("./%s/history-*", config.PathStorages))
	// if err != nil {
	// 	return err
	// }

	// for _, f := range files {
	// 	err = os.Remove(f)
	// 	if err != nil {
	// 		return err
	// 	}
	// }
	// // delete qr images
	// qrImages, err := filepath.Glob(fmt.Sprintf("./%s/scan-*", config.PathQrCode))
	// if err != nil {
	// 	return err
	// }

	// for _, f := range qrImages {
	// 	err = os.Remove(f)
	// 	if err != nil {
	// 		return err
	// 	}
	// }

	// delete senditems
	// qrItems, err := filepath.Glob(fmt.Sprintf("./%s/*", config.PathSendItems))
	// if err != nil {
	// 	return err
	// }

	// for _, f := range qrItems {
	// 	if !strings.Contains(f, ".gitignore") {
	// 		err = os.Remove(f)
	// 		if err != nil {
	// 			return err
	// 		}
	// 	}
	// }

	// err = service.WaCli.Logout()
	// return
}

func (service serviceApp) Reconnect(c *fiber.Ctx) (err error) {
	authPayload, err := auth.AuthPayload(c)
	if err != nil {
		return err
	}

	return whatsapp.WhatsAppReconnect(authPayload.User)
}

func (service serviceApp) FirstDevice(c *fiber.Ctx) (response domainApp.DevicesResponse, err error) {
	devices, err := service.db.GetFirstDevice()
	if err != nil {
		return response, err
	}

	response.Device = devices.ID.String()
	if devices.PushName != "" {
		response.Name = devices.PushName
	} else {
		response.Name = devices.BusinessName
	}

	return response, nil
}

func (service serviceApp) FetchDevices(_ *fiber.Ctx) (response []domainApp.DevicesResponse, err error) {
	devices, err := service.db.GetAllDevices()
	if err != nil {
		return nil, err
	}

	for _, device := range devices {
		var d domainApp.DevicesResponse
		d.Device = device.ID.String()
		if device.PushName != "" {
			d.Name = device.PushName
		} else {
			d.Name = device.BusinessName
		}

		response = append(response, d)
	}

	return response, nil
}
