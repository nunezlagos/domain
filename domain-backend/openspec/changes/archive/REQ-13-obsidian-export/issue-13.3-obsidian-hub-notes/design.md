# Design: issue-13.3-obsidian-hub-notes

## Decisión arquitectónica

### Session hub note generation

```go
type SessionHubFrontmatter struct {
    Type      string   `yaml:"type"`
    SessionID string   `yaml:"session_id"`
    Project   string   `yaml:"project,omitempty"`
    Directory string   `yaml:"directory,omitempty"`
    Tags      []string `yaml:"tags"`
    CreatedAt string   `yaml:"created_at"`
    EndedAt   string   `yaml:"ended_at,omitempty"`
}

func GenerateSessionHubs(ctx context.Context, reader StoreReader, vaultPath string, slugMap map[int64]string) error {
    sessions, err := reader.ListSessions(ctx)
    if err != nil { return err }

    for _, session := range sessions {
        observations, err := reader.ListObservationsBySession(ctx, session.ID)
        if err != nil { return err }
        if len(observations) == 0 { continue }

        frontmatter := SessionHubFrontmatter{
            Type:      "session-hub",
            SessionID: session.ID,
            Project:   session.Project,
            Directory: session.Directory,
            Tags:      []string{"session", session.Project},
            CreatedAt: session.CreatedAt,
            EndedAt:   session.EndedAt,
        }

        body := buildSessionBody(session, observations, slugMap)
        note := Note{Frontmatter: frontmatter, Body: body}
        content, _ := note.Render()

        dir := filepath.Join(vaultPath, "_sessions")
        os.MkdirAll(dir, 0755)
        os.WriteFile(filepath.Join(dir, session.ID+".md"), []byte(content), 0644)
    }
    return nil
}
```

### Session body structure

```md
# Session: {session.ID}

| Campo       | Valor          |
|-------------|----------------|
| **Proyecto** | {project}     |
| **Directorio** | {directory} |
| **Inicio**   | {created_at}  |
| **Fin**      | {ended_at}    |
| **Observaciones** | {count} |

## Observaciones

- [[observations/{slug1}]]
- [[observations/{slug2}]]
- [[observations/{slug3}]]
```

### Topic hub note generation

```go
type TopicHubFrontmatter struct {
    Type         string   `yaml:"type"`
    TopicPrefix  string   `yaml:"topic_prefix"`
    Project      string   `yaml:"project,omitempty"`
    ObsCount     int      `yaml:"observation_count"`
    SessionCount int      `yaml:"session_count"`
    Tags         []string `yaml:"tags"`
}

func GenerateTopicHubs(ctx context.Context, reader StoreReader, vaultPath string, slugMap map[int64]string) error {
    // 1. Listar todas las observaciones con topic_key no nulo
    observations, err := reader.ListObservations(ctx, ObservationFilter{})
    if err != nil { return err }

    // 2. Agrupar por topic_key
    topics := make(map[string][]Observation)
    for _, obs := range observations {
        if obs.TopicKey == "" { continue }
        topics[obs.TopicKey] = append(topics[obs.TopicKey], obs)
    }

    // 3. Solo topics con >= 2 observaciones
    for topicKey, group := range topics {
        if len(group) < 2 { continue }

        sessions := make(map[string]bool)
        for _, obs := range group {
            sessions[obs.SessionID] = true
        }

        frontmatter := TopicHubFrontmatter{
            Type:         "topic-hub",
            TopicPrefix:  topicKey,
            ObsCount:     len(group),
            SessionCount: len(sessions),
            Tags:         []string{"topic", topicKey},
        }

        body := buildTopicBody(topicKey, group, slugMap)
        note := Note{Frontmatter: frontmatter, Body: body}
        content, _ := note.Render()

        dir := filepath.Join(vaultPath, "_topics")
        os.MkdirAll(dir, 0755)
        os.WriteFile(filepath.Join(dir, topicKey+".md"), []byte(content), 0644)
    }
    return nil
}
```

### Topic body structure (agrupado por type)

```md
# Topic: {topicKey}

| Campo | Valor |
|-------|-------|
| **Observaciones** | {count} |
| **Sesiones** | {sessionCount} |

## fix
- [[observations/{slug1}]] — Bug en login
- [[observations/{slug2}]] — Fix timezone

## feat
- [[observations/{slug3}]] — Nueva feature auth
```

### Integración con Export pipeline

Al final de `Export()`, después de las observaciones individuales:

```go
if opts.IncludeHubNotes {
    if err := GenerateSessionHubs(ctx, reader, vaultPath, slugMap); err != nil {
        return fmt.Errorf("session hubs: %w", err)
    }
    if err := GenerateTopicHubs(ctx, reader, vaultPath, slugMap); err != nil {
        return fmt.Errorf("topic hubs: %w", err)
    }
}
```

Se agrega flag `--include-hub-notes` (default `true`).

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| Dataview queries en vez de hub notes estáticos | Dataview requiere plugin adicional y procesamiento runtime; las hub notes estáticas funcionan en cualquier vault |
| Un solo archivo con todas las sesiones | Archivos separados permiten wikilinks directos desde/hacia cada sesión |
| Topic hubs con todos los topics (incluyendo < 2 obs) | Generaría ruido; el threshold de 2 asegura clusters significativos |

## TDD plan

1. **Red:** `TestSessionHub` — sesión con 2 obs → crea _sessions/{id}.md con wikilinks → falla
2. **Green:** Implementar GenerateSessionHubs → pasa
3. **Red:** `TestSessionHubFrontmatter` — verificar campos YAML → falla
4. **Green:** Implementar frontmatter → pasa
5. **Red:** `TestTopicHub` — topic con 3 obs → crea _topics/{key}.md → falla
6. **Green:** Implementar GenerateTopicHubs → pasa
7. **Red:** `TestTopicHubThreshold` — topic con 1 obs → no crea archivo → falla
8. **Green:** Implementar threshold check → pasa
9. **Red:** `TestTopicHubGroupByType` — obs agrupadas por type en body → falla
10. **Green:** Implementar grouping → pasa
11. **Sabotaje:** No chequear threshold → topic con 1 obs genera archivo → test cae → restaurar

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Session ID con caracteres especiales en filename | IDs son generados por engram (UUID o similar); no deberían tener caracteres problemáticos |
| Topic key muy largos | Se usan tal cual; si hay issues, slugificar el filename pero mantener el prefix en frontmatter |
| Muchos hubs lentos en vaults grandes | La generación es O(n) sobre observaciones; aceptable para miles |
