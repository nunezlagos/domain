# Design: issue-07.3-sync-status

## Decisión arquitectónica

### Output format

```
Sync Status
══════════════

Local:
  Observations:  1,234
  Sessions:         56
  Active:            3

Remote (manifest):
  Total records:    800 (in 5 chunks)
  Last export:      2026-06-01 14:30:00 UTC

Diff:
  +434 observations local
  +0 sessions local
  → Run 'engram sync' to export local changes

Manifest Health:
  Status: ✅ healthy
  Chunks: 5/5 verified
```

### Status colors (auto-detect TTY)

```go
func (s *StatusReport) Format() string {
    isTTY := isatty.IsTerminal(os.Stdout.Fd())
    var b strings.Builder
    
    b.WriteString("Sync Status\n")
    b.WriteString("══════════════\n\n")
    b.WriteString("Local:\n")
    b.WriteString(fmt.Sprintf("  Observations:  %s\n", formatCount(s.Local.Observations, isTTY)))
    // ...
    
    return b.String()
}

func formatCount(n int, isTTY bool) string {
    if !isTTY {
        return fmt.Sprintf("%d", n)
    }
    return lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("%d", n))
}
```

### Health check implementation

```go
func (imp *Importer) HealthCheck() *HealthReport {
    report := &HealthReport{
        Total: len(imp.manifest.Chunks),
    }
    
    for _, c := range imp.manifest.Chunks {
        path := filepath.Join(imp.chunksDir, c.SHA256+".jsonl.gz")
        
        // Check existence
        data, err := os.ReadFile(path)
        if os.IsNotExist(err) {
            report.Missing++
            report.Details = append(report.Details, fmt.Sprintf("  %s: MISSING", c.SHA256[:12]))
            continue
        }
        if err != nil {
            report.Corrupt++
            report.Details = append(report.Details, fmt.Sprintf("  %s: ERROR (%v)", c.SHA256[:12], err))
            continue
        }
        
        // Check SHA-256
        gr, err := gzip.NewReader(bytes.NewReader(data))
        if err != nil {
            report.Corrupt++
            report.Details = append(report.Details, fmt.Sprintf("  %s: INVALID GZIP", c.SHA256[:12]))
            continue
        }
        decompressed, _ := io.ReadAll(gr)
        gr.Close()
        
        hash := sha256.Sum256(decompressed)
        if hex.EncodeToString(hash[:]) != c.SHA256 {
            report.Corrupt++
            report.Details = append(report.Details, fmt.Sprintf("  %s: SHA-256 MISMATCH", c.SHA256[:12]))
            continue
        }
        
        report.Verified++
    }
    
    // Determine overall status
    if report.Corrupt > 0 {
        report.Status = "corrupt"
    } else if report.Missing > 0 {
        report.Status = "degraded"
    } else {
        report.Status = "healthy"
    }
    
    return report
}
```

### Remote counts from manifest

```go
func remoteCounts(manifest *Manifest) RemoteCounts {
    var total int
    for _, c := range manifest.Chunks {
        total += c.RecordCount
    }
    return RemoteCounts{
        Records: total,
        Chunks:  len(manifest.Chunks),
    }
}
```

### Diff calculation

```go
func (s *StatusReport) Diff() string {
    obsDiff := s.Local.Observations - s.Remote.Records // aproximación
    // Nota: conteo exacto requeriría exportar y comparar; esto es un estimado
    if obsDiff > 0 {
        return fmt.Sprintf("+%d observations local\n→ Run 'engram sync' to export", obsDiff)
    }
    if obsDiff < 0 {
        return fmt.Sprintf("%d observations remote\n→ Run 'engram sync --import' to import", -obsDiff)
    }
    return "In sync"
}
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| JSON output flag | Se puede agregar después; texto plano es suficiente para MVP |
| Status watches (--watch flag) | Overengineering; el usuario puede ejecutar el comando cuando quiera |
| Comparación exacta (hash-based) | Muy costoso; conteos aproximados son suficientes para el caso de uso |
| Git status integrado | Sería mezclar responsabilidades; git status se ve con git status |

## TDD plan

1. **Red:** Test que local counts son correctos → falla
2. **Green:** Implementar store queries → pasa
3. **Red:** Test que remote counts suman recordCount → falla
4. **Green:** Implementar remoteCounts → pasa
5. **Red:** Test que health check detecta chunk healthy → falla
6. **Green:** Implementar HealthCheck básico → pasa
7. **Red:** Test que health check detecta chunk missing → falla
8. **Green:** Agregar detección de missing → pasa
9. **Red:** Test que health check detecta SHA mismatch → falla
10. **Green:** Agregar SHA-256 verification → pasa
11. **Red:** Test que output contiene "healthy" o "degraded" → falla
12. **Green:** Implementar status formatter → pasa
13. **Sabotaje:** Eliminar chunk del disco → health check degraded → test cae → restaurar

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| SHA-256 verify es caro I/O | Solo verificar primeros bytes + tamaño si performance es problema; pero chunks son pequeños |
| Conteo remoto puede no coincidir con real (recordCount del export no discrimina tipo) | Mejorar ChunkEntry con observationCount + sessionCount |
| Status sin manifest no tiene remote info | Mostrar "No manifest found" y solo local counts |
