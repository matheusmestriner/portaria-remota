// Deploy one bridge per real company, with an exclusive SQL schema/database.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

var (
	phonePattern      = regexp.MustCompile(`^\\+?[1-9][0-9]{7,14}$`)
	phoneDigitsPattern = regexp.MustCompile(`^[1-9][0-9]{7,14}$`)
	requestKeyPattern = regexp.MustCompile(`^[A-Za-z0-9:_-]{16,128}$`)
)

type Bridge struct {
	client     *whatsmeow.Client
	company    string
	api        string
	key        string
	http       *http.Client
	outbound   *sql.DB
	mu         sync.Mutex
	qr         string
	qrUntil    time.Time
	connecting bool
	messages   chan *events.Message
}

type sendRequest struct {
	To         string `json:"to"`
	Text       string `json:"text"`
	RequestKey string `json:"request_key"`
}

func main() {
	ctx := context.Background()
	company := os.Getenv("COMPANY_ID")
	key := os.Getenv("WHATSAPP_KEY")
	dsn := os.Getenv("WHATSAPP_DATABASE_URL")
	api := os.Getenv("API_URL")
	if company == "" || key == "" || dsn == "" || api == "" {
		log.Fatal("Configure COMPANY_ID, WHATSAPP_KEY, WHATSAPP_DATABASE_URL and API_URL")
	}

	outbound, err := sql.Open("postgres", dsn)
	must(err)
	must(outbound.PingContext(ctx))
	must(initOutbound(outbound))

	container, err := sqlstore.New(ctx, "postgres", dsn, nil)
	must(err)
	store, err := container.GetFirstDevice(ctx)
	must(err)
	cli := whatsmeow.NewClient(store, nil)

	b := &Bridge{
		client:   cli,
		company:  company,
		key:      key,
		api:      strings.TrimRight(api, "/"),
		http:     &http.Client{Timeout: 20 * time.Second},
		outbound: outbound,
		messages: make(chan *events.Message, 200),
	}

	cli.AddEventHandler(b.event)
	go func() {
		for m := range b.messages {
			b.message(m)
		}
	}()

	if cli.Store.ID != nil {
		if err := cli.Connect(); err != nil {
			log.Printf("connection pending: %v", err)
		}
	}

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			state := "disconnected"
			if cli.IsConnected() && cli.IsLoggedIn() {
				state = "connected"
			}
			b.reportStatus(state)
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /connect", b.connect)
	mux.HandleFunc("GET /status", b.status)
	mux.HandleFunc("POST /send", b.send)
	mux.HandleFunc("POST /disconnect", b.disconnect)

	server := &http.Server{
		Addr: ":8090",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Service-Key")), []byte(key)) != 1 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			mux.ServeHTTP(w, r)
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      20 * time.Second,
	}

	log.Fatal(server.ListenAndServe())
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func initOutbound(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS portalia_outbound_messages (
		request_key text PRIMARY KEY,
		recipient text NOT NULL,
		body_hash text NOT NULL,
		status text NOT NULL CHECK(status IN ('pending','sent','unknown')),
		provider_message_id text,
		created_at timestamptz NOT NULL DEFAULT now(),
		sent_at timestamptz,
		last_error text
	)`)
	return err
}

func normalizePhone(value string) (string, types.JID, error) {
	value = strings.TrimSpace(value)
	if !phonePattern.MatchString(value) {
		return "", types.JID{}, fmt.Errorf("invalid recipient")
	}
	digits := strings.TrimPrefix(value, "+")
	return "+" + digits, types.NewJID(digits, types.DefaultUserServer), nil
}

func (b *Bridge) phone() string {
	if b.client.Store.ID == nil {
		return ""
	}
	jid := b.client.Store.ID.ToNonAD()
	if jid.Server != types.DefaultUserServer || !phoneDigitsPattern.MatchString(jid.User) {
		return ""
	}
	return "+" + jid.User
}

func (b *Bridge) reportStatus(state string) {
	payload := map[string]any{
		"company_id": b.company,
		"status":     state,
	}
	if phone := b.phone(); phone != "" {
		payload["phone"] = phone
	}
	if err := b.post("/internal/whatsapp/status", payload, nil); err != nil {
		log.Printf("status report failed: %v", err)
	}
}

func (b *Bridge) post(path string, payload any, result any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", b.api+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-Key", b.key)

	resp, err := b.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("API status %d", resp.StatusCode)
	}
	if result != nil {
		return json.NewDecoder(io.LimitReader(resp.Body, 16384)).Decode(result)
	}
	return nil
}

func (b *Bridge) event(evt any) {
	switch e := evt.(type) {
	case *events.Message:
		if e.Info.IsFromMe || e.Info.IsGroup || e.IsEdit || e.SourceWebMsg != nil {
			return
		}
		select {
		case b.messages <- e:
		default:
			log.Print("incoming queue full; no authorization performed")
		}
	case *events.Connected:
		b.mu.Lock()
		b.qr = ""
		b.connecting = false
		b.mu.Unlock()
		b.reportStatus("connected")
	case *events.Disconnected:
		b.reportStatus("disconnected")
	case *events.LoggedOut:
		b.mu.Lock()
		b.connecting = false
		b.qr = ""
		b.mu.Unlock()
		b.reportStatus("logged_out")
	}
}

func (b *Bridge) message(e *events.Message) {
	if time.Since(e.Info.Timestamp) > 15*time.Minute {
		return
	}
	sender := e.Info.Sender.ToNonAD()
	if sender.Server == types.HiddenUserServer {
		resolved, err := b.client.Store.LIDs.GetPNForLID(context.Background(), sender)
		if err != nil || resolved.IsEmpty() {
			return
		}
		sender = resolved
	}
	if sender.Server != types.DefaultUserServer || !phoneDigitsPattern.MatchString(sender.User) {
		return
	}

	text := e.Message.GetConversation()
	if text == "" {
		text = e.Message.GetExtendedTextMessage().GetText()
	}
	if text == "" || len(text) > 2000 {
		return
	}

	var response struct {
		Reply string `json:"reply"`
	}
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		err = b.post("/internal/whatsapp/message", map[string]string{
			"company_id": b.company,
			"message_id": e.Info.ID,
			"sender":     "+" + sender.User,
			"text":       text,
		}, &response)
		if err == nil {
			break
		}
		time.Sleep(time.Duration(attempt+1) * time.Second)
	}
	if err != nil {
		response.Reply = "Não foi possível concluir sua solicitação. Confira seus convites no aplicativo antes de tentar novamente."
	}
	if response.Reply != "" {
		_, err = b.client.SendMessage(context.Background(), e.Info.Chat, &waE2E.Message{Conversation: proto.String(response.Reply)})
		if err != nil {
			log.Print("reply delivery failed; invite creation is protected against duplicates")
		}
	}
}

func (b *Bridge) connect(w http.ResponseWriter, r *http.Request) {
	b.mu.Lock()
	if b.connecting || b.client.IsConnected() {
		b.mu.Unlock()
		b.status(w, r)
		return
	}
	b.connecting = true
	b.mu.Unlock()

	if b.client.Store.ID != nil {
		err := b.client.Connect()
		if err != nil {
			b.mu.Lock()
			b.connecting = false
			b.mu.Unlock()
			http.Error(w, "connection failed", http.StatusServiceUnavailable)
			return
		}
		b.status(w, r)
		return
	}

	ch, err := b.client.GetQRChannel(context.Background())
	if err != nil {
		b.mu.Lock()
		b.connecting = false
		b.mu.Unlock()
		http.Error(w, "QR unavailable", http.StatusServiceUnavailable)
		return
	}
	go func() {
		for evt := range ch {
			b.mu.Lock()
			if evt.Event == "code" {
				b.qr = evt.Code
				b.qrUntil = time.Now().Add(evt.Timeout)
			} else if evt.Event != "success" {
				b.qr = ""
			}
			b.mu.Unlock()
		}
		b.mu.Lock()
		b.connecting = false
		b.mu.Unlock()
	}()

	if err = b.client.Connect(); err != nil {
		b.mu.Lock()
		b.connecting = false
		b.mu.Unlock()
		http.Error(w, "connection failed", http.StatusServiceUnavailable)
		return
	}
	b.status(w, r)
}

func (b *Bridge) status(w http.ResponseWriter, r *http.Request) {
	b.mu.Lock()
	defer b.mu.Unlock()

	qr := b.qr
	if time.Now().After(b.qrUntil) {
		qr = ""
	}
	state := "disconnected"
	if b.client.IsConnected() && b.client.IsLoggedIn() {
		state = "connected"
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"company_id":    b.company,
		"status":        state,
		"phone":         b.phone(),
		"qr":            qr,
		"qr_expires_at": b.qrUntil,
	})
}

func (b *Bridge) send(w http.ResponseWriter, r *http.Request) {
	if !b.client.IsConnected() || !b.client.IsLoggedIn() {
		http.Error(w, "whatsapp not connected", http.StatusServiceUnavailable)
		return
	}

	var in sendRequest
	decoder := json.NewDecoder(io.LimitReader(r.Body, 16384))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&in); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	recipient, jid, err := normalizePhone(in.To)
	if err != nil {
		http.Error(w, "invalid recipient", http.StatusBadRequest)
		return
	}
	in.Text = strings.TrimSpace(in.Text)
	if len(in.Text) < 1 || len(in.Text) > 2000 {
		http.Error(w, "invalid text", http.StatusBadRequest)
		return
	}
	if !requestKeyPattern.MatchString(in.RequestKey) {
		http.Error(w, "invalid request key", http.StatusBadRequest)
		return
	}

	sum := sha256.Sum256([]byte(in.Text))
	bodyHash := fmt.Sprintf("%x", sum[:])

	var existingRecipient, existingHash, state, providerID string
	err = b.outbound.QueryRow(
		"SELECT recipient,body_hash,status,coalesce(provider_message_id,'') FROM portalia_outbound_messages WHERE request_key=$1",
		in.RequestKey,
	).Scan(&existingRecipient, &existingHash, &state, &providerID)
	if err == nil {
		if existingRecipient != recipient || existingHash != bodyHash {
			http.Error(w, "request key collision", http.StatusConflict)
			return
		}
		if state == "sent" {
			writeJSON(w, map[string]any{
				"company_id":          b.company,
				"status":              "sent",
				"request_key":         in.RequestKey,
				"to":                  recipient,
				"provider_message_id": providerID,
			})
			return
		}
		http.Error(w, "previous send state is uncertain", http.StatusConflict)
		return
	}
	if err != sql.ErrNoRows {
		http.Error(w, "message store unavailable", http.StatusServiceUnavailable)
		return
	}

	result, err := b.outbound.Exec(
		"INSERT INTO portalia_outbound_messages(request_key,recipient,body_hash,status) VALUES($1,$2,$3,'pending') ON CONFLICT DO NOTHING",
		in.RequestKey, recipient, bodyHash,
	)
	if err != nil {
		http.Error(w, "message store unavailable", http.StatusServiceUnavailable)
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		http.Error(w, "request already in progress", http.StatusConflict)
		return
	}

	response, err := b.client.SendMessage(r.Context(), jid, &waE2E.Message{Conversation: proto.String(in.Text)})
	if err != nil {
		_, _ = b.outbound.Exec(
			"UPDATE portalia_outbound_messages SET status='unknown',last_error='provider_error' WHERE request_key=$1",
			in.RequestKey,
		)
		log.Printf("outbound send failed request_key=%s: %v", in.RequestKey, err)
		http.Error(w, "send state uncertain", http.StatusServiceUnavailable)
		return
	}

	providerID := string(response.ID)
	_, _ = b.outbound.Exec(
		"UPDATE portalia_outbound_messages SET status='sent',provider_message_id=$1,sent_at=now(),last_error=NULL WHERE request_key=$2",
		providerID, in.RequestKey,
	)

	writeJSON(w, map[string]any{
		"company_id":          b.company,
		"status":              "sent",
		"request_key":         in.RequestKey,
		"to":                  recipient,
		"provider_message_id": providerID,
	})
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func (b *Bridge) disconnect(w http.ResponseWriter, r *http.Request) {
	b.client.Disconnect()
	b.mu.Lock()
	b.qr = ""
	b.connecting = false
	b.mu.Unlock()
	b.reportStatus("disconnected")
	b.status(w, r)
}
