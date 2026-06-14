# issue-03.6-dedup-privacy

**Origen:** `REQ-03-memory-system`
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario
**Como** agente de IA
**Quiero** evitar duplicados mediante hash normalizado SHA-256 de (project_id+scope+type+title+content) con rolling window, y eliminar contenido marcado como privado
**Para** mantener la base de conocimiento limpia y respetar la privacidad de datos sensibles

## Criterios de aceptación

```gherkin
Feature: Deduplication and Privacy

  Background:
    Given el sistema de memoria tiene habilitada la deduplicación
    And existe la tabla observation_hashes

  Scenario: Detectar duplicado exacto por hash normalizado
    When guardo una observación con title="Fix login" y content="Se corrigió bug"
    And intento guardar la misma observación nuevamente
    Then el sistema detecta que el hash SHA-256 ya existe
    And rechaza el duplicado con error ErrDuplicateObservation
    And devuelve la observación original

  Scenario: Rolling window de deduplicación
    Given existen 1000 observaciones recientes con hashes indexados
    When guardo una observación duplicada de hace 900 observaciones
    Then el rolling window (últimas 1000) detecta el duplicado
    And si la observación duplicada está fuera del window (> 1000 atrás), se permite

  Scenario: Hash normalizado ignora whitespace y capitalización
    When guardo "Fix Login" y otro guarda "fix  login"
    Then ambos producen el mismo hash normalizado
    And el sistema los detecta como duplicados

  Scenario: Stripping de contenido privado
    When guardo una observación con contenido que contiene "<private>datos sensibles</private>"
    Then el contenido se almacena sin la porción privada
    And el tag <private>...</private> se elimina completamente
    And se guarda un registro de que se omitió contenido privado

  Scenario: Múltiples bloques privados
    When el contenido tiene "<private>token</private> y <private>password</private>"
    Then ambos bloques privados se eliminan
    And solo queda " y "

  Scenario: Defense in depth - validación en múltiples capas
    Given la capa de aplicación tiene hash check
    And la base de datos tiene unique constraint sobre hash
    When intento insertar un duplicado bypassando la app
    Then la constraint unique de la DB rechaza el insert
```

## Análisis breve

- **Qué pide realmente:** Hash SHA-256 normalizado de (project+scope+type+title+content) como fingerprint. Rolling window de últimas N observaciones. Pre-processing: normalizar whitepace, lowercasing. Privacy tag stripping con regex `<private>.*?</private>`.
- **Módulos sospechados:** `internal/memory/dedup.go`, `internal/memory/privacy.go`
- **Riesgos / dependencias:** Bajo. No depende de otras HUs. Defense in depth requiere unique constraint en DB.
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
- **Evidencia:**
- **Acción derivada:**
