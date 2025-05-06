package helpers

import (
	// "context"
	"mime/multipart"
	"time"

	domainApp "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/app"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/whatsapp"
)

func SetAutoConnectAfterBooting(service domainApp.IAppService) {
	time.Sleep(2 * time.Second)
	// _ = service.Reconnect(context.Background())
}

func SetAutoReconnectChecking(clients *map[string]*whatsapp.WhatsAppTenantClient) {
	// Run every 5 minutes to check if the connection is still alive, if not, reconnect
	go func() {
		for {
			for token, client := range *clients {
				client.Conn.IsConnected()

				time.Sleep(5 * time.Minute)
				if client.Conn != nil && !client.Conn.IsConnected() {
					_ = (*clients)[token].Conn.Connect()
				}
			}
		}
	}()
}

func MultipartFormFileHeaderToBytes(fileHeader *multipart.FileHeader) []byte {
	file, _ := fileHeader.Open()
	defer file.Close()

	fileBytes := make([]byte, fileHeader.Size)
	_, _ = file.Read(fileBytes)

	return fileBytes
}
