from django.db import connection


class ApiKeyRepository:
    def encrypt_and_store_ciphertext(self, api_key_id: str, plaintext: str, enc_key: str) -> None:
        with connection.cursor() as cursor:
            cursor.execute(
                "UPDATE auth_api_keys SET key_ciphertext = pgp_sym_encrypt(%s, %s) "
                "WHERE id = %s",
                [plaintext, enc_key, str(api_key_id)],
            )

    def decrypt_ciphertext(self, api_key_id: str, enc_key: str) -> str | None:
        with connection.cursor() as cursor:
            cursor.execute(
                "SELECT pgp_sym_decrypt(key_ciphertext, %s)::text "
                "FROM auth_api_keys WHERE id = %s",
                [enc_key, str(api_key_id)],
            )
            row = cursor.fetchone()
            return row[0] if row else None

    def has_ciphertext(self, api_key_id: str) -> tuple[bool, str | None]:
        with connection.cursor() as cursor:
            cursor.execute(
                "SELECT key_ciphertext IS NOT NULL, key_plaintext FROM auth_api_keys "
                "WHERE id = %s",
                [str(api_key_id)],
            )
            row = cursor.fetchone()
            if row is None:
                return False, None
            has_ciphertext, plaintext = row
            return bool(has_ciphertext), plaintext


_repository = ApiKeyRepository()


def get_repository() -> ApiKeyRepository:
    return _repository
