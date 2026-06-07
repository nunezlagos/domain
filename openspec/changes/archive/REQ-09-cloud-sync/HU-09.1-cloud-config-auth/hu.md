# HU-09.1-cloud-config-auth

**Origen:** `REQ-09-cloud-sync`
**Prioridad:** alta
**Tipo:** feature

## Historia de usuario

**Como** usuario de memoria
**Quiero** configurar la conexión al cloud con `engram cloud config --server URL`
**Para** conectar mi instancia local con un servidor de sincronización remoto

**Como** usuario
**Quiero** autenticarme usando un token (ENGRAM_CLOUD_TOKEN)
**Para** que solo clientes autorizados puedan acceder a mi cloud

**Como** desarrollador en entorno local
**Quiero** poder desactivar la autenticación con ENGRAM_CLOUD_INSECURE_NO_AUTH
**Para** desarrollar y testear sin configurar tokens

## Criterios de aceptación

```gherkin
Scenario: Configurar server URL via CLI
  Given no hay configuración cloud previa
  When se ejecuta `engram cloud config --server https://cloud.memoria.dev`
  Then la URL se persiste en cloud.json
  And el server URL es accesible desde GetCloudServer()

Scenario: Leer token de variable de entorno
  Given ENGRAM_CLOUD_TOKEN está seteado en "sk-abc123"
  When se ejecuta GetCloudToken()
  Then retorna "sk-abc123"

Scenario: Leer token de cloud.json si no hay env var
  Given ENGRAM_CLOUD_TOKEN no está seteado
  And cloud.json tiene token "sk-file-token"
  When se ejecuta GetCloudToken()
  Then retorna "sk-file-token"

Scenario: Insecure no auth mode
  Given ENGRAM_CLOUD_INSECURE_NO_AUTH=1
  When se ejecuta cualquier llamada cloud
  Then no se requiere token en los requests
  And se loguea un warning de seguridad

Scenario: cloud.json se crea en el directorio de config
  Given se ejecuta `engram cloud config --server URL`
  When se persiste la configuración
  Then cloud.json existe en $ENGRAM_CONFIG_DIR o ~/.config/engram/
  And contiene: {server, token?, insecure_no_auth?}

Scenario: Token no se muestra en logs ni en output
  Given cloud.json contiene un token
  When se imprime la configuración
  Then el token debe aparecer como "***" en el output
  And no debe aparecer en logs

Scenario: Configurar sin flags muestra config actual
  Given cloud.json tiene server configurado
  When se ejecuta `engram cloud config` sin flags
  Then mustra la configuración actual (con token oculto)

Scenario: Múltiples configs no solapan
  Given cloud.json tiene server A configurado
  When se configura server B
  Then cloud.json debe contener server B (reemplaza)
  And otras keys deben persistir
```

## Análisis breve

- **Qué pide realmente:** Comando `engram cloud config`, archivo `cloud.json`, lectura de env vars ENGRAM_CLOUD_TOKEN y ENGRAM_CLOUD_INSECURE_NO_AUTH, sanitización de token en output/logs
- **Módulos sospechados:** `internal/cloud/config.go`, `internal/cli/cloud.go`
- **Riesgos / dependencias:** Token en disco requiere permisos 0600; warning de seguridad en modo insecure
- **Esfuerzo tentativo:** S

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
- **Evidencia:** —
- **Acción derivada:** —
