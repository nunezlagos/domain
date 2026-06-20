<!--
Título del PR debe seguir Conventional Commits:
  feat(scope): description
  fix(scope): description
  docs(scope): description
  feat(scope)!: description (breaking)

Ver .claude/rules/git.md para detalles.
-->

## Resumen

<!-- 1-2 líneas qué hace este PR -->

## HU / Issue referenciado

- Refs: HU-XX.Y
- Closes: #NNN (si aplica)

## Tipo de cambio

- [ ] `feat` — nueva feature
- [ ] `fix` — bug fix
- [ ] `perf` — performance
- [ ] `refactor` — refactor sin cambio funcional
- [ ] `docs` — solo documentación
- [ ] `test` — tests
- [ ] `build` / `ci` / `chore`
- [ ] **Breaking change** — agrega `!` al tipo o `BREAKING CHANGE:` en commit body

## Checklist

- [ ] Commits siguen [Conventional Commits](.claude/rules/git.md)
- [ ] Tests agregados/actualizados para los cambios
- [ ] `make lint` pasa local
- [ ] `make test` pasa local
- [ ] CHANGELOG.md Unreleased actualizada (si feat/fix/breaking)
- [ ] Documentación actualizada si aplica
- [ ] Si toca schema DB: migration sigue convenciones (HU-25.3, HU-25.13)
- [ ] Si toca API: response shape valida (HU-13.9)
- [ ] Si introduce métrica/log/trace: respeta `.claude/rules/observability.md`

## Cómo probar

<!-- Steps para reproducir/validar manualmente, si aplica -->

```bash
# ejemplo
make dev-up
domain ...
```

## Notas para reviewer

<!-- Cosas que querés que miren con atención, decisiones non-obvias, alternativas consideradas -->
