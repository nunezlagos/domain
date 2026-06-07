# Tasks: HU-04.3-specs-designs

## Backend

- [ ] `migrations/XXXX_create_proposals.sql`: tabla + UNIQUE(hu_id, version) + FK
- [ ] `migrations/XXXX_create_designs.sql`: tabla + UNIQUE(hu_id, version) + FK a proposals
- [ ] `internal/opsx/spec.go`: structs `Proposal`, `Design`, `ProposalFilter`, `DesignFilter`
- [ ] `internal/store/pg/proposal.go`: interfaz `ProposalStore`
- [ ] Implementar `CreateProposal(proposal Proposal) (uuid.UUID, error)`
- [ ] Implementar `GetLatestProposal(huID uuid.UUID) (*Proposal, error)`
- [ ] Implementar `GetProposalVersion(huID uuid.UUID, version int) (*Proposal, error)`
- [ ] Implementar `ListProposalVersions(huID uuid.UUID) ([]Proposal, error)`
- [ ] Implementar `ChangeProposalStatus(id uuid.UUID, status string, reason string) error`
- [ ] `internal/store/pg/design.go`: interfaz `DesignStore`
- [ ] Implementar `CreateDesign(design Design) (uuid.UUID, error)`
- [ ] Implementar `GetLatestDesign(huID uuid.UUID) (*Design, error)`
- [ ] Implementar `ListDesignsByHU(huID uuid.UUID) ([]Design, error)`
- [ ] `internal/opsx/spec_service.go`: validaciones (status transitions, markdown fields)

## Tests

- [ ] Test unitario: versionado (v1 → v2 → latest = v2)
- [ ] Test unitario: status transitions válidas e inválidas
- [ ] Test de integración: crear proposal + design completo
- [ ] Test de integración: listar versiones de proposal
- [ ] Test de integración: design sin proposal_id (nullable)
- [ ] Sabotaje: crear proposal sin hu_id → error FK

## Cierre

- [ ] Verificación manual: crear proposal, aprobar, crear design, verificar en DB
- [ ] Suite verde
