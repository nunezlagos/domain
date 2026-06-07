// Package rbac — HU-02.2 RBAC (built-in roles) + HU-02.8 stub for custom roles.
//
// Roles built-in (jerárquicos):
//   owner > admin > maintainer > member > viewer
//
// Custom roles (HU-02.8) viven en tabla custom_roles (futuro). Esta API permite
// override de built-in via Resolver interface.
package rbac

import (
	"context"
	"errors"
	"net/http"

	"nunezlagos/domain/internal/auth/apikey"
)

// Role nombre canónico.
type Role string

const (
	RoleOwner      Role = "owner"
	RoleAdmin      Role = "admin"
	RoleMaintainer Role = "maintainer"
	RoleMember     Role = "member"
	RoleViewer     Role = "viewer"
)

// builtinHierarchy nivel jerárquico de cada role (mayor = más permisos).
var builtinHierarchy = map[Role]int{
	RoleViewer:     1,
	RoleMember:     2,
	RoleMaintainer: 3,
	RoleAdmin:      4,
	RoleOwner:      5,
}

// IsBuiltin true si role es one of los 5 built-in.
func IsBuiltin(r Role) bool {
	_, ok := builtinHierarchy[r]
	return ok
}

// AtLeast retorna true si actual cumple el role mínimo requerido (jerárquico).
// Custom roles SIEMPRE retornan false acá (HU-02.8 implementa Resolver custom).
func AtLeast(actual, required Role) bool {
	a, ok1 := builtinHierarchy[actual]
	r, ok2 := builtinHierarchy[required]
	if !ok1 || !ok2 {
		return false
	}
	return a >= r
}

// Resource entidad sobre la que se aplica un permission.
type Resource string

const (
	ResProject       Resource = "project"
	ResObservation   Resource = "observation"
	ResSession       Resource = "session"
	ResPrompt        Resource = "prompt"
	ResKnowledgeDoc  Resource = "knowledge_doc"
	ResSkill         Resource = "skill"
	ResAgent         Resource = "agent"
	ResFlow          Resource = "flow"
	ResRun           Resource = "run"
	ResSecret        Resource = "secret"
	ResMember        Resource = "member"
	ResPlan          Resource = "plan"
	ResBilling       Resource = "billing"
	ResAuditLog      Resource = "audit_log"
	ResActivityLog   Resource = "activity_log"
	ResRoleCustom    Resource = "role"
	ResAPIKey        Resource = "api_key"
	ResOrganization  Resource = "organization"
)

// Action operación específica.
type Action string

const (
	ActRead    Action = "read"
	ActWrite   Action = "write"
	ActDelete  Action = "delete"
	ActAdmin   Action = "admin"
	ActExecute Action = "execute"
	ActCancel  Action = "cancel"
)

// ErrForbidden returned por Check si no autorizado.
var ErrForbidden = errors.New("forbidden")

// matrix built-in: role → resource → set(actions).
// Diseño: viewer puede read básicos; member CRUD propio + execute skills/agents;
// maintainer + manage skills/agents/flows/projects; admin + members + custom_roles;
// owner + billing + plan + transfer.
var matrix = map[Role]map[Resource][]Action{
	RoleViewer: {
		ResProject:      {ActRead},
		ResObservation:  {ActRead},
		ResSession:      {ActRead},
		ResPrompt:       {ActRead},
		ResKnowledgeDoc: {ActRead},
		ResAgent:        {ActRead},
		ResFlow:         {ActRead},
		ResSkill:        {ActRead},
		ResRun:          {ActRead},
		ResActivityLog:  {ActRead},
	},
	RoleMember: {
		ResProject:      {ActRead},
		ResObservation:  {ActRead, ActWrite, ActDelete},
		ResSession:      {ActRead, ActWrite, ActDelete},
		ResPrompt:       {ActRead, ActWrite, ActDelete},
		ResKnowledgeDoc: {ActRead},
		ResAgent:        {ActRead, ActExecute},
		ResFlow:         {ActRead, ActExecute},
		ResSkill:        {ActRead, ActExecute},
		ResRun:          {ActRead, ActCancel},
		ResActivityLog:  {ActRead},
		ResAPIKey:       {ActRead, ActWrite, ActDelete}, // sólo las propias
	},
	RoleMaintainer: {
		ResProject:      {ActRead, ActWrite},
		ResObservation:  {ActRead, ActWrite, ActDelete},
		ResSession:      {ActRead, ActWrite, ActDelete},
		ResPrompt:       {ActRead, ActWrite, ActDelete},
		ResKnowledgeDoc: {ActRead, ActWrite, ActDelete},
		ResAgent:        {ActRead, ActWrite, ActDelete, ActExecute},
		ResFlow:         {ActRead, ActWrite, ActDelete, ActExecute},
		ResSkill:        {ActRead, ActWrite, ActDelete, ActExecute},
		ResRun:          {ActRead, ActCancel},
		ResActivityLog:  {ActRead},
		ResAPIKey:       {ActRead, ActWrite, ActDelete},
	},
	RoleAdmin: {
		ResOrganization: {ActRead, ActWrite},
		ResProject:      {ActRead, ActWrite, ActDelete, ActAdmin},
		ResObservation:  {ActRead, ActWrite, ActDelete},
		ResSession:      {ActRead, ActWrite, ActDelete},
		ResPrompt:       {ActRead, ActWrite, ActDelete},
		ResKnowledgeDoc: {ActRead, ActWrite, ActDelete},
		ResAgent:        {ActRead, ActWrite, ActDelete, ActExecute, ActAdmin},
		ResFlow:         {ActRead, ActWrite, ActDelete, ActExecute, ActAdmin},
		ResSkill:        {ActRead, ActWrite, ActDelete, ActExecute, ActAdmin},
		ResRun:          {ActRead, ActCancel},
		ResSecret:       {ActRead, ActWrite, ActDelete},
		ResMember:       {ActRead, ActWrite, ActDelete, ActAdmin},
		ResPlan:         {ActRead},
		ResAuditLog:     {ActRead},
		ResActivityLog:  {ActRead},
		ResRoleCustom:   {ActRead, ActWrite, ActDelete, ActAdmin},
		ResAPIKey:       {ActRead, ActWrite, ActDelete, ActAdmin},
	},
	RoleOwner: {
		// inherits admin + billing + plan + organization admin
		ResOrganization: {ActRead, ActWrite, ActDelete, ActAdmin},
		ResProject:      {ActRead, ActWrite, ActDelete, ActAdmin},
		ResObservation:  {ActRead, ActWrite, ActDelete},
		ResSession:      {ActRead, ActWrite, ActDelete},
		ResPrompt:       {ActRead, ActWrite, ActDelete},
		ResKnowledgeDoc: {ActRead, ActWrite, ActDelete},
		ResAgent:        {ActRead, ActWrite, ActDelete, ActExecute, ActAdmin},
		ResFlow:         {ActRead, ActWrite, ActDelete, ActExecute, ActAdmin},
		ResSkill:        {ActRead, ActWrite, ActDelete, ActExecute, ActAdmin},
		ResRun:          {ActRead, ActCancel},
		ResSecret:       {ActRead, ActWrite, ActDelete, ActAdmin},
		ResMember:       {ActRead, ActWrite, ActDelete, ActAdmin},
		ResPlan:         {ActRead, ActWrite, ActAdmin},
		ResBilling:      {ActRead, ActWrite, ActAdmin},
		ResAuditLog:     {ActRead},
		ResActivityLog:  {ActRead},
		ResRoleCustom:   {ActRead, ActWrite, ActDelete, ActAdmin},
		ResAPIKey:       {ActRead, ActWrite, ActDelete, ActAdmin},
	},
}

// Check verifica si role tiene permission sobre (resource, action) según matrix.
// Custom roles (HU-02.8) override via CustomResolver.
type Checker struct {
	CustomResolver CustomResolver // optional; nil = solo built-in
}

// CustomResolver consulta custom_roles en BD para roles no built-in.
type CustomResolver interface {
	HasPermission(ctx context.Context, orgID, roleSlug string, res Resource, act Action) (bool, error)
}

// Check retorna nil si autorizado, ErrForbidden si no.
// orgID se pasa para que CustomResolver pueda buscar custom_roles por org.
func (c *Checker) Check(ctx context.Context, orgID string, role Role, res Resource, act Action) error {
	if IsBuiltin(role) {
		actions, ok := matrix[role][res]
		if !ok {
			return ErrForbidden
		}
		for _, a := range actions {
			if a == act {
				return nil
			}
		}
		return ErrForbidden
	}
	// Custom role
	if c.CustomResolver == nil {
		return ErrForbidden
	}
	ok, err := c.CustomResolver.HasPermission(ctx, orgID, string(role), res, act)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	return nil
}

// RequireRole middleware: requiere que el principal tenga al menos `min` role.
// Usa apikey.FromContext para obtener el Principal autenticado.
func RequireRole(min Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := apikey.FromContext(r.Context())
			if !ok {
				writeForbidden(w)
				return
			}
			if !AtLeast(Role(p.Role), min) {
				writeForbidden(w)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequirePermission middleware: requiere permiso explícito (res, act).
func RequirePermission(checker *Checker, res Resource, act Action) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := apikey.FromContext(r.Context())
			if !ok {
				writeForbidden(w)
				return
			}
			if err := checker.Check(r.Context(), p.OrganizationID, Role(p.Role), res, act); err != nil {
				writeForbidden(w)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func writeForbidden(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`{"error":{"code":"forbidden","message":"forbidden"}}`))
}
