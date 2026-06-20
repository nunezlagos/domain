package smtpmail

import (
	"bufio"
	"context"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// mockSMTPServer implementa un MTA mínimo plain (sin TLS) para testing.
// Acepta el dialog estándar SMTP y captura el mensaje recibido.
type mockSMTPServer struct {
	listener net.Listener
	captured *capture
}

type capture struct {
	mu   sync.Mutex
	from string
	to   string
	data string
}

func (c *capture) From() string { c.mu.Lock(); defer c.mu.Unlock(); return c.from }
func (c *capture) To() string   { c.mu.Lock(); defer c.mu.Unlock(); return c.to }
func (c *capture) Data() string { c.mu.Lock(); defer c.mu.Unlock(); return c.data }

func startMockSMTP(t *testing.T) *mockSMTPServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := &mockSMTPServer{listener: ln, captured: &capture{}}
	go srv.acceptLoop()
	return srv
}

func (s *mockSMTPServer) Addr() (host string, port int) {
	tcp := s.listener.Addr().(*net.TCPAddr)
	return tcp.IP.String(), tcp.Port
}

func (s *mockSMTPServer) Close() { s.listener.Close() }

func (s *mockSMTPServer) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *mockSMTPServer) handle(conn net.Conn) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	w := bufio.NewWriter(conn)
	r := bufio.NewReader(conn)

	write := func(line string) {
		w.WriteString(line + "\r\n")
		w.Flush()
	}
	write("220 mock.local ESMTP ready")

	inData := false
	var dataBuf strings.Builder
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		if inData {
			if line == "." {
				s.captured.mu.Lock()
				s.captured.data = dataBuf.String()
				s.captured.mu.Unlock()
				inData = false
				write("250 OK")
				continue
			}
			dataBuf.WriteString(line)
			dataBuf.WriteString("\n")
			continue
		}
		up := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(up, "EHLO"), strings.HasPrefix(up, "HELO"):
			write("250-mock.local")
			write("250 OK")
		case strings.HasPrefix(up, "MAIL FROM:"):
			s.captured.mu.Lock()
			s.captured.from = extractAddr(line[10:])
			s.captured.mu.Unlock()
			write("250 OK")
		case strings.HasPrefix(up, "RCPT TO:"):
			s.captured.mu.Lock()
			s.captured.to = extractAddr(line[8:])
			s.captured.mu.Unlock()
			write("250 OK")
		case up == "DATA":
			write("354 End data with <CR><LF>.<CR><LF>")
			inData = true
		case up == "QUIT":
			write("221 bye")
			return
		case up == "RSET":
			write("250 OK")
		case strings.HasPrefix(up, "AUTH"):
			write("235 OK")
		default:
			write("250 OK")
		}
	}
}

func extractAddr(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "<")
	s = strings.TrimSuffix(s, ">")
	return s
}

func TestSend_HappyPath(t *testing.T) {
	srv := startMockSMTP(t)
	defer srv.Close()
	host, port := srv.Addr()

	m := New(Config{
		Host: host, Port: port, From: "noreply@test.local", Auth: "none",
	})
	err := m.Send(context.Background(), "user@x.com", "Hola", "Cuerpo del mail")
	require.NoError(t, err)
	require.Equal(t, "noreply@test.local", srv.captured.From())
	require.Equal(t, "user@x.com", srv.captured.To())
	require.Contains(t, srv.captured.Data(), "Subject: Hola")
	require.Contains(t, srv.captured.Data(), "Cuerpo del mail")
}

func TestSendOTP(t *testing.T) {
	srv := startMockSMTP(t)
	defer srv.Close()
	host, port := srv.Addr()
	m := New(Config{Host: host, Port: port, From: "noreply@test", Auth: "none"})

	err := m.SendOTP(context.Background(), "alice@x.com", "123456", 10*time.Minute)
	require.NoError(t, err)
	data := srv.captured.Data()
	require.Contains(t, data, "código de acceso")
	require.Contains(t, data, "123456")
	require.Contains(t, data, "10m")
}

func TestSend_EmptyToRejected(t *testing.T) {
	m := New(Config{Host: "x", Port: 25})
	err := m.Send(context.Background(), "", "x", "y")
	require.Error(t, err)
}

// Sabotaje: timeout corto sobre servidor muerto NO cuelga
func TestSabotage_Send_ConnectionTimeout(t *testing.T) {
	m := New(Config{
		Host: "127.0.0.1", Port: 1, // port inutilizado, conexión refused inmediato
		From: "x", Auth: "none", Timeout: 500 * time.Millisecond,
	})
	start := time.Now()
	err := m.Send(context.Background(), "x@x.com", "s", "b")
	elapsed := time.Since(start)
	require.Error(t, err)
	require.Less(t, elapsed, 5*time.Second, "debe fallar rápido, no quedarse colgado")
}
