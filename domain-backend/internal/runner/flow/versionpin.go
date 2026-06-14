// issue-09.7 fv-008 — run pinning: cada flow_run referencia la flow_version
// con la que arrancó y el engine lee SIEMPRE esa definition, no la actual.
package flowrunner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/service/flow"
)

// pinVersion garantiza que exista una flow_version cuyo definition sea el
// spec actual del flow (idempotente por hash) y la retorna. Best-effort:
// retorna nil si no se pudo versionar (el run corre igual, sin pin).
func (r *Runner) pinVersion(ctx context.Context, f *flow.Flow) *flow.FlowVersion {
	def, err := json.Marshal(f.Spec)
	if err != nil {
		return nil
	}
	sum := sha256.Sum256(def)
	hash := hex.EncodeToString(sum[:])

	vs := &flow.VersioningStore{Pool: r.Pool}
	if v, err := vs.FindByHash(ctx, f.ID, hash); err == nil {
		return v
	}
	v, err := vs.NewVersion(ctx, f.ID, def, hash, "auto-pin al iniciar run", nil)
	if err != nil {
		return nil
	}
	return v
}

// resolveVersionSpec carga una versión específica, valida que sea invokable
// y devuelve su Spec parseado.
func (r *Runner) resolveVersionSpec(ctx context.Context, flowID uuid.UUID, version int) (*flow.FlowVersion, *flow.Spec, error) {
	vs := &flow.VersioningStore{Pool: r.Pool}
	if err := vs.IsVersionInvokable(ctx, flowID, version); err != nil {
		return nil, nil, err
	}
	v, err := vs.GetVersion(ctx, flowID, version)
	if err != nil {
		return nil, nil, err
	}
	var spec flow.Spec
	if err := json.Unmarshal(v.Definition, &spec); err != nil {
		return nil, nil, err
	}
	return v, &spec, nil
}

// loadRunVersionSpec devuelve el Spec de la versión pinneada de un run, si tiene.
func (r *Runner) loadRunVersionSpec(ctx context.Context, versionID uuid.UUID) (*flow.Spec, bool) {
	var def []byte
	err := r.Pool.QueryRow(ctx,
		`SELECT definition FROM flow_versions WHERE id = $1`, versionID).Scan(&def)
	if err != nil {
		return nil, false
	}
	var spec flow.Spec
	if err := json.Unmarshal(def, &spec); err != nil {
		return nil, false
	}
	return &spec, true
}
