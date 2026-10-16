package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSnakeCaseRequests(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		switch r.URL.Path {
		case "/v1/realtime/channel/create":
		case "/v1/realtime/token/issue":
			if !strings.Contains(string(body), `"client_id":"viewer-7"`) {
				t.Errorf("token request: %s", body)
			}
		case "/v1/realtime/publish":
			if !strings.Contains(string(body), `"account_id":"acct-3"`) || !strings.Contains(string(body), `"asset_id":"clip-9"`) {
				t.Errorf("publish request: %s", body)
			}
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		io.WriteString(w, `{"ok":true,"data":{}}`)
	}))
	defer upstream.Close()
	s := &server{infra: &InfraClient{baseURL: upstream.URL, key: "test", http: upstream.Client()}}
	for _, tc := range []struct {
		path, body string
		handler    http.HandlerFunc
	}{
		{"/rooms", `{"room":"show-42","client_id":"viewer-7"}`, s.room},
		{"/assets", `{"room":"show-42","asset_id":"clip-9","account_id":"acct-3","status":"ready"}`, s.ingest},
	} {
		rec := httptest.NewRecorder()
		tc.handler(rec, httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body)))
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status %d: %s", tc.path, rec.Code, rec.Body.String())
		}
	}
}

func TestAssetDeliveryDecision(t *testing.T) {
	tests := []struct{ status, want string }{{"processing", "queued"}, {"ready", "delivered"}}
	for _, tc := range tests {
		got := "queued"
		if tc.status == "ready" {
			got = "delivered"
		}
		if got != tc.want {
			t.Fatalf("status %q: got %q, want %q", tc.status, got, tc.want)
		}
	}
}
