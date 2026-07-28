package mcpserver

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// El estado de arranque de sesión es POR USUARIO, no por proyecto. Vivía en la tabla
// projects (columnas de 000108), o sea una fila compartida: con dos personas en el mismo
// proyecto, A guardaba su HEAD y B recibía el de A como su last_known, de modo que el
// `git log last_known..current` del protocolo le mostraba commits ajenos como novedades
// propias. No fallaba con error — cada uno recibía un contexto que describía la sesión
// de otro (DOMAINSERV-188).
//
// La asimetría con el CONTENIDO del proyecto es deliberada: observaciones, policies,
// skills y tickets son conocimiento del EQUIPO y siguen compartidos, así que un usuario
// nuevo los recibe completos. El puntero de "hasta dónde vi yo" es personal y arranca
// vacío, para no atribuirle a alguien commits que no vio.

// sessionStateQuerier es la porción de pool/tx que necesita este estado. Interfaz chica
// definida en el consumidor, para que el test pueda pasar un pool y el handler una tx.
type sessionStateQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// UserProjectState es el puntero de una persona en un proyecto. Los campos van como
// string y no como *string: para el consumidor, "no tengo estado" y "tengo el vacío" son
// lo mismo —se trata como primera vez— y un puntero solo agregaría un nil que chequear.
type UserProjectState struct {
	LastKnownHead  string
	LastSeenBranch string
	LastSeenCwd    string
}

// ReadUserProjectState devuelve el estado del usuario en el proyecto. Sin fila —primer
// bootstrap de esa persona ahí— devuelve el cero-valor SIN error: es el caso normal, no
// una anomalía, y es lo que hace que un usuario nuevo arranque sin heredar el puntero de
// otro.
func ReadUserProjectState(ctx context.Context, q sessionStateQuerier, userID, projectID uuid.UUID) (UserProjectState, error) {
	var (
		head   *string
		branch *string
		cwd    *string
	)
	err := q.QueryRow(ctx,
		`SELECT last_known_head, last_seen_branch, last_seen_cwd
		   FROM project_user_session_state
		   WHERE user_id = $1 AND project_id = $2`,
		userID, projectID,
	).Scan(&head, &branch, &cwd)
	if errors.Is(err, pgx.ErrNoRows) {
		return UserProjectState{}, nil
	}
	if err != nil {
		return UserProjectState{}, fmt.Errorf("read user project state: %w", err)
	}
	return UserProjectState{
		LastKnownHead:  safeDeref(head),
		LastSeenBranch: safeDeref(branch),
		LastSeenCwd:    safeDeref(cwd),
	}, nil
}

// BumpUserProjectState upsertea el estado del usuario. El COALESCE(NULLIF(...)) conserva
// el criterio del código original: un bootstrap que llega sin git_head —desde un
// directorio que no es repo, por ejemplo— NO puede borrar el puntero que la persona ya
// tenía.
func BumpUserProjectState(ctx context.Context, q sessionStateQuerier, userID, projectID uuid.UUID, head, branch, cwd string) error {
	_, err := q.Exec(ctx,
		`INSERT INTO project_user_session_state
		     (user_id, project_id, last_known_head, last_seen_branch, last_seen_cwd, last_seen_at)
		 VALUES ($1, $2, NULLIF($3,''), NULLIF($4,''), NULLIF($5,''), NOW())
		 ON CONFLICT (user_id, project_id) DO UPDATE
		    SET last_known_head  = COALESCE(NULLIF($3,''), project_user_session_state.last_known_head),
		        last_seen_branch = COALESCE(NULLIF($4,''), project_user_session_state.last_seen_branch),
		        last_seen_cwd    = COALESCE(NULLIF($5,''), project_user_session_state.last_seen_cwd),
		        last_seen_at     = NOW()`,
		userID, projectID, head, branch, cwd,
	)
	if err != nil {
		return fmt.Errorf("bump user project state: %w", err)
	}
	return nil
}
