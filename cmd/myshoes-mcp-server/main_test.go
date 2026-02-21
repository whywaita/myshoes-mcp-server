package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	uuid "github.com/satori/go.uuid"
	myshoesapi "github.com/whywaita/myshoes/api/myshoes"
	"github.com/whywaita/myshoes/pkg/web"
)

var testTargetID = uuid.NewV4().String()

var testTarget = web.UserTarget{
	UUID:         uuid.FromStringOrNil(testTargetID),
	Scope:        "octocat/hello-world",
	ResourceType: "nano",
	ProviderURL:  "http://provider.example.com",
	Status:       "active",
	CreatedAt:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	UpdatedAt:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
}

func newTestServer(t *testing.T) (*httptest.Server, *MyshoesMCPServer) {
	t.Helper()

	mux := http.NewServeMux()

	// GET /target/{id} - get target
	// POST /target/{id} - update target
	// DELETE /target/{id} - delete target
	mux.HandleFunc("/target/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/target/")
		if id == "" {
			http.Error(w, "missing id", http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodGet:
			if id != testTargetID {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(web.ErrorResponse{Error: "target not found"})
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(testTarget)

		case http.MethodPost:
			if id != testTargetID {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(web.ErrorResponse{Error: "target not found"})
				return
			}
			var param map[string]any
			if err := json.NewDecoder(r.Body).Decode(&param); err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(web.ErrorResponse{Error: "json decode error"})
				return
			}
			updated := testTarget
			if rt, ok := param["resource_type"]; ok {
				if s, ok := rt.(string); ok && s != "" {
					updated.ResourceType = s
				}
			}
			if pu, ok := param["provider_url"]; ok {
				if s, ok := pu.(string); ok && s != "" {
					updated.ProviderURL = s
				}
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(updated)

		case http.MethodDelete:
			if id != testTargetID {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(web.ErrorResponse{Error: "target not found"})
				return
			}
			w.WriteHeader(http.StatusNoContent)

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// POST /target - create target
	// GET /target - list targets (not tested here, already exists)
	mux.HandleFunc("/target", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			// For list, return array
			if r.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode([]web.UserTarget{testTarget})
				return
			}
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var param map[string]any
		if err := json.NewDecoder(r.Body).Decode(&param); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(web.ErrorResponse{Error: "json decode error"})
			return
		}

		scope, _ := param["scope"].(string)
		if scope == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(web.ErrorResponse{Error: "scope must be set"})
			return
		}

		created := web.UserTarget{
			UUID:         uuid.NewV4(),
			Scope:        scope,
			ResourceType: "nano",
			Status:       "active",
			CreatedAt:    time.Now().UTC(),
			UpdatedAt:    time.Now().UTC(),
		}
		if rt, ok := param["resource_type"]; ok {
			if s, ok := rt.(string); ok && s != "" {
				created.ResourceType = s
			}
		}
		if pu, ok := param["provider_url"]; ok {
			if s, ok := pu.(string); ok && s != "" {
				created.ProviderURL = s
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(created)
	})

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	client, err := myshoesapi.NewClient(ts.URL, ts.Client(), log.New(io.Discard, "", log.LstdFlags))
	if err != nil {
		t.Fatalf("failed to create myshoes client: %v", err)
	}

	mms := &MyshoesMCPServer{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		client: client,
	}

	return ts, mms
}

func TestGetTargetHandler(t *testing.T) {
	_, mms := newTestServer(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		input := GetTargetInput{TargetID: testTargetID}
		result, _, err := mms.getTargetHandler(ctx, &mcp.CallToolRequest{}, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.Content) == 0 {
			t.Fatal("expected content in result")
		}

		text := result.Content[0].(*mcp.TextContent).Text
		var got web.UserTarget
		if err := json.Unmarshal([]byte(text), &got); err != nil {
			t.Fatalf("failed to unmarshal result: %v", err)
		}
		if got.Scope != testTarget.Scope {
			t.Errorf("scope: got %q, want %q", got.Scope, testTarget.Scope)
		}
		if got.ResourceType != testTarget.ResourceType {
			t.Errorf("resource_type: got %q, want %q", got.ResourceType, testTarget.ResourceType)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		input := GetTargetInput{TargetID: uuid.NewV4().String()}
		_, _, err := mms.getTargetHandler(ctx, &mcp.CallToolRequest{}, input)
		if err == nil {
			t.Fatal("expected error for non-existent target")
		}
	})
}

func TestCreateTargetHandler(t *testing.T) {
	_, mms := newTestServer(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		input := CreateTargetInput{
			Scope:        "octocat/new-repo",
			ResourceType: "small",
			ProviderURL:  "http://provider.example.com",
			RunnerUser:   "runner",
		}
		result, _, err := mms.createTargetHandler(ctx, &mcp.CallToolRequest{}, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.Content) == 0 {
			t.Fatal("expected content in result")
		}

		text := result.Content[0].(*mcp.TextContent).Text
		var got web.UserTarget
		if err := json.Unmarshal([]byte(text), &got); err != nil {
			t.Fatalf("failed to unmarshal result: %v", err)
		}
		if got.Scope != "octocat/new-repo" {
			t.Errorf("scope: got %q, want %q", got.Scope, "octocat/new-repo")
		}
		if got.ResourceType != "small" {
			t.Errorf("resource_type: got %q, want %q", got.ResourceType, "small")
		}
	})

	t.Run("missing_scope", func(t *testing.T) {
		input := CreateTargetInput{
			ResourceType: "nano",
		}
		_, _, err := mms.createTargetHandler(ctx, &mcp.CallToolRequest{}, input)
		if err == nil {
			t.Fatal("expected error for missing scope")
		}
	})
}

func TestUpdateTargetHandler(t *testing.T) {
	_, mms := newTestServer(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		input := UpdateTargetInput{
			TargetID:     testTargetID,
			ResourceType: "medium",
			ProviderURL:  "http://new-provider.example.com",
		}
		result, _, err := mms.updateTargetHandler(ctx, &mcp.CallToolRequest{}, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.Content) == 0 {
			t.Fatal("expected content in result")
		}

		text := result.Content[0].(*mcp.TextContent).Text
		var got web.UserTarget
		if err := json.Unmarshal([]byte(text), &got); err != nil {
			t.Fatalf("failed to unmarshal result: %v", err)
		}
		if got.ResourceType != "medium" {
			t.Errorf("resource_type: got %q, want %q", got.ResourceType, "medium")
		}
		if got.ProviderURL != "http://new-provider.example.com" {
			t.Errorf("provider_url: got %q, want %q", got.ProviderURL, "http://new-provider.example.com")
		}
	})

	t.Run("not_found", func(t *testing.T) {
		input := UpdateTargetInput{
			TargetID:     uuid.NewV4().String(),
			ResourceType: "medium",
		}
		_, _, err := mms.updateTargetHandler(ctx, &mcp.CallToolRequest{}, input)
		if err == nil {
			t.Fatal("expected error for non-existent target")
		}
	})
}

func TestDeleteTargetHandler(t *testing.T) {
	_, mms := newTestServer(t)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		input := DeleteTargetInput{TargetID: testTargetID}
		result, _, err := mms.deleteTargetHandler(ctx, &mcp.CallToolRequest{}, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.Content) == 0 {
			t.Fatal("expected content in result")
		}

		text := result.Content[0].(*mcp.TextContent).Text
		if !strings.Contains(text, testTargetID) {
			t.Errorf("expected success message to contain target ID %q, got %q", testTargetID, text)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		input := DeleteTargetInput{TargetID: uuid.NewV4().String()}
		_, _, err := mms.deleteTargetHandler(ctx, &mcp.CallToolRequest{}, input)
		if err == nil {
			t.Fatal("expected error for non-existent target")
		}
	})
}
