# Proposal: HU-01.5-project-merge

## Intención

Resolver el problema de identidad de proyectos: cuando un repo cambia, cuando dos proyectos se superponen, o cuando querés compartir memorias entre proyectos. Domain debe poder (1) detectar el proyecto por git remote, (2) mergear proyectos completos con resolución de conflictos, (3) permitir cross-project references de solo lectura, y (4) relocalizar un proyecto cuando el repo cambia.

## Scope

**Incluye:**
- `domain project detect`: detecta proyecto por git remote
- `domain project merge --from X --to Y`: migra todas las entidades con resolución de conflictos
- `domain project relocate --new-repo URL`: actualiza repository_url
- `domain project link --project X`: enlaza proyecto como dependencia de lectura
- `domain project links`: lista enlaces
- Tabla `project_links` (project_id, linked_project_id, access_level)
- Tabla `project_merges` (source_project_id, target_project_id, merge_log)
- Modificar `domain_mem_search` para incluir resultados de proyectos linkeados
- Backup automático pre-merge (snapshot de project_merges)
- Flag `--dry-run` en merge para preview

**Excluye:**
- Cross-project writes (solo read por ahora)
- Merge automático por scheduling
- Revert de merge (se puede hacer restore desde backup)

## Enfoque técnico

1. **Detect**: leer `git remote get-url origin`, buscar en projects.repository_url
2. **Merge**: transacción que migra observations (con dedup), skills (rename si conflicto), flows (rename), crons (rename), agents (rename). Todo en una TX.
3. **Link**: insert en project_links, modificar search queries con UNION de proyectos linkeados
4. **Relocate**: UPDATE projects SET repository_url WHERE id = X
5. **Backup**: antes de merge, copiar datos relevantes a project_merges.merge_log

## Riesgos

| Riesgo | Impacto | Mitigación |
|--------|---------|------------|
| Merge falla a mitad | Alto | Todo en una TX; si falla, rollback completo |
| Datos duplicados post-merge | Medio | Dedup por hash SHA-256, rename de conflictos |
| Cross-project search es lento | Bajo | Indexar project_id, limitar a N proyectos linkeados |
| Repositorio con múltiples remotes | Bajo | Usar `origin` por defecto, flag `--remote` para override |

## Testing

- **Unitarios**: merge logic con store mockeado, conflict detection
- **Integración**: merge real de dos proyectos con datos, verificar resultados
- **Regression**: merge con datos duplicados, confirmar que se omiten
- **Cross-project**: search desde proyecto "hijo" incluye resultados del "padre"
- **Sabotaje**: merge sin permisos de escritura → error claro
