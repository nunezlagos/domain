package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// DOMAINSERV-222 — guard de borrado destructivo.
//
// EL INCIDENTE (medido): `docker exec docker-ace-did-app rm -f .env.qa`. /app era un
// BIND MOUNT del repo host, así que el rm "dentro del container" borró el original.
//
// EL CRITERIO (segunda ronda, decisión del usuario): el guard NO mira el FLAG, mira el
// RADIO DE DAÑO del objetivo — "lo que me gustaría es que no pueda hacer rm -rf del
// proyecto o directorio parados". Medición que forzó el rediseño: sobre 21.105 comandos
// Bash reales de 972 transcripts, la regla vieja (cualquier rm con -f) daba 334 disparos
// y solo 2 eran de la clase del incidente — señal:ruido 1:333, con 265 de los 334 (79%)
// producidos por `rm -f $SOCK` y parientes. Un guard con ese ruido se aprueba por
// reflejo y deja de proteger, así que `rm -f archivo` SALIÓ del alcance.
//
// Alcance actual, por MOTIVO (el motivo viaja en la razón como "[motivo]" y se compara
// también en el test de paridad: no basta con que los dos clientes disparen, tienen que
// disparar por lo MISMO):
//
//	radio             rm RECURSIVO cuyo objetivo resuelve al cwd del payload, a la raíz
//	                  del repo, a un ANCESTRO de cualquiera de los dos, a /, a $HOME,
//	                  a /home/<user>, al .git entero, o a una raíz de app de container
//	                  (/app /srv /repo /workspace /var/www /usr/src/app /code). Bajo
//	                  envoltura remota (ssh/docker/kubectl) NO se puede resolver nada,
//	                  así que ahí cualquier absoluto que no sea efímero ni un subdir de
//	                  rutina cuenta como catastrófico.
//	radio-indecidible rm RECURSIVO cuyo objetivo no se resuelve ($VAR / $(…) / backtick /
//	                  ~usuario / ~- / ${VAR:-x} sin sufijo concreto): fail-CLOSED, se escala.
//
// EL PRINCIPIO, invertido en la TERCERA RONDA — es la corrección más importante del ticket:
//
//	Si el objetivo de un `rm` recursivo no es un literal que se pueda resolver con CERTEZA,
//	se ESCALA. No se adivina.
//
// Antes el guard intentaba resolver la expresión y, cuando NO podía, concluía "entonces no
// es catastrófico": fail-OPEN. Esa era la causa ÚNICA de los tres bugs que el juez midió, y
// se manifestaba en tres formas que parecían independientes:
//
//	B1 el mapa de asignaciones se envenenaba. Un barrido de `(\w+)=(\S*)` sobre el texto
//	   entero no distinguía una asignación real de un COMENTARIO, de un argumento ni de un
//	   prefijo de entorno, así que `rm -rf $PWD # PWD=tmp` resolvía $PWD a "tmp" y el guard
//	   se apagaba con su propia herramienta. Hoy: la asignación tiene que estar al principio
//	   del segmento, un prefijo de entorno se descarta (bash expande los argumentos ANTES de
//	   armar ese entorno), el mapa es POSICIONAL (una asignación posterior no resuelve un
//	   objetivo anterior) y PWD/HOME/REPO/PROJECT no son sobrescribibles.
//	B2 la flag se clasificaba antes de resolverse, así que en `F=-rf; rm $F .` el `$F` caía
//	   en objetivos, rec quedaba en false y la rama entera del radio se salteaba.
//	B3 los metacaracteres de glob se comparaban como texto: `.g*` (= .git .github
//	   .gitignore), `.*`, `.gi?`, `{.git,dist}`, `~+` y `/ap*` pasaban. Hoy el patrón se
//	   convierte a regex y se prueba contra el conjunto de paths catastróficos — lo que
//	   PUEDE expandir al cwd, al proyecto, a un ancestro o al .git escala; un glob acotado a
//	   un subdirectorio (node_modules/*, coverage*, dist/*) NO, y un glob no recursivo
//	   tampoco (borra archivos de un directorio, no el árbol).
//
// Costo medido del fail-closed: CERO. Sobre 11.358 comandos Bash únicos de 980 transcripts
// reales, el guard viejo daba 37 disparos y el nuevo da 35 — los 2 que bajan son los falsos
// positivos del hueco 6, y ninguno de los 13 comandos de rutina del corpus cambió.
//	sensible          rm de un secreto NO trackeado en git (.env*, *.key, *.pem, id_rsa,
//	                  *credential*, *secret*, *.p12), con o sin flags. Es el incidente
//	                  original y midió 2 disparos en 21.105 comandos: es gratis.
//	sql / sql-opaco   SQL destructivo, o un cliente SQL que ejecuta un archivo/stdin que
//	                  el guard no puede leer.
//	automarker        el comando escribe el marker de bypass de ESTE guard.
//
// El harness Go es el único de hooks que corre en CI (ci-install-user.yml solo hace
// `bash -n` sobre los .sh), así que los casos viven acá y se espejan en la suite bash.
//
// ─────────────────────────────────────────────────────────────────────────────────────
// HUECOS CONOCIDOS (verificados: pasan SIN guard, a propósito o por límite del parser)
// ─────────────────────────────────────────────────────────────────────────────────────
// El header del hook promete esta lista; acá está, y TestGuardDestructivo_HuecosConocidos
// la pinnea contra la implementación real. Si un hueco se cierra, ese test falla: hay que
// mover el caso al corpus y borrarlo de acá, no al revés.
//
//  0. CERRADO en la tercera ronda: `eval` SIN comillas. `eval "rm -rf ."` disparaba (el
//     comando viajaba en un literal y la recursión lo miraba adentro), pero
//     `eval rm -rf .` no tenía literal ninguno: toks[0] quedaba en "eval", posiciones_rm
//     no encontraba el rm en la posición 0 y NADA se evaluaba. Apagaba las TRES clases a
//     la vez (radio, sensible y sql) y es la forma que un agente escribe naturalmente.
//     eval pasó a ser una envoltura LOCAL (lo que sigue corre en ESTE shell, con ESTE
//     cwd), igual que xargs y find.
//  1. COMANDO POR EXPANSIÓN INDECIDIBLE. `$(echo rm) -rf /app`, `rm${IFS}-rf /app`,
//     `R=rm; $R -rf /app`, `echo /app | xargs rm -rf` (el objetivo llega por stdin, no
//     está en la línea). No se emula el expansor de bash: sin ejecutar nada no hay forma
//     de saber qué queda. Se cubre lo que SÍ es decidible (comillas, escapes, /bin/rm).
//  2. SQL EN ARCHIVO NO LEÍDO. `psql -f x.sql` escala como sql-opaco, pero NO se sabe
//     qué hay adentro: si el archivo trae un DROP no se puede afirmar. Y `pg_dump`/
//     `pg_restore` con `--clean` no se inspeccionan.
//  3. VERBOS DE BORRADO FUERA DE rm. `shred -u .env`, `unlink x`, `truncate -s 0 x`,
//     `> archivo` (trunca), `dd of=archivo`, `rsync -a --delete src/ dst/`, `mv x /dev/
//     null`. El alcance acordado es rm; el resto queda fuera y NO está cubierto.
//  4. TOOLING DE VOLÚMENES Y DB. `docker compose down -v` (borra volúmenes con datos),
//     `docker volume rm`, `docker system prune -af --volumes`, `dropdb`, `pg_restore
//     --clean`, `migrate down`, `alembic downgrade base`, `prisma migrate reset`. Nada
//     de esto pasa por rm ni por un cliente SQL con SQL en la línea.
//  5. ANIDAMIENTO > 3. La recursión en literales corta en hondura 3: es un tope
//     deliberado contra el blowup. En la práctica no hay caso REAL que llegue ahí, porque
//     con dos tipos de comilla solo se anidan 2 niveles limpios y el tercero exige
//     escapes — que rompen antes el pareo de comillas (hueco 6). Por eso este hueco no
//     tiene caso pinneado: el que se probó (`ssh a "ssh b \"sh -c …\""`) SÍ dispara.
//  6. CERRADO en la tercera ronda. El enmascarado era por pares de comillas del mismo tipo
//     (una pasada de `'…'`, después una de `"…"`), y eso rompía por los DOS lados:
//     `sh -c "psql -c \"DROP TABLE x\""` perdía el literal interno, y
//     `echo "it's" && rm -rf . && echo "that's"` pareaba los dos apóstrofos entre sí y se
//     comía el rm del medio. Ahora es un escáner IZQUIERDA-A-DERECHA que respeta lo que
//     bash respeta: la comilla que abre primero manda, y adentro de `"` el backslash
//     escapa `" \ $ ` ` y el newline. De paso arregló 2 falsos positivos medidos (un
//     `UPDATE … WHERE` anidado que el pareo viejo partía en dos).
//  7. FLAGS TRAS `--`. `rm -- -rf` trata `-rf` como nombre de archivo (correcto), pero
//     el guard tampoco distingue el caso inverso raro `rm -- -rf .` (ahí `.` sí dispara).
//  8. rm RELATIVO REMOTO. `ssh host rm -rf mydir` no dispara: del otro lado no hay cwd
//     conocido y tratar todo nombre relativo como catastrófico volvería el guard ruido.
//  9. PARIDAD DE ESTADO, NO DE ALCANCE. El test de paridad compara DECISIONES sobre un
//     corpus; un hueco COMPARTIDO por los dos clientes sale verde. Los huecos de esta
//     lista son exactamente eso.
//  10. EL `cd` DENTRO DEL COMANDO NO SE SIGUE. El radio se mide contra el cwd del payload
//     (más la raíz git de ese cwd), así que en `cd /otro/lado && rm -rf ../x` el `../x` se
//     resuelve contra el cwd de la sesión, no contra /otro/lado. Puede sobre-disparar o
//     sub-disparar en ese caso; midió 1 solo disparo de clase radio en 21.166 comandos,
//     así que seguir el cd no paga su propia superficie de bugs. Lo que NO cambia: un
//     `rm -rf .` o `rm -rf *` dispara igual, porque borra el directorio parado sea cual
//     sea. Y las asignaciones del propio comando (VAR=…) SÍ se resuelven.
//  11. rm CON `--` ANTES DE LAS FLAGS. `rm -- -rf .` se lee como "borrar el archivo -rf
//     y el archivo ." — el `.` dispara igual, pero la recursividad no se detecta.
//
// Y una advertencia sobre el resto del sistema: NO te apoyes en el gate SDD como red de
// contención de un borrado. El gate se APAGA con un flow activo (que es justo cuando el
// agente está trabajando), solo mira extensiones de CÓDIGO, y su rama Bash es una
// heurística de "parece edición". El guard destructivo corre ANTES del early-exit por
// flow precisamente porque el gate no protege nada de esto.

const sesionGuard = "sesion-guard-destructivo"

var reMotivo = regexp.MustCompile(`\[([a-z-]+)\]`)

type decisionHook struct {
	Decision string
	Razon    string
}

func (d decisionHook) esDelGuard() bool { return strings.Contains(d.Razon, "destructive-guard") }

// motivo extrae el "[radio]" / "[sensible]" / … de la razón. Es lo que hace que la
// paridad entre clientes sea de CRITERIO y no solo de "alguno disparó".
func (d decisionHook) motivo() string {
	if !d.esDelGuard() {
		return ""
	}
	if m := reMotivo.FindStringSubmatch(d.Razon); m != nil {
		return m[1]
	}
	return "?"
}

// ─── corpus compartido: lo consumen los tests del hook bash Y el de paridad con JS ──

// Cada comando de acá tiene que DISPARAR el guard, con el motivo declarado. Ninguno se
// ejecuta: se le pasa como payload al hook y se lee la decisión.
var comandosDestructivos = []struct{ nombre, cmd, motivo string }{
	// ── radio: el directorio parado y la raíz del repo ──
	{"rm -rf del cwd", "rm -rf .", "radio"},
	{"rm -rf del cwd con barra", "rm -rf ./", "radio"},
	{"rm -rf de $PWD", "rm -rf $PWD", "radio"},
	{"rm -rf de $(pwd)", `rm -rf "$(pwd)"`, "radio"},
	{"rm -rf del glob del cwd", "rm -rf ./*", "radio"},
	{"rm -rf de todo el cwd", "rm -rf *", "radio"},
	{"rm -rf del padre", "rm -rf ..", "radio"},
	{"rm -rf del abuelo", "rm -rf ../..", "radio"},
	{"rm -rf del glob del padre", "rm -rf ../*", "radio"},
	{"rm -rf del .git entero", "rm -rf .git", "radio"},
	// ── radio: raíces absolutas ──
	{"rm -rf de la raíz", "rm -rf /", "radio"},
	{"rm -rf del glob de la raíz", "rm -rf /*", "radio"},
	{"rm -rf de $HOME", "rm -rf $HOME", "radio"},
	{"rm -rf de ~", "rm -rf ~", "radio"},
	{"rm -rf de /home/<user>", "rm -rf /home/nunezlagos", "radio"},
	// ── radio: raíces de app de container (el caso que mordió al usuario) ──
	{"rm -rf de /app", "rm -rf /app", "radio"},
	{"rm -rf de /srv", "rm -rf /srv", "radio"},
	{"rm -rf de /repo", "rm -rf /repo", "radio"},
	{"rm -rf de /workspace", "rm -rf /workspace", "radio"},
	{"rm -rf de /var/www", "rm -rf /var/www", "radio"},
	{"rm -rf de /usr/src/app", "rm -rf /usr/src/app", "radio"},
	// ── radio: formas del flag recursivo ──
	{"rm -R", "rm -R .", "radio"},
	{"rm --recursive", "rm --recursive .", "radio"},
	{"rm -fr", "rm -fr /app", "radio"},
	{"rm -rf sin force", "rm -r /app", "radio"},
	// ── radio: envolturas con el comando ENTRECOMILLADO (el incidente entrecomillado) ──
	{"docker exec con el rm entre comillas", `docker exec docker-ace-did-app "rm -rf /repo"`, "radio"},
	{"ssh con el rm entre comillas (la forma del deploy de este repo)", `ssh vps-domain 'rm -rf /srv/domain'`, "radio"},
	{"kubectl exec con el rm entre comillas", `kubectl exec pod -- "rm -rf /data"`, "radio"},
	{"ssh sin comillas", "ssh vps-domain rm -rf /opt/services", "radio"},
	{"docker run sobre la raíz de app", "docker run --rm -v /repo:/app alpine rm -rf /app", "radio"},
	{"sh -c", `bash -c "rm -rf /"`, "radio"},
	{"eval", `eval "rm -rf $HOME"`, "radio"},
	// ── radio: wrappers que consumen el VALOR de su flag ──
	{"sudo -u con valor", "sudo -u www-data rm -rf /var/www", "radio"},
	{"timeout con duración", "timeout 5 rm -rf /app", "radio"},
	{"nice -n con valor", "nice -n 10 rm -rf /app", "radio"},
	{"env por path absoluto", "/usr/bin/env rm -rf /app", "radio"},
	{"setsid", "setsid rm -rf /app", "radio"},
	{"stdbuf", "stdbuf -oL rm -rf /app", "radio"},
	{"doas", "doas rm -rf /app", "radio"},
	{"exec", "exec rm -rf /app", "radio"},
	// ── radio: gramática de shell ──
	{"dentro de un if/then", "if [ -d x ]; then rm -rf /app; fi", "radio"},
	{"dentro de un for/do", "for f in a b; do rm -rf /app; done", "radio"},
	{"con continuación de línea", "rm -rf \\\n /app", "radio"},
	// ── radio: el token de comando expandido ──
	{"rm entre comillas simples", `'rm' -rf /app`, "radio"},
	{"rm con backslash", `\rm -rf /app`, "radio"},
	{"path absoluto entrecomillado", `"/bin/rm" -rf /app`, "radio"},
	// ── radio: find ──
	{"find -exec rm -rf", "find . -exec rm -rf {} +", "radio"},
	{"find -delete", "find . -delete", "radio"},
	// ── radio: variables asignadas EN EL MISMO comando sí se resuelven ──
	{"objetivo por variable asignada al lado", "D=/; rm -rf $D", "radio"},
	{"cadena de variables hasta $HOME", "H=$HOME; rm -rf $H", "radio"},
	{"raíz de app por variable", "A=/app; rm -rf $A", "radio"},
	// ── radio: el mapa de asignaciones NO se puede envenenar (tercera ronda) ──
	//
	// El mapa existe para RESOLVER `WT=/tmp/x; rm -rf $WT`. Un barrido de `(\w+)=(\S*)`
	// sobre el texto entero convierte ese mapa en un arma: cualquier `PWD=tmp` que NO sea
	// una asignación real (un comentario, el argumento de un echo, un prefijo de entorno)
	// sobrescribía $PWD y el guard concluía "esto borra ./tmp, es inocuo". Verificado con
	// payload al hook: los cuatro PASABAN mientras `rm -rf $PWD` a secas daba ask.
	{"$PWD con un comentario que finge la asignación", "rm -rf $PWD # PWD=tmp", "radio"},
	{"asignación falsa en el argumento de un echo", "echo PWD=tmp; rm -rf $PWD", "radio"},
	// bash expande $PWD ANTES de armar el entorno del rm: el prefijo NO cambia el valor
	// que el rm recibe. Verificado con `PWD=tmp bash -c 'echo $PWD'` → el cwd real.
	{"prefijo de entorno no cambia lo que bash expande", "PWD=tmp rm -rf $PWD", "radio"},
	{"$HOME con un comentario que finge la asignación", "rm -rf $HOME # HOME=tmp", "radio"},
	// ── radio: la FLAG puede llegar por variable, así que se resuelve antes de clasificar ──
	//
	// `t.startswith("-")` corría ANTES de resolver el mapa, así que `$F` caía en objetivos,
	// rec quedaba en False y toda la rama de radio se salteaba. Este caso NUNCA tuvo test.
	{"la flag llega por variable", "F=-rf; rm $F .", "radio"},
	{"la flag llega por variable y el objetivo es /", "F=-rf; rm $F /", "radio"},
	// el NOMBRE del comando también sale del mapa de asignaciones: comparar el token crudo
	// contra "rm" dejaba pasar esto, y era uno de los dos que el sandbox Docker encontró
	{"el nombre del comando llega por variable", "RM=rm; $RM -rf .", "radio"},
	{"el nombre del comando por variable, objetivo absoluto", "R=rm; $R -rf /app", "radio"},
	// el objetivo llega por stdin y no está en el texto: no hay nada que resolver, escala
	{"objetivo por stdin de xargs", "echo /app | xargs rm -rf", "radio-indecidible"},
	// el cwd cambia DENTRO del comando. Se escribe con "." y no con el nombre del proyecto
	// para que el caso no dependa de cómo se llame el directorio del fixture.
	{"cd al padre y después borra ese padre", "cd .. && rm -rf .", "radio"},
	// ── radio: globs y formas de path que BASH expande y el guard comparaba como string ──
	//
	// Verificado con `echo` (read-only): `.g*` → `.git .github .gitignore`, `~+` → el cwd,
	// `domain-service*` → el proyecto entero. El guard solo trataba el glob trailing `/*`.
	{"~+ es el cwd", "rm -rf ~+", "radio"},
	{"glob que alcanza el .git", "rm -rf .g*", "radio"},
	{"glob de los ocultos alcanza el .git", "rm -rf .*", "radio"},
	{"glob con ./ que alcanza el .git", "rm -rf ./.g*", "radio"},
	{"? en el glob que alcanza el .git", "rm -rf .gi?", "radio"},
	{"glob de ocultos con ? alcanza el .git", "rm -rf .?*", "radio"},
	{"clase de caracteres que alcanza el .git", "rm -rf .gi[t]", "radio"},
	{"brace expansion que alcanza el .git", "rm -rf {.git,dist}", "radio"},
	{"glob de un ancestro absoluto", "rm -rf /ap*", "radio"},
	// el escape INTERIOR: bash lo resuelve y el guard lo comparaba como texto. Verificado
	// con `echo .gi\t` → `.git` (read-only, no borra nada).
	{"escape interior en el nombre", `rm -rf .gi\t`, "radio"},
	{"escape interior con ./", `rm -rf ./\.git`, "radio"},
	// ── radio-indecidible: fail-closed ──
	{"objetivo por variable sin resolver", "rm -rf $DIR", "radio-indecidible"},
	{"objetivo por sustitución de comando", "rm -rf $(git rev-parse --show-toplevel)", "radio-indecidible"},
	{"la asignación POSTERIOR no resuelve el objetivo", "rm -rf $D; D=/tmp/x", "radio-indecidible"},
	{"${VAR:-/} con default no se resuelve", "rm -rf ${DIR:-/}", "radio-indecidible"},
	{"~user es el home de OTRO usuario", "rm -rf ~ubuntu/app", "radio-indecidible"},
	{"~- es el $OLDPWD y no se conoce", "rm -rf ~-", "radio-indecidible"},
	// ── eval SIN comillas. `eval "rm -rf ."` disparaba porque el comando viajaba en un
	// literal y la recursión lo miraba adentro; `eval rm -rf .` no tiene literal ninguno,
	// así que toks[0] era "eval", posiciones_rm no encontraba el rm en la posición 0 y
	// NADA se evaluaba. Apagaba las tres clases a la vez (radio, sensible y sql), y es la
	// forma que un agente escribe naturalmente. eval es una envoltura LOCAL: lo que sigue
	// se ejecuta en ESTE shell, con ESTE cwd.
	{"eval sin comillas sobre el cwd", "eval rm -rf .", "radio"},
	{"eval sin comillas sobre /", "eval rm -rf /", "radio"},
	{"eval sin comillas sobre $PWD", "eval rm -rf $PWD", "radio"},
	{"eval detrás de un wrapper", "nice eval rm -rf .", "radio"},
	{"eval detrás de sudo sobre una raíz de app", "sudo eval rm -rf /srv/domain", "radio"},
	{"eval sin comillas de un sensible", "eval rm -f .env.qa", "sensible"},
	{"eval sin comillas de SQL destructivo", `eval psql -c "DROP TABLE observations"`, "sql"},
	// ── los evasores que el juez dejó abiertos ──
	{"subshell entre paréntesis", "(rm -rf .)", "radio"},
	{"docker-compose v1 (con guión)", "docker-compose exec app rm -rf /app", "radio"},
	{"podman exec", "podman exec c rm -rf /app", "radio"},
	// colisión de apóstrofos: el pareo de single-quotes se comía el rm del medio
	{"apóstrofos en los echos vecinos", `echo "it's" && rm -rf . && echo "that's"`, "radio"},
	// /srv es raíz de app: /srv/<app> ES la app, no un subdirectorio cualquiera
	{"subdirectorio de app bajo una raíz de app", "sudo -u www-data rm -rf /srv/domain", "radio"},
	{"raíz de app bajo /var/www", "rm -rf /var/www/html", "radio"},
	// comilla doble anidada con escape: el enmascarado por pares perdía el literal interno
	{"comilla doble anidada con escape", `docker exec pg sh -c "psql -c \"DROP TABLE observations\""`, "sql"},
	// el automarker cerrado SOLO en su forma literal
	{"marker de bypass por variable", "M=$HOME/.local/state/domain/destructive-bypass-x; echo r > $M", "automarker"},
	{"marker de bypass dentro de un sh -c", `sh -c "echo r > ~/.local/state/domain/destructive-bypass-x"`, "automarker"},
	{"marker de bypass por python -c", `python3 -c "open('/home/u/.local/state/domain/destructive-bypass-x','w')"`, "automarker"},
	// ── sensible: el incidente original ──
	{"el incidente", "docker exec docker-ace-did-app rm -f .env.qa", "sensible"},
	{"rm -f de un .env", "rm -f .env.qa", "sensible"},
	{"rm sin flags de un .pem", "rm deploy/server.pem", "sensible"},
	{"rm sin flags de .env", "rm .env.production", "sensible"},
	{"rm sin flags de id_rsa", "rm config/id_rsa", "sensible"},
	{"rm sin flags de .key", "rm certs/tls.key", "sensible"},
	{"rm sin flags de credential", "rm secrets/app-credentials.json", "sensible"},
	{"rm sin flags de .p12", "rm store.p12", "sensible"},
	{"rm de un sensible entrecomillado", `rm "$HOME/.env"`, "sensible"},
	// ── sql ──
	{"DROP TABLE via psql -c", `psql -c "DROP TABLE observations"`, "sql"},
	{"DROP DATABASE via docker exec psql", `docker exec pg psql -U postgres -c "DROP DATABASE domain"`, "sql"},
	{"DROP SCHEMA via mysql -e", `mysql -e "DROP SCHEMA app"`, "sql"},
	{"TRUNCATE", `psql -c "TRUNCATE observations"`, "sql"},
	{"TRUNCATE por heredoc", "psql -U domain <<SQL\nTRUNCATE observations;\nSQL", "sql"},
	{"TRUNCATE por heredoc con terminador indentado", "psql -U domain <<-SQL\n\tTRUNCATE observations;\n\tSQL", "sql"},
	{"DELETE FROM sin WHERE", `psql -c "DELETE FROM observations"`, "sql"},
	{"UPDATE sin WHERE", `psql -c "UPDATE observations SET body = 'x'"`, "sql"},
	{"DROP con comentario de bloque en medio", `psql -c "DROP/**/TABLE observations"`, "sql"},
	{"DELETE con WHERE true", `psql -c "DELETE FROM observations WHERE true"`, "sql"},
	{"DELETE con WHERE 1=1", `psql -c "DELETE FROM observations WHERE 1=1"`, "sql"},
	{"DELETE con el WHERE comentado", `psql -c "DELETE FROM observations /* WHERE id = 1 */"`, "sql"},
	{"DROP OWNED BY", `psql -c "DROP OWNED BY app"`, "sql"},
	{"ALTER TABLE DROP COLUMN", `psql -c "ALTER TABLE observations DROP COLUMN body"`, "sql"},
	{"DROP INDEX", `psql -c "DROP INDEX idx_obs"`, "sql"},
	{"DROP MATERIALIZED VIEW", `psql -c "DROP MATERIALIZED VIEW mv_obs"`, "sql"},
	{"DROP TABLE por pipe (el pipe separaba los segmentos)", `echo 'DROP TABLE observations' | psql`, "sql"},
	// ── sql-opaco: el guard no puede leer el archivo ──
	{"psql -f", "psql -f migrate.sql", "sql-opaco"},
	{"psql con redirección", "psql < dump.sql", "sql-opaco"},
	{"dump por pipe a psql", "cat dump.sql | psql", "sql-opaco"},
	{"psql con \\i", `psql -c "\i x.sql"`, "sql-opaco"},
	// ── automarker: el guard no se auto-autoriza ──
	{"echo al marker de bypass", "echo motivo > $HOME/.local/state/domain/destructive-bypass-abc", "automarker"},
	{"printf al marker de bypass", `printf 'motivo\n' > ~/.local/state/domain/destructive-bypass-abc`, "automarker"},
	{"tee al marker de bypass", "echo motivo | tee ~/.local/state/domain/destructive-bypass-abc", "automarker"},
	{"cp al marker de bypass", "cp /tmp/razon ~/.local/state/domain/destructive-bypass-abc", "automarker"},
	{"mv al marker de bypass", "mv /tmp/razon ~/.local/state/domain/destructive-bypass-abc", "automarker"},
	{"touch del marker de bypass", "touch ~/.local/state/domain/destructive-bypass-abc", "automarker"},
}

// Cada comando de acá NO tiene que disparar el guard. Si alguno dispara, el guard es
// RUIDO: el humano lo empieza a aprobar por reflejo y deja de proteger.
var comandosInofensivos = []struct{ nombre, cmd string }{
	// ── la clase que producía el 79% de los disparos medidos ──
	{"rm -f de un socket por variable", "rm -f $SOCK"},
	{"rm -f de un socket", "rm -f /tmp/domain.sock"},
	{"rm -f de un artefacto", "rm -f coverage.out"},
	{"rm de una nota, sin flags", "rm notas.md"},
	// ── rutina de desarrollo: subdirectorios que se regeneran ──
	{"rm -rf de node_modules", "rm -rf node_modules"},
	{"rm -rf de dist", "rm -rf dist"},
	{"rm -rf de build", "rm -rf build"},
	{"rm -rf de vendor", "rm -rf vendor"},
	{"rm -rf de target", "rm -rf target"},
	{"rm -rf de .next", "rm -rf .next"},
	{"rm -rf de coverage", "rm -rf coverage"},
	{"rm -rf de un subdir con ./", "rm -rf ./dist"},
	{"rm -rf de un dist bajo variable con sufijo concreto", `rm -rf "$BUILD_DIR/dist"`},
	// las 32 de esta clase eran el segundo bloque de ruido: la variable está asignada
	// UNA LÍNEA ARRIBA, así que es resoluble y no hay nada indecidible
	{"rm -rf de un temp propio por variable", `WT=/tmp/claude/wt; rm -rf "$WT"`},
	{"rm -rf de un mktemp", "d=$(mktemp -d); cd $d; rm -rf $d"},
	{"rm -rf y remake de un scratch propio", "SB=/tmp/sab; rm -rf $SB; mkdir -p $SB"},
	{"rm -rf de un lock dentro de .git", "rm -rf .git/index.lock"},
	{"rm -rf bajo /tmp", "rm -rf /tmp/build"},
	{"rm -rf del cache del home", "rm -rf ~/.cache/go-build"},
	// ── el fail-closed de globs no puede tragarse la rutina (tercera ronda) ──
	//
	// Lo que escala es el glob que PUEDE expandir al proyecto, al cwd, a un ancestro o al
	// .git. Un glob cuyo prefijo literal ya cae dentro de un subdirectorio acotado, no: se
	// verifica convirtiendo el patrón a regex y probándolo contra el conjunto de paths
	// catastróficos, en vez de escalar por el solo hecho de ver un `*`.
	{"glob acotado a node_modules", "rm -rf node_modules/*"},
	{"glob acotado a dist", "rm -rf dist/*"},
	{"glob de artefactos que no alcanza el .git", "rm -rf coverage*"},
	{"glob bajo el cache del home", "rm -rf ~/.cache/go-build/*"},
	{"glob acotado a un subdir de /tmp", "rm -rf /tmp/build/*"},
	// no recursivo: un glob de archivos no se lleva el árbol, y `rm -f *.log` es rutina
	{"glob no recursivo de logs", "rm -f *.log"},
	{"glob no recursivo de artefactos", "rm -f coverage.*"},
	// Estos dos NO escalan a propósito, y es el criterio del fail-closed funcionando, no un
	// hueco: un glob que estructuralmente NO PUEDE expandir a nada catastrófico no cuesta un
	// disparo. Verificado read-only con `bash -c 'shopt -s nullglob; echo ? ; echo ..?*'` →
	// los dos expanden a NADA en este repo. `?` es exactamente un carácter y el glob de bash
	// no matchea el punto inicial, así que no puede ser `.`, `..` ni `.git` (4 caracteres);
	// `..?*` exige `..` más un carácter, así que tampoco alcanza `..`. En cambio `.?*` SÍ
	// escala, porque alcanza `.git` — está en el corpus destructivo.
	{"glob de un solo carácter no alcanza . ni .. ni .git", "rm -rf ?"},
	{"glob de ..?* no alcanza el padre", "rm -rf ..?*"},
	// el comentario se strippea: lo que está detrás de un # no se ejecuta
	{"comentario que menciona un rm -rf", "ls -la # ojo con rm -rf /"},
	// decisión EXPLÍCITA del usuario: "el .claude me da igual"
	{"rm -rf de ~/.claude", "rm -rf $HOME/.claude"},
	// ── secretos que NO son el incidente ──
	{"rm de un .env.example", "rm -f .env.example"},
	{"rm de un scratch en /tmp con secret en el nombre", "rm /tmp/no-secret.txt"},
	// ── read-only: el literal viaja DENTRO del patrón (DOMAINSERV-114/146) ──
	{"grep con rm -rf en el patrón", `grep "rm -rf /" install-user/hooks/domain-pre-edit.sh`},
	{"rg con DROP TABLE en el patrón", `rg -n "DROP TABLE" services/domain-mcp/internal/migrate/migrations/`},
	{"git log -S", `git log -S "rm -rf"`},
	{"grep del rm -f que el propio hook usa", `grep -n 'rm -f' install-user/hooks/domain-pre-edit.sh`},
	{"grep del propio marker de bypass", `grep -rn "destructive-bypass" install-user/`},
	{"ls del marker de bypass", "ls ~/.local/state/domain/destructive-bypass-abc"},
	{"commit que documenta el guard", `git commit -m "feat(guard): bloquea rm -rf / y DROP TABLE"`},
	{"commit por heredoc que documenta el guard", "git commit -F - <<MSG\nfeat: el guard corta rm -rf / y TRUNCATE\nMSG"},
	// DOMAINSERV-114 en su forma más filosa: el cuerpo del heredoc ARRANCA con el
	// comando. Sin el enmascarado de literales, el salto de línea parte el heredoc en
	// segmentos y esa línea se lee como un comando a ejecutar.
	{"commit cuyo mensaje arranca con rm -rf", "git commit -F - <<MSG\nrm -rf / ya no se corre a mano: lo hace el installer\nMSG"},
	// falso positivo del terminador indentado: DOCUMENTAR el guard se auto-bloqueaba
	{"doc por heredoc con terminador indentado", "cat >> notas.md <<-DOC\n\trm -rf / es lo que el guard bloquea\n\tDOC"},
	{"heredoc que ESCRIBE un rm, no lo ejecuta", "cat > script.sh <<'EOF'\nrm -rf /\nEOF"},
	{"echo que menciona rm -rf", `echo "no corras rm -rf /"`},
	// ── SQL benigno ──
	{"DELETE con WHERE", `psql -c "DELETE FROM observations WHERE id = 1"`},
	{"UPDATE con WHERE", `psql -c "UPDATE observations SET body = 'x' WHERE id = 1"`},
	{"SELECT", `psql -c "SELECT count(*) FROM observations"`},
	{"SELECT con pipe a grep", `psql -c "SELECT 1" | grep -c 1`},
	{"SELECT por pipe desde echo", `echo "SELECT 1" | psql`},
	{"mysql -f es --force, no --file", `mysql -f -e "SELECT 1"`},
	// el cliente SQL tiene que estar en POSICIÓN DE COMANDO: que la palabra aparezca no
	// alcanza (esto disparaba sql-opaco y era el primer bloque de ruido de la clase)
	{"grep de containers mysql", `docker ps --format '{{.Names}}' | grep -i mysql`},
	{"which mysql", "which mysql 2>/dev/null || echo no-mysql"},
	{"SQL por stdin desde printf a un psql remoto", `printf 'select 1' | ssh host 'psql -f -'`},
	// ── envolturas que no borran nada ──
	{"docker rm de un container", "docker rm docker-ace-did-app"},
	{"docker run con --rm", "docker run --rm alpine echo hola"},
	{"rm -rf de un subdir de rutina bajo la raíz de app", "docker run --rm -v /repo:/app alpine rm -rf /app/dist"},
	{"docker exec con rm -rf de node_modules", "docker exec ctr rm -rf node_modules"},
	{"ssh read-only", `ssh host "ls -la /srv"`},
	{"ls del binario rm", "ls -la /usr/bin/rm"},
	{"xargs rm -f (la clase de ruido, ahora fuera de alcance)", `find . -name "*.log" | xargs rm -f`},
	{"find filtrado que limpia node_modules", `find . -type d -name node_modules -exec rm -rf {} +`},
	{"loop de shell con rm -f", "for f in *.log; do rm -f $f; done"},
	{"correr los propios tests del guard", "go test ./install-user/ -run TestGuardDestructivo"},
}

// ─── plomería ────────────────────────────────────────────────────────────────────

// dirLimpio: NO usar t.TempDir() para el cwd ni el HOME de estos tests. t.TempDir()
// mete el NOMBRE del subtest en el path, y los subtests de este archivo se llaman
// "rm -rf de $PWD" / "$HOME": un cwd que contiene un "$" hace que el guard lea su propio
// cwd como una expansión sin resolver y decida distinto. Mordió de verdad: tres casos del
// corpus pasaban sin guard solo por el nombre del directorio temporal.
func dirLimpio(t *testing.T) string {
	t.Helper()
	d, err := os.MkdirTemp("", "domain-guard-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(d) })
	return d
}

func decisionDeBash(t *testing.T, home, cmd, modo string) decisionHook {
	t.Helper()
	return decisionDeBashEnCwd(t, home, dirLimpio(t), cmd, modo)
}

func decisionDeBashEnCwd(t *testing.T, home, cwd, cmd, modo string) decisionHook {
	t.Helper()
	return decisionDelHookEnCwd(t, home, cwd, map[string]any{
		"session_id":      sesionGuard,
		"hook_event_name": "PreToolUse",
		"tool_name":       "Bash",
		"permission_mode": modo,
		"cwd":             cwd,
		"tool_input":      map[string]any{"command": cmd},
	})
}

func decisionDelHook(t *testing.T, home string, payload map[string]any) decisionHook {
	t.Helper()
	return decisionDelHookEnCwd(t, home, dirLimpio(t), payload)
}

func decisionDelHookEnCwd(t *testing.T, home, cwd string, payload map[string]any) decisionHook {
	t.Helper()
	cmd := exec.Command("bash", hookAbsoluto(t, "domain-pre-edit.sh"))
	cmd.Stdin = strings.NewReader(payloadJSON(t, payload))
	// cwd aislado por default: el guard mide el radio contra el cwd y no queremos que
	// mire este repo salvo cuando el test lo pide explícitamente
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("domain-pre-edit.sh falló: %v", err)
	}
	return parsearDecision(t, string(out))
}

func parsearDecision(t *testing.T, out string) decisionHook {
	t.Helper()
	if strings.TrimSpace(out) == "" {
		return decisionHook{}
	}
	var v struct {
		HookSpecificOutput struct {
			PermissionDecision       string
			PermissionDecisionReason string
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("el hook no emitió JSON parseable: %v\n%s", err, out)
	}
	return decisionHook{
		Decision: v.HookSpecificOutput.PermissionDecision,
		Razon:    v.HookSpecificOutput.PermissionDecisionReason,
	}
}

// ─── el incidente ────────────────────────────────────────────────────────────────

func TestGuardDestructivo_ElIncidente_DockerExecRmF(t *testing.T) {
	d := decisionDeBash(t, dirLimpio(t), "docker exec docker-ace-did-app rm -f .env.qa", "default")
	if !d.esDelGuard() {
		t.Fatalf("el rm -f que borró el .env.qa del host pasó sin guard: decision=%q razon=%q",
			d.Decision, d.Razon)
	}
	if d.motivo() != "sensible" {
		t.Errorf("el incidente es de la clase sensible, no %q", d.motivo())
	}
	if d.Decision != "ask" {
		t.Errorf("en modo default el humano tiene que poder decidir: decision=%q", d.Decision)
	}
}

// ─── el conjunto que debe disparar, con su motivo ────────────────────────────────

func TestGuardDestructivo_ComandosDestructivos_Disparan(t *testing.T) {
	for _, c := range comandosDestructivos {
		t.Run(c.nombre, func(t *testing.T) {
			d := decisionDeBash(t, dirLimpio(t), c.cmd, "bypassPermissions")
			if !d.esDelGuard() {
				t.Fatalf("pasó sin guard: %s\ndecision=%q razon=%q", c.cmd, d.Decision, d.Razon)
			}
			if d.motivo() != c.motivo {
				t.Errorf("motivo equivocado en %q: esperaba %q, obtuve %q", c.cmd, c.motivo, d.motivo())
			}
		})
	}
}

// ─── el conjunto que NO debe disparar ────────────────────────────────────────────

func TestGuardDestructivo_ComandosInofensivos_NoDisparan(t *testing.T) {
	for _, c := range comandosInofensivos {
		t.Run(c.nombre, func(t *testing.T) {
			d := decisionDeBash(t, dirLimpio(t), c.cmd, "bypassPermissions")
			if d.esDelGuard() {
				t.Errorf("el guard bloqueó un comando que no borra el proyecto: %s\nrazon=%q",
					c.cmd, d.Razon)
			}
		})
	}
}

// ─── el radio se mide contra el cwd REAL de la sesión ────────────────────────────
//
// Este es el corazón del rediseño: el mismo `rm -rf <x>` dispara o no según DÓNDE esté
// parada la sesión. Sin el cwd del payload el guard no puede decidir esto.
func TestGuardDestructivo_RadioSeMideContraElCwdDelPayload(t *testing.T) {
	home := dirLimpio(t)
	raiz := dirLimpio(t)
	hijo := filepath.Join(raiz, "sub", "hondo")
	if err := os.MkdirAll(hijo, 0o755); err != nil {
		t.Fatal(err)
	}
	casos := []struct {
		nombre  string
		cwd     string
		cmd     string
		dispara bool
	}{
		{"el cwd mismo", hijo, "rm -rf .", true},
		{"un ancestro del cwd por path absoluto", hijo, "rm -rf " + raiz, true},
		{"el padre por ..", hijo, "rm -rf ..", true},
		{"un hermano NO es catastrófico", hijo, "rm -rf " + filepath.Join(raiz, "sub", "otro"), false},
		{"un hijo del cwd NO es catastrófico", hijo, "rm -rf artefactos", false},
		// glob que EXPANDE al cwd desde el padre: bash resuelve `hond*` a `hondo`, que es
		// justo el directorio parado. Verificado con `echo` antes de escribir el caso.
		{"glob del padre que alcanza el cwd", hijo, "rm -rf ../hond*", true},
		{"glob absoluto que alcanza el cwd", hijo, "rm -rf " + filepath.Join(raiz, "sub", "hond*"), true},
		// el mismo glob, un carácter distinto: ya NO puede expandir al cwd
		{"glob del padre que NO alcanza el cwd", hijo, "rm -rf ../otra-cos*", false},
		{"glob de un hijo del cwd NO es catastrófico", hijo, "rm -rf artefact*", false},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			d := decisionDeBashEnCwd(t, home, c.cwd, c.cmd, "bypassPermissions")
			if got := d.esDelGuard(); got != c.dispara {
				t.Errorf("cwd=%s cmd=%q: esperaba dispara=%v, obtuve %v (razon=%q)",
					c.cwd, c.cmd, c.dispara, got, d.Razon)
			}
		})
	}
}

// La raíz del repo git también es radio catastrófico aunque la sesión esté parada en un
// subdirectorio: `rm -rf ../..` desde services/x se lleva el proyecto entero.
func TestGuardDestructivo_RadioIncluyeLaRaizDelRepo(t *testing.T) {
	home := dirLimpio(t)
	raiz := dirLimpio(t)
	gitInit(t, raiz)
	hijo := filepath.Join(raiz, "services", "domain-mcp")
	if err := os.MkdirAll(hijo, 0o755); err != nil {
		t.Fatal(err)
	}
	if d := decisionDeBashEnCwd(t, home, hijo, "rm -rf ../..", "bypassPermissions"); !d.esDelGuard() {
		t.Errorf("borrar la raíz del repo desde un subdir pasó sin guard: %q", d.Razon)
	}
	if d := decisionDeBashEnCwd(t, home, hijo, "rm -rf internal/tmp", "bypassPermissions"); d.esDelGuard() {
		t.Errorf("un subdirectorio del propio módulo no es radio catastrófico: %q", d.Razon)
	}
}

// ─── el secreto trackeado en git NO es irrecuperable ─────────────────────────────
//
// Falso positivo medido: el guard disparaba sobre archivos que `git checkout --`
// recupera. Ese caso es del git-guard, no de este.
func TestGuardDestructivo_SecretoTrackeado_NoDispara(t *testing.T) {
	home := dirLimpio(t)
	repo := dirLimpio(t)
	gitInit(t, repo)
	escribir(t, filepath.Join(repo, ".env.example"), "API_KEY=\n")
	escribir(t, filepath.Join(repo, ".env.local"), "API_KEY=secreto\n")
	gitAdd(t, repo, ".env.example")

	if d := decisionDeBashEnCwd(t, home, repo, "rm .env.example", "bypassPermissions"); d.esDelGuard() {
		t.Errorf("un .env.example TRACKEADO no se pierde: lo recupera git checkout -- (%q)", d.Razon)
	}
	d := decisionDeBashEnCwd(t, home, repo, "rm .env.local", "bypassPermissions")
	if !d.esDelGuard() {
		t.Errorf("un .env NO trackeado sí se pierde para siempre y tiene que disparar: %q", d.Razon)
	}
	if d.motivo() != "sensible" {
		t.Errorf("motivo esperado sensible, obtuve %q", d.motivo())
	}
}

func gitInit(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "t@t"},
		{"config", "user.name", "t"},
	} {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Skipf("sin git usable no se puede verificar el criterio de trackeado: %v\n%s", err, out)
		}
	}
}

func gitAdd(t *testing.T, dir, path string) {
	t.Helper()
	c := exec.Command("git", "add", "--", path)
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
}

func escribir(t *testing.T, path, contenido string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contenido), 0o600); err != nil {
		t.Fatal(err)
	}
}

// ─── el archivo SQL se LEE antes de escalar ──────────────────────────────────────
//
// `psql -f x.sql` / `psql < x.sql` no se puede afirmar benigno… salvo que el archivo
// esté acá y se pueda leer. Escalar a ciegas costaba 31 disparos medidos, casi todos
// probando migraciones en un container de descarte. Leerlo convierte esos 31 en la
// decisión correcta y de paso ATRAPA el DROP que el archivo trae adentro.
func TestGuardDestructivo_ArchivoSql_SeLeeAntesDeEscalar(t *testing.T) {
	home := dirLimpio(t)
	cwd := dirLimpio(t)
	escribir(t, filepath.Join(cwd, "limpio.sql"), "SELECT count(*) FROM observations;\n")
	escribir(t, filepath.Join(cwd, "sucio.sql"), "BEGIN;\nDROP TABLE observations;\nCOMMIT;\n")

	casos := []struct{ nombre, cmd, motivo string }{
		{"archivo legible y benigno", "psql -f limpio.sql", ""},
		{"archivo legible y benigno por redirección", "psql < limpio.sql", ""},
		{"archivo legible con DROP", "psql -f sucio.sql", "sql"},
		{"archivo legible con DROP por redirección", "docker exec -i pg psql -U d -d d < sucio.sql", "sql"},
		{"archivo que no se puede leer", "psql -f no-existe.sql", "sql-opaco"},
		{"archivo del otro lado del ssh", `ssh vps 'psql -f /tmp/bf.sql'`, "sql-opaco"},
		{"stdin explícito con el SQL a la vista", `printf 'SELECT 1' | psql -f -`, ""},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			d := decisionDeBashEnCwd(t, home, cwd, c.cmd, "bypassPermissions")
			if d.motivo() != c.motivo {
				t.Errorf("%q: esperaba motivo %q, obtuve %q (razon=%q)", c.cmd, c.motivo, d.motivo(), d.Razon)
			}
		})
	}
}

// El guard es de Bash. Las migraciones del repo LLEVAN `DROP TABLE IF EXISTS` en su
// .down.sql por policy (verificado: 000002_create_organizations.down.sql y ~180 más),
// así que gatear el CONTENIDO de un Write dejaría el repo inmantenible.
func TestGuardDestructivo_WriteDeMigracionConDropTable_NoDispara(t *testing.T) {
	d := decisionDelHook(t, dirLimpio(t), map[string]any{
		"session_id":      sesionGuard,
		"hook_event_name": "PreToolUse",
		"tool_name":       "Write",
		"permission_mode": "bypassPermissions",
		"tool_input": map[string]any{
			"file_path": "/repo/migrations/000222_x.down.sql",
			"content":   "DROP TABLE IF EXISTS observations;\nTRUNCATE otra;\n",
		},
	})
	if d.esDelGuard() {
		t.Errorf("el guard se metió en un Write de migración: razon=%q", d.Razon)
	}
}

// ─── el corazón del mecanismo: ask vs deny ───────────────────────────────────────

// acceptEdits es INTERACTIVO (se activa con shift+tab: hay humano al teclado), así que
// ahí el ask SÍ llega a una persona y un deny sería un muro sin salida. El deny duro
// queda SOLO para bypassPermissions, que es el único modo sin nadie mirando. Y un modo
// DESCONOCIDO cae en ask: un modo nuevo de Claude Code no debe volverse un deny mudo.
func TestGuardDestructivo_ModoDePermisos_DecideAskODeny(t *testing.T) {
	casos := map[string]string{
		"default":               "ask",
		"plan":                  "ask",
		"acceptEdits":           "ask",
		"bypassPermissions":     "deny",
		"modoNuevoDeClaudeAlgo": "ask",
	}
	for modo, esperada := range casos {
		t.Run(modo, func(t *testing.T) {
			d := decisionDeBash(t, dirLimpio(t), "rm -rf /app", modo)
			if !d.esDelGuard() {
				t.Fatalf("el guard no disparó en modo %s: %q", modo, d.Razon)
			}
			if d.Decision != esperada {
				t.Errorf("modo %s: esperaba %q, obtuve %q", modo, esperada, d.Decision)
			}
		})
	}
}

// El deny tiene que ser satisfacible: si no nombra la ruta del bypass, el humano no
// tiene cómo autorizar un borrado legítimo en modo automático (DOMAINSERV-195).
func TestGuardDestructivo_ElDenyNombraElBypass(t *testing.T) {
	d := decisionDeBash(t, dirLimpio(t), "rm -rf /app", "bypassPermissions")
	if !strings.Contains(d.Razon, "destructive-bypass-") {
		t.Errorf("el deny no dice CÓMO autorizarlo: %q", d.Razon)
	}
}

func rutaBypassDestructivo(home string) string {
	return filepath.Join(home, ".local", "state", "domain", "destructive-bypass-"+sesionGuard)
}

func escribirBypass(t *testing.T, home, razon string) {
	t.Helper()
	p := rutaBypassDestructivo(home)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(razon+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestGuardDestructivo_Bypass_EsDeUnSoloUso(t *testing.T) {
	home := dirLimpio(t)
	escribirBypass(t, home, "hay que rearmar /app en el container de QA")

	if d := decisionDeBash(t, home, "rm -rf /app", "bypassPermissions"); d.esDelGuard() {
		t.Fatalf("con bypass el borrado tiene que pasar: %q", d.Razon)
	}
	if _, err := os.Stat(rutaBypassDestructivo(home)); !os.IsNotExist(err) {
		t.Error("el bypass no se consumió: un solo uso, no una sesión entera abierta")
	}
	if d := decisionDeBash(t, home, "rm -rf /app", "bypassPermissions"); !d.esDelGuard() {
		t.Errorf("consumido el bypass, el segundo borrado tiene que volver a bloquearse: %q", d.Razon)
	}
}

// ─── el bypass no es auto-anulable ───────────────────────────────────────────────
//
// Verificado antes del fix: el agente escribía su propio marker con un `echo >` y el
// siguiente rm -rf pasaba en bypassPermissions. O sea que el deny duro era decorativo
// contra un agente. Autorizar tiene que costar una intervención HUMANA.
func TestGuardDestructivo_EscribirElPropioBypass_EstaBloqueado(t *testing.T) {
	home := dirLimpio(t)
	marker := rutaBypassDestructivo(home)
	cmd := "echo 'lo necesito' > " + marker

	d := decisionDeBash(t, home, cmd, "bypassPermissions")
	if !d.esDelGuard() {
		t.Fatalf("el agente pudo escribirse el bypass: %q", d.Razon)
	}
	if d.motivo() != "automarker" {
		t.Errorf("motivo esperado automarker, obtuve %q", d.motivo())
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Error("el hook no debería haber creado el marker")
	}
}

// Y un bypass ya emitido NO habilita renovarse: si lo hiciera, un solo OK humano se
// volvería una sesión entera de borrados.
func TestGuardDestructivo_ElBypassNoHabilitaRenovarse(t *testing.T) {
	home := dirLimpio(t)
	escribirBypass(t, home, "un borrado legítimo")
	d := decisionDeBash(t, home, "echo 'otro mas' > "+rutaBypassDestructivo(home), "bypassPermissions")
	if !d.esDelGuard() || d.motivo() != "automarker" {
		t.Errorf("con un bypass vivo el agente pudo renovárselo: decision=%q razon=%q",
			d.Decision, d.Razon)
	}
	if _, err := os.Stat(rutaBypassDestructivo(home)); err != nil {
		t.Error("el bypass no se tenía que consumir: la operación fue RECHAZADA, no autorizada")
	}
}

// ─── huecos conocidos: se pinnean para que la lista del header no mienta ─────────
//
// Estos comandos PASAN sin guard. No es un test de que "esté bien": es un test de que la
// documentación de arriba describe la implementación real. Si alguno se cierra, este test
// falla y hay que mover el caso al corpus destructivo y borrarlo de la lista.
func TestGuardDestructivo_HuecosConocidos(t *testing.T) {
	huecos := []struct{ hueco, cmd string }{
		{"1. comando por expansión indecidible", "$(echo rm) -rf /app"},
		{"1. comando por expansión indecidible", "rm${IFS}-rf /app"},
		// "comando por variable" y "objetivo por stdin de xargs" estaban acá y se CERRARON:
		// el nombre del comando pasa por el mapa de asignaciones igual que un objetivo, y un
		// rm recursivo sin objetivo en el texto escala. Los casos viven ahora en el corpus
		// destructivo. Este test los detectó al cerrarse, que es para lo que existe.
		{"3. verbos fuera de rm: shred", "shred -u .env"},
		{"3. verbos fuera de rm: unlink", "unlink .env"},
		{"3. verbos fuera de rm: truncate", "truncate -s 0 .env"},
		{"3. verbos fuera de rm: redirección que trunca", "> .env"},
		{"3. verbos fuera de rm: dd", "dd if=/dev/zero of=.env bs=1 count=0"},
		{"3. verbos fuera de rm: rsync --delete", "rsync -a --delete /tmp/vacio/ ./"},
		{"4. tooling de volúmenes: compose down -v", "docker compose down -v"},
		{"4. tooling de volúmenes: docker volume rm", "docker volume rm domain_pgdata"},
		{"4. tooling de DB: dropdb", "dropdb domain"},
		{"4. tooling de DB: pg_restore --clean", "pg_restore --clean -d domain dump.sql"},
		{"4. tooling de DB: migrate down", "migrate -path migrations -database $DSN down"},
		{"8. rm relativo remoto", "ssh host rm -rf mydir"},
	}
	for _, h := range huecos {
		t.Run(h.hueco+" :: "+h.cmd, func(t *testing.T) {
			d := decisionDeBash(t, dirLimpio(t), h.cmd, "bypassPermissions")
			if d.esDelGuard() {
				t.Errorf("HUECO CERRADO (buena noticia): %q ahora dispara [%s]. Movelo al corpus "+
					"destructivo y borralo de la lista de huecos del header.", h.cmd, d.motivo())
			}
		})
	}
}

// ─── paridad con el mirror de OpenCode ───────────────────────────────────────────
//
// Esta es la única defensa contra la divergencia que YA se sufrió: el commit-gate de
// opencode-sdd-gate.js quedó reducido a un chequeo de mtime mientras el de bash
// validaba el hash del árbol. Un guard que solo existe en un cliente no es un guard.
//
// Ojo con el alcance de esta red: compara DECISIONES sobre un corpus, así que un hueco
// COMPARTIDO por los dos clientes sale verde (hueco 9 del header).

// El driver ejercita el PLUGIN (tool.execute.before), no la función suelta: el guard
// puede estar perfecto y no estar cableado. Pasó exactamente eso durante este ticket —
// guardDestructivo existía y nadie lo llamaba, y un test sobre la función pura lo
// habría dado por bueno. Devuelve el MOTIVO (o "-"), no un booleano: dos clientes que
// disparan por razones distintas están divergiendo igual.
const driverParidad = `import { DomainSddGate } from "./guard.mjs"

let entrada = ""
process.stdin.setEncoding("utf8")
for await (const c of process.stdin) entrada += c

const plugin = await DomainSddGate({ directory: process.cwd() })
const antes = plugin["tool.execute.before"]
const salida = []
for (const cmd of JSON.parse(entrada)) {
  let r = "-"
  try {
    await antes({ tool: "bash", sessionID: "paridad" }, { args: { command: cmd } })
  } catch (e) {
    const m = String(e && e.message).match(/destructive-guard \(DOMAINSERV-222\) \[([a-z-]+)\]/)
    if (m) r = m[1]
  }
  salida.push(r)
}
process.stdout.write(salida.join("\n") + "\n")
`

// decisionesDeOpenCode evalúa el corpus contra el plugin del template JS. NO ejecuta
// ningún comando: le pasa cada uno al tool.execute.before y lee el motivo del guard.
func decisionesDeOpenCode(t *testing.T, cmds []string) []string {
	res, _ := decisionesDeOpenCodeConBypass(t, cmds, "")
	return res
}

// Devuelve además el HOME aislado para poder verificar que el bypass se consumió.
func decisionesDeOpenCodeConBypass(t *testing.T, cmds []string, bypassRazon string) ([]string, string) {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skipf("sin node no se puede verificar la paridad con el mirror de OpenCode: %v", err)
	}
	dir := t.TempDir()
	if bypassRazon != "" {
		estado := filepath.Join(dir, ".local", "state", "domain")
		if err := os.MkdirAll(estado, 0o700); err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(estado, "destructive-bypass-paridad")
		if err := os.WriteFile(p, []byte(bypassRazon+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// se copia como .mjs porque node trata .js como CJS salvo package.json type=module,
	// y el template es ESM (OpenCode lo carga con su propio loader)
	origen, err := os.ReadFile(filepath.Join("templates", "opencode-sdd-gate.js"))
	if err != nil {
		t.Fatal(err)
	}
	for nombre, contenido := range map[string][]byte{"guard.mjs": origen, "driver.mjs": []byte(driverParidad)} {
		if err := os.WriteFile(filepath.Join(dir, nombre), contenido, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	entrada, err := json.Marshal(cmds)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(node, "driver.mjs")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(string(entrada))
	// HOME aislado: el plugin resuelve STATE_DIR y las credenciales desde homedir(),
	// y no queremos que lea el marker de bypass ni el .env reales
	cmd.Env = append(os.Environ(), "HOME="+dir, "DOMAIN_VPS_URL=", "DOMAIN_API_KEY=")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("el guard de OpenCode no corrió: %v\n%s", err, out)
	}
	lineas := strings.Fields(strings.TrimSpace(string(out)))
	if len(lineas) != len(cmds) {
		t.Fatalf("el driver devolvió %d decisiones para %d comandos:\n%s", len(lineas), len(cmds), out)
	}
	return lineas, dir
}

// El mirror de OpenCode no tiene el par ask/deny (un plugin solo puede throw), así que
// su deny es SIEMPRE duro. Sin bypass de un solo uso quedaría insatisfacible.
func TestGuardDestructivo_OpenCode_BypassEsDeUnSoloUso(t *testing.T) {
	res, home := decisionesDeOpenCodeConBypass(t, []string{"rm -rf /app", "rm -rf /app"},
		"hay que rearmar /app en el container de QA")
	if res[0] != "-" {
		t.Errorf("con el bypass presente el primer borrado tiene que pasar, obtuve %q", res[0])
	}
	if res[1] != "radio" {
		t.Errorf("consumido el bypass, el segundo borrado tiene que volver a bloquearse, obtuve %q", res[1])
	}
	marker := filepath.Join(home, ".local", "state", "domain", "destructive-bypass-paridad")
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Error("el bypass no se consumió: habilitaría la sesión entera")
	}
}

// Mismo sabotaje que en bash: el mirror tampoco puede dejarse escribir su propio marker.
func TestGuardDestructivo_OpenCode_EscribirElPropioBypass_EstaBloqueado(t *testing.T) {
	res, _ := decisionesDeOpenCodeConBypass(t,
		[]string{"echo 'lo necesito' > ~/.local/state/domain/destructive-bypass-paridad"},
		"un borrado legítimo")
	if res[0] != "automarker" {
		t.Errorf("el mirror dejó que el agente se escriba el bypass: motivo=%q", res[0])
	}
}

func TestGuardDestructivo_ParidadBashVsOpenCode(t *testing.T) {
	var cmds []string
	for _, c := range comandosDestructivos {
		cmds = append(cmds, c.cmd)
	}
	for _, c := range comandosInofensivos {
		cmds = append(cmds, c.cmd)
	}
	js := decisionesDeOpenCode(t, cmds)
	for i, c := range cmds {
		d := decisionDeBash(t, dirLimpio(t), c, "bypassPermissions")
		bash := "-"
		if d.esDelGuard() {
			bash = d.motivo()
		}
		if bash != js[i] {
			t.Errorf("DIVERGENCIA en %q: bash=%s opencode=%s", c, bash, js[i])
		}
	}
}

// Red de contención para entornos sin node: si el alcance se amplía en un solo cliente,
// esto lo caza sin ejecutar JS. No reemplaza al test de arriba (solo mira los tokens
// que enumera), pero cierra el caso "node no está y la paridad quedó sin verificar".
func TestGuardDestructivo_ParidadEstatica_MismosTokensDeAlcance(t *testing.T) {
	fuentes := map[string]string{
		"hooks/domain-pre-edit.sh":       "",
		"templates/opencode-sdd-gate.js": "",
	}
	for f := range fuentes {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		fuentes[f] = string(b)
	}
	tokens := []string{
		"DOMAINSERV-222", "destructive-bypass-", "mysqlsh", "TRUNCATE",
		"radio-indecidible", "automarker", "sql-opaco",
		// radio de daño
		"/usr/src/app", "/workspace", "node_modules", "coverage", "index.lock",
		// alcance de secretos
		"id_(?:rsa|dsa|ecdsa|ed25519)", "key|pem|p12|pfx|jks", "ls-files",
		// SQL
		"OWNED\\s+BY", "MATERIALIZED\\s+VIEW", "DELETE\\s+FROM", "1\\s*=\\s*1",
		// envolturas y wrappers
		"kubectl", "setsid", "stdbuf", "ionice", "doas",
	}
	for f, contenido := range fuentes {
		for _, tk := range tokens {
			if !strings.Contains(contenido, tk) {
				t.Errorf("%s no cubre %q: los dos clientes tienen que cubrir el MISMO conjunto", f, tk)
			}
		}
	}
}

// El bypass del commit-gate NO es una llave maestra: autorizar UN commit no autoriza
// borrar el working tree.
func TestGuardDestructivo_BypassDelCommitGate_NoAutorizaBorrar(t *testing.T) {
	home := dirLimpio(t)
	dir := filepath.Join(home, ".local", "state", "domain")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gate-bypass-"+sesionGuard), []byte("razon\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if d := decisionDeBash(t, home, "rm -rf /app", "bypassPermissions"); !d.esDelGuard() {
		t.Errorf("el bypass del commit-gate abrió el guard destructivo: %q", d.Razon)
	}
}
