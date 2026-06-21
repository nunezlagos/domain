"""Test runner que flipea modelos managed=False → True solo durante tests.

En prod las tablas (users/roles/user_roles) las administra domain-mcp, por eso
los modelos son managed=False y Django no las migra. En test necesitamos que el
runner cree el schema en la DB efímera, así que las marcamos managed=True justo
antes de crear la DB de test (cuando el app registry ya está cargado).
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
