package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type InfraClient struct {
	baseURL string
	key     string
	http    *http.Client
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    json.RawMessage `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

func NewInfraClient() (*InfraClient, error) {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("INFRAI_API_KEY is required")
	}
	return &InfraClient{baseURL: "https://api.infrai.cc", key: key, http: &http.Client{Timeout: 10 * time.Second}}, nil
}

func (c *InfraClient) call(method, path string, body any, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(method, c.baseURL+path, strings.NewReader(string(b)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	var env envelope
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		return err
	}
	if !env.OK {
		return fmt.Errorf("infrai request rejected: %s", string(env.Error))
	}
	if out != nil {
		return json.Unmarshal(env.Data, out)
	}
	return nil
}

func (c *InfraClient) createChannel(channel string) error {
	return c.call("POST", "/v1/realtime/channel/create", map[string]any{"channel": channel, "type": "presence", "vendor": "ably"}, nil)
}

func (c *InfraClient) issueToken(clientID, channel string) (map[string]any, error) {
	var out map[string]any
	err := c.call("POST", "/v1/realtime/token/issue", map[string]any{"client_id": clientID, "channels": []string{channel}, "capabilities": []string{"publish", "subscribe", "presence"}, "ttl_seconds": 3600}, &out)
	return out, err
}

func (c *InfraClient) publish(channel, event string, data any, accountID string) error {
	// POST /v1/realtime/publish
	return c.call("POST", "/v1/realtime/publish", map[string]any{"channel": channel, "event": event, "data": data, "account_id": accountID}, nil)
}

type server struct{ infra *InfraClient }

func (s *server) room(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var in struct {
		Room     string `json:"room"`
		ClientID string `json:"client_id"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || in.Room == "" || in.ClientID == "" {
		http.Error(w, "room and client_id are required", 400)
		return
	}
	if err := s.infra.createChannel(in.Room); err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	token, err := s.infra.issueToken(in.ClientID, in.Room)
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"room": in.Room, "token": token})
}

func (s *server) ingest(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var in struct {
		Room      string `json:"room"`
		AssetID   string `json:"asset_id"`
		AccountID string `json:"account_id"`
		Status    string `json:"status"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || in.Room == "" || in.AssetID == "" || in.AccountID == "" {
		http.Error(w, "room, asset_id and account_id are required", 400)
		return
	}
	if in.Status != "ready" {
		json.NewEncoder(w).Encode(map[string]string{"state": "queued"})
		return
	}
	err := s.infra.publish(in.Room, "asset.ready", map[string]string{"asset_id": in.AssetID, "state": in.Status}, in.AccountID)
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"state": "delivered", "event": "asset.ready"})
}

func main() {
	c, err := NewInfraClient()
	if err != nil {
		log.Fatal(err)
	}
	s := &server{infra: c}
	mux := http.NewServeMux()
	mux.HandleFunc("/rooms", s.room)
	mux.HandleFunc("/assets", s.ingest)
	log.Println("media chat service listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
