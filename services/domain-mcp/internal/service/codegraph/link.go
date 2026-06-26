// Package codegraph — fase 3: CRUCE memoria/código. Vincula decisiones/memorias
// (knowledge_observations) con nodos del grafo de CÓDIGO (code_nodes) vía la
// tabla knowledge_observation_code_links (mig 000177).
//
// Los métodos del cruce viven en CodegraphService (ya tiene Pool + acceso a las
// queries de nodos para resolver símbolos). Aislamiento single-tenant: TODO se
// acota por project_id. El project se VALIDA: la observation y el code_node deben
// pertenecer al mismo project antes de crear el vínculo; nunca se confía en un
// project provisto a ciegas.
package codegraph

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"nunezlagos/domain/internal/service/codegraph/codegraphdb"
)

// Errores de dominio del cruce. El handler MCP los mapea a respuestas claras.
var (
	// ErrLinkExists: ya existe un vínculo activo (mismo observation/node/tipo).
	ErrLinkExists = errors.New("active observation-code link already exists")
	// ErrLinkNotFound: no existe vínculo activo para el unlink pedido.
	ErrLinkNotFound = errors.New("observation-code link not found")
	// ErrObservationNotFound: la observation no existe (o está borrada).
	ErrObservationNotFound = errors.New("observation not found")
	// ErrLinkCrossProject: observation y code_node pertenecen a projects distintos.
	ErrLinkCrossProject = errors.New("observation and code node must belong to the same project")
	// ErrLinkBadType: link_type fuera del CHECK de la mig 000177.
	ErrLinkBadType = errors.New("invalid link_type")
)

// validLinkTypes refleja el CHECK de la mig 000177.
var validLinkTypes = map[string]bool{
	"affects":    true,
	"decided_in": true,
	"references": true,
	"implements": true,
}

// ObsCodeLink es la representación de dominio de un vínculo memoria -> código.
type ObsCodeLink struct {
	ID            uuid.UUID
	ProjectID     uuid.UUID
	ObservationID uuid.UUID
	CodeNodeID    uuid.UUID
	LinkType      string
	Note          string
	Metadata      map[string]any
	CreatedBy     *uuid.UUID
	CreatedAt     time.Time
}

// LinkedObservation es una memoria/decisión vinculada a un nodo de código, con
// su contenido (resultado del JOIN a knowledge_observations).
type LinkedObservation struct {
	LinkID               uuid.UUID
	LinkType             string
	Note                 string
	LinkedAt             time.Time
	ObservationID        uuid.UUID
	Content              string
	ObservationType      string
	ObservationCreatedAt time.Time
}

// LinkInput describe la creación de un vínculo observation -> code_node.
//
// El code_node se resuelve por CodeNodeID (si se da) o por Symbol (qualified_name
// o nombre simple, debe resolver a UN solo nodo). El project se DERIVA y VALIDA:
// debe coincidir entre la observation y el nodo.
type LinkInput struct {
	ObservationID uuid.UUID
	// CodeNodeID resuelve el nodo directo; si es uuid.Nil se usa Symbol.
	CodeNodeID uuid.UUID
	// Symbol: qualified_name o nombre simple del nodo (alternativa a CodeNodeID).
	Symbol    string
	ProjectID uuid.UUID // project para resolver Symbol y validar aislamiento.
	LinkType  string
	Note      string
	CreatedBy *uuid.UUID
}

// LinkObservationToCode crea (idempotente) un vínculo observation -> code_node.
// Valida link_type, resuelve el nodo (por id o símbolo), y verifica que tanto la
// observation como el nodo pertenezcan al mismo project (ProjectID).
func (s *CodegraphService) LinkObservationToCode(ctx context.Context, in LinkInput) (*ObsCodeLink, error) {
	if in.ProjectID == uuid.Nil {
		return nil, fmt.Errorf("link: project_id required")
	}
	if in.ObservationID == uuid.Nil {
		return nil, fmt.Errorf("link: observation_id required")
	}
	if !validLinkTypes[in.LinkType] {
		return nil, ErrLinkBadType
	}

	// Validar la observation: existe y pertenece al project.
	obs, err := s.q(ctx).GetObservationProject(ctx, in.ObservationID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrObservationNotFound
		}
		return nil, fmt.Errorf("link get observation: %w", err)
	}
	if obs.ProjectID != in.ProjectID {
		return nil, ErrLinkCrossProject
	}

	// Resolver el code_node por id o por símbolo, validando que sea del project.
	nodeID, err := s.resolveCodeNode(ctx, in.ProjectID, in.CodeNodeID, in.Symbol)
	if err != nil {
		return nil, err
	}

	meta, _ := json.Marshal(map[string]any{})
	row, err := s.q(ctx).InsertObsCodeLinkIfAbsent(ctx, codegraphdb.InsertObsCodeLinkIfAbsentParams{
		ProjectID:     in.ProjectID,
		ObservationID: in.ObservationID,
		CodeNodeID:    nodeID,
		LinkType:      in.LinkType,
		Note:          strPtr(in.Note),
		Metadata:      meta,
		CreatedBy:     in.CreatedBy,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// ON CONFLICT DO NOTHING: ya existía un vínculo activo.
		return nil, ErrLinkExists
	}
	if err != nil {
		return nil, fmt.Errorf("link insert: %w", err)
	}
	return &ObsCodeLink{
		ID:            row.ID,
		ProjectID:     row.ProjectID,
		ObservationID: row.ObservationID,
		CodeNodeID:    row.CodeNodeID,
		LinkType:      row.LinkType,
		Note:          deref(row.Note),
		Metadata:      metaMap(row.Metadata),
		CreatedBy:     row.CreatedBy,
		CreatedAt:     row.CreatedAt,
	}, nil
}

// UnlinkInput describe la baja (soft-delete) de un vínculo. El nodo se resuelve
// por id o por símbolo, igual que en LinkInput.
type UnlinkInput struct {
	ObservationID uuid.UUID
	CodeNodeID    uuid.UUID
	Symbol        string
	ProjectID     uuid.UUID
	LinkType      string
}

// UnlinkObservationFromCode soft-deletea el vínculo activo (observation, node,
// tipo) del project. ErrLinkNotFound si no había vínculo activo.
func (s *CodegraphService) UnlinkObservationFromCode(ctx context.Context, in UnlinkInput) error {
	if in.ProjectID == uuid.Nil {
		return fmt.Errorf("unlink: project_id required")
	}
	if in.ObservationID == uuid.Nil {
		return fmt.Errorf("unlink: observation_id required")
	}
	if !validLinkTypes[in.LinkType] {
		return ErrLinkBadType
	}
	nodeID, err := s.resolveCodeNode(ctx, in.ProjectID, in.CodeNodeID, in.Symbol)
	if err != nil {
		return err
	}
	rows, err := s.q(ctx).SoftDeleteObsCodeLink(ctx, codegraphdb.SoftDeleteObsCodeLinkParams{
		ProjectID:     in.ProjectID,
		ObservationID: in.ObservationID,
		CodeNodeID:    nodeID,
		LinkType:      in.LinkType,
	})
	if err != nil {
		return fmt.Errorf("unlink: %w", err)
	}
	if rows == 0 {
		return ErrLinkNotFound
	}
	return nil
}

// ObservationsForCode devuelve las memorias/decisiones vinculadas a un nodo de
// código (resuelto por símbolo dentro del project), con su contenido y tipo.
func (s *CodegraphService) ObservationsForCode(ctx context.Context, projectID uuid.UUID, symbol string) ([]LinkedObservation, error) {
	if projectID == uuid.Nil {
		return nil, fmt.Errorf("observations for code: project_id required")
	}
	if symbol == "" {
		return nil, fmt.Errorf("observations for code: symbol required")
	}
	nodeID, err := s.resolveSingle(ctx, projectID, symbol)
	if err != nil {
		return nil, err
	}
	rows, err := s.q(ctx).ListObservationsByCodeNode(ctx, codegraphdb.ListObservationsByCodeNodeParams{
		ProjectID:  projectID,
		CodeNodeID: nodeID,
	})
	if err != nil {
		return nil, fmt.Errorf("observations for code: %w", err)
	}
	out := make([]LinkedObservation, 0, len(rows))
	for _, r := range rows {
		out = append(out, LinkedObservation{
			LinkID:               r.LinkID,
			LinkType:             r.LinkType,
			Note:                 deref(r.Note),
			LinkedAt:             r.LinkedAt,
			ObservationID:        r.ObservationID,
			Content:              r.Content,
			ObservationType:      r.ObservationType,
			ObservationCreatedAt: r.ObservationCreatedAt,
		})
	}
	return out, nil
}

// CodeForObservation devuelve los nodos de código vinculados a una memoria,
// hidratando cada vínculo con los datos del nodo (kind/name/file:line).
func (s *CodegraphService) CodeForObservation(ctx context.Context, observationID uuid.UUID) ([]ObsCodeLink, []CodeNode, error) {
	if observationID == uuid.Nil {
		return nil, nil, fmt.Errorf("code for observation: observation_id required")
	}
	rows, err := s.q(ctx).ListLinksByObservation(ctx, observationID)
	if err != nil {
		return nil, nil, fmt.Errorf("code for observation: %w", err)
	}
	links := make([]ObsCodeLink, 0, len(rows))
	nodes := make([]CodeNode, 0, len(rows))
	for _, r := range rows {
		links = append(links, ObsCodeLink{
			ID:            r.ID,
			ProjectID:     r.ProjectID,
			ObservationID: r.ObservationID,
			CodeNodeID:    r.CodeNodeID,
			LinkType:      r.LinkType,
			Note:          deref(r.Note),
			Metadata:      metaMap(r.Metadata),
			CreatedBy:     r.CreatedBy,
			CreatedAt:     r.CreatedAt,
		})
		// Hidratar el nodo (puede estar borrado: en ese caso se omite del detalle).
		if n, ok := s.nodeByID(ctx, r.ProjectID, r.CodeNodeID); ok {
			nodes = append(nodes, n)
		}
	}
	return links, nodes, nil
}

// resolveCodeNode resuelve un nodo por id directo o por símbolo, validando que
// pertenezca al project. Si se da CodeNodeID, verifica project. Si no, resuelve
// el símbolo a UN solo nodo (ErrSymbolAmbiguous si hay varios).
func (s *CodegraphService) resolveCodeNode(ctx context.Context, projectID, codeNodeID uuid.UUID, symbol string) (uuid.UUID, error) {
	if codeNodeID != uuid.Nil {
		node, err := s.q(ctx).GetCodeNodeProject(ctx, codeNodeID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return uuid.Nil, ErrNodeNotFound
			}
			return uuid.Nil, fmt.Errorf("resolve code node by id: %w", err)
		}
		if node.ProjectID != projectID {
			return uuid.Nil, ErrLinkCrossProject
		}
		return node.ID, nil
	}
	if symbol == "" {
		return uuid.Nil, fmt.Errorf("resolve code node: symbol or code_node_id required")
	}
	return s.resolveSingle(ctx, projectID, symbol)
}

// nodeByID hidrata un CodeNode activo por id dentro del project. ok=false si no
// existe nodo activo con ese id (p.ej. fue soft-deleteado por un re-build).
func (s *CodegraphService) nodeByID(ctx context.Context, projectID, nodeID uuid.UUID) (CodeNode, bool) {
	nodes, err := s.q(ctx).ListNodesByProject(ctx, codegraphdb.ListNodesByProjectParams{
		ProjectID: projectID,
		Kind:      nil,
	})
	if err != nil {
		return CodeNode{}, false
	}
	for _, n := range nodes {
		if n.ID == nodeID {
			return nodeFromList(n), true
		}
	}
	return CodeNode{}, false
}
