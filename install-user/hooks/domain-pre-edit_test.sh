#!/usr/bin/env bash
# Test del gate PreToolUse (domain-pre-edit.sh). Black-box: alimenta payloads por
# stdin con un HOME aislado (sin markers de flow) y verifica el permissionDecision.
# Regresión DOMAINSERV-103: payload no-parseable con python3 presente → fail-closed
# (deny), no fail-open (exit 0 sin decisión).
set -uo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
HOOK="$SCRIPT_DIR/domain-pre-edit.sh"
FAKE_HOME="$(mktemp -d)"
# cwd aislado para el guard destructivo (DOMAINSERV-222): mide el radio de daño contra
# el cwd de la sesión, y no queremos que los casos dependan del directorio real.
FAKE_CWD="$(mktemp -d)"
trap 'rm -rf "$FAKE_HOME" "$FAKE_CWD"' EXIT

FAILS=0
# run <payload> → imprime stdout del hook con HOME aislado
run() { printf '%s' "$1" | HOME="$FAKE_HOME" bash "$HOOK" 2>/dev/null; }

# normaliza espacios: json.dumps (emit_decision) mete "k": "v"; los echo compactos
# usan "k":"v". Comparamos sin espacios para tolerar ambos formatos.
nospace() { printf '%s' "$1" | tr -d ' '; }

check_contains() { # descripción, esperado-substring, actual
  if nospace "$3" | grep -qF -- "$(nospace "$2")"; then
    printf 'PASS: %s\n' "$1"
  else
    printf 'FAIL: %s (esperaba contener %q, obtuve %q)\n' "$1" "$2" "$3"; FAILS=$((FAILS + 1))
  fi
}
check_not_contains() { # descripción, prohibido-substring, actual
  if nospace "$3" | grep -qF -- "$(nospace "$2")"; then
    printf 'FAIL: %s (NO debía contener %q, obtuve %q)\n' "$1" "$2" "$3"; FAILS=$((FAILS + 1))
  else
    printf 'PASS: %s\n' "$1"
  fi
}

# 1) payload corrupto (no JSON) con python3 presente → fail-closed deny (DOMAINSERV-103)
out="$(run 'esto no es json {{{')"
check_contains "payload corrupto -> deny" '"permissionDecision":"deny"' "$out"
check_contains "payload corrupto -> razón DOMAINSERV-103" 'DOMAINSERV-103' "$out"

# 2) payload JSON válido, Edit sin flow, modo default → ask del gate normal,
#    NO el deny de payload corrupto
valid='{"session_id":"test-sess-103","tool_name":"Edit","permission_mode":"default","tool_input":{"file_path":"/tmp/x.go"}}'
out="$(run "$valid")"
check_contains "JSON válido sin flow (default) -> ask" '"permissionDecision":"ask"' "$out"
check_not_contains "JSON válido no dispara el deny de payload corrupto" 'DOMAINSERV-103' "$out"

# 3) COMMIT-GATE micro (DOMAINSERV-108): flow marker con field3=micro → el
#    commit se exime del requisito de tests (no debe emitir el deny del gate).
mkdir -p "$FAKE_HOME/.local/state/domain"
printf 'faketoken\t2099-01-01T00:00:00+00:00\tmicro\n' > "$FAKE_HOME/.local/state/domain/flow-micro-sess"
commit_micro='{"session_id":"micro-sess","tool_name":"Bash","permission_mode":"acceptEdits","tool_input":{"command":"git commit -m \"fix: texto\""}}'
out="$(run "$commit_micro")"
check_not_contains "commit-gate: micro exento (no exige tests)" 'commit-gate' "$out"

# 4) COMMIT-GATE no-micro (DOMAINSERV-74 intacto): flow marker con field3=express
#    y sin marker tests-ok → deny en modo automático.
printf 'faketoken\t2099-01-01T00:00:00+00:00\texpress\n' > "$FAKE_HOME/.local/state/domain/flow-exp-sess"
commit_exp='{"session_id":"exp-sess","tool_name":"Bash","permission_mode":"acceptEdits","tool_input":{"command":"git commit -m \"feat: x\""}}'
out="$(run "$commit_exp")"
check_contains "commit-gate: express NO exento -> deny sin tests-ok" 'commit-gate' "$out"

# 5) DOMAINSERV-111: Edit/Write sobre archivo NO-código (doc/nota) → sin gate.
#    El gate solo cubre CÓDIGO; hasta ahora la rama Edit/Write no miraba la
#    extensión y bloqueaba un .md, mientras el mismo archivo vía Bash heredoc
#    pasaba (la rama Bash sí filtra por code_ext). Asimetría corregida.
for nc in /tmp/notas.md /tmp/consultas.txt /home/u/.claude/memory/x.md /tmp/salida.log /tmp/data.csv; do
  p="{\"session_id\":\"nc-sess\",\"tool_name\":\"Write\",\"permission_mode\":\"acceptEdits\",\"tool_input\":{\"file_path\":\"$nc\"}}"
  check_not_contains "no-código pasa sin gate: $nc" '"permissionDecision"' "$(run "$p")"
done

# 6) DOMAINSERV-111 (contra-prueba): el código SIGUE gateado en Edit/Write.
for c in /tmp/x.go /repo/svc.py /repo/q.sql /repo/Dockerfile /repo/Makefile /repo/deploy.tf; do
  p="{\"session_id\":\"c-sess\",\"tool_name\":\"Write\",\"permission_mode\":\"acceptEdits\",\"tool_input\":{\"file_path\":\"$c\"}}"
  check_contains "código sigue gateado: $c" '"permissionDecision":"deny"' "$(run "$p")"
done

# 7) DOMAINSERV-111: git-guard NO bloquea subcomandos read-only de stash.
for ro in 'git stash list' 'git stash show -p' 'git stash list --oneline'; do
  p="{\"session_id\":\"ro-sess\",\"tool_name\":\"Bash\",\"permission_mode\":\"acceptEdits\",\"tool_input\":{\"command\":\"$ro\"}}"
  check_not_contains "git-guard permite read-only: $ro" 'git-guard' "$(run "$p")"
done

# 8) DOMAINSERV-111 (contra-prueba): el stash MUTANTE sigue gateado.
#    DOMAINSERV-278: se afirma la DECISIÓN, no solo que el guard opinó. Buscar 'git-guard'
#    en la razón es un falso verde — al reclasificar `stash pop` de deny a ask la razón
#    sigue diciendo git-guard, así que el chequeo pasaba con el comportamiento cambiado.
while IFS='|' read -r mu esperada; do
  p="{\"session_id\":\"mu-sess\",\"tool_name\":\"Bash\",\"permission_mode\":\"acceptEdits\",\"tool_input\":{\"command\":\"$mu\"}}"
  out="$(run "$p")"
  check_contains "git-guard: $mu -> $esperada" "\"permissionDecision\":\"$esperada\"" "$out"
done <<'MUS'
git stash|ask
git stash push -m wip|ask
git stash pop|ask
git -C /repo stash|ask
git stash drop|deny
git stash clear|deny
git worktree remove wt|ask
git worktree remove --force wt|deny
git worktree remove wt --force|deny
MUS

# 9) DOMAINSERV-111: el git-guard no debe disparar por MENCIONES dentro de un
#    literal (mensaje de commit, heredoc, string). Un mensaje que documenta el
#    guard bloqueaba el propio commit que lo arreglaba.
# payload <session> <command> → JSON bien formado (el escapado a mano corrompe
# el JSON y el hook responde el deny de DOMAINSERV-103, no lo que se testea)
payload() {
  # DOMAINSERV-222: el guard destructivo mide el RADIO contra el cwd de la sesión, así
  # que el payload lo declara. Va a un temp aislado (no al repo) para que `rm -rf .`
  # decida siempre igual, corran los tests desde donde corran.
  S="$1" C="$2" W="${3:-$FAKE_CWD}" python3 -c '
import json, os
print(json.dumps({"session_id": os.environ["S"], "tool_name": "Bash",
                  "permission_mode": "acceptEdits", "cwd": os.environ["W"],
                  "tool_input": {"command": os.environ["C"]}}))'
}

hd='git commit -F - <<MSG
fix: git stash pop y git reset --hard siguen denegados
MSG'
check_not_contains "git-guard ignora menciones en heredoc" 'git-guard' \
  "$(run "$(payload lit-sess "$hd")")"
check_not_contains "git-guard ignora menciones en -m entrecomillado" 'git-guard' \
  "$(run "$(payload lit-sess 'git commit -m "fix: git reset --hard ya no hace falta"')")"

# 10) DOMAINSERV-111 (contra-prueba): el strip de literales NO debe abrir un
#     bypass. Un intérprete que EJECUTA el literal sigue siendo destructivo.
# DOMAINSERV-278: se afirma la decisión esperada por comando. El punto del bloque es que un
# intérprete no abre bypass, y eso se sostiene igual con la clasificación nueva: `restore`
# es recuperable y cae en ask, pero el guard lo ve — lo que no puede pasar es que se cuele
# sin decisión.
while IFS='|' read -r ev esperada; do
  check_contains "bypass por intérprete sigue gateado: $ev -> $esperada" \
    "\"permissionDecision\":\"$esperada\"" "$(run "$(payload ev-sess "$ev")")"
done <<'EVS'
bash -c "git reset --hard"|deny
sh -c 'git clean -fd'|deny
eval "git restore ."|ask
EVS
check_contains "destructivo real sigue bloqueado" 'git-guard' \
  "$(run "$(payload ev-sess 'git reset --hard HEAD~1')")"
# el literal se reemplaza por un token, NO se borra: vaciarlo dejaría
# `git -C  reset --hard` y el normalizador de opciones se comería "reset"
check_contains "path entrecomillado no abre bypass" 'git-guard' \
  "$(run "$(payload ev-sess 'git -C "/tmp/mi repo" reset --hard')")"

# 11) DOMAINSERV-146 — FALSO POSITIVO DEL COMMIT-GATE. El repro exacto del reporte:
#     un grep cuyo PATRÓN contiene el literal "git commit". 100% read-only y quedaba
#     bloqueado, porque el commit-gate matcheaba sobre la línea entera. Es el mismo
#     linaje que DOMAINSERV-114 (arriba) pero en la rama C, que había quedado sin el
#     tratamiento de literales-como-datos.
while IFS= read -r ro; do
  check_not_contains "commit-gate NO dispara en read-only: $ro" 'commit-gate' \
    "$(run "$(payload ro-sess "$ro")")"
done <<'ROS'
grep -nE "commit|git commit|tests-ok" install-user/templates/opencode-sdd-gate.js
rg 'git commit' install-user/hooks/
echo "para commitear corré: git commit -m msg"
awk '/git commit/ {print}' historial.txt
git log --oneline --grep="git commit"
ROS

# El primer token manda: un git NO-commit tampoco dispara el gate de commits.
check_not_contains "commit-gate NO dispara con git status" 'commit-gate' \
  "$(run "$(payload ro-sess 'git status --porcelain')")"

# 12) SABOTAJE del fix anterior: el commit REAL sigue bloqueado sin marker de tests,
#     incluidas las formas que la normalización de prefijos debe seguir viendo.
while IFS= read -r real; do
  check_contains "commit real SIGUE bloqueado: $real" 'commit-gate' \
    "$(run "$(payload real-sess "$real")")"
done <<'REALS'
git commit -m "fix: algo"
cd /tmp/repo && git commit -m x
git -C /tmp/repo commit -m x
/usr/bin/git commit --no-verify -m x
GIT_AUTHOR_NAME=x git commit -m y
sudo git commit -m x
sh -c "git commit -m x"
bash -c 'git commit -am x'
eval "git commit -m x"
REALS

# 13) DOMAINSERV-146 — FALSO POSITIVO DEL GATE SDD. Un heredoc que solo LEE no es
#     edición: el patrón exigía apenas una extensión de código en cualquier parte
#     después del marcador, así que mencionar un .js o un .sh alcanzaba.
ssh_ro="ssh domain-vps 'bash -s' <<'REMOTE'
grep -c retention_stream /opt/services/services/domain-mcp/deploy/monitoring/loki-config.yml
md5sum /opt/services/services/install.sh
wget -qO- http://localhost/healthz
REMOTE"
check_not_contains "gate SDD NO dispara con heredoc read-only" '"permissionDecision"' \
  "$(run "$(payload ssh-sess "$ssh_ro")")"

# 14) SABOTAJE del 13: un heredoc que SÍ escribe código sigue gateado. Si esto pasara,
#     el fix habría abierto el agujero que el patrón laxo tapaba por accidente.
#     Los casos van en un array y NO en un heredoc leído con `read -r`: cada comando es
#     multilínea, y leerlo línea por línea partía el heredoc en tres payloads sueltos
#     que ninguno era un heredoc completo. El test daba rojo por eso, no por el hook.
escrituras=(
  "cat > internal/x.go <<'EOF'
package main
EOF"
  "cat <<'EOF' > internal/x.go
package main
EOF"
  "tee install-user/hooks/x.sh <<'EOF'
echo hola
EOF"
  "python3 - <<'PY'
open(\"internal/x.go\", \"w\").write(\"package main\")
PY"
)
for wr in "${escrituras[@]}"; do
  check_contains "heredoc que ESCRIBE sigue gateado: ${wr%%$'\n'*}" '"permissionDecision"' \
    "$(run "$(payload wr-sess "$wr")")"
done

# 15) DOMAINSERV-195: el deny del commit-gate tiene que nombrar la ruta del marker
#     de bypass. Sin eso el gate es insatisfacible en modos automáticos: el usuario
#     aprueba "commiteá igual" y esa aprobación nunca llega a ofrecerse como prompt.
printf 'faketoken\t2099-01-01T00:00:00+00:00\texpress\n' > "$FAKE_HOME/.local/state/domain/flow-msg-sess"
out="$(run "$(payload msg-sess 'git commit -m "feat: x"')")"
check_contains "commit-gate: el deny nombra el marker de bypass" 'gate-bypass-' "$out"

# 16) DOMAINSERV-195: con el marker de bypass presente el commit pasa...
printf 'faketoken\t2099-01-01T00:00:00+00:00\texpress\n' > "$FAKE_HOME/.local/state/domain/flow-byp-sess"
printf 'suite exige VPN, contenido ya testeado aguas arriba\n' > "$FAKE_HOME/.local/state/domain/gate-bypass-byp-sess"
out="$(run "$(payload byp-sess 'git commit -m "feat: x"')")"
check_not_contains "commit-gate: con bypass el commit pasa" 'commit-gate' "$out"

# 17) ...y el marker se CONSUME: un solo uso, no una sesión entera abierta.
if [ -e "$FAKE_HOME/.local/state/domain/gate-bypass-byp-sess" ]; then
  printf 'FAIL: el bypass no se consumió (el marker sigue existiendo)\n'; FAILS=$((FAILS + 1))
else
  printf 'PASS: el bypass se consumió al usarlo\n'
fi

# 18) SABOTAJE del 16: consumido el bypass, el segundo commit de la misma sesión
#     vuelve a rechazarse. Si esto pasara, un bypass habilitaría la sesión entera.
out="$(run "$(payload byp-sess 'git commit -m "feat: otro"')")"
check_contains "commit-gate: segundo commit sin bypass vuelve a denegar" 'commit-gate' "$out"

# 19) SABOTAJE: el bypass es SOLO del commit-gate, no una llave maestra. Con el
#     marker presente, escribir código sin flow activo sigue gateado.
printf 'razon\n' > "$FAKE_HOME/.local/state/domain/gate-bypass-sab-sess"
sab='{"session_id":"sab-sess","tool_name":"Write","permission_mode":"acceptEdits","tool_input":{"file_path":"/tmp/x.go"}}'
check_contains "el bypass NO habilita escribir código" '"permissionDecision":"deny"' "$(run "$sab")"

# 20) DOMAINSERV-222 — GUARD DESTRUCTIVO, rediseñado por RADIO DE DAÑO.
#     EL INCIDENTE, textual: el usuario corrió `docker exec docker-ace-did-app rm -f
#     .env.qa` creyendo que borraba una copia DENTRO del container; /app era un bind
#     mount del repo host, así que se llevó el original. Fue `rm -f` (NO -rf) y llegó por
#     `docker exec` (rm NO era el primer token).
#     EL CRITERIO NUEVO (decisión del usuario): el guard NO mira el flag, mira el radio.
#     `rm -f archivo` salió del alcance — medía 265 de 334 disparos sobre 21.105 comandos
#     reales. Alcance y HUECOS CONOCIDOS: ../hook_destructive_guard_test.go.
out="$(run "$(payload inc-sess 'docker exec docker-ace-did-app rm -f .env.qa')")"
check_contains "destructive-guard: el incidente dispara" 'destructive-guard' "$out"
check_contains "destructive-guard: el incidente es de clase sensible" '[sensible]' "$out"

# El alcance acordado, caso por caso. Nada de esto se ejecuta: es el PAYLOAD del hook.
# El formato es "<motivo esperado>|<comando>": el motivo también se verifica, porque dos
# implementaciones que disparan por razones distintas ya están divergiendo.
while IFS= read -r linea; do
  motivo="${linea%%|*}"; destr="${linea#*|}"
  out="$(run "$(payload destr-sess "$destr")")"
  check_contains "destructive-guard [$motivo] bloquea: $destr" "[$motivo]" "$out"
done <<'DESTRS'
radio|rm -rf .
radio|rm -rf ./*
radio|rm -rf ..
radio|rm -rf /
radio|rm -rf $HOME
radio|rm -rf ~
radio|rm -rf /app
radio|rm -rf /srv
radio|rm -rf /var/www
radio|rm -rf /usr/src/app
radio|rm -rf .git
radio|rm -R .
radio|rm --recursive .
radio|docker exec docker-ace-did-app "rm -rf /repo"
radio|ssh vps-domain 'rm -rf /srv/domain'
radio|kubectl exec pod -- "rm -rf /data"
radio|docker run --rm -v /repo:/app alpine rm -rf /app
radio|sudo -u www-data rm -rf /var/www
radio|timeout 5 rm -rf /app
radio|nice -n 10 rm -rf /app
radio|/usr/bin/env rm -rf /app
radio|setsid rm -rf /app
radio|if [ -d x ]; then rm -rf /app; fi
radio|for f in a b; do rm -rf /app; done
radio|'rm' -rf /app
radio|"/bin/rm" -rf /app
radio|find . -exec rm -rf {} +
radio|find . -delete
radio|sh -c "rm -rf /"
radio|eval "rm -rf $HOME"
radio|D=/; rm -rf $D
radio|H=$HOME; rm -rf $H
radio|rm -rf $PWD # PWD=tmp
radio|echo PWD=tmp; rm -rf $PWD
radio|PWD=tmp rm -rf $PWD
radio|F=-rf; rm $F .
radio|F=-rf; rm $F /
radio|rm -rf ~+
radio|rm -rf .g*
radio|rm -rf .*
radio|rm -rf ./.g*
radio|rm -rf .gi?
radio|rm -rf .?*
radio|rm -rf .gi[t]
radio|rm -rf {.git,dist}
radio|rm -rf /ap*
radio|(rm -rf .)
radio|docker-compose exec app rm -rf /app
radio|podman exec c rm -rf /app
radio|sudo -u www-data rm -rf /srv/domain
radio|rm -rf /var/www/html
radio|eval rm -rf .
radio|eval rm -rf /
radio|eval rm -rf $PWD
radio|nice eval rm -rf .
radio|sudo eval rm -rf /srv/domain
radio-indecidible|rm -rf $DIR
radio-indecidible|rm -rf $D; D=/tmp/x
radio-indecidible|rm -rf ${DIR:-/}
radio-indecidible|rm -rf ~ubuntu/app
radio-indecidible|rm -rf ~-
sensible|eval rm -f .env.qa
sql|eval psql -c "DROP TABLE observations"
sensible|rm -f .env.qa
sensible|rm deploy/server.pem
sensible|rm config/id_rsa
sensible|rm secrets/app-credentials.json
sql|psql -c "DROP TABLE observations"
sql|docker exec pg psql -U postgres -c "DROP DATABASE domain"
sql|mysql -e "DROP SCHEMA app"
sql|psql -c "TRUNCATE observations"
sql|psql -c "DELETE FROM observations"
sql|psql -c "DROP/**/TABLE observations"
sql|psql -c "DELETE FROM observations WHERE true"
sql|psql -c "ALTER TABLE observations DROP COLUMN body"
sql|echo 'DROP TABLE observations' | psql
sql-opaco|psql -f migrate.sql
sql-opaco|psql < dump.sql
DESTRS

# el heredoc con terminador INDENTADO (<<-SQL con tabs) evadía el enmascarado
sql_hd='psql -U domain <<-SQL
	TRUNCATE observations;
	SQL'
check_contains "destructive-guard: heredoc con terminador indentado" '[sql]' \
  "$(run "$(payload destr-sess "$sql_hd")")"

# TERCERA RONDA. Estos tres no entran en el heredoc de arriba porque su quoting ES el
# caso: van con el comando armado a mano.
#   1. colisión de apóstrofos: el pareo de comillas simples se comía el rm del medio, así
#      que el comando pasaba ENTERO.
apostrofos='echo "it'"'"'s" && rm -rf . && echo "that'"'"'s"'
check_contains "destructive-guard: apóstrofos vecinos no esconden el rm" '[radio]' \
  "$(run "$(payload destr-sess "$apostrofos")")"
#   2. comilla doble ANIDADA con escape: el literal interno quedaba invisible (era el
#      hueco 6 del header, ahora cerrado por el escáner izquierda-a-derecha).
anidada='docker exec pg sh -c "psql -c \"DROP TABLE observations\""'
check_contains "destructive-guard: comilla doble anidada con escape" '[sql]' \
  "$(run "$(payload destr-sess "$anidada")")"
#   3. el marker de bypass con el path ENTERO en una variable: el literal
#      destructive-bypass no está en la línea y el chequeo no lo veía.
marker_var='M=$HOME/.local/state/domain/destructive-bypass-x; echo razon > $M'
check_contains "destructive-guard: el marker por variable sigue siendo automarker" \
  '[automarker]' "$(run "$(payload destr-sess "$marker_var")")"
#   Y su contra-prueba: el UPDATE con WHERE real dentro de comillas anidadas NO es
#   destructivo. El pareo viejo lo partía y disparaba: 2 falsos positivos medidos sobre
#   11.358 comandos reales.
where_ok='docker exec db sh -c '"'"'mysql -u a d -e "UPDATE users SET role=\"x\" WHERE id=1;"'"'"''
check_not_contains "destructive-guard NO dispara: UPDATE con WHERE anidado" \
  'destructive-guard' "$(run "$(payload dro-sess "$where_ok")")"

# 21) CONTRA-PRUEBAS del 20. Si alguna falla, el guard es RUIDO: el humano lo aprueba
#     por reflejo y deja de proteger. Las primeras son el linaje DOMAINSERV-114 + 146
#     (el literal viaja DENTRO del patrón de un comando read-only); las del medio son la
#     clase que producía el 79% de los disparos medidos y que SALIÓ del alcance.
while IFS= read -r ro; do
  check_not_contains "destructive-guard NO dispara: $ro" 'destructive-guard' \
    "$(run "$(payload dro-sess "$ro")")"
done <<'DROS'
grep "rm -rf /" install-user/hooks/domain-pre-edit.sh
rg -n "DROP TABLE" services/domain-mcp/internal/migrate/migrations/
git log -S "rm -rf"
grep -n 'rm -f' install-user/hooks/domain-pre-edit.sh
grep -rn "destructive-bypass" install-user/
psql -c "DELETE FROM observations WHERE id = 1"
psql -c "UPDATE observations SET body = 'x' WHERE id = 1"
psql -c "SELECT count(*) FROM observations"
echo "SELECT 1" | psql
mysql -f -e "SELECT 1"
rm -f $SOCK
rm -f /tmp/domain.sock
rm -f coverage.out
rm notas.md
rm -rf node_modules
rm -rf dist
rm -rf build
rm -rf vendor
rm -rf target
rm -rf .next
rm -rf ./dist
rm -rf coverage
rm -rf .git/index.lock
rm -rf /tmp/build
rm -rf $HOME/.claude
rm -rf ~/.cache/go-build
rm -rf "$BUILD_DIR/dist"
WT=/tmp/claude/wt; rm -rf "$WT"
d=$(mktemp -d); cd $d; rm -rf $d
SB=/tmp/sab; rm -rf $SB; mkdir -p $SB
docker ps --format '{{.Names}}' | grep -i mysql
which mysql 2>/dev/null || echo no-mysql
printf 'select 1' | ssh host 'psql -f -'
rm -f .env.example
rm /tmp/no-secret.txt
docker rm docker-ace-did-app
docker run --rm alpine echo hola
docker run --rm -v /repo:/app alpine rm -rf /app/dist
docker exec ctr rm -rf node_modules
ssh host "ls -la /srv"
ls -la /usr/bin/rm
find . -name "*.log" | xargs rm -f
find . -type d -name node_modules -exec rm -rf {} +
for f in *.log; do rm -f $f; done
rm -rf node_modules/*
rm -rf dist/*
rm -rf coverage*
rm -rf ~/.cache/go-build/*
rm -rf /tmp/build/*
rm -f *.log
rm -f coverage.*
rm -rf ?
rm -rf ..?*
ls -la # ojo con rm -rf /
DROS

# El propio hook contiene `rm -f "$bypass"` para consumir el marker del commit-gate: un
# commit que DOCUMENTE este guard no puede auto-bloquearse. El git-guard cayó en esa
# trampa exacta (ver su comentario en el bloque A).
check_not_contains "destructive-guard ignora el rm que documenta el commit" 'destructive-guard' \
  "$(run "$(payload dro-sess 'git commit -m "feat(guard): bloquea rm -rf / y DROP TABLE"')")"
msg_hd='git commit -F - <<MSG
feat: el guard corta rm -rf / y TRUNCATE
MSG'
check_not_contains "destructive-guard ignora menciones en heredoc" 'destructive-guard' \
  "$(run "$(payload dro-sess "$msg_hd")")"
# DOMAINSERV-114 en su forma mas filosa: el cuerpo del heredoc ARRANCA con el comando.
# Sin el enmascarado de literales, el salto de linea parte el heredoc en segmentos y esa
# linea se lee como un comando a ejecutar.
msg_hd2='git commit -F - <<MSG
rm -rf / ya no se corre a mano: lo hace el installer
MSG'
check_not_contains "destructive-guard ignora un mensaje que ARRANCA con rm -rf" 'destructive-guard' \
  "$(run "$(payload dro-sess "$msg_hd2")")"
# terminador de heredoc INDENTADO: DOCUMENTAR el guard con tabs se auto-bloqueaba
doc_hd='cat >> notas.md <<-DOC
	rm -rf / es lo que el guard bloquea
	DOC'
check_not_contains "destructive-guard ignora un doc con heredoc indentado" 'destructive-guard' \
  "$(run "$(payload dro-sess "$doc_hd")")"
# el heredoc ESCRIBE un rm en un script; escribirlo no es ejecutarlo (sí lo gatea el SDD)
esc_hd="cat > script.sh <<'EOF'
rm -rf /
EOF"
check_not_contains "destructive-guard ignora un rm escrito por heredoc" 'destructive-guard' \
  "$(run "$(payload dro-sess "$esc_hd")")"

# 22) DOMAINSERV-222 — el mecanismo. acceptEdits es INTERACTIVO (shift+tab, hay humano al
#     teclado): ahí el ask SÍ llega a una persona y el deny sería un muro sin salida. El
#     deny duro queda SOLO para bypassPermissions. Un modo DESCONOCIDO cae en ask: un modo
#     nuevo de Claude Code no puede volverse un deny mudo.
modo_payload() {
  S="$1" M="$2" C="$3" W="$FAKE_CWD" python3 -c '
import json, os
print(json.dumps({"session_id": os.environ["S"], "tool_name": "Bash",
                  "permission_mode": os.environ["M"], "cwd": os.environ["W"],
                  "tool_input": {"command": os.environ["C"]}}))'
}
for modo in default plan acceptEdits modoNuevoDeClaude; do
  check_contains "destructive-guard: $modo -> ask" '"permissionDecision":"ask"' \
    "$(run "$(modo_payload mod-sess "$modo" 'rm -rf /app')")"
done
check_contains "destructive-guard: bypassPermissions -> deny" '"permissionDecision":"deny"' \
  "$(run "$(modo_payload mod-sess bypassPermissions 'rm -rf /app')")"
check_contains "destructive-guard: el deny nombra el marker de bypass" 'destructive-bypass-' \
  "$(run "$(payload mod-sess 'rm -rf /app')")"

# 23) Bypass de UN SOLO USO: el deny en bypassPermissions es duro, así que sin esto el
#     guard quedaría insatisfacible (mismo criterio que DOMAINSERV-195).
printf 'hay que rearmar /app en el container de QA\n' > "$FAKE_HOME/.local/state/domain/destructive-bypass-dbyp-sess"
check_not_contains "destructive-guard: con bypass el borrado pasa" 'destructive-guard' \
  "$(run "$(payload dbyp-sess 'rm -rf /app')")"
if [ -e "$FAKE_HOME/.local/state/domain/destructive-bypass-dbyp-sess" ]; then
  printf 'FAIL: el bypass destructivo no se consumió\n'; FAILS=$((FAILS + 1))
else
  printf 'PASS: el bypass destructivo se consumió al usarlo\n'
fi
check_contains "destructive-guard: segundo borrado vuelve a bloquearse" 'destructive-guard' \
  "$(run "$(payload dbyp-sess 'rm -rf /app')")"

# 24) SABOTAJE 1: el bypass del commit-gate NO es llave maestra. Autorizar UN commit no
#     autoriza borrar el working tree.
printf 'razon\n' > "$FAKE_HOME/.local/state/domain/gate-bypass-cross-sess"
check_contains "el bypass del commit-gate no abre el guard destructivo" 'destructive-guard' \
  "$(run "$(payload cross-sess 'rm -rf /app')")"

# 24b) SABOTAJE 2 (el más grave, verificado antes del fix): el agente se escribía SU
#      PROPIO marker con un `echo >` y el rm -rf siguiente pasaba. El deny duro era
#      decorativo contra un agente. Autorizar tiene que costar una intervención HUMANA.
auto_marker="$FAKE_HOME/.local/state/domain/destructive-bypass-auto-sess"
out="$(run "$(payload auto-sess "echo 'lo necesito' > $auto_marker")")"
check_contains "el agente no puede escribirse el bypass" '[automarker]' "$out"
if [ -e "$auto_marker" ]; then
  printf 'FAIL: el hook creó el marker de bypass\n'; FAILS=$((FAILS + 1))
else
  printf 'PASS: el marker de bypass no se creó\n'
fi
# y un bypass VIVO no habilita renovarse: un OK humano no puede volverse una sesión
printf 'un borrado legítimo\n' > "$auto_marker"
check_contains "un bypass vivo no habilita renovarse" '[automarker]' \
  "$(run "$(payload auto-sess "echo 'otro mas' > $auto_marker")")"
if [ -e "$auto_marker" ]; then
  printf 'PASS: el bypass no se consumió en una operación RECHAZADA\n'
else
  printf 'FAIL: el bypass se consumió aunque la operación fue rechazada\n'; FAILS=$((FAILS + 1))
fi
rm -f "$auto_marker"

# 25) Defensa en profundidad: el guard va ANTES del early-exit por flow, igual que el
#     git-guard. Un flow SDD activo autoriza EDITAR, no borrar.
printf 'faketoken\t2099-01-01T00:00:00+00:00\tlite\n' > "$FAKE_HOME/.local/state/domain/flow-flw-sess"
check_contains "destructive-guard aplica incluso con flow activo" 'destructive-guard' \
  "$(run "$(payload flw-sess 'rm -rf /app')")"

# 26) El guard es de BASH. Las migraciones del repo llevan DROP TABLE IF EXISTS en su
#     .down.sql por policy: gatear el contenido de un Write dejaría el repo inmantenible.
mig='{"session_id":"mig-sess","tool_name":"Write","permission_mode":"acceptEdits","tool_input":{"file_path":"/repo/migrations/000222_x.down.sql","content":"DROP TABLE IF EXISTS observations;\n"}}'
check_not_contains "destructive-guard no se mete en un Write de migración" 'destructive-guard' \
  "$(run "$mig")"

# 26b) El archivo que ejecuta un cliente SQL se LEE antes de escalar: escalar a ciegas
#      costaba 31 disparos medidos (probar migraciones en un container de descarte) y
#      leerlo además ATRAPA el DROP que el archivo trae adentro.
printf 'SELECT count(*) FROM observations;\n' > "$FAKE_CWD/limpio.sql"
printf 'BEGIN;\nDROP TABLE observations;\nCOMMIT;\n' > "$FAKE_CWD/sucio.sql"
check_not_contains "sql: archivo legible y benigno no escala" 'destructive-guard' \
  "$(run "$(payload sqlf-sess 'psql -f limpio.sql')")"
check_contains "sql: archivo legible con DROP dispara [sql]" '[sql]' \
  "$(run "$(payload sqlf-sess 'psql < sucio.sql')")"
check_contains "sql: archivo ilegible escala a [sql-opaco]" '[sql-opaco]' \
  "$(run "$(payload sqlf-sess 'psql -f no-existe.sql')")"

# 27) El radio se mide contra el cwd REAL de la sesión: el MISMO comando dispara o no
#     según dónde esté parada. Sin el cwd del payload el guard no puede decidir esto.
sub_cwd="$FAKE_CWD/sub/hondo"
mkdir -p "$sub_cwd"
check_contains "radio: un ancestro del cwd dispara" '[radio]' \
  "$(run "$(payload radio-sess "rm -rf $FAKE_CWD" "$sub_cwd")")"
check_not_contains "radio: un hijo del cwd no dispara" 'destructive-guard' \
  "$(run "$(payload radio-sess 'rm -rf artefactos' "$sub_cwd")")"

if [[ "$FAILS" -gt 0 ]]; then
  printf '\n%d test(s) FALLARON\n' "$FAILS"; exit 1
fi
printf '\nTodos los tests pasaron\n'
