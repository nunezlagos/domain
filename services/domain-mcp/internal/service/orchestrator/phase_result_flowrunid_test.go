package orchestrator

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"
)

// DOMAINSERV-181: el hook post-orchestrate del cliente mintea el token del gate
// SDD a partir del flow_run_id del response. Un PhaseResultResult que se
// devuelva sin poblarlo deja al agente sin poder editar con el flow running, y
// la falla es MUDA: el step se cierra bien, el flow sigue vivo, y recién se nota
// cuando el gate deniega la siguiente edición.
//
// Es un test de fuente porque el defecto es "alguien agrega un return y se
// olvida del campo": eso no lo atrapa un test de comportamiento sobre los
// caminos que hoy existen.
func TestPhaseResultResult_TodoRetorno_PoblaElFlowRunID(t *testing.T) {
	fset := token.NewFileSet()
	archivo, err := parser.ParseFile(fset, "phase_result.go", nil, 0)
	require.NoError(t, err)

	var literales int
	ast.Inspect(archivo, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		ident, ok := lit.Type.(*ast.Ident)
		if !ok || ident.Name != "PhaseResultResult" {
			return true
		}
		literales++
		for _, elt := range lit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			if k, ok := kv.Key.(*ast.Ident); ok && k.Name == "FlowRunID" {
				return true
			}
		}
		t.Errorf("PhaseResultResult en %s no pobla FlowRunID: el hook del gate no podrá mintear el token y el agente quedará sin poder editar",
			fset.Position(lit.Pos()))
		return true
	})

	require.NotZero(t, literales, "no se encontró ningún PhaseResultResult: el test dejó de cubrir lo que dice cubrir")
}

// El campo tiene que serializar en snake_case: el hook busca "flow_run_id", y
// sin el tag Go lo emitiría como "FlowRunID" — que es exactamente el defecto que
// este ticket corrige.
func TestPhaseResultResult_FlowRunID_SerializaEnSnakeCase(t *testing.T) {
	fset := token.NewFileSet()
	archivo, err := parser.ParseFile(fset, "phase_result.go", nil, 0)
	require.NoError(t, err)

	var encontrado bool
	ast.Inspect(archivo, func(n ast.Node) bool {
		campo, ok := n.(*ast.Field)
		if !ok || len(campo.Names) != 1 || campo.Names[0].Name != "FlowRunID" {
			return true
		}
		encontrado = true
		require.NotNil(t, campo.Tag, "FlowRunID sin tag json: saldría como \"FlowRunID\" y el hook no lo lee")
		require.Contains(t, campo.Tag.Value, `json:"flow_run_id"`)
		return false
	})
	require.True(t, encontrado, "no existe el campo FlowRunID en PhaseResultResult")
}
