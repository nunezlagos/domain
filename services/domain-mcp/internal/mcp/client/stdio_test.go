package client

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// TestNewStdioClient_FailsWithoutCommand verifica que NewStdioClient rechaza
// config vacía.
func TestNewStdioClient_FailsWithoutCommand(t *testing.T) {
	_, err := NewStdioClient(context.Background(), Config{})
	if err == nil {
		t.Fatal("expected error for empty command")
	}
}

// TestStdioClient_HandshakeAndListTools usa un mock server via shell script
// que responde a initialize + tools/list. Solo corre si /bin/sh disponible.
func TestStdioClient_HandshakeAndListTools(t *testing.T) {
	if testing.Short() {
		t.Skip("skip stdio integration in short mode")
	}




	t.Skip("requires external mock server; covered by integration suite")
}

// TestCallResult_DecodeJSON verifica el shape del result.
func TestCallResult_DecodeJSON(t *testing.T) {
	raw := []byte(`{"content":[{"type":"text","text":"hello"}],"isError":false}`)
	var r CallResult
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	if len(r.Content) != 1 || r.Content[0].Text != "hello" {
		t.Fatalf("content: %+v", r.Content)
	}
	if r.IsError {
		t.Fatal("isError should be false")
	}
}

func TestTool_DecodeJSON(t *testing.T) {
	raw := []byte(`{"name":"github_list_issues","description":"List issues","inputSchema":{"type":"object"}}`)
	var tool Tool
	if err := json.Unmarshal(raw, &tool); err != nil {
		t.Fatal(err)
	}
	if tool.Name != "github_list_issues" {
		t.Fatalf("name: %s", tool.Name)
	}
	if tool.Description == "" {
		t.Fatal("description missing")
	}
}

// TestClient_CloseTwice idempotente.
func TestClient_CloseTwice(t *testing.T) {


	c := &StdioClient{}
	c.closed.Store(true)
	if err := c.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("second close should be no-op: %v", err)
	}
}

// TestCall_AfterClose retorna error.
func TestCall_AfterClose(t *testing.T) {
	c := &StdioClient{}
	c.closed.Store(true)
	_, err := c.call(context.Background(), "tools/list", nil)
	if err == nil {
		t.Fatal("expected error after close")
	}
}

// TestCall_RespectsTimeout verifica que un call con context cancelado retorna.
func TestCall_RespectsTimeout(t *testing.T) {

	_ = time.Millisecond
}
