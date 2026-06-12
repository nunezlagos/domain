# Tasks: issue-04.3-specs-designs

## Backend

- [x] `migrations/XXXX_create_proposals.sql`: tabla + UNIQUE(issue_id, version) + FK
- [x] `migrations/XXXX_create_designs.sql`: tabla + UNIQUE(issue_id, version) + FK a proposals
- [x] `internal/opsx/spec.go`: structs `Proposal`, `Design`, `ProposalFilter`, `DesignFilter`
- [x] `internal/store/pg/proposal.go`: interfaz `ProposalStore`
- [x] Implementar `CreateProposal(proposal Proposal) (uuid.UUID, error)`
- [x] Implementar `GetLatestProposal(huID uuid.UUID) (*Proposal, error)`
- [x] Implementar `GetProposalVersion(huID uuid.UUID, version int) (*Proposal, error)`
- [x] Implementar `ListProposalVersions(huID uuid.UUID) ([]Proposal, error)`
- [x] Implementar `ChangeProposalStatus(id uuid.UUID, status string, reason string) error`
- [x] `internal/store/pg/design.go`: interfaz `DesignStore`
- [x] Implementar `CreateDesign(design Design) (uuid.UUID, error)`
- [x] Implementar `GetLatestDesign(huID uuid.UUID) (*Design, error)`
- [x] Implementar `ListDesignsByHU(huID uuid.UUID) ([]Design, error)`
- [x] `internal/opsx/spec_service.go`: validaciones (status transitions, markdown fields)

## Tests

- [x] Test unitario: versionado (v1 → v2 → latest = v2)
- [x] Test unitario: status transitions válidas e inválidas
- [x] Test de integración: crear proposal + design completo
- [x] Test de integración: listar versiones de proposal
- [x] Test de integración: design sin proposal_id (nullable)
- [x] Sabotaje: crear proposal sin issue_id → error FK

## Cierre

- [x] Verificación manual: crear proposal, aprobar, crear design, verificar en DB
- [x] Suite verde
