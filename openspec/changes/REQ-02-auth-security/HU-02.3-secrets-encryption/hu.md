# HU-02.3-secrets-encryption

**Origen:** `REQ-02-auth-security`
**Persona:** security-officer
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** administrador de la plataforma
**Quiero** que todos los secrets almacenados (API keys de LLM, webhook secrets, tokens) se encripten automáticamente con AES-256-GCM al escribir y se desencripten al leer
**Para** proteger credenciales de terceros incluso si la base de datos se ve comprometida

## Criterios de aceptación

### Escenario 1: Encriptación automática al crear un secret

```gherkin
Dado que existe una clave de encriptación configurada en DOMAIN_ENCRYPTION_KEY
Cuando creo un secret con valor `sk-llm-secret-12345`
Entonces el valor almacenado en la columna `encrypted_value` NO es texto plano
Y el valor almacenado es un string binario en Base64
Y el valor almacenado incluye el nonce (12 bytes) + ciphertext + auth tag
Y el campo `encryption_algorithm` es `aes-256-gcm`
```

### Escenario 2: Desencriptación automática al leer un secret

```gherkin
Dado que existe un secret almacenado encriptado
Cuando leo el secret desde la API
Entonces el valor devuelto está desencriptado
Y coincide con el valor original antes de encriptar
```

### Escenario 3: Error con clave inválida

```gherkin
Dado que un secret fue encriptado con la clave A
Y la aplicación se reinicia con la clave B (diferente)
Cuando intento leer el secret
Entonces recibo un error de desencriptación
Y la aplicación no crasha, solo falla esa operación
```

### Escenario 4: Key rotation — re-encriptar

```gherkin
Dado que existen secrets encriptados con la clave anterior
Cuando ejecuto el comando `domain secrets re-encrypt`
Entonces todos los secrets se leen con la clave anterior
Y se re-encriptan con la nueva clave
Y después de la operación, todos los secrets son legibles con la nueva clave
```

### Escenario 5: AES-256-GCM nonce único

```gherkin
Dado que encripto el mismo valor dos veces seguidas
Entonces los ciphertexts son diferentes (nonce aleatorio)
Y ambos se desencriptan correctamente al valor original
```

## Análisis breve

- **Qué pide realmente:** Capa de encriptación transparente AES-256-GCM para secrets. Encrypt en write, decrypt en read. Rotación de clave. Nonce aleatorio por operación.
- **Módulos sospechados:** `internal/crypto/`, `internal/secrets/`, `internal/config/`
- **Riesgos / dependencias:** La clave de encriptación viene de DOMAIN_ENCRYPTION_KEY (mín 32 bytes = 256 bits). Si se pierde la clave, los secrets son irrecuperables. El nonce debe ser único por operación con la misma clave.
- **Esfuerzo tentativo:** M

## Verificación previa

- [ ] Revisar codebase (grep)
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
