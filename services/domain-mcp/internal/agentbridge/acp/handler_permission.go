package acp

import (
	"context"

	acpsdk "github.com/coder/acp-go-sdk"
)

// RequestPermission rechaza por default (deny-all): el agente nunca obtiene una
// opción "permitida" desde el server. Mantiene el invariante seguro del núcleo:
// toda acción sensible que requiera consentimiento se cancela.
func (h *handler) RequestPermission(context.Context, acpsdk.RequestPermissionRequest) (acpsdk.RequestPermissionResponse, error) {
	return acpsdk.RequestPermissionResponse{
		Outcome: acpsdk.RequestPermissionOutcome{Cancelled: &acpsdk.RequestPermissionOutcomeCancelled{}},
	}, nil
}
