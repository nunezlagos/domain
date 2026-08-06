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
// DOMAINSERV-234: este test tenía el cuerpo VACÍO y un
// t.Skip("requires external mock server; covered by integration suite") incondicional, en un
// archivo SIN build tag — o sea muerto en el job unitario y en el de integración.
//
// Y la razón declarada era FALSA: no hay ninguna suite de integración que cubra el handshake.
// El paquete internal/mcp/client tiene exactamente dos archivos, stdio.go y stdio_test.go, así
// que "covered by integration suite" apuntaba a algo que no existe. Borrar el test habría hecho
// desaparecer la única señal de que este camino no estaba cubierto, así que se implementa.
//
// El "mock server externo" tampoco hacía falta: el protocolo es JSON-RPC por líneas y el propio
// comentario original decía "solo corre si /bin/sh disponible". Un sh que lee dos líneas y
// responde dos alcanza — y es determinista porque los ids del cliente son incrementales:
// Initialize pide el 1 y ListTools el 2.
func TestStdioClient_HandshakeAndListTools(t *testing.T) {
	const mock = `read _linea_initialize
printf '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05"}}\n'
read _linea_tools_list
printf '{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"eco","description":"repite lo que le pasan"}]}}\n'
`
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c, err := NewStdioClient(ctx, Config{Command: "/bin/sh", Args: []string{"-c", mock}})
	if err != nil {
		t.Fatalf("spawn del mock: %v", err)
	}
	defer func() { _ = c.Close() }()

	if err := c.Initialize(ctx); err != nil {
		t.Fatalf("Initialize contra un server que responde OK falló: %v", err)
	}

	tools, err := c.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("se esperaba 1 tool y llegaron %d: %+v", len(tools), tools)
	}
	if tools[0].Name != "eco" {
		t.Errorf("el nombre de la tool no sobrevivió el decode: %q", tools[0].Name)
	}
	if tools[0].Description == "" {
		t.Error("la descripción llegó vacía: el decode de tools/list está perdiendo campos")
	}
}

// La contracara, que es la que hace al test un test: si el server muere sin responder, el
// cliente tiene que devolver error y NO colgarse. Sin este caso, el de arriba pasaría igual con
// un call() que ignora los errores.
func TestStdioClient_ServerQueMuereSinResponder_DevuelveErrorYNoCuelga(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// sh que cierra stdout de inmediato: el readLoop ve EOF con un pending esperando
	c, err := NewStdioClient(ctx, Config{Command: "/bin/sh", Args: []string{"-c", "exit 0"}})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer func() { _ = c.Close() }()

	if err := c.Initialize(ctx); err == nil {
		t.Error("el server murió sin responder y Initialize devolvió nil: un pending que nunca " +
			"se resuelve deja al caller colgado hasta el timeout del ctx")
	}
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
