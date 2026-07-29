"""Guard: el portal no puede publicar un cero que nadie midio (DOMAINSERV-174).

`_build_portal_ctx` hardcodeaba `"calls": 0, "success": 100` para TODA skill y
`charts.js:45-49` suma esas mismas claves, asi que el dashboard entero daba cero
con aire de medicion. La fuente real es `skill_metrics_daily`, que escribe el cron
`SkillMetricsAggregator` del MCP; sin fila agregada la respuesta es None y la
tabla renderiza un guion.

Unitario sobre la funcion pura + inspeccion del JS: no toca la DB. La tabla
`skill_metrics_daily` no existe en la DB efimera del runner (solo crea los modelos
declarados), asi que un test de integracion aca no probaria nada.
"""
from __future__ import annotations

import unittest
from pathlib import Path

from config.views import _SIN_MEDICION, _skill_metrics_rows_to_map

_DEMO_JS = Path(__file__).resolve().parent.parent.parent / "static" / "demo" / "js"
_VIEWS_JS = _DEMO_JS / "views.js"
_PORTAL_JS = _DEMO_JS / "portal.js"


class PortalMetricasRealesTests(unittest.TestCase):
    def test_rows_to_map_con_invocaciones_contables_devuelve_la_tasa_real(self):
        m = _skill_metrics_rows_to_map([("s1", 10, 7, 3)])
        self.assertEqual(m["s1"]["calls"], 10)
        self.assertEqual(m["s1"]["success"], 70.0)

    def test_rows_to_map_sin_invocaciones_contables_no_afirma_cero_por_ciento(self):
        m = _skill_metrics_rows_to_map([("s2", 0, 0, 0)])
        self.assertEqual(m["s2"]["calls"], 0, "0 calls SI es una medicion real")
        self.assertIsNone(
            m["s2"]["success"],
            "sin exitos ni fallos no hay tasa: 0% afirmaria que todas fallaron",
        )

    def test_skill_sin_fila_agregada_queda_sin_medicion(self):
        self.assertIsNone(_SIN_MEDICION["calls"])
        self.assertIsNone(_SIN_MEDICION["success"])

    def test_la_tabla_de_skills_no_renderiza_la_metrica_cruda(self):
        js = _VIEWS_JS.read_text(encoding="utf-8")
        self.assertNotIn(
            "<td>${row.calls}</td><td>${row.success}%</td>",
            js,
            "el valor crudo imprime 'null' cuando no hay medicion",
        )
        self.assertIn("fmtMetric(row.calls)", js)
        self.assertIn("fmtMetric(row.success, '%')", js)

    def test_el_modal_no_reinyecta_metricas_al_editar_una_fila(self):
        # el spread `{...editingItem, ...newItem}` pone newItem segundo, asi que
        # cualquier metrica que traiga pisa la que el cron midio: guardar el modal
        # sin cambiar nada devolvia la fila a 0 calls y 100% de exito
        js = _PORTAL_JS.read_text(encoding="utf-8")
        self.assertNotIn("success: 100", js, "el 100% inventado volvio al modal")
        self.assertNotIn("calls: 0, success", js)

