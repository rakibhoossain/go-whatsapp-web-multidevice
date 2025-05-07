package whatsapp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	goLog "log"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"database/sql"

	"google.golang.org/protobuf/proto"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/internal/websocket"
	pkgError "github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/error"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/whatsapp/types"
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
	qrCode "github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	meowTypes "go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// Type definitions
type ExtractedMedia struct {
	MediaPath string `json:"media_path"`
	MimeType  string `json:"mime_type"`
	Caption   string `json:"caption"`
}

type evtReaction struct {
	ID      string `json:"id,omitempty"`
	Message string `json:"message,omitempty"`
}

type evtMessage struct {
	ID            string `json:"id,omitempty"`
	Text          string `json:"text,omitempty"`
	RepliedId     string `json:"replied_id,omitempty"`
	QuotedMessage string `json:"quoted_message,omitempty"`
}

type WhatsAppTenantUser struct {
	JID        string `json:"jid"`
	UserToken  string `json:"token"`
	WebhookURL string `json:"webhook_url"`
	ClientId   int64  `json:"client_id"`
	StatusCode int    `json:"status_code"`
}

type WhatsAppTenantClient struct {
	Conn *whatsmeow.Client   // Explicitly named connection
	User *WhatsAppTenantUser // User data
}

var WhatsAppDatastore *sqlstore.Container
var WhatsAppActiveTenantClient = make(map[string]*WhatsAppTenantClient)

var Db *sql.DB

// Global variables
var (
	log           waLog.Logger
	historySyncID int32
	startupTime   = time.Now().Unix()
)

func Startup() *map[string]*WhatsAppTenantClient {
	goLog.Println("Running Startup Tasks")

	// Load All WhatsApp Client Devices from Datastore
	devices, err := WhatsAppDatastore.GetAllDevices()
	if err != nil {
		goLog.Fatalln("Failed to Load WhatsApp Client Devices from Datastore")
	}

	jidTokenMap := getDeviceTokens(devices)

	// Do Reconnect for Every Device in Datastore
	for _, device := range devices {
		user := jidTokenMap[device.ID.String()]

		if user == nil {
			continue
		}

		// Mask JID for Logging Information
		jid := device.ID.String()
		maskJID := jid[0:len(jid)-4] + "xxxx"

		// Print Restore Log
		goLog.Println("Restoring WhatsApp Client for " + maskJID)
		goLog.Println("Restoring WhatsApp Client for UUID " + user.UserToken)

		// Initialize WhatsApp Client
		WhatsAppInitClient(device, user)

		// Reconnect WhatsApp Client WebSocket
		err = WhatsAppReconnect(user)
		if err != nil {
			goLog.Fatalln(err.Error())
		}
	}

	return &WhatsAppActiveTenantClient
}

func getDeviceTokens(devices []*store.Device) map[string]*WhatsAppTenantUser {
	var jids []string
	for _, device := range devices {
		jids = append(jids, device.ID.String())
	}

	jidTokenMap := make(map[string]*WhatsAppTenantUser)

	batchSize := 100
	for i := 0; i < len(jids); i += batchSize {
		end := i + batchSize
		if end > len(jids) {
			end = len(jids)
		}
		batch := jids[i:end]

		// Build the PostgreSQL placeholders: $1, $2, ...
		placeholders := make([]string, len(batch))
		args := make([]interface{}, len(batch))
		for i, jid := range batch {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
			args[i] = jid
		}

		query := fmt.Sprintf(`
			SELECT p.jid, p.token, c.webhook_url, c.status_code, c.id AS client_id
			FROM whatsmeow_device_client_pivot p
			INNER JOIN whatsmeow_clients c ON p.client_id = c.id
			WHERE p.jid IN (%s)
			AND c.status_code = 1`, strings.Join(placeholders, ","))

		rows, err := Db.Query(query, args...)
		if err != nil {
			goLog.Fatalln("Failed to query pivot table: " + err.Error())
			continue
		}

		func() {
			defer rows.Close()

			for rows.Next() {
				var user WhatsAppTenantUser
				var jid, webhookURL sql.NullString

				if err := rows.Scan(&jid, &user.UserToken, &webhookURL, &user.StatusCode, &user.ClientId); err != nil {
					goLog.Fatalln("Failed to scan pivot row: " + err.Error())
					continue
				}

				if jid.Valid {
					user.JID = jid.String
				}
				if webhookURL.Valid {
					user.WebhookURL = webhookURL.String
				}

				jidTokenMap[user.JID] = &user
			}
		}()
	}

	return jidTokenMap
}

// InitWaDB initializes the WhatsApp database connection
func InitWaDB() (*sqlstore.Container, *sql.DB) {
	log = waLog.Stdout("Main", config.WhatsappLogLevel, true)
	dbLog := waLog.Stdout("Database", config.WhatsappLogLevel, true)

	storeContainer, dbClient, err := initDatabase(dbLog)
	if err != nil {
		log.Errorf("Database initialization error: %v", err)
		panic(pkgError.InternalServerError(fmt.Sprintf("Database initialization error: %v", err)))
	}

	return storeContainer, dbClient
}

// initDatabase creates and returns a database store container based on the configured URI
func initDatabase(dbLog waLog.Logger) (*sqlstore.Container, *sql.DB, error) {
	db, err := sql.Open("postgres", config.DBURI)
	if err != nil {
		return nil, nil, fmt.Errorf("Error Connect WhatsApp Client Datastore: %w", err)
	}

	container := sqlstore.NewWithDB(db, "postgres", dbLog)
	WhatsAppDatastore = container
	Db = db

	err = updateDatabase()
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to migrate db tables: %w", err)
	}

	return container, Db, nil
}

func WhatsAppGenerateQR(qrChan <-chan whatsmeow.QRChannelItem) (string, int) {
	qrChanCode := make(chan string)
	qrChanTimeout := make(chan int)

	// Get QR Code Data and Timeout
	go func() {
		for evt := range qrChan {
			if evt.Event == "code" {
				qrChanCode <- evt.Code
				qrChanTimeout <- int(evt.Timeout.Seconds())
			}
		}
	}()

	// Generate QR Code Data to PNG Image
	qrTemp := <-qrChanCode
	qrPNG, _ := qrCode.Encode(qrTemp, qrCode.Medium, 256)

	// Return QR Code PNG in Base64 Format and Timeout Information
	return base64.StdEncoding.EncodeToString(qrPNG), <-qrChanTimeout
}

func WhatsAppLogin(user *WhatsAppTenantUser) (string, int, error) {
	if WhatsAppActiveTenantClient[user.UserToken] != nil {
		// Make Sure WebSocket Connection is Disconnected
		WhatsAppActiveTenantClient[user.UserToken].Conn.Disconnect()

		if WhatsAppActiveTenantClient[user.UserToken].Conn.Store.ID == nil {

			// Clean history
			delete(WhatsAppActiveTenantClient, user.UserToken)
			WhatsAppInitClient(nil, user)

			if WhatsAppActiveTenantClient[user.UserToken] != nil {

				// Device ID is not Exist
				// Generate QR Code
				qrChanGenerate, _ := WhatsAppActiveTenantClient[user.UserToken].Conn.GetQRChannel(context.Background())

				// Connect WebSocket while Initialize QR Code Data to be Sent
				err := WhatsAppActiveTenantClient[user.UserToken].Conn.Connect()
				if err != nil {
					return "", 0, err
				}

				// Get Generated QR Code and Timeout Information
				qrImage, qrTimeout := WhatsAppGenerateQR(qrChanGenerate)

				// Return QR Code in Base64 Format and Timeout Information
				return "data:image/png;base64," + qrImage, qrTimeout, nil
			}

			return "", 0, errors.New("Please try again")

		} else {
			// Device ID is Exist
			// Reconnect WebSocket
			err := WhatsAppReconnect(user)
			if err != nil {
				return "", 0, err
			}

			return "WhatsApp Client is Reconnected", 0, nil
		}
	}

	// Return Error WhatsApp Client is not Valid
	return "", 0, errors.New("WhatsAppLogin WhatsApp Client is not Valid")
}

func WhatsAppLoginPair(user *WhatsAppTenantUser) (string, error) {
	if WhatsAppActiveTenantClient[user.UserToken] != nil {
		// Make Sure WebSocket Connection is Disconnected
		WhatsAppActiveTenantClient[user.UserToken].Conn.Disconnect()

		if WhatsAppActiveTenantClient[user.UserToken].Conn.Store.ID == nil {
			// Connect WebSocket while also Requesting Pairing Code
			err := WhatsAppActiveTenantClient[user.UserToken].Conn.Connect()
			if err != nil {
				return "", err
			}

			jid := WhatsAppDecomposeJID(user.JID)
			// Request Pairing Code
			code, err := WhatsAppActiveTenantClient[user.UserToken].Conn.PairPhone(jid, true, whatsmeow.PairClientChrome, "Chrome ("+WhatsAppGetUserOS()+")")
			if err != nil {
				return "", err
			}

			return code, nil
		} else {
			// Device ID is Exist
			// Reconnect WebSocket
			err := WhatsAppReconnect(user)
			if err != nil {
				return "", err
			}

			return "WhatsApp Client is Reconnected", nil
		}
	}

	// Return Error WhatsApp Client is not Valid
	return "", errors.New("WhatsAppLoginPair WhatsApp Client is not Valid")
}

// InitWaCLI initializes the WhatsApp client
func WhatsAppInitClient(device *store.Device, user *WhatsAppTenantUser) {
	if WhatsAppActiveTenantClient[user.UserToken] == nil {
		if device == nil {
			// Initialize New WhatsApp Client Device in Datastore
			device = WhatsAppDatastore.NewDevice()
		}

		// Configure device properties
		osName := fmt.Sprintf("%s %s", config.AppOs, config.AppVersion)
		store.DeviceProps.PlatformType = &config.AppPlatform
		store.DeviceProps.Os = &osName
		store.DeviceProps.RequireFullSync = proto.Bool(false)

		// Initialize New WhatsApp Client
		// And Save it to The Map
		var wc WhatsAppTenantClient
		wc.Conn = whatsmeow.NewClient(device, waLog.Stdout("Client", config.WhatsappLogLevel, true))
		wc.User = user

		WhatsAppActiveTenantClient[user.UserToken] = &wc
		WhatsAppActiveTenantClient[user.UserToken].Conn.AddEventHandler(createEventHandler(user))

		// Set WhatsApp Client Auto Reconnect
		WhatsAppActiveTenantClient[user.UserToken].Conn.EnableAutoReconnect = true

		// Set WhatsApp Client Auto Trust Identity
		WhatsAppActiveTenantClient[user.UserToken].Conn.AutoTrustIdentity = true
	}
}

func WhatsAppReconnect(user *WhatsAppTenantUser) error {
	if WhatsAppActiveTenantClient[user.UserToken] != nil {
		// Make Sure WebSocket Connection is Disconnected
		WhatsAppActiveTenantClient[user.UserToken].Conn.Disconnect()

		// Make Sure Store ID is not Empty
		// To do Reconnection
		if WhatsAppActiveTenantClient[user.UserToken] != nil {
			err := WhatsAppActiveTenantClient[user.UserToken].Conn.Connect()
			if err != nil {
				return err
			}

			return nil
		}

		return errors.New("WhatsApp Client Store ID is Empty, Please Re-Login and Scan QR Code Again")
	}

	return errors.New("WhatsAppReconnect WhatsApp Client is not Valid")
}

func WhatsAppPresence(user *WhatsAppTenantUser, isAvailable bool) {
	if isAvailable {
		_ = WhatsAppActiveTenantClient[user.UserToken].Conn.SendPresence(meowTypes.PresenceAvailable)
	} else {
		_ = WhatsAppActiveTenantClient[user.UserToken].Conn.SendPresence(meowTypes.PresenceUnavailable)
	}
}

func WhatsAppLogout(user *WhatsAppTenantUser) error {
	if WhatsAppActiveTenantClient[user.UserToken] != nil {
		// Make Sure Store ID is not Empty
		if WhatsAppActiveTenantClient[user.UserToken] != nil {
			var err error

			// Set WhatsApp Client Presence to Unavailable
			WhatsAppPresence(user, false)

			// Logout WhatsApp Client and Disconnect from WebSocket
			err = WhatsAppActiveTenantClient[user.UserToken].Conn.Logout()
			if err != nil {
				// Force Disconnect
				WhatsAppActiveTenantClient[user.UserToken].Conn.Disconnect()

				// Manually Delete Device from Datastore Store
				err = WhatsAppActiveTenantClient[user.UserToken].Conn.Store.Delete()
				if err != nil {
					return err
				}
			}

			// Free WhatsApp Client Map
			WhatsAppActiveTenantClient[user.UserToken] = nil
			delete(WhatsAppActiveTenantClient, user.UserToken)

			return nil
		}

		return errors.New("WhatsApp Client Store ID is Empty, Please Re-Login and Scan QR Code Again")
	}

	// Return Error WhatsApp Client is not Valid
	return errors.New("WhatsAppLogout WhatsApp Client is not Valid")
}

// handler is the main event handler for WhatsApp events
func createEventHandler(user *WhatsAppTenantUser) func(interface{}) {
	return func(evt interface{}) {
		switch v := evt.(type) {
		case *events.DeleteForMe:
			handleDeleteForMe(user, v)
		case *events.AppStateSyncComplete:
			handleAppStateSyncComplete(user, v)
		case *events.PairSuccess:
			handlePairSuccess(user, v)
		case *events.LoggedOut:
			handleLoggedOut(user)
		case *events.Connected, *events.PushNameSetting:
			handleConnectionEvents(user)
		case *events.StreamReplaced:
			handleStreamReplaced(user)
		case *events.Message:
			handleMessage(user, v)
		case *events.Receipt:
			handleReceipt(user, v)
		case *events.Presence:
			handlePresence(user, v)
		case *events.HistorySync:
			handleHistorySync(user, v)
		case *events.AppState:
			handleAppState(user, v)
		}
	}
}

// Event handler functions
func handleDeleteForMe(user *WhatsAppTenantUser, evt *events.DeleteForMe) {
	log.Infof("Deleted message %s for %s token %s", evt.MessageID, evt.SenderJID.String(), user.UserToken)
}

func handleAppStateSyncComplete(user *WhatsAppTenantUser, evt *events.AppStateSyncComplete) {

	tenantClient, err := GetWhatsappTenantClient(&WhatsAppActiveTenantClient, user)
	if err != nil {
		return
	}

	if len(tenantClient.Conn.Store.PushName) > 0 && evt.Name == appstate.WAPatchCriticalBlock {
		if err := tenantClient.Conn.SendPresence(meowTypes.PresenceAvailable); err != nil {
			log.Warnf("Failed to send available presence: %v", err)
		} else {
			log.Infof("Marked self as available")
		}
	}
}

func handlePairSuccess(user *WhatsAppTenantUser, evt *events.PairSuccess) {
	// Broadcast a message to all clients in "support-team" channel
	websocket.Broadcast <- struct {
		Channel string
		Message websocket.BroadcastMessage
	}{
		Channel: user.UserToken,
		Message: websocket.BroadcastMessage{
			Code:    "LOGIN_SUCCESS",
			Message: fmt.Sprintf("Successfully pair with %s", evt.ID.String()),
			Result:  map[string]interface{}{"ticket_id": 12345},
		},
	}
}

func handleLoggedOut(user *WhatsAppTenantUser) {
	// Broadcast a message to all clients in "support-team" channel
	websocket.Broadcast <- struct {
		Channel string
		Message websocket.BroadcastMessage
	}{
		Channel: user.UserToken,
		Message: websocket.BroadcastMessage{
			Code:   "LIST_DEVICES",
			Result: nil,
		},
	}
}

func handleConnectionEvents(user *WhatsAppTenantUser) {

	tenantClient, err := GetWhatsappTenantClient(&WhatsAppActiveTenantClient, user)
	if err != nil {
		return
	}

	if len(tenantClient.Conn.Store.PushName) == 0 {
		return
	}

	// Send presence available when connecting and when the pushname is changed.
	// This makes sure that outgoing messages always have the right pushname.
	if err := tenantClient.Conn.SendPresence(meowTypes.PresenceAvailable); err != nil {
		log.Warnf("Failed to send available presence: %v", err)
	} else {
		log.Infof("Marked self as available")
	}
}

func handleStreamReplaced(user *WhatsAppTenantUser) {
	os.Exit(0)
}

func handleMessage(user *WhatsAppTenantUser, evt *events.Message) {
	// Log message metadata
	metaParts := buildMessageMetaParts(evt)
	log.Infof("Received message %s from %s (%s): %+v",
		evt.Info.ID,
		evt.Info.SourceString(),
		strings.Join(metaParts, ", "),
		evt.Message,
	)

	// Record the message
	message := ExtractMessageText(evt)
	utils.RecordMessage(evt.Info.ID, evt.Info.Sender.String(), message)

	// Handle image message if present
	handleImageMessage(user, evt)

	// Handle auto-reply if configured
	handleAutoReply(user, evt)

	// Forward to webhook if configured
	handleWebhookForward(user, evt)
}

func buildMessageMetaParts(evt *events.Message) []string {
	metaParts := []string{
		fmt.Sprintf("pushname: %s", evt.Info.PushName),
		fmt.Sprintf("timestamp: %s", evt.Info.Timestamp),
	}
	if evt.Info.Type != "" {
		metaParts = append(metaParts, fmt.Sprintf("type: %s", evt.Info.Type))
	}
	if evt.Info.Category != "" {
		metaParts = append(metaParts, fmt.Sprintf("category: %s", evt.Info.Category))
	}
	if evt.IsViewOnce {
		metaParts = append(metaParts, "view once")
	}
	return metaParts
}

func handleImageMessage(user *WhatsAppTenantUser, evt *events.Message) {

	tenantClient, err := GetWhatsappTenantClient(&WhatsAppActiveTenantClient, user)
	if err != nil {
		return
	}

	if img := evt.Message.GetImageMessage(); img != nil {
		if path, err := ExtractMedia(tenantClient.Conn, config.PathStorages, img); err != nil {
			log.Errorf("Failed to download image: %v for token %s", err, user.UserToken)
		} else {
			log.Infof("Image downloaded to %s", path)
		}
	}
}

func handleAutoReply(user *WhatsAppTenantUser, evt *events.Message) {

	tenantClient, err := GetWhatsappTenantClient(&WhatsAppActiveTenantClient, user)
	if err != nil {
		return
	}

	if config.WhatsappAutoReplyMessage != "" &&
		!isGroupJid(evt.Info.Chat.String()) &&
		!evt.Info.IsIncomingBroadcast() &&
		evt.Message.GetExtendedTextMessage().GetText() != "" {
		_, _ = tenantClient.Conn.SendMessage(
			context.Background(),
			FormatJID(evt.Info.Sender.String()),
			&waE2E.Message{Conversation: proto.String(config.WhatsappAutoReplyMessage)},
		)
	}
}

func handleWebhookForward(user *WhatsAppTenantUser, evt *events.Message) {

	tenantClient, err := GetWhatsappTenantClient(&WhatsAppActiveTenantClient, user)
	if err != nil {
		return
	}

	if len(user.WebhookURL) > 0 &&
		!strings.Contains(evt.Info.SourceString(), "broadcast") &&
		!isFromMySelf(tenantClient.Conn, evt.Info.SourceString()) {
		go func(evt *events.Message) {
			if err := forwardToWebhook(tenantClient, evt); err != nil {
				logrus.Error("Failed forward to webhook: ", err, " for token: ", user.UserToken)
			}
		}(evt)
	}
}

func handleReceipt(user *WhatsAppTenantUser, evt *events.Receipt) {
	if evt.Type == meowTypes.ReceiptTypeRead || evt.Type == meowTypes.ReceiptTypeReadSelf {
		log.Infof("%v was read by %s at %s", evt.MessageIDs, evt.SourceString(), evt.Timestamp)
	} else if evt.Type == meowTypes.ReceiptTypeDelivered {
		log.Infof("%s was delivered to %s at %s token %s", evt.MessageIDs[0], evt.SourceString(), evt.Timestamp, user.UserToken)
	}
}

func handlePresence(user *WhatsAppTenantUser, evt *events.Presence) {
	if evt.Unavailable {
		if evt.LastSeen.IsZero() {
			log.Infof("%s is now offline", evt.From)
		} else {
			log.Infof("%s is now offline (last seen: %s), token %s", evt.From, evt.LastSeen, user.UserToken)
		}
	} else {
		log.Infof("%s is now online", evt.From)
	}
}

func handleHistorySync(user *WhatsAppTenantUser, evt *events.HistorySync) {

	tenantClient, err := GetWhatsappTenantClient(&WhatsAppActiveTenantClient, user)
	if err != nil {
		return
	}

	id := atomic.AddInt32(&historySyncID, 1)
	fileName := fmt.Sprintf("%s/history-%d-%s-%d-%s.json",
		config.PathStorages,
		startupTime,
		tenantClient.Conn.Store.ID.String(),
		id,
		evt.Data.SyncType.String(),
	)

	file, err := os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		log.Errorf("Failed to open file to write history sync: %v", err)
		return
	}
	defer file.Close()

	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	if err = enc.Encode(evt.Data); err != nil {
		log.Errorf("Failed to write history sync: %v", err)
		return
	}

	log.Infof("Wrote history sync to %s", fileName)
}

func handleAppState(user *WhatsAppTenantUser, evt *events.AppState) {
	log.Debugf("App state event: %+v / %+v", evt.Index, evt.SyncActionValue)
}

// Multitenant
func updateDatabase() error {
	var err error

	_, err = Db.Exec(`
		CREATE TABLE IF NOT EXISTS whatsmeow_clients (
		  id SERIAL PRIMARY KEY,
		  client_name TEXT NOT NULL,
		  uuid UUID NOT NULL UNIQUE,
		  secret_key TEXT NOT NULL,
		  webhook_url TEXT,
		  status_code INTEGER NOT NULL DEFAULT 1,
		  updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		  created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
		);
		
		CREATE INDEX IF NOT EXISTS idx_uuid ON whatsmeow_clients (uuid);
		
		CREATE TABLE IF NOT EXISTS whatsmeow_device_client_pivot (
		  id SERIAL PRIMARY KEY,
		  client_id INTEGER NOT NULL,
		  jid TEXT,
		  token TEXT NOT NULL UNIQUE,
		  updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		  created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		  CONSTRAINT fk_client FOREIGN KEY (client_id) REFERENCES whatsmeow_clients(id) ON DELETE CASCADE
		);
	`)

	return err
}

func saveUUID(jid meowTypes.JID, user *WhatsAppTenantUser) error {
	_, err := Db.Exec(`
		INSERT INTO whatsmeow_device_client_pivot (jid, token, updated_at, client_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT(token) DO UPDATE SET 
			updated_at = EXCLUDED.updated_at;
		`,
		jid, user.UserToken, time.Now(), user.ClientId,
	)
	return err
}

func removeByUUID(user *WhatsAppTenantUser) error {
	_, err := Db.Exec(`
		DELETE FROM whatsmeow_device_client_pivot
		WHERE token = $1
	`, user.UserToken)
	return err
}

func CreateClient(c *fiber.Ctx) error {
	req := new(types.CreateClientRequest)

	// Parse form data
	if err := c.BodyParser(req); err != nil {
		return utils.ResponseBadRequest(c, "Invalid request format")
	}

	// Generate UUID and secret key
	uuid := utils.GenerateUUID()
	secretKey := utils.GenerateSecretKey()

	// Store in database
	var id int64
	err := Db.QueryRow(`
        INSERT INTO whatsmeow_clients 
        (client_name, uuid, secret_key, webhook_url, status_code) 
        VALUES ($1, $2, $3, $4, 1)
        RETURNING id`,
		req.Name, uuid, secretKey, req.WebhookURL,
	).Scan(&id)

	if err != nil {
		fmt.Println(err)
		return utils.ResponseBadRequest(c, "Failed to create client")
	}

	// Prepare response data
	responseData := map[string]interface{}{
		"id":          id,
		"client_name": req.Name,
		"uuid":        uuid,
		"webhook_url": req.WebhookURL,
		"secretKey":   secretKey,
		"status":      "active",
	}

	return utils.ResponseSuccessWithData(c, "Successfully created client", responseData)
}

// ClientStatus returns client status with user count
func ClientStatus(c *fiber.Ctx) error {
	uuid := c.Params("uuid")

	var response types.ClientStatusResponse
	err := Db.QueryRow(`
        SELECT 
            c.id, 
            c.client_name, 
            c.uuid, 
            c.webhook_url, 
            c.secret_key,
            c.status_code,
            CASE c.status_code 
                WHEN 1 THEN 'active' 
                WHEN 0 THEN 'inactive' 
                ELSE 'unknown' 
            END as status,
            c.created_at,
            c.updated_at,
            COUNT(p.id) as user_count
        FROM whatsmeow_clients c
        LEFT JOIN whatsmeow_device_client_pivot p ON c.id = p.client_id
        WHERE c.uuid = $1
        GROUP BY c.id`,
		uuid,
	).Scan(
		&response.ID,
		&response.ClientName,
		&response.UUID,
		&response.WebhookURL,
		&response.SecretKey,
		&response.StatusCode,
		&response.Status,
		&response.CreatedAt,
		&response.UpdatedAt,
		&response.UserCount,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return utils.ResponseNotFound(c, "Client not found")
		}
		return utils.ResponseBadRequest(c, "Failed to get client status")
	}

	return utils.ResponseSuccessWithData(c, "Client status retrieved", response)
}

// ClientStatusEdit updates client status

func ClientStatusEdit(c *fiber.Ctx) error {
	uuid := c.Params("uuid")
	req := new(types.UpdateClientStatusRequest)

	// Parse form data
	if err := c.BodyParser(req); err != nil {
		return utils.ResponseBadRequest(c, "Invalid request format")
	}

	// Convert string form value to int
	statusStr := c.FormValue("status_code")
	statusCode, err := strconv.Atoi(statusStr)
	if err != nil {
		return utils.ResponseBadRequest(c, "status_code must be a number (0 or 1)")
	}

	// Update status in database
	result, err := Db.Exec(`
        UPDATE whatsmeow_clients 
        SET status_code = $1, 
            updated_at = CURRENT_TIMESTAMP
        WHERE uuid = $2`,
		statusCode,
		uuid,
	)
	if err != nil {
		return utils.ResponseBadRequest(c, "Failed to update client status")
	}

	// Check if client was found
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return utils.ResponseBadRequest(c, "Failed to verify update")
	}

	if rowsAffected == 0 {
		return utils.ResponseNotFound(c, "Client not found")
	}

	// Return updated client status
	return ClientStatus(c) // Reuse the GET endpoint to return updated status
}

// ClientDelete handles client deletion
func ClientDelete(c *fiber.Ctx) error {
	uuid := c.Params("uuid")

	tx, err := Db.Begin()
	if err != nil {
		return utils.ResponseBadRequest(c, "Failed to start transaction")
	}

	// Delete client (ON DELETE CASCADE will handle related pivot rows)
	var deletedID int64
	err = tx.QueryRow(`
		DELETE FROM whatsmeow_clients 
		WHERE uuid = $1 
		RETURNING id
	`, uuid).Scan(&deletedID)

	if err == sql.ErrNoRows {
		tx.Rollback()
		return utils.ResponseNotFound(c, "Client not found")
	} else if err != nil {
		tx.Rollback()
		return utils.ResponseBadRequest(c, "Failed to delete client")
	}

	if err := tx.Commit(); err != nil {
		return utils.ResponseBadRequest(c, "Failed to complete deletion")
	}

	return utils.ResponseSuccess(c, "Client deleted successfully")
}

func GetWhatsAppUserWithToken(uuid string, clientName string, clientPassword string) (*WhatsAppTenantUser, error) {
	var (
		user       WhatsAppTenantUser
		jid        sql.NullString
		webhookURL sql.NullString
	)

	query1 := `
		SELECT 
			p.jid, 
			c.webhook_url, 
			c.status_code, 
			c.id AS client_id
		FROM whatsmeow_device_client_pivot p
		JOIN whatsmeow_clients c ON p.client_id = c.id
		WHERE p.token = $1
			AND c.uuid = $2
			AND c.secret_key = $3
			AND c.status_code = 1
		LIMIT 1
	`

	query2 := `
		SELECT 
			c.webhook_url, 
			c.status_code, 
			c.id AS client_id
		FROM whatsmeow_clients c
		WHERE c.uuid = $1
			AND c.secret_key = $2
			AND c.status_code = 1
		LIMIT 1
	`

	err := Db.QueryRow(query1, uuid, clientName, clientPassword).Scan(
		&jid,
		&webhookURL,
		&user.StatusCode,
		&user.ClientId,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			// Try fallback query if no device-user match found
			err = Db.QueryRow(query2, clientName, clientPassword).Scan(
				&webhookURL,
				&user.StatusCode,
				&user.ClientId,
			)
			if err != nil {
				if err == sql.ErrNoRows {
					return nil, fmt.Errorf("user not found")
				}
				return nil, fmt.Errorf("database error: %w", err)
			}
			jid = sql.NullString{} // No device row means no JID
		} else {
			return nil, fmt.Errorf("database error: %w", err)
		}
	}

	// Convert nullable fields to Go strings
	if jid.Valid {
		user.JID = jid.String
	}
	if webhookURL.Valid {
		user.WebhookURL = webhookURL.String
	}

	user.UserToken = uuid

	return &user, nil
}
