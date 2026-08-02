package main

import (
	"sort"
	"strings"
	"testing"
)

// DOMAINSERV-137: el installer distribuía UN agente nombrado a mano, así que los agentes
// nuevos del catálogo quedaban en el repo sin llegar al cliente. El catálogo se enumera
// desde templates/agents/ para que agregar un agente sea agregar un archivo.

func TestAgentCatalog_EnumeraCadaAgenteDelDirectorio(t *testing.T) {
	cat, err := agentCatalog()
	if err != nil {
		t.Fatalf("agentCatalog: %v", err)
	}

	esperados := []string{"domain-memory", "gherkin-verify", "git-archaeology", "knowledge-ingest", "policy-lookup", "repo-scout", "ticket-triage"}
	if len(cat) != len(esperados) {
		t.Fatalf("el catálogo tiene %d agentes, se esperaban %d: %v", len(cat), len(esperados), slugs(cat))
	}
	for i, quiero := range esperados {
		if cat[i].slug != quiero {
			t.Errorf("cat[%d].slug = %q, se esperaba %q (el orden debe ser estable)", i, cat[i].slug, quiero)
		}
	}
}

// El sufijo .opencode.md es una VARIANTE del mismo agente, no un agente aparte: los
// esquemas de frontmatter son incompatibles (ver embed.go). Tratarla como agente propio
// la instalaría en Claude Code, donde su `model: anthropic/...` y su `permission:` no
// significan nada.
func TestAgentCatalog_LaVarianteOpencodeNoEsUnAgentePropio(t *testing.T) {
	cat, err := agentCatalog()
	if err != nil {
		t.Fatalf("agentCatalog: %v", err)
	}

	for _, a := range cat {
		if strings.HasSuffix(a.slug, ".opencode") || strings.Contains(a.slug, ".opencode") {
			t.Errorf("la variante %q se enumeró como agente propio", a.slug)
		}
	}

	dm := buscar(t, cat, "domain-memory")
	if len(dm.opencode) == 0 {
		t.Fatal("domain-memory tiene variante .opencode.md en el repo pero el catálogo no la trae")
	}
	if !strings.Contains(string(dm.opencode), "mode: subagent") {
		t.Error("la variante de opencode no parece ser la de OpenCode")
	}
	if strings.Contains(string(dm.opencode), "disallowedTools:") {
		t.Error("la variante de opencode trae campos de Claude Code")
	}
}

// Un agente sin variante se instala SOLO en Claude Code. Reusar su template para OpenCode
// le daría un frontmatter malformado, que es peor que no instalarlo.
func TestAgentCatalog_AgenteSinVariante_NoInventaUnaParaOpencode(t *testing.T) {
	cat, err := agentCatalog()
	if err != nil {
		t.Fatalf("agentCatalog: %v", err)
	}

	// git-archaeology no lleva variante a propósito: su guard es un hook PreToolUse y en
	// OpenCode los hooks son plugins JS GLOBALES, no por agente, así que su restricción no
	// es expresable ahí. Se instala solo donde puede estar acotado.
	ga := buscar(t, cat, "git-archaeology")
	if len(ga.opencode) != 0 {
		t.Errorf("git-archaeology no debe tener variante de OpenCode: su guard no es expresable ahí (%d bytes)", len(ga.opencode))
	}
	if len(ga.claude) == 0 {
		t.Error("git-archaeology debe traer su template de Claude Code")
	}
}

// Dos agentes con el mismo `name` en el mismo directorio hacen que Claude Code cargue uno
// de los dos por orden de lectura del filesystem, sin precedencia documentada. Es un fallo
// silencioso: conviene detectarlo antes de copiar, no después.
func TestAgentCatalog_NombresDuplicados_SonUnError(t *testing.T) {
	cat := []agentTemplate{
		{slug: "uno", claude: []byte("---\nname: repetido\nmodel: haiku\n---\n")},
		{slug: "dos", claude: []byte("---\nname: repetido\nmodel: haiku\n---\n")},
	}

	err := validarNombresUnicos(cat)

	if err == nil {
		t.Fatal("dos agentes con el mismo name deben ser un error")
	}
	if !strings.Contains(err.Error(), "repetido") {
		t.Errorf("el error debe nombrar el name en conflicto, dice: %v", err)
	}
}

func TestAgentCatalog_NombresUnicos_NoEsError(t *testing.T) {
	if err := validarNombresUnicos([]agentTemplate{
		{slug: "uno", claude: []byte("---\nname: uno\n---\n")},
		{slug: "dos", claude: []byte("---\nname: dos\n---\n")},
	}); err != nil {
		t.Errorf("names distintos no deben dar error: %v", err)
	}
}

// El catálogo real tiene que cumplir el invariante, no solo el caso sintético.
func TestAgentCatalog_ElCatalogoRealNoTieneNombresDuplicados(t *testing.T) {
	cat, err := agentCatalog()
	if err != nil {
		t.Fatalf("agentCatalog: %v", err)
	}
	if err := validarNombresUnicos(cat); err != nil {
		t.Errorf("el catálogo del repo tiene names duplicados: %v", err)
	}
}

// Un agente sin `model` hereda el de la sesión, que es lo que DOMAINSERV-135 vino a
// arreglar: el caso de uso más mecánico corriendo en el modelo más caro.
func TestAgentCatalog_TodoAgenteDeclaraNameYModelo(t *testing.T) {
	cat, err := agentCatalog()
	if err != nil {
		t.Fatalf("agentCatalog: %v", err)
	}

	for _, a := range cat {
		fm := frontmatter(t, a.claude)
		for _, campo := range []string{"name:", "model:"} {
			if !strings.Contains(fm, campo) {
				t.Errorf("%s: falta %q en el frontmatter", a.slug, campo)
			}
		}
		if strings.Contains(fm, "model: fable") {
			t.Errorf("%s: el catálogo no usa fable (modelo-por-clase-de-tarea)", a.slug)
		}
		// El campo acepta TIERS, no IDs de API. Un ID pineado devuelve 404 el día que el
		// modelo se retira —le pasó a claude-3-7-sonnet-20250219 desde el 2026-02-19— y con
		// ~26 agentes serían 26 ediciones por cada lanzamiento. Va sobre TODO el catálogo:
		// el chequeo equivalente en agent_templates_test.go solo cubre domain-memory, y un
		// sabotaje sobre otro agente pasaba sin que nada lo notara.
		if strings.Contains(fm, "model: claude-") || strings.Contains(fm, "model: anthropic/") {
			t.Errorf("%s: model lleva TIER (haiku|sonnet|opus), no un ID pineado", a.slug)
		}
	}
}

// DOMAINSERV-206: knowledge-ingest es el ÚNICO agente del catálogo con una tool de
// ESCRITURA (domain_knowledge_save), y la regla que lo habilita es acotada: se delega
// escritura cuando el agente EJECUTA una decisión ya tomada, no cuando la TOMA. Lo que
// Claude Code hace cumplir es `tools`, así que la allowlist tiene que enumerar
// EXACTAMENTE lo permitido: una entrada de más es una escritura que nadie decidió.
func TestAgentCatalog_KnowledgeIngest_AllowlistEnumeraExactamenteLoPermitido(t *testing.T) {
	cat, err := agentCatalog()
	if err != nil {
		t.Fatalf("agentCatalog: %v", err)
	}

	ki := buscar(t, cat, "knowledge-ingest")
	declaradas := toolsDeclaradas(lineaDe(frontmatter(t, ki.claude), "tools:"))
	esperadas := map[string]bool{
		"mcp__domain-mcp__domain_knowledge_save": true,
		"Read":                                   true,
		"Glob":                                   true,
		"ToolSearch":                             true,
	}

	minima := sortedKeys(esperadas)
	for _, tool := range declaradas {
		if !esperadas[tool] {
			t.Errorf("la allowlist declara %q, fuera de la mínima del ticket: %v", tool, minima)
		}
		delete(esperadas, tool)
	}
	for _, faltante := range sortedKeys(esperadas) {
		t.Errorf("falta %q en la allowlist: sin ella el agente no puede completar su procedimiento", faltante)
	}
}

// "No basta con no listarlas": una tool omitida de `tools` queda fuera por efecto, pero
// nada documenta la intención ni frena a quien agregue una entrada más adelante. Las
// prohibidas van DENEGADAS de forma explícita, además de ausentes de la allowlist.
func TestAgentCatalog_KnowledgeIngest_LasProhibidasEstanDenegadasNoSoloOmitidas(t *testing.T) {
	cat, err := agentCatalog()
	if err != nil {
		t.Fatalf("agentCatalog: %v", err)
	}

	fm := frontmatter(t, buscar(t, cat, "knowledge-ingest").claude)
	permitidas := lineaDe(fm, "tools:")
	denegadas := lineaDe(fm, "disallowedTools:")

	// WebFetch/WebSearch entran a la lista por el requisito 3 del ticket: un agente que
	// ingesta contenido web y puede escribir convierte una página hostil en instrucción
	// PERSISTENTE, re-inyectada en toda sesión futura.
	for _, prohibida := range []string{
		"mcp__domain-mcp__domain_mem_save", "Write", "Edit", "NotebookEdit", "Bash",
		"WebFetch", "WebSearch",
	} {
		if strings.Contains(permitidas, prohibida) {
			t.Errorf("la allowlist incluye %q, que el ticket prohíbe", prohibida)
		}
		if !strings.Contains(denegadas, prohibida) {
			t.Errorf("%q no figura en disallowedTools: la prohibición queda implícita", prohibida)
		}
	}
}

// La contención de este agente ES su allowlist, y OpenCode SÍ la expresa por agente: las
// claves de `permission` se matchean como patrones contra el NOMBRE de la tool, MCP incluidas
// —la doc de OpenCode usa `"mymcp_*": "deny"` para denegar un server entero y `"mymcp_search":
// "ask"` para una sola—, y `"*"` cubre las no enumeradas. Por eso la variante no puede
// quedarse en el molde read-only del catálogo (edit/write/bash deny): ahí todo el server
// domain-mcp quedaría permitido y se distribuiría el único agente con escritura sin su guard.
func TestAgentCatalog_KnowledgeIngest_LaVarianteOpencodeAcotaIgualDeFuerte(t *testing.T) {
	cat, err := agentCatalog()
	if err != nil {
		t.Fatalf("agentCatalog: %v", err)
	}

	ki := buscar(t, cat, "knowledge-ingest")
	if len(ki.opencode) == 0 {
		t.Fatalf("knowledge-ingest no trae variante %s: la aceptación de DOMAINSERV-206 la pide", varianteOpencode)
	}
	perm := lineaDe(frontmatter(t, ki.opencode), "permission:")

	if !strings.Contains(perm, `"*": deny`) {
		t.Error(`la variante no arranca en default-deny ("*": deny): toda tool no enumerada queda permitida, incluido todo domain-mcp`)
	}
	esperadas := map[string]bool{
		"domain-mcp_domain_knowledge_save": true, "read": true, "glob": true,
	}
	minima := sortedKeys(esperadas)
	for _, tool := range clavesConAccion(perm, "allow") {
		if !esperadas[tool] {
			t.Errorf("permission permite %q, fuera de la mínima del ticket: %v", tool, minima)
		}
		delete(esperadas, tool)
	}
	for _, faltante := range sortedKeys(esperadas) {
		t.Errorf("falta %q permitida: sin ella el agente no puede completar su procedimiento", faltante)
	}

	// Mismo criterio que la variante de Claude Code: con default-deny las prohibidas ya quedan
	// fuera por efecto, pero nada documenta la intención ni frena a quien afloje el `"*"`.
	for _, prohibida := range []string{
		"domain-mcp_domain_mem_save", "write", "edit", "bash", "webfetch", "websearch",
	} {
		if !strings.Contains(perm, prohibida+": deny") {
			t.Errorf("%q no figura denegada de forma explícita en permission: la prohibición queda implícita en el default", prohibida)
		}
	}
}

// clavesConAccion devuelve las claves del bloque `permission:` cuyo valor es la acción dada.
// Verifica los permitidos como CONJUNTO: un Contains por nombre da verde con una entrada de
// más, que en el único agente con escritura del catálogo es justo el riesgo.
func clavesConAccion(bloque, accion string) []string {
	var out []string
	for _, l := range strings.Split(bloque, "\n") {
		clave, valor, ok := strings.Cut(strings.TrimSpace(l), ":")
		if !ok || strings.TrimSpace(valor) != accion {
			continue
		}
		out = append(out, strings.Trim(clave, `"`))
	}
	return out
}

// ausenciaRatificada son los agentes que a propósito NO llevan variante de OpenCode, con la
// razón. DOMAINSERV-206 entró con el template de Claude Code y sin la variante que su
// aceptación pide, y nada falló: la paridad no se puede dejar en que el próximo se acuerde.
var ausenciaRatificada = map[string]string{
	// su guard es un hook PreToolUse y en OpenCode los hooks son plugins JS GLOBALES, no por
	// agente, así que su restricción no es expresable ahí
	"git-archaeology": "guard de hook no expresable por agente",
}

func TestAgentCatalog_ParidadDeVariantes_TodaAusenciaEstaRatificada(t *testing.T) {
	cat, err := agentCatalog()
	if err != nil {
		t.Fatalf("agentCatalog: %v", err)
	}

	for _, a := range cat {
		razon, ratificada := ausenciaRatificada[a.slug]
		if len(a.opencode) == 0 && !ratificada {
			t.Errorf("%s no trae %s y la ausencia no está ratificada: se distribuye solo a Claude Code sin que nadie lo haya decidido", a.slug, varianteOpencode)
		}
		if len(a.opencode) != 0 && ratificada {
			t.Errorf("%s trae %s pero figura como ausencia ratificada (%s): la ratificación quedó obsoleta", a.slug, varianteOpencode, razon)
		}
		if len(a.claude) == 0 {
			t.Errorf("%s no trae su template de Claude Code", a.slug)
		}
	}
}

// El retorno es el ack, y solo el ack: si devuelve el texto del documento se pierde toda
// la ganancia que justifica el agente (~20k tokens del documento contra ~80 del ack).
func TestAgentCatalog_KnowledgeIngest_DeclaraElAckYProhibeElContenido(t *testing.T) {
	cat, err := agentCatalog()
	if err != nil {
		t.Fatalf("agentCatalog: %v", err)
	}

	cuerpo := string(buscar(t, cat, "knowledge-ingest").claude)
	for _, campo := range []string{"chunks", "ids"} {
		if !strings.Contains(cuerpo, campo) {
			t.Errorf("el formato de retorno no declara %q: el ack es lo único que este agente devuelve", campo)
		}
	}
	if !strings.Contains(cuerpo, "source_url") {
		t.Error("el template no nombra source_url: es el argumento por el que un documento de origen web entraría, y la prohibición tiene que ser sobre el mecanismo concreto")
	}
}

// DOMAINSERV-155: el plan de fan-out de sdd-verify existe desde el arranque del epic y
// caía en general-purpose, que hereda modelo y effort de la sesión. El agente que lo
// ejecuta no inventa su salida: cumple el contrato que el template de la fase ya define,
// y un campo que falte deja al orquestador sin cómo agregar los N lotes en un veredicto.
func TestAgentCatalog_GherkinVerify_DeclaraElContratoDeSddVerify(t *testing.T) {
	cat, err := agentCatalog()
	if err != nil {
		t.Fatalf("agentCatalog: %v", err)
	}

	cuerpo := string(buscar(t, cat, "gherkin-verify").claude)
	for _, campo := range []string{
		"scenarios_total", "scenarios_passed", "scenarios_failed",
		"scenarios_uncovered", "coverage_estimate", "verdict",
	} {
		if !strings.Contains(cuerpo, campo) {
			t.Errorf("el template no declara %q del contrato de sdd-verify: el orquestador no puede agregar los lotes", campo)
		}
	}
	// partial y no fail cuando falta cobertura: lo manda el prompt de la fase, y confundirlos
	// convierte un hueco de tests en un flow abortado.
	if !strings.Contains(cuerpo, "partial") {
		t.Error("el template no nombra el verdict partial: sin cobertura el veredicto no es fail")
	}
	// Es el agente que decide si algo pasó. Sin la sección donde declararlo, un lote sin
	// evidencia de test ejecutado se reporta igual que uno verificado.
	if !strings.Contains(cuerpo, "## Candidato a memoria") {
		t.Error("gherkin-verify descubre territorio nuevo: le falta la sección Candidato a memoria")
	}
}

// La decisión Bash/no-Bash fija el riesgo del agente: con Bash necesita un guard propio
// —como git-archaeology— porque `go test` compila y ejecuta código del repo. Este agente
// NO lo lleva: la suite la corre quien delega y le pasa la salida, así que el agente mapea
// escenarios a tests con Read/Grep/Glob y se queda read-only sin depender de ningún hook.
func TestAgentCatalog_GherkinVerify_ReadOnlySinBashYSinGuard(t *testing.T) {
	cat, err := agentCatalog()
	if err != nil {
		t.Fatalf("agentCatalog: %v", err)
	}

	gv := buscar(t, cat, "gherkin-verify")
	fm := frontmatter(t, gv.claude)
	permitidas := toolsDeclaradas(lineaDe(fm, "tools:"))
	esperadas := map[string]bool{"Read": true, "Grep": true, "Glob": true}
	minima := sortedKeys(esperadas)

	for _, tool := range permitidas {
		if !esperadas[tool] {
			t.Errorf("la allowlist declara %q, fuera de la mínima read-only: %v", tool, minima)
		}
		delete(esperadas, tool)
	}
	for _, faltante := range sortedKeys(esperadas) {
		t.Errorf("falta %q: sin ella el agente no puede mapear escenarios a tests", faltante)
	}
	if guards := guardsDeclarados(gv.claude); len(guards) != 0 {
		t.Errorf("gherkin-verify declara guards %v y no tiene Bash: un hook que no acota nada es deuda", guards)
	}
	// El agente reporta el veredicto del lote; el checkpoint de verificación lo escribe el
	// hilo principal, que es el único que ve los N lotes.
	if denegadas := lineaDe(fm, "disallowedTools:"); !strings.Contains(denegadas, "domain_verify_update_item") {
		t.Error("domain_verify_update_item debe estar denegado: el agente valida un lote, no cierra el checkpoint")
	}
}

// toolsDeclaradas parte una línea `tools:` (una sola o con continuación indentada) en los
// nombres que declara. Sirve para verificar la allowlist como CONJUNTO: un Contains por
// nombre da verde con entradas de más, que en un agente con escritura es justo el riesgo.
func toolsDeclaradas(linea string) []string {
	_, valor, _ := strings.Cut(linea, ":")
	var out []string
	for _, campo := range strings.FieldsFunc(valor, func(r rune) bool {
		return r == ',' || r == '\n' || r == ' ' || r == '\t'
	}) {
		if campo != "" && campo != "-" {
			out = append(out, campo)
		}
	}
	return out
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func slugs(cat []agentTemplate) []string {
	out := make([]string, 0, len(cat))
	for _, a := range cat {
		out = append(out, a.slug)
	}
	return out
}

func buscar(t *testing.T, cat []agentTemplate, slug string) agentTemplate {
	t.Helper()
	for _, a := range cat {
		if a.slug == slug {
			return a
		}
	}
	t.Fatalf("el catálogo no trae %q; trae %v", slug, slugs(cat))
	return agentTemplate{}
}

// DOMAINSERV-137: los invariantes de la variante valen para TODO el catálogo, no solo para
// domain-memory. Sin esto, la próxima variante puede divergir del original o mezclar
// esquemas sin que nada lo note.
func TestAgentCatalog_TodaVariante_MismoBodyYSinMezclarEsquemas(t *testing.T) {
	cat, err := agentCatalog()
	if err != nil {
		t.Fatalf("agentCatalog: %v", err)
	}

	body := func(tpl []byte) string {
		s := string(tpl)
		i := strings.Index(s[4:], "\n---")
		if i < 0 {
			return s
		}
		return strings.TrimSpace(s[4+i+4:])
	}

	conVariante := 0
	for _, a := range cat {
		if len(a.opencode) == 0 {
			continue
		}
		conVariante++

		if body(a.claude) != body(a.opencode) {
			t.Errorf("%s: el body de las dos variantes divergió, es el mismo agente", a.slug)
		}

		fmOpencode := frontmatter(t, a.opencode)
		for _, soloClaude := range []string{"effort:", "disallowedTools:", "tools:", "model: haiku"} {
			if strings.Contains(fmOpencode, soloClaude) {
				t.Errorf("%s: %q es de Claude Code y OpenCode no lo entiende", a.slug, soloClaude)
			}
		}
		if !strings.Contains(fmOpencode, "mode: subagent") {
			t.Errorf("%s: la variante de OpenCode necesita mode: subagent", a.slug)
		}
		if !strings.Contains(fmOpencode, "model: anthropic/") {
			t.Errorf("%s: OpenCode necesita el modelo como provider/model-id", a.slug)
		}

		fmClaude := frontmatter(t, a.claude)
		for _, soloOpencode := range []string{"mode:", "permission:", "temperature:"} {
			if strings.Contains(fmClaude, soloOpencode) {
				t.Errorf("%s: %q es de OpenCode y Claude Code no lo entiende", a.slug, soloOpencode)
			}
		}
	}

	if conVariante < 4 {
		t.Errorf("se esperaban al menos 4 agentes con variante, hay %d", conVariante)
	}
}
