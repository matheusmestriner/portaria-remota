// The agent never opens a device from an arbitrary cloud-provided URL.
// Device identifiers resolve only through a locally managed allowlist.
package main

import (
 "bytes"
 "context"
 "crypto/subtle"
 "encoding/json"
 "fmt"
 "io"
 "log"
 "net/http"
 "net/url"
 "os"
 "path/filepath"
 "strings"
 "sync"
 "time"
)

type Device struct { URL string `json:"url"`; Token string `json:"token"` }
type Config struct { API string `json:"api"`; Token string `json:"token"`; LocalToken string `json:"local_token"`; Listen string `json:"listen"`; Journal string `json:"journal"`; Devices map[string]Device `json:"devices"` }
type Command struct { ID string `json:"id"`; DeviceID string `json:"device_id"`; AccessID string `json:"access_id"`; ExpiresAt time.Time `json:"expires_at"` }
type Pending struct { ID string `json:"id"`; State string `json:"state"` }
type Agent struct { config Config; client *http.Client; mu sync.Mutex; executed map[string]string }

func main(){
 path:=os.Getenv("AGENT_CONFIG");if path==""{path="config.json"};raw,err:=os.ReadFile(path);must(err);var cfg Config;must(json.Unmarshal(raw,&cfg))
 if cfg.API==""||cfg.Token==""||cfg.LocalToken==""||cfg.Journal==""{log.Fatal("API, token, local_token and durable journal path are required")}
 u,err:=url.Parse(cfg.API);must(err);if u.Scheme!="https"{log.Fatal("The remote API must use HTTPS")}
 if cfg.Listen==""{cfg.Listen="127.0.0.1:8091"}
 a:=&Agent{config:cfg,client:&http.Client{Timeout:4*time.Second,CheckRedirect:func(*http.Request,[]*http.Request)error{return http.ErrUseLastResponse}},executed:map[string]string{}}
 must(os.MkdirAll(filepath.Dir(cfg.Journal),0700));if b,e:=os.ReadFile(cfg.Journal);e==nil{must(json.Unmarshal(b,&a.executed))}else if !os.IsNotExist(e){must(e)}
 mux:=http.NewServeMux();mux.HandleFunc("POST /credentials/validate",a.validate);mux.HandleFunc("POST /events",a.event);mux.HandleFunc("GET /health",func(w http.ResponseWriter,r *http.Request){write(w,200,map[string]string{"status":"ok"})})
 go func(){server:=http.Server{Addr:cfg.Listen,Handler:mux,ReadHeaderTimeout:5*time.Second,ReadTimeout:10*time.Second,WriteTimeout:10*time.Second};log.Fatal(server.ListenAndServe())}()
 ticker:=time.NewTicker(2*time.Second);defer ticker.Stop();for{a.poll();a.syncCredentials();<-ticker.C}
}
func must(e error){if e!=nil{log.Fatal(e)}}
func (a *Agent) request(path string,data any,out any)error{b,e:=json.Marshal(data);if e!=nil{return e};req,e:=http.NewRequestWithContext(context.Background(),"POST",strings.TrimRight(a.config.API,"/")+path,bytes.NewReader(b));if e!=nil{return e};req.Header.Set("Authorization","Bearer "+a.config.Token);req.Header.Set("Content-Type","application/json");resp,e:=a.client.Do(req);if e!=nil{return e};defer resp.Body.Close();if resp.StatusCode<200||resp.StatusCode>=300{return fmt.Errorf("API status %d",resp.StatusCode)};if out!=nil{return json.NewDecoder(io.LimitReader(resp.Body,1<<20)).Decode(out)};return nil}
func (a *Agent) persist()error{b,e:=json.Marshal(a.executed);if e!=nil{return e};tmp:=a.config.Journal+".tmp";f,e:=os.OpenFile(tmp,os.O_CREATE|os.O_TRUNC|os.O_WRONLY,0600);if e!=nil{return e};if _,e=f.Write(b);e!=nil{f.Close();return e};if e=f.Sync();e!=nil{f.Close();return e};if e=f.Close();e!=nil{return e};return os.Rename(tmp,a.config.Journal)}
func (a *Agent) poll(){var commands []Command;if err:=a.request("/agent/poll",map[string]string{},&commands);err!=nil{log.Printf("poll unavailable: %v",err);return};for _,cmd:=range commands{state:=a.execute(cmd);if err:=a.request("/agent/commands/"+cmd.ID+"/ack",map[string]string{"state":state},nil);err!=nil{log.Printf("ack unavailable for %s; command will not be executed again",cmd.ID)}}}
func (a *Agent) execute(cmd Command)string{
 a.mu.Lock();defer a.mu.Unlock();if prior,ok:=a.executed[cmd.ID];ok{return prior};if !time.Now().Before(cmd.ExpiresAt){return "unknown"};device,ok:=a.config.Devices[cmd.DeviceID];if !ok{return "failed"}
 // Persist intent BEFORE I/O. A crash between intent and response is unknown, never retried.
 a.executed[cmd.ID]="unknown";if err:=a.persist();err!=nil{log.Printf("durable journal unavailable: %v",err);return "unknown"}
 state:=a.openDevice(device,cmd);a.executed[cmd.ID]=state;if err:=a.persist();err!=nil{log.Printf("journal update unavailable: %v",err)};return state
}
func (a *Agent) openDevice(device Device,cmd Command)string{if !time.Now().Before(cmd.ExpiresAt){return "unknown"};b,_:=json.Marshal(map[string]any{"command_id":cmd.ID,"access_id":cmd.AccessID,"expires_at":cmd.ExpiresAt});ctx,cancel:=context.WithDeadline(context.Background(),cmd.ExpiresAt);defer cancel();req,err:=http.NewRequestWithContext(ctx,"POST",strings.TrimRight(device.URL,"/")+"/open",bytes.NewReader(b));if err!=nil{return "failed"};req.Header.Set("Authorization","Bearer "+device.Token);req.Header.Set("Content-Type","application/json");req.Header.Set("Idempotency-Key",cmd.ID);resp,err:=a.client.Do(req);if err!=nil{return "unknown"};defer resp.Body.Close();if resp.StatusCode<200||resp.StatusCode>=300{return "failed"};var result struct{Confirmed bool `json:"confirmed"`;CommandID string `json:"command_id"`};if json.NewDecoder(io.LimitReader(resp.Body,4096)).Decode(&result)!=nil||result.CommandID!=cmd.ID||!result.Confirmed{return "unknown"};return "confirmed"}
func (a *Agent) authorized(r *http.Request)bool{return subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")),[]byte("Bearer "+a.config.LocalToken))==1}
func (a *Agent) validate(w http.ResponseWriter,r *http.Request){if !a.authorized(r){write(w,401,map[string]string{"error":"unauthorized"});return};var b struct{Credential string `json:"credential"`; AccessID string `json:"access_id"`; RequestKey string `json:"request_key"`;DeviceID string `json:"device_id"`};if json.NewDecoder(io.LimitReader(r.Body,8192)).Decode(&b)!=nil||b.RequestKey==""{write(w,400,map[string]string{"error":"invalid request"});return};var result map[string]any;if err:=a.request("/agent/redeem",map[string]string{"credential":b.Credential,"access_id":b.AccessID,"request_key":b.RequestKey},&result);err!=nil{write(w,403,map[string]string{"error":"access denied or offline"});return};if result["authorized"]!=true{write(w,200,result);return};deviceID,_:=result["device_id"].(string);cmdID,_:=result["command_id"].(string);if deviceID==""||cmdID==""{write(w,502,map[string]string{"error":"incomplete authorization"});return};state:=a.execute(Command{ID:cmdID,DeviceID:deviceID,AccessID:b.AccessID,ExpiresAt:parseExpiry(result["expires_at"])});_ = a.request("/agent/commands/"+cmdID+"/ack",map[string]string{"state":state},nil);write(w,200,map[string]any{"command_id":cmdID,"state":state,"passage_confirmed":false})}
func (a *Agent) event(w http.ResponseWriter,r *http.Request){if !a.authorized(r){write(w,401,nil);return};var body map[string]any;if json.NewDecoder(io.LimitReader(r.Body,8192)).Decode(&body)!=nil{write(w,400,nil);return};if err:=a.request("/agent/events",body,nil);err!=nil{write(w,503,map[string]string{"error":"event not acknowledged; retry with the same source_key"});return};write(w,200,map[string]bool{"received":true})}
type Credential struct{ ID string `json:"id"`; Data struct{DeviceID string `json:"device_id"`; Status string `json:"status"`;Type string `json:"type"`;Reference string `json:"reference"`} `json:"data"` }
func(a *Agent) syncCredentials(){var rows []Credential;if a.request("/agent/credentials/pending",map[string]string{},&rows)!=nil{return};for _,c:=range rows{device,ok:=a.config.Devices[c.Data.DeviceID];if !ok{continue};desired:="active";if c.Data.Status=="revocation_pending"{desired="revoked"};payload,_:=json.Marshal(map[string]string{"credential_id":c.ID,"type":c.Data.Type,"reference":c.Data.Reference,"desired_state":desired});req,e:=http.NewRequest("PUT",strings.TrimRight(device.URL,"/")+"/credentials/"+c.ID,bytes.NewReader(payload));if e!=nil{continue};req.Header.Set("Authorization","Bearer "+device.Token);req.Header.Set("Content-Type","application/json");resp,e:=a.client.Do(req);if e!=nil{continue};var confirmed struct{ID string `json:"credential_id"`;State string `json:"state"`};e=json.NewDecoder(io.LimitReader(resp.Body,4096)).Decode(&confirmed);resp.Body.Close();if e==nil&&resp.StatusCode==200&&confirmed.ID==c.ID&&confirmed.State==desired{_ = a.request("/agent/credentials/"+c.ID+"/ack",map[string]string{"state":desired},nil)}}}
func write(w http.ResponseWriter,status int,value any){w.Header().Set("Content-Type","application/json");w.WriteHeader(status);_ = json.NewEncoder(w).Encode(value)}

func parseExpiry(value any) time.Time { text,_:=value.(string); date,err:=time.Parse(time.RFC3339Nano,text); if err!=nil{return time.Time{}}; return date }
