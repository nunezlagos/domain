package domain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// envelope refleja el shape estándar de respuesta {"data": ..., "pagination": ...}.
type envelope struct {
	Data       json.RawMessage `json:"data"`
	Pagination *Pagination     `json:"pagination,omitempty"`
}

type apiErrorBody struct {
	Error struct {
		Code      string        `json:"code"`
		Message   string        `json:"message"`
		RequestID string        `json:"request_id"`
		Details   []ErrorDetail `json:"details"`
	} `json:"error"`
}

// do ejecuta la request, agrega auth + headers, decodifica el envelope y lo
// escribe en out (si no es nil). Devuelve la *Pagination cuando el response
// la trae (típico de endpoints de listing).
//
// path debe ser relativo a /api/v1 (ej: "/projects", no "/api/v1/projects").
// query es opcional y puede ser nil.
func (c *Client) do(
	ctx context.Context,
	method, path string,
	query url.Values,
	body any,
	out any,
) (*Pagination, error) {
	u := c.baseURL + apiPrefix + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		reader = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {

		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()


	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if len(raw) == 0 || out == nil {
			return nil, nil
		}

		var env envelope
		if err := json.Unmarshal(raw, &env); err == nil && len(env.Data) > 0 {
			if err := json.Unmarshal(env.Data, out); err != nil {
				return nil, fmt.Errorf("decode data: %w", err)
			}
			return env.Pagination, nil
		}
		if err := json.Unmarshal(raw, out); err != nil {
			return nil, fmt.Errorf("decode body: %w", err)
		}
		return nil, nil
	}

	return nil, parseAPIError(resp, raw)
}

func parseAPIError(resp *http.Response, raw []byte) error {
	apiErr := &APIError{
		StatusCode: resp.StatusCode,
		RequestID:  resp.Header.Get("X-Request-Id"),
	}

	var body apiErrorBody
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &body)
	}
	apiErr.Code = body.Error.Code
	apiErr.Message = body.Error.Message
	apiErr.Details = body.Error.Details
	if body.Error.RequestID != "" {
		apiErr.RequestID = body.Error.RequestID
	}
	if apiErr.Message == "" {
		apiErr.Message = strings.TrimSpace(string(raw))
		if apiErr.Message == "" {
			apiErr.Message = http.StatusText(resp.StatusCode)
		}
	}

	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		apiErr.sentinel = ErrUnauthorized
	case http.StatusNotFound:
		apiErr.sentinel = ErrNotFound
	case http.StatusConflict:
		apiErr.sentinel = ErrConflict
	case http.StatusUnprocessableEntity:
		apiErr.sentinel = ErrValidation
	case http.StatusPaymentRequired:
		apiErr.sentinel = ErrQuota
	case http.StatusTooManyRequests:
		apiErr.sentinel = ErrRateLimited
		apiErr.RetryAfter = parseRetryAfter(resp.Header.Get("Retry-After"))
	}

	return apiErr
}

func parseRetryAfter(h string) int {
	if h == "" {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(h))
	if err != nil || n < 0 {
		return 0
	}
	return n
}
