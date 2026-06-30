from typing import Protocol


class ApiKeyServiceInterface(Protocol):
    def list_api_keys(
        self,
        search: str = "",
        page: int = 1,
        per_page: int = 20,
        user_id=None,
        status=None,
    ) -> dict: ...

    def get_api_key_plaintext(self, api_key_id: str) -> str | None: ...
