// Tests unitarios de los tool builders + handlers de clients (mandantes).
// Sin DB: validamos shape de las tools y caminos de error que no
// requieren ClientService inicializado (sin principal / sin service).
package mcpserver

import (
	"context"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"nunezlagos/domain/internal/auth/apikey"
)

func TestToolClientCreate_TieneCamposRequeridos(t *testing.T) {
	tool := toolClientCreate()
	if tool.Name != "domain_client_create" {
		t.Fatalf("name=%s want domain_client_create", tool.Name)
	}
	if !strings.Contains(tool.Description, "cliente") && !strings.Contains(tool.Description, "mandante") {
		t.Errorf("description deberia mencionar cliente o mandante, got: %s", tool.Description)
	}
}

func TestToolClientNames(t *testing.T) {
	cases := map[string]mcp.Tool{
		"domain_client_create":     toolClientCreate(),
		"domain_client_list":       toolClientList(),
		"domain_client_get":        toolClientGet(),
		"domain_client_update":     toolClientUpdate(),
		"domain_client_delete":     toolClientDelete(),
		"domain_client_restore":    toolClientRestore(),
		"domain_client_set_status": toolClientSetStatus(),
	}
	for want, tool := range cases {
		if tool.Name != want {
			t.Errorf("tool name=%s want=%s", tool.Name, want)
		}
		if tool.Description == "" {
			t.Errorf("tool %s: description vacio", want)
		}
	}
}

func TestHandleClientCreate_SinPrincipal_Error(t *testing.T) {
	d := &Deps{}
	req := mcp.CallToolRequest{}
	res, err := d.handleClientCreate(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatalf("expected error result, got %+v", res)
	}
}

func TestHandleClientList_SinPrincipal_Error(t *testing.T) {
	d := &Deps{}
	res, err := d.handleClientList(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected error, got success")
	}
}

func TestHandleClientCreate_SinService_Error(t *testing.T) {
	d := &Deps{Principal: &apikey.Principal{OrganizationID: "11111111-1111-1111-1111-111111111111"}}
	res, _ := d.handleClientCreate(context.Background(), mcp.CallToolRequest{})
	if !res.IsError {
		t.Fatalf("expected error (Clients == nil)")
	}
}

func TestHandleClientGet_SinService_Error(t *testing.T) {
	d := &Deps{
		Principal: &apikey.Principal{
			OrganizationID: "11111111-1111-1111-1111-111111111111",
			UserID:         "22222222-2222-2222-2222-222222222222",
		},
	}
	res, _ := d.handleClientGet(context.Background(), mcp.CallToolRequest{})
	if !res.IsError {
		t.Fatalf("expected error con Clients == nil")
	}
}

func TestHandleClientDelete_SinService_Error(t *testing.T) {
	d := &Deps{
		Principal: &apikey.Principal{
			OrganizationID: "11111111-1111-1111-1111-111111111111",
		},
	}
	res, _ := d.handleClientDelete(context.Background(), mcp.CallToolRequest{})
	if !res.IsError {
		t.Fatalf("expected error con Clients == nil")
	}
}

func TestRegisterClientTools_CountAndNames(t *testing.T) {
	wrap := NewResilientWrapper(defaultBudget)
	tools := registerClientTools(wrap, Deps{})
	if got, want := len(tools), 7; got != want {
		t.Fatalf("tools count=%d want=%d", got, want)
	}
	expected := map[string]bool{
		"domain_client_create":     true,
		"domain_client_list":       true,
		"domain_client_get":        true,
		"domain_client_update":     true,
		"domain_client_delete":     true,
		"domain_client_restore":    true,
		"domain_client_set_status": true,
	}
	for _, st := range tools {
		if !expected[st.Tool.Name] {
			t.Errorf("tool inesperada: %s", st.Tool.Name)
		}
		delete(expected, st.Tool.Name)
	}
	if len(expected) != 0 {
		t.Errorf("tools faltantes: %v", expected)
	}
}
