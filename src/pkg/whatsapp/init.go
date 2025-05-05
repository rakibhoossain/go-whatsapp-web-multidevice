package whatsapp

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"database/sql"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/internal/websocket"
	pkgError "github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/error"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
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
	cli           *whatsmeow.Client
	log           waLog.Logger
	historySyncID int32
	startupTime   = time.Now().Unix()
)

// InitWaDB initializes the WhatsApp database connection
func InitWaDB() *sqlstore.Container {
	log = waLog.Stdout("Main", config.WhatsappLogLevel, true)
	dbLog := waLog.Stdout("Database", config.WhatsappLogLevel, true)

	storeContainer, err := initDatabase(dbLog)
	if err != nil {
		log.Errorf("Database initialization error: %v", err)
		panic(pkgError.InternalServerError(fmt.Sprintf("Database initialization error: %v", err)))
	}

	return storeContainer
}

// initDatabase creates and returns a database store container based on the configured URI
func initDatabase(dbLog waLog.Logger) (*sqlstore.Container, error) {
	db, err := sql.Open("postgres", config.DBURI)
	if err != nil {
		return nil, fmt.Errorf("Error Connect WhatsApp Client Datastore: %w", err)
	}

	container := sqlstore.NewWithDB(db, "postgres", dbLog)
	WhatsAppDatastore = container
	Db = db

	err = updateDatabase()
	if err != nil {
		return nil, fmt.Errorf("Failed to migrate db tables: %w", err)
	}

	return container, nil
}

// InitWaCLI initializes the WhatsApp client
func InitWaCLI(storeContainer *sqlstore.Container) *whatsmeow.Client {

	device, err := storeContainer.GetFirstDevice()

	if err != nil {
		log.Errorf("Failed to get device: %v", err)
		panic(err)
	}

	if device == nil {
		log.Errorf("No device found")
		panic("No device found")
	}

	// Configure device properties
	osName := fmt.Sprintf("%s %s", config.AppOs, config.AppVersion)
	store.DeviceProps.PlatformType = &config.AppPlatform
	store.DeviceProps.Os = &osName

	// Create and configure the client
	cli = whatsmeow.NewClient(device, waLog.Stdout("Client", config.WhatsappLogLevel, true))
	cli.EnableAutoReconnect = true
	cli.AutoTrustIdentity = true
	cli.AddEventHandler(handler)

	return cli
}

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

// handler is the main event handler for WhatsApp events
func handler(rawEvt interface{}) {
	switch evt := rawEvt.(type) {
	case *events.DeleteForMe:
		handleDeleteForMe(evt)
	case *events.AppStateSyncComplete:
		handleAppStateSyncComplete(evt)
	case *events.PairSuccess:
		handlePairSuccess(evt)
	case *events.LoggedOut:
		handleLoggedOut()
	case *events.Connected, *events.PushNameSetting:
		handleConnectionEvents()
	case *events.StreamReplaced:
		handleStreamReplaced()
	case *events.Message:
		handleMessage(evt)
	case *events.Receipt:
		handleReceipt(evt)
	case *events.Presence:
		handlePresence(evt)
	case *events.HistorySync:
		handleHistorySync(evt)
	case *events.AppState:
		handleAppState(evt)
	}
}

// Event handler functions

func handleDeleteForMe(evt *events.DeleteForMe) {
	log.Infof("Deleted message %s for %s", evt.MessageID, evt.SenderJID.String())
}

func handleAppStateSyncComplete(evt *events.AppStateSyncComplete) {
	if len(cli.Store.PushName) > 0 && evt.Name == appstate.WAPatchCriticalBlock {
		if err := cli.SendPresence(types.PresenceAvailable); err != nil {
			log.Warnf("Failed to send available presence: %v", err)
		} else {
			log.Infof("Marked self as available")
		}
	}
}

func handlePairSuccess(evt *events.PairSuccess) {
	websocket.Broadcast <- websocket.BroadcastMessage{
		Code:    "LOGIN_SUCCESS",
		Message: fmt.Sprintf("Successfully pair with %s", evt.ID.String()),
	}
}

func handleLoggedOut() {
	websocket.Broadcast <- websocket.BroadcastMessage{
		Code:   "LIST_DEVICES",
		Result: nil,
	}
}

func handleConnectionEvents() {
	if len(cli.Store.PushName) == 0 {
		return
	}

	// Send presence available when connecting and when the pushname is changed.
	// This makes sure that outgoing messages always have the right pushname.
	if err := cli.SendPresence(types.PresenceAvailable); err != nil {
		log.Warnf("Failed to send available presence: %v", err)
	} else {
		log.Infof("Marked self as available")
	}
}

func handleStreamReplaced() {
	os.Exit(0)
}

func handleMessage(evt *events.Message) {
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
	handleImageMessage(evt)

	// Handle auto-reply if configured
	handleAutoReply(evt)

	// Forward to webhook if configured
	handleWebhookForward(evt)
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

func handleImageMessage(evt *events.Message) {
	if img := evt.Message.GetImageMessage(); img != nil {
		if path, err := ExtractMedia(config.PathStorages, img); err != nil {
			log.Errorf("Failed to download image: %v", err)
		} else {
			log.Infof("Image downloaded to %s", path)
		}
	}
}

func handleAutoReply(evt *events.Message) {
	// if config.WhatsappAutoReplyMessage != "" &&
	// 	!isGroupJid(evt.Info.Chat.String()) &&
	// 	!evt.Info.IsIncomingBroadcast() &&
	// 	evt.Message.GetExtendedTextMessage().GetText() != "" {
	// 	_, _ = cli.SendMessage(
	// 		context.Background(),
	// 		FormatJID(evt.Info.Sender.String()),
	// 		&waE2E.Message{Conversation: proto.String(config.WhatsappAutoReplyMessage)},
	// 	)
	// }
}

func handleWebhookForward(evt *events.Message) {
	if len(config.WhatsappWebhook) > 0 &&
		!strings.Contains(evt.Info.SourceString(), "broadcast") &&
		!isFromMySelf(evt.Info.SourceString()) {
		go func(evt *events.Message) {
			if err := forwardToWebhook(evt); err != nil {
				logrus.Error("Failed forward to webhook: ", err)
			}
		}(evt)
	}
}

func handleReceipt(evt *events.Receipt) {
	if evt.Type == types.ReceiptTypeRead || evt.Type == types.ReceiptTypeReadSelf {
		log.Infof("%v was read by %s at %s", evt.MessageIDs, evt.SourceString(), evt.Timestamp)
	} else if evt.Type == types.ReceiptTypeDelivered {
		log.Infof("%s was delivered to %s at %s", evt.MessageIDs[0], evt.SourceString(), evt.Timestamp)
	}
}

func handlePresence(evt *events.Presence) {
	if evt.Unavailable {
		if evt.LastSeen.IsZero() {
			log.Infof("%s is now offline", evt.From)
		} else {
			log.Infof("%s is now offline (last seen: %s)", evt.From, evt.LastSeen)
		}
	} else {
		log.Infof("%s is now online", evt.From)
	}
}

func handleHistorySync(evt *events.HistorySync) {
	id := atomic.AddInt32(&historySyncID, 1)
	fileName := fmt.Sprintf("%s/history-%d-%s-%d-%s.json",
		config.PathStorages,
		startupTime,
		cli.Store.ID.String(),
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

func handleAppState(evt *events.AppState) {
	log.Debugf("App state event: %+v / %+v", evt.Index, evt.SyncActionValue)
}
