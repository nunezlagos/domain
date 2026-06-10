# issue-18.2-s3-replication

**Origen:** `REQ-18-backup-dr`
**Prioridad tentativa:** alta
**Tipo:** infrastructure

## Historia de usuario

**Como** operador
**Quiero** replicación cross-region del bucket S3 principal con versionado y lifecycle
**Para** sobrevivir caídas regionales y borrados accidentales sin pérdida de attachments/exports

## Criterios de aceptación

### Escenario 1: Versioning activado

```gherkin
Dado que el bucket `domain-assets` existe
Cuando aplico Terraform/CloudFormation
Entonces el bucket tiene `Versioning: Enabled`
Y `MFA Delete: Disabled` (solo en buckets críticos)
Y bloqueo de public ACL `Block all public access: On`
```

### Escenario 2: Cross-Region Replication

```gherkin
Dado que existe bucket destino `domain-assets-dr` en otra región
Cuando subo un objeto a `domain-assets`
Entonces el objeto aparece en `domain-assets-dr` en <15 minutos
Y la métrica `domain_s3_replication_lag_seconds` se publica
```

### Escenario 3: Lifecycle policies

```gherkin
Dado que existe lifecycle rule sobre `domain-assets`
Cuando un objeto cumple condiciones
Entonces transitions configuradas:
  | edad         | acción                              |
  | 30 días      | mover a S3 Infrequent Access        |
  | 90 días      | mover a S3 Glacier Instant Retrieval|
  | 365 días     | mover a Glacier Deep Archive        |
  | versiones    | expirar versiones >90 días          |
```

### Escenario 4: Soft-delete vía versioning

```gherkin
Dado que un attachment fue borrado
Cuando consulto versiones previas con `aws s3api list-object-versions`
Entonces la versión previa sigue existiendo (delete marker)
Y puede restaurarse eliminando el delete marker
```

## Análisis breve

- **Qué pide:** Terraform/CloudFormation declarando bucket policies + replication + lifecycle
- **Esfuerzo:** S
- **Riesgos:** Costo de replicación cross-region; versiones acumuladas si lifecycle mal configurado
