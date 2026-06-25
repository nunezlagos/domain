




package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"time"
)

// mailpitBaseURL deriva la URL del UI/API de mailpit (docker local).
func mailpitBaseURL() string {
	port := envOr("HOST_MAILPIT_UI_PORT", "8025")
	return "http://localhost:" + port
}

// mailpitAvailable chequea que la API de mailpit responda.
func mailpitAvailable() bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(mailpitBaseURL() + "/api/v1/messages?limit=1")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

type mailpitMessage struct {
	ID      string `json:"ID"`
	Created string `json:"Created"`
	To      []struct {
		Address string `json:"Address"`
	} `json:"To"`
}

var otpCodeRe = regexp.MustCompile(`\b(\d{6})\b`)

// fetchOTPFromMailpit pollea mailpit hasta encontrar un mensaje para
// `email` creado después de `since`, y extrae el código de 6 dígitos.
func fetchOTPFromMailpit(email string, since time.Time, timeout time.Duration) (string, error) {
	client := &http.Client{Timeout: 3 * time.Second}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		code, err := tryFetchOTP(client, email, since)
		if err == nil && code != "" {
			return code, nil
		}
		time.Sleep(700 * time.Millisecond)
	}
	return "", fmt.Errorf("no llegó el código a mailpit en %s (¿el email %q existe en la BD?)", timeout, email)
}

func tryFetchOTP(client *http.Client, email string, since time.Time) (string, error) {
	resp, err := client.Get(mailpitBaseURL() + "/api/v1/messages?limit=20")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var list struct {
		Messages []mailpitMessage `json:"messages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return "", err
	}
	for _, msg := range list.Messages {
		created, err := time.Parse(time.RFC3339, msg.Created)
		if err != nil || created.Before(since) {
			continue
		}
		match := false
		for _, to := range msg.To {
			if to.Address == email {
				match = true
				break
			}
		}
		if !match {
			continue
		}

		body, err := client.Get(mailpitBaseURL() + "/api/v1/message/" + msg.ID)
		if err != nil {
			continue
		}
		var detail struct {
			Text string `json:"Text"`
			HTML string `json:"HTML"`
		}
		decErr := json.NewDecoder(body.Body).Decode(&detail)
		body.Body.Close()
		if decErr != nil {
			continue
		}
		if m := otpCodeRe.FindStringSubmatch(detail.Text + " " + detail.HTML); m != nil {
			return m[1], nil
		}
	}
	return "", nil
}

// otpFlowViaServer ejecuta el flujo real de auth contra el server local:
// request-otp → fetch del código en mailpit → verify-otp. Retorna la
// API key plaintext + ids para armar credentials.
type otpVerifyResult struct {
	UserID   string `json:"user_id"`
	OrgID    string `json:"organization_id"`
	Email    string `json:"email"`
	APIKey   string `json:"api_key"`
	APIKeyID string `json:"api_key_id"`
}

func otpFlowViaServer(baseURL, email string) (*otpVerifyResult, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	since := time.Now().UTC().Add(-2 * time.Second)

	reqBody, _ := json.Marshal(map[string]string{"identifier": email})
	resp, err := client.Post(baseURL+"/api/v1/auth/request-otp", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("request-otp: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request-otp: status %d", resp.StatusCode)
	}
	fmt.Fprintf(os.Stderr, "  código OTP solicitado para %s; buscando en mailpit...\n", email)

	code, err := fetchOTPFromMailpit(email, since, 45*time.Second)
	if err != nil {
		return nil, err
	}
	fmt.Fprintln(os.Stderr, "  código encontrado en mailpit; verificando...")

	verifyBody, _ := json.Marshal(map[string]string{
		"identifier": email, "code": code, "key_name": "domain-install",
	})
	resp, err = client.Post(baseURL+"/api/v1/auth/verify-otp", "application/json", bytes.NewReader(verifyBody))
	if err != nil {
		return nil, fmt.Errorf("verify-otp: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("verify-otp: status %d", resp.StatusCode)
	}
	var out struct {
		Data otpVerifyResult `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("verify-otp decode: %w", err)
	}
	if out.Data.APIKey == "" {
		return nil, fmt.Errorf("verify-otp: respuesta sin api_key")
	}
	return &out.Data, nil
}
