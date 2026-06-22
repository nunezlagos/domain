"""Test runner que flipea modelos managed=False -> True solo durante tests.

CANONICO: este es el runner oficial del proyecto. (El de users.tests.runner
queda por compatibilidad, pero apunta TEST_RUNNER aqui:
"core.tests.runner.ManagedModelTestRunner".)

En prod las tablas reales las administra domain-mcp, por eso los modelos son
managed=False y Django no las migra. En test necesitamos que el runner cree el
schema en la DB efimera, asi que las marcamos managed=True justo antes de crear
la DB de test (cuando el app registry ya esta cargado) y las restauramos al
terminar.
"""
from django.test.runner import DiscoverRunner


class ManagedModelTestRunner(DiscoverRunner):
    def setup_test_environment(self, **kwargs):
        from django.apps import apps

        self._unmanaged_models = [
            m for m in apps.get_models() if not m._meta.managed
        ]
        for m in self._unmanaged_models:
            m._meta.managed = True
        super().setup_test_environment(**kwargs)

    def teardown_test_environment(self, **kwargs):
        super().teardown_test_environment(**kwargs)
        for m in self._unmanaged_models:
            m._meta.managed = False
