package flow

import "time"

// UmbralDeRenovacion: por debajo de esta vigencia restante, una validación exitosa corre la
// expiración hacia adelante y devuelve un token nuevo (DOMAINSERV-218).
//
// Es la mitad del TTL a propósito. Más chico y una fase que valide poco se quedaría sin margen;
// más grande y renovaría en casi toda validación, escribiendo en la base en el camino caliente
// del pre-edit.
const UmbralDeRenovacion = 15 * time.Minute

// NecesitaRenovacion decide si a un token le queda poca vigencia. Con esto el TTL mide
// INACTIVIDAD y no duración de tarea: mientras el agente edite, su autorización se corre; si se
// queda quieto un TTL entero, vence.
//
// Un token ya vencido devuelve false: renovarlo sería resucitarlo, y el TTL dejaría de acotar
// nada.
func NecesitaRenovacion(expiresAtUnix int64, ahora time.Time) bool {
	restante := time.Unix(expiresAtUnix, 0).Sub(ahora)
	return restante > 0 && restante < UmbralDeRenovacion
}
