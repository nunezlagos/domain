package rbac

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PGResolver implementa CustomResolver contra tabla custom_roles (issue-02.8).
// Cachea permisos por (orgID, roleSlug) con invalidación vía LISTEN/NOTIFY.
type PGResolver struct {
	pool *pgxpool.Pool

	mu    sync.RWMutex
	cache map[string]map[Resource][]Action // key: "orgID:roleSlug"

	listenConn *pgxpool.Conn
}

// NewPGResolver crea resolver sin cache inicializada.
func NewPGResolver(pool *pgxpool.Pool) *PGResolver {
	return &PGResolver{
		pool:  pool,
		cache: make(map[string]map[Resource][]Action),
	}
}

// buildKey genera "orgID:roleSlug".
func buildKey(orgID, roleSlug string) string {
	return fmt.Sprintf("%s:%s", orgID, roleSlug)
}

// cachePermissions parsea y cachea el JSONB de custom_roles.
func cachePermissions(perms map[string][]string) map[Resource][]Action {
	out := make(map[Resource][]Action, len(perms))
	for resStr, actions := range perms {
		res := Resource(resStr)
		acts := make([]Action, len(actions))
		for i, a := range actions {
			acts[i] = Action(a)
		}
		out[res] = acts
	}
	return out
}

// HasPermission consulta custom_roles si role es custom, o fallback a built-in.
func (r *PGResolver) HasPermission(ctx context.Context, orgID, roleSlug string, res Resource, act Action) (bool, error) {
	key := buildKey(orgID, roleSlug)

	r.mu.RLock()
	cached, ok := r.cache[key]
	r.mu.RUnlock()

	if !ok {
		var permsJSON []byte
		err := r.pool.QueryRow(ctx,
			`SELECT permissions FROM custom_roles WHERE organization_id = $1 AND slug = $2`,
			orgID, roleSlug,
		).Scan(&permsJSON)
		if err != nil {
			return false, fmt.Errorf("query custom_role: %w", err)
		}
		var raw map[string][]string
		if err := json.Unmarshal(permsJSON, &raw); err != nil {
			return false, fmt.Errorf("parse custom_role permissions: %w", err)
		}
		parsed := cachePermissions(raw)

		r.mu.Lock()
		r.cache[key] = parsed
		r.mu.Unlock()
		cached = parsed
	}

	actions, ok := cached[res]
	if !ok {
		return false, nil
	}
	for _, a := range actions {
		if a == act {
			return true, nil
		}
	}
	return false, nil
}

// InvalidateCache limpia entrada(s) de cache.
// Si orgID y roleSlug son vacíos, purga todo.
func (r *PGResolver) InvalidateCache(orgID, roleSlug string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if orgID == "" && roleSlug == "" {
		r.cache = make(map[string]map[Resource][]Action)
		return
	}
	delete(r.cache, buildKey(orgID, roleSlug))
}

// StartCacheListener escucha NOTIFY 'custom_roles_changed' e invalida cache
// en todos los nodos. Bloqueante, corre en goroutine.
func (r *PGResolver) StartCacheListener(ctx context.Context) error {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire listen conn: %w", err)
	}
	r.listenConn = conn

	_, err = conn.Exec(ctx, "LISTEN custom_roles_changed")
	if err != nil {
		conn.Release()
		return fmt.Errorf("listen custom_roles_changed: %w", err)
	}

	go r.listenLoop(ctx)
	return nil
}

func (r *PGResolver) listenLoop(ctx context.Context) {
	for {
		n, err := r.listenConn.Conn().WaitForNotification(ctx)
		if err != nil {
			// ctx cancelled or connection lost — clean exit
			return
		}
		if n == nil {
			continue
		}
		r.InvalidateCache("", "")
	}
}

// StopCacheListener libera la conexión de escucha.
func (r *PGResolver) StopCacheListener() {
	if r.listenConn != nil {
		r.listenConn.Release()
	}
}
