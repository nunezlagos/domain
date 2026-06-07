# Proposal: HU-14.1-cli-core-commands

## Intención

Implementar una CLI completa usando Cobra con comandos para todas las entidades. Estructura `domain {entity} {action} [flags]`. Cliente HTTP configurable que se comunica con la API REST de memoria.

## Scope

**Incluye:**
- Comando raíz `domain` con ayuda general
- Comandos de entidad: memory, skill, agent, flow, cron, config
- Subcomandos CRUD: save (create), list, get, delete, search
- Comandos de acción: run, execute
- Cliente HTTP genérico contra API REST
- Config file: `~/.config/domain/config.yaml` (endpoint, api_key, format)
- Flags globales: `--api-endpoint`, `--output`, `--verbose`
- Manejo de errores con mensajes legibles
- Exit codes: 0 éxito, 1 error

**Entidades y comandos:**

| Entidad | Subcomandos |
|---------|-------------|
| memory  | save, list, get, delete, search |
| skill   | list, get, run, create, delete |
| agent   | list, get, run, create, delete |
| flow    | list, get, execute, create, delete |
| cron    | list, get, create, delete |
| config  | get, set, view |

**Excluye:**
- Output formatting (HU-14.2)
- Autocompletion (HU-14.3)
- Watch mode / streaming
- Interactive mode (TUI)

## Enfoque técnico

**Estructura Cobra:**
```go
var rootCmd = &cobra.Command{
    Use:   "Domain",
    Short: "Domain CLI - AI Memory Platform",
}

var memoryCmd = &cobra.Command{
    Use:   "memory",
    Short: "Manage memories (observations)",
}

var memorySaveCmd = &cobra.Command{
    Use:   "save [content]",
    Short: "Save a new memory",
    RunE: func(cmd *cobra.Command, args []string) error {
        return client.CreateObservation(ctx, &Observation{
            Title:   viper.GetString("title"),
            Content: viper.GetString("content"),
            Type:    viper.GetString("type"),
        })
    },
}
```

**Client HTTP genérico:**
```go
type Client struct {
    baseURL    string
    apiKey     string
    httpClient *http.Client
}

func (c *Client) Do(method, path string, body, respBody any) error {
    req, _ := http.NewRequest(method, c.baseURL+path, jsonBody(body))
    req.Header.Set("Authorization", "Bearer "+c.apiKey)
    req.Header.Set("Content-Type", "application/json")
    resp, err := c.httpClient.Do(req)
    return json.NewDecoder(resp.Body).Decode(respBody)
}
```

**Config file:**
```yaml
# ~/.config/domain/config.yaml
api_endpoint: http://localhost:8080/api/v1
api_key: sk-xxx
default_output: table
```

**Viper integration:** Cobra + Viper para lectura de flags, env vars, y config file. Prioridad: flags > env > config file > defaults.

## Riesgos

| Riesgo | Mitigación |
|--------|------------|
| Comandos demasiado anidados | Mantener profundidad máxima de 2 niveles: entity + action |
| Config file en diferentes paths según OS | Usar os.UserConfigDir() para cross-platform |
| API key en config file en texto plano | Advertir en docs, future: keyring integration |
| Error handling inconsistente entre comandos | Wrapper RunE que captura errores y formatea mensaje |

## Testing

- Unit: Cobra command registration + flag parsing
- Integration: mock HTTP server, test CLI output
- Golden files para output de ayuda
- Sabotaje: comando sin flags requeridos → error message
