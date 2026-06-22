"""Guard contra schema drift entre los models del admin y la BD viva.

PROBLEMA QUE RESUELVE: los models son managed=False en prod, pero el runner de
test los flipea a managed=True y crea el schema desde los PROPIOS models. Eso
significa que si un model declara una columna que NO existe en la BD real, los
tests igual PASAN (Django crea esa columna en la DB efimera). managed=True
NUNCA caza drift por si solo.

Este test cierra ese hueco: carga `real_schema.json` (snapshot de las columnas
reales de la BD viva) y, para CADA model de los apps de mantenedor, verifica
que toda columna declarada por el model exista en la tabla real. Si un model
declara una columna inexistente, falla con un mensaje claro.

Mantenimiento: cuando la BD real gana/pierde columnas, actualiza
real_schema.json (es la fuente de verdad de este guard).
"""
from __future__ import annotations

import json
from pathlib import Path

from django.apps import apps
from django.test import SimpleTestCase

# Apps de mantenedor cuyos models deben calzar con la BD real.
MAINTAINER_APPS = (
    "users",
    "projects",
    "apikeys",
    "agents",
    "skills",
    "flows",
    "crons",
    "prompts",
    "projectpolicies",
)

_SCHEMA_PATH = Path(__file__).resolve().parent / "real_schema.json"

# Tablas excluidas del guard. user_roles ya NO se excluye: el model UserRole se
# reconcilio (sin columna `id`, user como primary_key db_column user_id), asi que
# sus columnas calzan con la tabla real y el guard lo valida.
_SKIP_TABLES: set[str] = set()


def _load_real_schema() -> dict[str, set[str]]:
    with _SCHEMA_PATH.open(encoding="utf-8") as fh:
        raw = json.load(fh)
    return {table: set(cols) for table, cols in raw.items()}


class SchemaDriftTests(SimpleTestCase):
    """Cada columna declarada por un model debe existir en la tabla real."""

    @classmethod
    def setUpClass(cls):
        super().setUpClass()
        cls.real_schema = _load_real_schema()

    def _iter_models(self):
        for app_label in MAINTAINER_APPS:
            try:
                config = apps.get_app_config(app_label)
            except LookupError:
                continue
            for model in config.get_models():
                yield app_label, model

    def test_no_drift_columnas_inexistentes(self):
        problems: list[str] = []
        checked = 0

        for app_label, model in self._iter_models():
            db_table = model._meta.db_table
            # Saltar models cuya tabla no este en el snapshot o este excluida.
            if db_table not in self.real_schema or db_table in _SKIP_TABLES:
                continue

            checked += 1
            real_cols = self.real_schema[db_table]
            model_cols = [f.column for f in model._meta.concrete_fields]
            missing = [c for c in model_cols if c not in real_cols]

            if missing:
                problems.append(
                    f"[{app_label}.{model.__name__}] tabla '{db_table}' "
                    f"declara columnas inexistentes en la BD real: "
                    f"{sorted(missing)}. "
                    f"Columnas reales: {sorted(real_cols)}."
                )

        self.assertEqual(
            problems,
            [],
            "Schema drift detectado (model declara columnas que no existen en la "
            "BD viva):\n\n" + "\n\n".join(problems),
        )
        # Sanity: el test debe haber chequeado al menos un model, si no el
        # iterador/labels estan mal y el guard seria un falso verde.
        self.assertGreater(
            checked, 0,
            "El guard no chequeo ningun model: revisa MAINTAINER_APPS / labels.",
        )
