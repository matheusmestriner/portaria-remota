// One gateway is deployed per tenant/revenda. It can host multiple isolated WhatsApp sessions:
// one default account for the revenda and optional accounts for individual condominiums.
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
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

var (
	phonePattern       = regexp.MustCompile(`^\+?[1-9][0-9]{7,14}$`)
	phoneDigitsPattern = regexp.MustCompile(`^[1-9][0-9]{7,14}$`)
	requestKeyPattern  = regexp.MustCompile(`^[A-Za-z0-9:_-]{16,128}$`)
	accountKeyPattern  = regexp.MustCompile(`^[A-Za-z0-9:_-]{1,128}$`)
)

type Gateway struct {
	container *sqlstore.Container
	company   string
	api       string
	key       string
	http      *http.Client
	db        *sql.DB
	mu        sync.Mutex
	sessions  map[string]*Session
}

type Session struct {
	gateway    *Gateway
	accountKey string
	condoID    string
	client     *whatsmeow.Client
	mu         sync.Mutex
	qr         string
	qrUntil    time.Time
	pairCode   string
	pairPhone  string
	pairUntil  time.Time
	connecting bool
	messages   chan *events.Message
}

type accountRequest struct {
	AccountKey string `json:"account_key"`
	CondoID    string `json:"condo_id,omitempty"`
}

type sendRequest struct {
	AccountKey string `json:"account_key"`
	CondoID    string `json:"condo_id,omitempty"`
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

	db, err := sql.Open("postgres", dsn)
	must(err)
	must(db.PingContext(ctx))
	must(initGatewayTables(db))

	container, err := sqlstore.New(ctx, "postgres", dsn, nil)
	must(err)

	g := &Gateway{
		container: container,
		company:   company,
		key:       key,
		api:       strings.TrimRight(api, "/"),
		http:      &http.Client{Timeout: 20 * time.Second},
		db:        db,
		sessions:  map[string]*Session{},
	}
	must(g.loadSessions(ctx))

	mux := http.NewServeMux()
	mux.HandleFunc("POST /connect", g.connect)
	mux.HandleFunc("POST /pair-code", g.pairCode)
	mux.HandleFunc("GET /status", g.status)
	mux.HandleFunc("POST /send", g.send)
	mux.HandleFunc("POST /disconnect", g.disconnect)

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
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      25 * time.Second,
	}

	log.Printf("whatsmeow gateway ready company=%s sessions=%d", company, len(g.sessions))
	log.Fatal(server.ListenAndServe())
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func initGatewayTables(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS portalia_whatsapp_sessions (
 account_key text PRIMARY KEY,
 condo_id text,
 device_jid text UNIQUE,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS portalia_whatsapp_sessions_condo_unique
 ON portalia_whatsapp_sessions(condo_id)
 WHERE condo_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS portalia_outbound_messages_v2 (
 account_key text NOT NULL,
 request_key text NOT NULL,
 recipient text NOT NULL,
 body_hash text NOT NULL,
 status text NOT NULL CHECK(status IN ('pending','sent','unknown')),
 provider_message_id text,
 created_at timestamptz NOT NULL DEFAULT now(),
 sent_at timestamptz,
 last_error text,
 PRIMARY KEY(account_key,request_key)
);

INSERT INTO portalia_outbound_messages_v2(account_key,request_key,recipient,body_hash,status,provider_message_id,created_at,sent_at,last_error)
SELECT 'default',request_key,recipient,body_hash,status,provider_message_id,created_at,sent_at,last_error
FROM portalia_outbound_messages
ON CONFLICT DO NOTHING;
`)
	return err
}

func (g *Gateway) loadSessions(ctx context.Context) error {
	rows, err := g.db.QueryContext(ctx, "SELECT account_key,coalesce(condo_id,''),device_jid FROM portalia_whatsapp_sessions WHERE device_jid IS NOT NULL")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var key, condo, rawJID string
		if err := rows.Scan(&key, &condo, &rawJID); err != nil {
			return err
		}
		jid, err := types.ParseJID(rawJID)
		if err != nil {
			log.Printf("invalid stored jid account=%s: %v", key, err)
			continue
		}
		device, err := g.container.GetDevice(ctx, jid)
		if err != nil || device == nil {
			log.Printf("stored WhatsApp device unavailable account=%s: %v", key, err)
			continue
		}
		s := g.newSession(key, condo, device)
		g.sessions[key] = s
		if err := s.client.Connect(); err != nil {
			log.Printf("connection pending account=%s: %v", key, err)
		}
	}
	return rows.Err()
}

func (g *Gateway) newSession(key, condo string, device *store.Device) *Session {
	s := &Session{
		gateway:    g,
		accountKey: key,
		condoID:    condo,
		client:     whatsmeow.NewClient(device, nil),
		messages:   make(chan *events.Message, 200),
	}
	s.client.AddEventHandler(s.event)
	go func() {
		for msg := range s.messages {
			s.message(msg)
		}
	}()
	return s
}

func (g *Gateway) session(ctx context.Context, key, condo string) (*Session, error) {
	if !accountKeyPattern.MatchString(key) {
		return nil, fmt.Errorf("invalid account key")
	}
	if condo != "" {
		if _, err := types.ParseJID("1@" + types.DefaultUserServer); err != nil {
			return nil, err
		}
	}

	g.mu.Lock()
	if existing := g.sessions[key]; existing != nil {
		g.mu.Unlock()
		if existing.condoID != condo {
			return nil, fmt.Errorf("account scope mismatch")
		}
		return existing, nil
	}
	g.mu.Unlock()

	var storedCondo string
	var rawJID sql.NullString
	err := g.db.QueryRowContext(ctx,
		"SELECT coalesce(condo_id,''),device_jid FROM portalia_whatsapp_sessions WHERE account_key=$1",
		key,
	).Scan(&storedCondo, &rawJID)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if err == nil && storedCondo != condo {
		return nil, fmt.Errorf("account scope mismatch")
	}
	if err == sql.ErrNoRows {
		_, err = g.db.ExecContext(ctx,
			"INSERT INTO portalia_whatsapp_sessions(account_key,condo_id) VALUES($1,nullif($2,''))",
			key, condo,
		)
		if err != nil {
			return nil, err
		}
	}

	var device *store.Device
	if rawJID.Valid {
		jid, err := types.ParseJID(rawJID.String)
		if err != nil {
			return nil, err
		}
		device, err = g.container.GetDevice(ctx, jid)
		if err != nil {
			return nil, err
		}
	}
	if device == nil {
		device = g.container.NewDevice()
	}

	s := g.newSession(key, condo, device)
	g.mu.Lock()
	if winner := g.sessions[key]; winner != nil {
		g.mu.Unlock()
		return winner, nil
	}
	g.sessions[key] = s
	g.mu.Unlock()
	return s, nil
}

func (s *Session) persistIdentity() {
	if s.client.Store.ID == nil {
		return
	}
	_, err := s.gateway.db.Exec(
		"UPDATE portalia_whatsapp_sessions SET device_jid=$1,updated_at=now() WHERE account_key=$2",
		s.client.Store.ID.String(), s.accountKey,
	)
	if err != nil {
		log.Printf("failed to persist WhatsApp identity account=%s: %v", s.accountKey, err)
	}
}

func (s *Session) phone() string {
	if s.client.Store.ID == nil {
		return ""
	}
	jid := s.client.Store.ID.ToNonAD()
	if jid.Server != types.DefaultUserServer || !phoneDigitsPattern.MatchString(jid.User) {
		return ""
	}
	return "+" + jid.User
}

func (s *Session) reportStatus(state string) {
	payload := map[string]any{
		"company_id":  s.gateway.company,
		"account_key": s.accountKey,
		"status":      state,
	}
	if s.condoID != "" {
		payload["condo_id"] = s.condoID
	}
	if phone := s.phone(); phone != "" {
		payload["phone"] = phone
	}
	if err := s.gateway.post("/internal/whatsapp/status", payload, nil); err != nil {
		log.Printf("status report failed account=%s: %v", s.accountKey, err)
	}
}

func (g *Gateway) post(path string, payload any, result any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", g.api+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-Key", g.key)
	resp, err := g.http.Do(req)
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

func (s *Session) event(evt any) {
	switch e := evt.(type) {
	case *events.Message:
		if e.Info.IsFromMe || e.Info.IsGroup || e.IsEdit || e.SourceWebMsg != nil {
			return
		}
		select {
		case s.messages <- e:
		default:
			log.Printf("incoming queue full account=%s; no authorization performed", s.accountKey)
		}
	case *events.PairSuccess:
		s.persistIdentity()
	case *events.Connected:
		s.persistIdentity()
		s.mu.Lock()
		s.qr = ""
		s.pairCode = ""
		s.pairPhone = ""
		s.pairUntil = time.Time{}
		s.connecting = false
		s.mu.Unlock()
		s.reportStatus("connected")
	case *events.Disconnected:
		s.reportStatus("disconnected")
	case *events.LoggedOut:
		s.mu.Lock()
		s.connecting = false
		s.qr = ""
		s.pairCode = ""
		s.pairPhone = ""
		s.pairUntil = time.Time{}
		s.mu.Unlock()
		_, _ = s.gateway.db.Exec("UPDATE portalia_whatsapp_sessions SET device_jid=NULL,updated_at=now() WHERE account_key=$1", s.accountKey)
		s.reportStatus("logged_out")
	}
}

func (s *Session) message(e *events.Message) {
	if time.Since(e.Info.Timestamp) > 15*time.Minute {
		return
	}
	sender := e.Info.Sender.ToNonAD()
	if sender.Server == types.HiddenUserServer {
		resolved, err := s.client.Store.LIDs.GetPNForLID(context.Background(), sender)
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

	payload := map[string]any{
		"company_id":  s.gateway.company,
		"account_key": s.accountKey,
		"message_id":  e.Info.ID,
		"sender":      "+" + sender.User,
		"text":        text,
	}
	if s.condoID != "" {
		payload["condo_id"] = s.condoID
	}
	var response struct {
		Reply string `json:"reply"`
	}
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		err = s.gateway.post("/internal/whatsapp/message", payload, &response)
		if err == nil {
			break
		}
		time.Sleep(time.Duration(attempt+1) * time.Second)
	}
	if err != nil {
		response.Reply = "Não foi possível concluir sua solicitação. Confira seus convites no aplicativo antes de tentar novamente."
	}
	if response.Reply != "" {
		_, err = s.client.SendMessage(context.Background(), e.Info.Chat, &waE2E.Message{Conversation: proto.String(response.Reply)})
		if err != nil {
			log.Printf("reply delivery failed account=%s", s.accountKey)
		}
	}
}

func readAccountRequest(r *http.Request) (accountRequest, error) {
	var in accountRequest
	decoder := json.NewDecoder(io.LimitReader(r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&in); err != nil {
		return in, err
	}
	if !accountKeyPattern.MatchString(in.AccountKey) {
		return in, fmt.Errorf("invalid account key")
	}
	return in, nil
}

func (g *Gateway) connect(w http.ResponseWriter, r *http.Request) {
	in, err := readAccountRequest(r)
	if err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	s, err := g.session(r.Context(), in.AccountKey, in.CondoID)
	if err != nil {
		http.Error(w, "account unavailable", http.StatusConflict)
		return
	}
	s.connectQR(w, r)
}

func (s *Session) connectQR(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	if s.connecting || s.client.IsConnected() {
		s.mu.Unlock()
		s.writeStatus(w)
		return
	}
	s.connecting = true
	s.pairCode = ""
	s.pairPhone = ""
	s.pairUntil = time.Time{}
	s.mu.Unlock()

	if s.client.Store.ID != nil {
		if err := s.client.Connect(); err != nil {
			s.resetPairing()
			http.Error(w, "connection failed", http.StatusServiceUnavailable)
			return
		}
		s.writeStatus(w)
		return
	}

	ch, err := s.client.GetQRChannel(context.Background())
	if err != nil {
		s.resetPairing()
		http.Error(w, "QR unavailable", http.StatusServiceUnavailable)
		return
	}
	go func() {
		for evt := range ch {
			s.mu.Lock()
			if evt.Event == "code" {
				s.qr = evt.Code
				s.qrUntil = time.Now().Add(evt.Timeout)
			} else if evt.Event != "success" {
				s.qr = ""
			}
			s.mu.Unlock()
		}
		s.mu.Lock()
		s.connecting = false
		s.mu.Unlock()
	}()
	if err = s.client.Connect(); err != nil {
		s.resetPairing()
		http.Error(w, "connection failed", http.StatusServiceUnavailable)
		return
	}
	s.writeStatus(w)
}

func (g *Gateway) pairCode(w http.ResponseWriter, r *http.Request) {
	var in struct {
		AccountKey string `json:"account_key"`
		CondoID    string `json:"condo_id,omitempty"`
		Phone      string `json:"phone"`
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&in); err != nil || !accountKeyPattern.MatchString(in.AccountKey) {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	phone, _, err := normalizePhone(in.Phone)
	if err != nil {
		http.Error(w, "invalid phone", http.StatusBadRequest)
		return
	}
	s, err := g.session(r.Context(), in.AccountKey, in.CondoID)
	if err != nil {
		http.Error(w, "account unavailable", http.StatusConflict)
		return
	}
	s.pairByCode(w, r, phone)
}

func (s *Session) pairByCode(w http.ResponseWriter, r *http.Request, phone string) {
	if s.client.Store.ID != nil || s.client.IsLoggedIn() {
		http.Error(w, "whatsapp already paired", http.StatusConflict)
		return
	}
	s.mu.Lock()
	if s.connecting || s.client.IsConnected() {
		s.mu.Unlock()
		http.Error(w, "pairing already in progress", http.StatusConflict)
		return
	}
	s.connecting = true
	s.qr = ""
	s.qrUntil = time.Time{}
	s.pairCode = ""
	s.pairPhone = phone
	s.pairUntil = time.Time{}
	s.mu.Unlock()

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	ch, err := s.client.GetQRChannel(ctx)
	if err != nil {
		s.resetPairing()
		http.Error(w, "pairing unavailable", http.StatusServiceUnavailable)
		return
	}
	if err = s.client.Connect(); err != nil {
		s.resetPairing()
		http.Error(w, "connection failed", http.StatusServiceUnavailable)
		return
	}
	select {
	case evt, ok := <-ch:
		if !ok || evt.Event != "code" {
			s.client.Disconnect()
			s.resetPairing()
			http.Error(w, "pairing handshake unavailable", http.StatusServiceUnavailable)
			return
		}
	case <-ctx.Done():
		s.client.Disconnect()
		s.resetPairing()
		http.Error(w, "pairing handshake timeout", http.StatusGatewayTimeout)
		return
	}

	code, err := s.client.PairPhone(context.Background(), strings.TrimPrefix(phone, "+"), true, whatsmeow.PairClientChrome, "Chrome (Windows)")
	if err != nil {
		s.client.Disconnect()
		s.resetPairing()
		http.Error(w, "pairing code unavailable", http.StatusServiceUnavailable)
		return
	}
	expires := time.Now().Add(150 * time.Second)
	s.mu.Lock()
	s.pairCode = code
	s.pairPhone = phone
	s.pairUntil = expires
	s.mu.Unlock()

	go func() {
		for range ch {
		}
		s.mu.Lock()
		if s.client.Store.ID == nil {
			s.connecting = false
			s.pairCode = ""
			s.pairPhone = ""
			s.pairUntil = time.Time{}
		}
		s.mu.Unlock()
	}()

	writeJSON(w, map[string]any{
		"company_id":     s.gateway.company,
		"account_key":    s.accountKey,
		"condo_id":       nullableString(s.condoID),
		"status":         "pairing",
		"pairing_method": "code",
		"phone":          phone,
		"pair_code":      code,
		"expires_at":     expires,
	})
}

func (g *Gateway) status(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("account_key")
	condo := r.URL.Query().Get("condo_id")
	if !accountKeyPattern.MatchString(key) {
		http.Error(w, "invalid account key", http.StatusBadRequest)
		return
	}
	s, err := g.session(r.Context(), key, condo)
	if err != nil {
		http.Error(w, "account unavailable", http.StatusConflict)
		return
	}
	s.writeStatus(w)
}

func (s *Session) writeStatus(w http.ResponseWriter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	qr := s.qr
	if time.Now().After(s.qrUntil) {
		qr = ""
	}
	pairCode := s.pairCode
	pairPhone := s.pairPhone
	pairUntil := s.pairUntil
	if !pairUntil.IsZero() && time.Now().After(pairUntil) {
		pairCode = ""
		pairPhone = ""
	}
	state := "disconnected"
	if s.client.IsConnected() && s.client.IsLoggedIn() {
		state = "connected"
	} else if s.connecting {
		state = "connecting"
	}
	writeJSON(w, map[string]any{
		"company_id":      s.gateway.company,
		"account_key":     s.accountKey,
		"condo_id":        nullableString(s.condoID),
		"status":          state,
		"phone":           s.phone(),
		"qr":              qr,
		"qr_expires_at":   s.qrUntil,
		"pair_code":       pairCode,
		"pair_phone":      pairPhone,
		"pair_expires_at": pairUntil,
	})
}

func (g *Gateway) send(w http.ResponseWriter, r *http.Request) {
	var in sendRequest
	decoder := json.NewDecoder(io.LimitReader(r.Body, 16384))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&in); err != nil || !accountKeyPattern.MatchString(in.AccountKey) {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	s, err := g.session(r.Context(), in.AccountKey, in.CondoID)
	if err != nil {
		http.Error(w, "account unavailable", http.StatusConflict)
		return
	}
	s.sendMessage(w, r, in)
}

func (s *Session) sendMessage(w http.ResponseWriter, r *http.Request, in sendRequest) {
	if !s.client.IsConnected() || !s.client.IsLoggedIn() {
		http.Error(w, "whatsapp not connected", http.StatusServiceUnavailable)
		return
	}
	recipient, jid, err := normalizePhone(in.To)
	if err != nil {
		http.Error(w, "invalid recipient", http.StatusBadRequest)
		return
	}
	in.Text = strings.TrimSpace(in.Text)
	if len(in.Text) < 1 || len(in.Text) > 2000 || !requestKeyPattern.MatchString(in.RequestKey) {
		http.Error(w, "invalid message", http.StatusBadRequest)
		return
	}
	sum := sha256.Sum256([]byte(in.Text))
	bodyHash := fmt.Sprintf("%x", sum[:])

	var existingRecipient, existingHash, state, providerID string
	err = s.gateway.db.QueryRow(
		"SELECT recipient,body_hash,status,coalesce(provider_message_id,'') FROM portalia_outbound_messages_v2 WHERE account_key=$1 AND request_key=$2",
		s.accountKey, in.RequestKey,
	).Scan(&existingRecipient, &existingHash, &state, &providerID)
	if err == nil {
		if existingRecipient != recipient || existingHash != bodyHash {
			http.Error(w, "request key collision", http.StatusConflict)
			return
		}
		if state == "sent" {
			writeJSON(w, map[string]any{"company_id": s.gateway.company, "account_key": s.accountKey, "status": "sent", "request_key": in.RequestKey, "to": recipient, "provider_message_id": providerID})
			return
		}
		http.Error(w, "previous send state is uncertain", http.StatusConflict)
		return
	}
	if err != sql.ErrNoRows {
		http.Error(w, "message store unavailable", http.StatusServiceUnavailable)
		return
	}

	result, err := s.gateway.db.Exec(
		"INSERT INTO portalia_outbound_messages_v2(account_key,request_key,recipient,body_hash,status) VALUES($1,$2,$3,$4,'pending') ON CONFLICT DO NOTHING",
		s.accountKey, in.RequestKey, recipient, bodyHash,
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

	response, err := s.client.SendMessage(r.Context(), jid, &waE2E.Message{Conversation: proto.String(in.Text)})
	if err != nil {
		_, _ = s.gateway.db.Exec("UPDATE portalia_outbound_messages_v2 SET status='unknown',last_error='provider_error' WHERE account_key=$1 AND request_key=$2", s.accountKey, in.RequestKey)
		log.Printf("outbound send failed account=%s request_key=%s: %v", s.accountKey, in.RequestKey, err)
		http.Error(w, "send state uncertain", http.StatusServiceUnavailable)
		return
	}
	providerID = string(response.ID)
	_, _ = s.gateway.db.Exec("UPDATE portalia_outbound_messages_v2 SET status='sent',provider_message_id=$1,sent_at=now(),last_error=NULL WHERE account_key=$2 AND request_key=$3", providerID, s.accountKey, in.RequestKey)
	writeJSON(w, map[string]any{"company_id": s.gateway.company, "account_key": s.accountKey, "status": "sent", "request_key": in.RequestKey, "to": recipient, "provider_message_id": providerID})
}

func (g *Gateway) disconnect(w http.ResponseWriter, r *http.Request) {
	in, err := readAccountRequest(r)
	if err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	s, err := g.session(r.Context(), in.AccountKey, in.CondoID)
	if err != nil {
		http.Error(w, "account unavailable", http.StatusConflict)
		return
	}
	s.client.Disconnect()
	s.resetPairing()
	s.reportStatus("disconnected")
	s.writeStatus(w)
}

func (s *Session) resetPairing() {
	s.mu.Lock()
	s.connecting = false
	s.qr = ""
	s.qrUntil = time.Time{}
	s.pairCode = ""
	s.pairPhone = ""
	s.pairUntil = time.Time{}
	s.mu.Unlock()
}

func normalizePhone(value string) (string, types.JID, error) {
	value = strings.TrimSpace(value)
	if !phonePattern.MatchString(value) {
		return "", types.JID{}, fmt.Errorf("invalid recipient")
	}
	digits := strings.TrimPrefix(value, "+")
	return "+" + digits, types.NewJID(digits, types.DefaultUserServer), nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}
