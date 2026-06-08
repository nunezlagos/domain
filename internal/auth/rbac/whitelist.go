package rbac

// AllowedResources define los resources y actions válidos para custom_roles (HU-02.8).
// Versionado en código intencionalmente: para agregar resource/action nuevo se requiere
// deploy, no datos en DB.
var AllowedResources = map[Resource][]Action{
	ResProject:       {ActRead, ActWrite, ActDelete, ActAdmin},
	ResObservation:   {ActRead, ActWrite, ActDelete},
	ResSession:       {ActRead, ActWrite, ActDelete},
	ResPrompt:        {ActRead, ActWrite, ActDelete},
	ResKnowledgeDoc:  {ActRead, ActWrite, ActDelete},
	ResSkill:         {ActRead, ActWrite, ActDelete, ActExecute},
	ResAgent:         {ActRead, ActWrite, ActDelete, ActExecute},
	ResFlow:          {ActRead, ActWrite, ActDelete, ActExecute},
	ResRun:           {ActRead, ActCancel},
	ResSecret:        {ActRead, ActWrite, ActDelete},
	ResMember:        {ActRead, ActWrite, ActDelete, ActAdmin},
	ResPlan:          {ActRead, ActWrite},
	ResBilling:       {ActRead, ActWrite},
	ResAuditLog:      {ActRead},
	ResActivityLog:   {ActRead},
	ResRoleCustom:    {ActRead, ActWrite, ActDelete, ActAdmin},
	ResAPIKey:        {ActRead, ActWrite, ActDelete, ActAdmin},
	ResOrganization:  {ActRead, ActWrite, ActDelete, ActAdmin},
}

// ValidatePermissions chequea que cada resource/action esté en AllowedResources.
// Retorna error con detalle del primer resource o action inválido.
func ValidatePermissions(perms map[Resource][]Action) error {
	for res, actions := range perms {
		allowed, ok := AllowedResources[res]
		if !ok {
			return &ValidationError{Resource: string(res), Message: "unknown resource"}
		}
		for _, act := range actions {
			if !containsAction(allowed, act) {
				return &ValidationError{
					Resource: string(res),
					Action:   string(act),
					Message:  "action not allowed for resource",
				}
			}
		}
	}
	return nil
}

// ValidationError struct público para que handlers puedan devolver 422.
type ValidationError struct {
	Resource string `json:"resource"`
	Action   string `json:"action,omitempty"`
	Message  string `json:"message"`
}

func (e *ValidationError) Error() string {
	return e.Message + ": " + e.Resource + "/" + e.Action
}

func containsAction(actions []Action, act Action) bool {
	for _, a := range actions {
		if a == act {
			return true
		}
	}
	return false
}
