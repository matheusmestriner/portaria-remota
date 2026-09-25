// Deploy one bridge per real company, with an exclusive SQL schema/database.
package main
import(
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
 "go.mau.fi/whatsmeow/store/sqlstore"
 "go.mau.fi/whatsmeow/types"
 "go.mau.fi/whatsmeow/types/events"
 "go.mau.fi/whatsmeow/proto/waE2E"
 "google.golang.org/protobuf/proto"
)
type Bridge struct{client *whatsmeow.Client;company,api,key string;http *http.Client;outbound *sql.DB;mu sync.Mutex;qr string;qrUntil time.Time;connecting bool;messages chan *events.Message}
func main(){ctx:=context.Background();company:=os.Getenv("COMPANY_ID");key:=os.Getenv("WHATSAPP_KEY");dsn:=os.Getenv("WHATSAPP_DATABASE_URL");api:=os.Getenv("API_URL");if company==""||key==""||dsn==""||api==""{log.Fatal("Configure COMPANY_ID, WHATSAPP_KEY, WHATSAPP_DATABASE_URL and API_URL")};outbound,err:=sql.Open("postgres",dsn);must(err);must(initOutbound(outbound));container,err:=sqlstore.New(ctx,"postgres",dsn,nil);must(err);store,err:=container.GetFirstDevice(ctx);must(err);cli:=whatsmeow.NewClient(store,nil);b:=&Bridge{client:cli,company:company,key:key,api:strings.TrimRight(api,"/"),http:&http.Client{Timeout:20*time.Second},outbound:outbound,messages:make(chan *events.Message,200)};cli.AddEventHandler(b.event);go func(){for m:=range b.messages{b.message(m)}}();if cli.Store.ID!=nil{if e:=cli.Connect();e!=nil{log.Printf("connection pending: %v",e)}}
 go func(){ticker:=time.NewTicker(30*time.Second);defer ticker.Stop();for range ticker.C{state:="disconnected";if cli.IsConnected()&&cli.IsLoggedIn(){state="connected"};b.reportStatus(state)}}();mux:=http.NewServeMux();mux.HandleFunc("POST /connect",b.connect);mux.HandleFunc("GET /status",b.status);mux.HandleFunc("POST /send",b.send);mux.HandleFunc("POST /disconnect",b.disconnect);server:=&http.Server{Addr:":8090",Handler:http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Service-Key")),[]byte(key))!=1{http.Error(w,"unauthorized",401);return};w.Header().Set("Cache-Control","no-store");mux.ServeHTTP(w,r)}),ReadHeaderTimeout:5*time.Second,ReadTimeout:10*time.Second,WriteTimeout:20*time.Second};log.Fatal(server.ListenAndServe())}
func must(e error){if e!=nil{log.Fatal(e)}}
func initOutbound(db *sql.DB)error{_,err:=db.Exec(`CREATE TABLE IF NOT EXISTS portalia_outbound_messages (
 request_key text PRIMARY KEY,
 recipient text NOT NULL,
 body_hash text NOT NULL,
 status text NOT NULL CHECK(status IN ('pending','sent','unknown')),
 provider_message_id text,
 created_at timestamptz NOT NULL DEFAULT now(),
 sent_at timestamptz,
 last_error text
)`);return err}
func normalizePhone(value string)(string,types.JID,error){value=strings.TrimSpace(value);if !regexp.MustCompile(`^\\+?[1-9][0-9]{7,14}// Deploy one bridge per real company, with an exclusive SQL schema/database.
package main
import(
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
 "go.mau.fi/whatsmeow/store/sqlstore"
 "go.mau.fi/whatsmeow/types"
 "go.mau.fi/whatsmeow/types/events"
 "go.mau.fi/whatsmeow/proto/waE2E"
 "google.golang.org/protobuf/proto"
)
type Bridge struct{client *whatsmeow.Client;company,api,key string;http *http.Client;outbound *sql.DB;mu sync.Mutex;qr string;qrUntil time.Time;connecting bool;messages chan *events.Message}
func main(){ctx:=context.Background();company:=os.Getenv("COMPANY_ID");key:=os.Getenv("WHATSAPP_KEY");dsn:=os.Getenv("WHATSAPP_DATABASE_URL");api:=os.Getenv("API_URL");if company==""||key==""||dsn==""||api==""{log.Fatal("Configure COMPANY_ID, WHATSAPP_KEY, WHATSAPP_DATABASE_URL and API_URL")};outbound,err:=sql.Open("postgres",dsn);must(err);must(initOutbound(outbound));container,err:=sqlstore.New(ctx,"postgres",dsn,nil);must(err);store,err:=container.GetFirstDevice(ctx);must(err);cli:=whatsmeow.NewClient(store,nil);b:=&Bridge{client:cli,company:company,key:key,api:strings.TrimRight(api,"/"),http:&http.Client{Timeout:20*time.Second},outbound:outbound,messages:make(chan *events.Message,200)};cli.AddEventHandler(b.event);go func(){for m:=range b.messages{b.message(m)}}();if cli.Store.ID!=nil{if e:=cli.Connect();e!=nil{log.Printf("connection pending: %v",e)}}
 go func(){ticker:=time.NewTicker(30*time.Second);defer ticker.Stop();for range ticker.C{state:="disconnected";if cli.IsConnected()&&cli.IsLoggedIn(){state="connected"};b.reportStatus(state)}}();mux:=http.NewServeMux();mux.HandleFunc("POST /connect",b.connect);mux.HandleFunc("GET /status",b.status);mux.HandleFunc("POST /send",b.send);mux.HandleFunc("POST /disconnect",b.disconnect);server:=&http.Server{Addr:":8090",Handler:http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Service-Key")),[]byte(key))!=1{http.Error(w,"unauthorized",401);return};w.Header().Set("Cache-Control","no-store");mux.ServeHTTP(w,r)}),ReadHeaderTimeout:5*time.Second,ReadTimeout:10*time.Second,WriteTimeout:20*time.Second};log.Fatal(server.ListenAndServe())}
).MatchString(value){return "",types.JID{},fmt.Errorf("invalid recipient")};digits:=strings.TrimPrefix(value,"+");return "+"+digits,types.NewJID(digits,types.DefaultUserServer),nil}
func(b *Bridge)phone()string{if b.client.Store.ID==nil{return ""};jid:=b.client.Store.ID.ToNonAD();if jid.Server!=types.DefaultUserServer||!regexp.MustCompile(`^[1-9][0-9]{7,14}// Deploy one bridge per real company, with an exclusive SQL schema/database.
package main
import(
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
 "go.mau.fi/whatsmeow/store/sqlstore"
 "go.mau.fi/whatsmeow/types"
 "go.mau.fi/whatsmeow/types/events"
 "go.mau.fi/whatsmeow/proto/waE2E"
 "google.golang.org/protobuf/proto"
)
type Bridge struct{client *whatsmeow.Client;company,api,key string;http *http.Client;outbound *sql.DB;mu sync.Mutex;qr string;qrUntil time.Time;connecting bool;messages chan *events.Message}
func main(){ctx:=context.Background();company:=os.Getenv("COMPANY_ID");key:=os.Getenv("WHATSAPP_KEY");dsn:=os.Getenv("WHATSAPP_DATABASE_URL");api:=os.Getenv("API_URL");if company==""||key==""||dsn==""||api==""{log.Fatal("Configure COMPANY_ID, WHATSAPP_KEY, WHATSAPP_DATABASE_URL and API_URL")};outbound,err:=sql.Open("postgres",dsn);must(err);must(initOutbound(outbound));container,err:=sqlstore.New(ctx,"postgres",dsn,nil);must(err);store,err:=container.GetFirstDevice(ctx);must(err);cli:=whatsmeow.NewClient(store,nil);b:=&Bridge{client:cli,company:company,key:key,api:strings.TrimRight(api,"/"),http:&http.Client{Timeout:20*time.Second},outbound:outbound,messages:make(chan *events.Message,200)};cli.AddEventHandler(b.event);go func(){for m:=range b.messages{b.message(m)}}();if cli.Store.ID!=nil{if e:=cli.Connect();e!=nil{log.Printf("connection pending: %v",e)}}
 go func(){ticker:=time.NewTicker(30*time.Second);defer ticker.Stop();for range ticker.C{state:="disconnected";if cli.IsConnected()&&cli.IsLoggedIn(){state="connected"};b.reportStatus(state)}}();mux:=http.NewServeMux();mux.HandleFunc("POST /connect",b.connect);mux.HandleFunc("GET /status",b.status);mux.HandleFunc("POST /send",b.send);mux.HandleFunc("POST /disconnect",b.disconnect);server:=&http.Server{Addr:":8090",Handler:http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Service-Key")),[]byte(key))!=1{http.Error(w,"unauthorized",401);return};w.Header().Set("Cache-Control","no-store");mux.ServeHTTP(w,r)}),ReadHeaderTimeout:5*time.Second,ReadTimeout:10*time.Second,WriteTimeout:20*time.Second};log.Fatal(server.ListenAndServe())}
).MatchString(jid.User){return ""};return "+"+jid.User}
func(b *Bridge)reportStatus(state string){payload:=map[string]any{"company_id":b.company,"status":state};if phone:=b.phone();phone!=""{payload["phone"]=phone};if err:=b.post("/internal/whatsapp/status",payload,nil);err!=nil{log.Printf("status report failed: %v",err)}}
func(b *Bridge)post(path string,payload any,result any)error{data,e:=json.Marshal(payload);if e!=nil{return e};req,e:=http.NewRequest("POST",b.api+path,bytes.NewReader(data));if e!=nil{return e};req.Header.Set("Content-Type","application/json");req.Header.Set("X-Service-Key",b.key);resp,e:=b.http.Do(req);if e!=nil{return e};defer resp.Body.Close();if resp.StatusCode<200||resp.StatusCode>=300{return fmt.Errorf("API status %d",resp.StatusCode)};if result!=nil{return json.NewDecoder(io.LimitReader(resp.Body,16384)).Decode(result)};return nil}
func(b *Bridge)event(evt any){switch e:=evt.(type){case *events.Message:if e.Info.IsFromMe||e.Info.IsGroup||e.IsEdit||e.SourceWebMsg!=nil{return};select{case b.messages<-e:default:log.Print("incoming queue full; no authorization performed")};case *events.Connected:b.mu.Lock();b.qr="";b.connecting=false;b.mu.Unlock();b.reportStatus("connected");case *events.Disconnected:b.reportStatus("disconnected");case *events.LoggedOut:b.mu.Lock();b.connecting=false;b.mu.Unlock();b.reportStatus("logged_out")}}
func(b *Bridge)message(e *events.Message){
 if time.Since(e.Info.Timestamp)>15*time.Minute{return};sender:=e.Info.Sender.ToNonAD();if sender.Server==types.HiddenUserServer{resolved,err:=b.client.Store.LIDs.GetPNForLID(context.Background(),sender);if err!=nil||resolved.IsEmpty(){return};sender=resolved};if sender.Server!=types.DefaultUserServer||!regexp.MustCompile(`^[1-9][0-9]{7,14}$`).MatchString(sender.User){return};text:=e.Message.GetConversation();if text==""{text=e.Message.GetExtendedTextMessage().GetText()};if text==""||len(text)>2000{return};var response struct{Reply string `json:"reply"`};var err error;for attempt:=0;attempt<3;attempt++{err=b.post("/internal/whatsapp/message",map[string]string{"company_id":b.company,"message_id":e.Info.ID,"sender":"+"+sender.User,"text":text},&response);if err==nil{break};time.Sleep(time.Duration(attempt+1)*time.Second)};if err!=nil{response.Reply="Não foi possível concluir sua solicitação. Confira seus convites no aplicativo antes de tentar novamente."};if response.Reply!=""{_,err=b.client.SendMessage(context.Background(),e.Info.Chat,&waE2E.Message{Conversation:proto.String(response.Reply)});if err!=nil{log.Print("reply delivery failed; invite creation is protected against duplicates")}}
}
func(b *Bridge)connect(w http.ResponseWriter,r *http.Request){b.mu.Lock();if b.connecting||b.client.IsConnected(){b.mu.Unlock();b.status(w,r);return};b.connecting=true;b.mu.Unlock();if b.client.Store.ID!=nil{err:=b.client.Connect();if err!=nil{b.mu.Lock();b.connecting=false;b.mu.Unlock();http.Error(w,"connection failed",503);return};b.status(w,r);return};ch,err:=b.client.GetQRChannel(context.Background());if err!=nil{http.Error(w,"QR unavailable",503);return};go func(){for evt:=range ch{b.mu.Lock();if evt.Event=="code"{b.qr=evt.Code;b.qrUntil=time.Now().Add(evt.Timeout)}else if evt.Event!="success"{b.qr=""};b.mu.Unlock()};b.mu.Lock();b.connecting=false;b.mu.Unlock()}();if err=b.client.Connect();err!=nil{b.mu.Lock();b.connecting=false;b.mu.Unlock();http.Error(w,"connection failed",503);return};b.status(w,r)}
func(b *Bridge)status(w http.ResponseWriter,r *http.Request){b.mu.Lock();defer b.mu.Unlock();qr:=b.qr;if time.Now().After(b.qrUntil){qr=""};state:="disconnected";if b.client.IsConnected()&&b.client.IsLoggedIn(){state="connected"};w.Header().Set("Content-Type","application/json");json.NewEncoder(w).Encode(map[string]any{"company_id":b.company,"status":state,"phone":b.phone(),"qr":qr,"qr_expires_at":b.qrUntil})}
func(b *Bridge)send(w http.ResponseWriter,r *http.Request){
 if !b.client.IsConnected()||!b.client.IsLoggedIn(){http.Error(w,"whatsapp not connected",503);return}
 var in struct{To string `json:"to"`;Text string `json:"text"`;RequestKey string `json:"request_key"`}
 dec:=json.NewDecoder(io.LimitReader(r.Body,16384));dec.DisallowUnknownFields();if err:=dec.Decode(&in);err!=nil{http.Error(w,"invalid payload",400);return}
 recipient,jid,err:=normalizePhone(in.To);if err!=nil{http.Error(w,"invalid recipient",400);return};in.Text=strings.TrimSpace(in.Text);if len(in.Text)<1||len(in.Text)>2000{http.Error(w,"invalid text",400);return};if !regexp.MustCompile(`^[A-Za-z0-9:_-]{16,128}// Deploy one bridge per real company, with an exclusive SQL schema/database.
package main
import(
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
 "go.mau.fi/whatsmeow/store/sqlstore"
 "go.mau.fi/whatsmeow/types"
 "go.mau.fi/whatsmeow/types/events"
 "go.mau.fi/whatsmeow/proto/waE2E"
 "google.golang.org/protobuf/proto"
)
type Bridge struct{client *whatsmeow.Client;company,api,key string;http *http.Client;outbound *sql.DB;mu sync.Mutex;qr string;qrUntil time.Time;connecting bool;messages chan *events.Message}
func main(){ctx:=context.Background();company:=os.Getenv("COMPANY_ID");key:=os.Getenv("WHATSAPP_KEY");dsn:=os.Getenv("WHATSAPP_DATABASE_URL");api:=os.Getenv("API_URL");if company==""||key==""||dsn==""||api==""{log.Fatal("Configure COMPANY_ID, WHATSAPP_KEY, WHATSAPP_DATABASE_URL and API_URL")};outbound,err:=sql.Open("postgres",dsn);must(err);must(initOutbound(outbound));container,err:=sqlstore.New(ctx,"postgres",dsn,nil);must(err);store,err:=container.GetFirstDevice(ctx);must(err);cli:=whatsmeow.NewClient(store,nil);b:=&Bridge{client:cli,company:company,key:key,api:strings.TrimRight(api,"/"),http:&http.Client{Timeout:20*time.Second},outbound:outbound,messages:make(chan *events.Message,200)};cli.AddEventHandler(b.event);go func(){for m:=range b.messages{b.message(m)}}();if cli.Store.ID!=nil{if e:=cli.Connect();e!=nil{log.Printf("connection pending: %v",e)}}
 go func(){ticker:=time.NewTicker(30*time.Second);defer ticker.Stop();for range ticker.C{state:="disconnected";if cli.IsConnected()&&cli.IsLoggedIn(){state="connected"};b.reportStatus(state)}}();mux:=http.NewServeMux();mux.HandleFunc("POST /connect",b.connect);mux.HandleFunc("GET /status",b.status);mux.HandleFunc("POST /send",b.send);mux.HandleFunc("POST /disconnect",b.disconnect);server:=&http.Server{Addr:":8090",Handler:http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Service-Key")),[]byte(key))!=1{http.Error(w,"unauthorized",401);return};w.Header().Set("Cache-Control","no-store");mux.ServeHTTP(w,r)}),ReadHeaderTimeout:5*time.Second,ReadTimeout:10*time.Second,WriteTimeout:20*time.Second};log.Fatal(server.ListenAndServe())}
func must(e error){if e!=nil{log.Fatal(e)}}
func initOutbound(db *sql.DB)error{_,err:=db.Exec(`CREATE TABLE IF NOT EXISTS portalia_outbound_messages (
 request_key text PRIMARY KEY,
 recipient text NOT NULL,
 body_hash text NOT NULL,
 status text NOT NULL CHECK(status IN ('pending','sent','unknown')),
 provider_message_id text,
 created_at timestamptz NOT NULL DEFAULT now(),
 sent_at timestamptz,
 last_error text
)`);return err}
func normalizePhone(value string)(string,types.JID,error){value=strings.TrimSpace(value);if !regexp.MustCompile(`^\\+?[1-9][0-9]{7,14}// Deploy one bridge per real company, with an exclusive SQL schema/database.
package main
import(
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
 "go.mau.fi/whatsmeow/store/sqlstore"
 "go.mau.fi/whatsmeow/types"
 "go.mau.fi/whatsmeow/types/events"
 "go.mau.fi/whatsmeow/proto/waE2E"
 "google.golang.org/protobuf/proto"
)
type Bridge struct{client *whatsmeow.Client;company,api,key string;http *http.Client;outbound *sql.DB;mu sync.Mutex;qr string;qrUntil time.Time;connecting bool;messages chan *events.Message}
func main(){ctx:=context.Background();company:=os.Getenv("COMPANY_ID");key:=os.Getenv("WHATSAPP_KEY");dsn:=os.Getenv("WHATSAPP_DATABASE_URL");api:=os.Getenv("API_URL");if company==""||key==""||dsn==""||api==""{log.Fatal("Configure COMPANY_ID, WHATSAPP_KEY, WHATSAPP_DATABASE_URL and API_URL")};outbound,err:=sql.Open("postgres",dsn);must(err);must(initOutbound(outbound));container,err:=sqlstore.New(ctx,"postgres",dsn,nil);must(err);store,err:=container.GetFirstDevice(ctx);must(err);cli:=whatsmeow.NewClient(store,nil);b:=&Bridge{client:cli,company:company,key:key,api:strings.TrimRight(api,"/"),http:&http.Client{Timeout:20*time.Second},outbound:outbound,messages:make(chan *events.Message,200)};cli.AddEventHandler(b.event);go func(){for m:=range b.messages{b.message(m)}}();if cli.Store.ID!=nil{if e:=cli.Connect();e!=nil{log.Printf("connection pending: %v",e)}}
 go func(){ticker:=time.NewTicker(30*time.Second);defer ticker.Stop();for range ticker.C{state:="disconnected";if cli.IsConnected()&&cli.IsLoggedIn(){state="connected"};b.reportStatus(state)}}();mux:=http.NewServeMux();mux.HandleFunc("POST /connect",b.connect);mux.HandleFunc("GET /status",b.status);mux.HandleFunc("POST /send",b.send);mux.HandleFunc("POST /disconnect",b.disconnect);server:=&http.Server{Addr:":8090",Handler:http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Service-Key")),[]byte(key))!=1{http.Error(w,"unauthorized",401);return};w.Header().Set("Cache-Control","no-store");mux.ServeHTTP(w,r)}),ReadHeaderTimeout:5*time.Second,ReadTimeout:10*time.Second,WriteTimeout:20*time.Second};log.Fatal(server.ListenAndServe())}
).MatchString(value){return "",types.JID{},fmt.Errorf("invalid recipient")};digits:=strings.TrimPrefix(value,"+");return "+"+digits,types.NewJID(digits,types.DefaultUserServer),nil}
func(b *Bridge)phone()string{if b.client.Store.ID==nil{return ""};jid:=b.client.Store.ID.ToNonAD();if jid.Server!=types.DefaultUserServer||!regexp.MustCompile(`^[1-9][0-9]{7,14}// Deploy one bridge per real company, with an exclusive SQL schema/database.
package main
import(
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
 "go.mau.fi/whatsmeow/store/sqlstore"
 "go.mau.fi/whatsmeow/types"
 "go.mau.fi/whatsmeow/types/events"
 "go.mau.fi/whatsmeow/proto/waE2E"
 "google.golang.org/protobuf/proto"
)
type Bridge struct{client *whatsmeow.Client;company,api,key string;http *http.Client;outbound *sql.DB;mu sync.Mutex;qr string;qrUntil time.Time;connecting bool;messages chan *events.Message}
func main(){ctx:=context.Background();company:=os.Getenv("COMPANY_ID");key:=os.Getenv("WHATSAPP_KEY");dsn:=os.Getenv("WHATSAPP_DATABASE_URL");api:=os.Getenv("API_URL");if company==""||key==""||dsn==""||api==""{log.Fatal("Configure COMPANY_ID, WHATSAPP_KEY, WHATSAPP_DATABASE_URL and API_URL")};outbound,err:=sql.Open("postgres",dsn);must(err);must(initOutbound(outbound));container,err:=sqlstore.New(ctx,"postgres",dsn,nil);must(err);store,err:=container.GetFirstDevice(ctx);must(err);cli:=whatsmeow.NewClient(store,nil);b:=&Bridge{client:cli,company:company,key:key,api:strings.TrimRight(api,"/"),http:&http.Client{Timeout:20*time.Second},outbound:outbound,messages:make(chan *events.Message,200)};cli.AddEventHandler(b.event);go func(){for m:=range b.messages{b.message(m)}}();if cli.Store.ID!=nil{if e:=cli.Connect();e!=nil{log.Printf("connection pending: %v",e)}}
 go func(){ticker:=time.NewTicker(30*time.Second);defer ticker.Stop();for range ticker.C{state:="disconnected";if cli.IsConnected()&&cli.IsLoggedIn(){state="connected"};b.reportStatus(state)}}();mux:=http.NewServeMux();mux.HandleFunc("POST /connect",b.connect);mux.HandleFunc("GET /status",b.status);mux.HandleFunc("POST /send",b.send);mux.HandleFunc("POST /disconnect",b.disconnect);server:=&http.Server{Addr:":8090",Handler:http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Service-Key")),[]byte(key))!=1{http.Error(w,"unauthorized",401);return};w.Header().Set("Cache-Control","no-store");mux.ServeHTTP(w,r)}),ReadHeaderTimeout:5*time.Second,ReadTimeout:10*time.Second,WriteTimeout:20*time.Second};log.Fatal(server.ListenAndServe())}
).MatchString(jid.User){return ""};return "+"+jid.User}
func(b *Bridge)reportStatus(state string){payload:=map[string]any{"company_id":b.company,"status":state};if phone:=b.phone();phone!=""{payload["phone"]=phone};if err:=b.post("/internal/whatsapp/status",payload,nil);err!=nil{log.Printf("status report failed: %v",err)}}
func(b *Bridge)post(path string,payload any,result any)error{data,e:=json.Marshal(payload);if e!=nil{return e};req,e:=http.NewRequest("POST",b.api+path,bytes.NewReader(data));if e!=nil{return e};req.Header.Set("Content-Type","application/json");req.Header.Set("X-Service-Key",b.key);resp,e:=b.http.Do(req);if e!=nil{return e};defer resp.Body.Close();if resp.StatusCode<200||resp.StatusCode>=300{return fmt.Errorf("API status %d",resp.StatusCode)};if result!=nil{return json.NewDecoder(io.LimitReader(resp.Body,16384)).Decode(result)};return nil}
func(b *Bridge)event(evt any){switch e:=evt.(type){case *events.Message:if e.Info.IsFromMe||e.Info.IsGroup||e.IsEdit||e.SourceWebMsg!=nil{return};select{case b.messages<-e:default:log.Print("incoming queue full; no authorization performed")};case *events.Connected:b.mu.Lock();b.qr="";b.connecting=false;b.mu.Unlock();b.reportStatus("connected");case *events.Disconnected:b.reportStatus("disconnected");case *events.LoggedOut:b.mu.Lock();b.connecting=false;b.mu.Unlock();b.reportStatus("logged_out")}}
func(b *Bridge)message(e *events.Message){
 if time.Since(e.Info.Timestamp)>15*time.Minute{return};sender:=e.Info.Sender.ToNonAD();if sender.Server==types.HiddenUserServer{resolved,err:=b.client.Store.LIDs.GetPNForLID(context.Background(),sender);if err!=nil||resolved.IsEmpty(){return};sender=resolved};if sender.Server!=types.DefaultUserServer||!regexp.MustCompile(`^[1-9][0-9]{7,14}$`).MatchString(sender.User){return};text:=e.Message.GetConversation();if text==""{text=e.Message.GetExtendedTextMessage().GetText()};if text==""||len(text)>2000{return};var response struct{Reply string `json:"reply"`};var err error;for attempt:=0;attempt<3;attempt++{err=b.post("/internal/whatsapp/message",map[string]string{"company_id":b.company,"message_id":e.Info.ID,"sender":"+"+sender.User,"text":text},&response);if err==nil{break};time.Sleep(time.Duration(attempt+1)*time.Second)};if err!=nil{response.Reply="Não foi possível concluir sua solicitação. Confira seus convites no aplicativo antes de tentar novamente."};if response.Reply!=""{_,err=b.client.SendMessage(context.Background(),e.Info.Chat,&waE2E.Message{Conversation:proto.String(response.Reply)});if err!=nil{log.Print("reply delivery failed; invite creation is protected against duplicates")}}
}
func(b *Bridge)connect(w http.ResponseWriter,r *http.Request){b.mu.Lock();if b.connecting||b.client.IsConnected(){b.mu.Unlock();b.status(w,r);return};b.connecting=true;b.mu.Unlock();if b.client.Store.ID!=nil{err:=b.client.Connect();if err!=nil{b.mu.Lock();b.connecting=false;b.mu.Unlock();http.Error(w,"connection failed",503);return};b.status(w,r);return};ch,err:=b.client.GetQRChannel(context.Background());if err!=nil{http.Error(w,"QR unavailable",503);return};go func(){for evt:=range ch{b.mu.Lock();if evt.Event=="code"{b.qr=evt.Code;b.qrUntil=time.Now().Add(evt.Timeout)}else if evt.Event!="success"{b.qr=""};b.mu.Unlock()};b.mu.Lock();b.connecting=false;b.mu.Unlock()}();if err=b.client.Connect();err!=nil{b.mu.Lock();b.connecting=false;b.mu.Unlock();http.Error(w,"connection failed",503);return};b.status(w,r)}
).MatchString(in.RequestKey){http.Error(w,"invalid request key",400);return}
 sum:=sha256.Sum256([]byte(in.Text));bodyHash:=fmt.Sprintf("%x",sum[:])
 var existingRecipient,existingHash,state,providerID string
 err=b.outbound.QueryRow("SELECT recipient,body_hash,status,coalesce(provider_message_id,'') FROM portalia_outbound_messages WHERE request_key=$1",in.RequestKey).Scan(&existingRecipient,&existingHash,&state,&providerID)
 if err==nil{if existingRecipient!=recipient||existingHash!=bodyHash{http.Error(w,"request key collision",409);return};if state=="sent"{w.Header().Set("Content-Type","application/json");json.NewEncoder(w).Encode(map[string]any{"status":"sent","request_key":in.RequestKey,"to":recipient,"provider_message_id":providerID});return};http.Error(w,"previous send state is uncertain",409);return}
 if err!=sql.ErrNoRows{http.Error(w,"message store unavailable",503);return}
 if _,err=b.outbound.Exec("INSERT INTO portalia_outbound_messages(request_key,recipient,body_hash,status) VALUES($1,$2,$3,'pending')",in.RequestKey,recipient,bodyHash);err!=nil{http.Error(w,"message store unavailable",503);return}
 response,err:=b.client.SendMessage(r.Context(),jid,&waE2E.Message{Conversation:proto.String(in.Text)})
 if err!=nil{_,_=b.outbound.Exec("UPDATE portalia_outbound_messages SET status='unknown',last_error='provider_error' WHERE request_key=$1",in.RequestKey);log.Printf("outbound send failed request_key=%s: %v",in.RequestKey,err);http.Error(w,"send state uncertain",503);return}
 providerID:=string(response.ID);_,_=b.outbound.Exec("UPDATE portalia_outbound_messages SET status='sent',provider_message_id=$1,sent_at=now(),last_error=NULL WHERE request_key=$2",providerID,in.RequestKey)
 w.Header().Set("Content-Type","application/json");json.NewEncoder(w).Encode(map[string]any{"status":"sent","request_key":in.RequestKey,"to":recipient,"provider_message_id":providerID})
}
func(b *Bridge)disconnect(w http.ResponseWriter,r *http.Request){b.client.Disconnect();b.mu.Lock();b.qr="";b.connecting=false;b.mu.Unlock();b.reportStatus("disconnected");b.status(w,r)}
