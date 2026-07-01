package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// maybeRemoveEngram deshabilita el plugin engram si está activo y el usuario
// lo permite. Política de control:
//
//   - Engram NO activo → no hace nada (silencio)
//   - Engram activo + --remove-engram → deshabilita sin preguntar
//   - Engram activo + sin flag + interactive (TTY) → pregunta (default-yes)
//   - Engram activo + sin flag + non-interactive → warn, no toca nada
//
// En modo interactive, la idea es dar a conocer que hay 2 sistemas de memoria
//并存iendo y dejar al usuario elegir. En modo no-interactive, no tocamos
// config del usuario sin consentimiento explícito (--remove-engram).
func maybeRemoveEngram(home string, opts installOptions, in *bufio.Reader) {
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if !fileExists(settingsPath) {
		return
	}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		warnL("no pude leer " + settingsPath + ": " + err.Error())
		return
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		warnL(settingsPath + " corrupto, no toco nada")
		return
	}

	plugins, _ := cfg["enabledPlugins"].(map[string]any)
	if plugins == nil {
		return
	}
	if _, ok := plugins["engram@engram"]; !ok {
		// Engram no está activo: nada que hacer
		return
	}

	// Engram detectado activo
	switch {
	case opts.RemoveEngram:
		// Flag explícito: deshabilitar sin preguntar
		disableEngramInCfg(cfg, plugins)
		writeSettingsOrWarn(settingsPath, cfg)
		ok("engram deshabilitado (--remove-engram): domain queda como único sistema de memoria")

	case opts.NonInteractive:
		// Non-interactive: solo warn, no tocamos
		warnL("engram detectado activo (sistema de memoria legacy).")
		warnL("puede causar confusión con domain (2 sistemas de memoria并存iendo).")
		warnL("para deshabilitarlo: re-corré con --remove-engram o manualmente:")
		warnL("  python3 -c \"import json; p='" + settingsPath + "'; d=json.load(open(p)); d['enabledPlugins'].pop('engram@engram', None); d['allowedTools']=[t for t in d.get('allowedTools',[]) if not t.startswith('mcp__engram__')]; json.dump(d, open(p,'w'), indent=2)\"")

	default:
		// Interactive: preguntar
		if !isTTY() {
			// No hay TTY: caemos a comportamiento non-interactive
			warnL("engram detectado activo. Re-corré con --remove-engram para deshabilitarlo (sin TTY no puedo preguntar).")
			return
		}
		fmt.Print("  engram detectado activo (sistema de memoria legacy). ")
		fmt.Print("¿Deshabilitarlo para que domain sea el único? (Y/n): ")
		if confirm(in, "") {
			disableEngramInCfg(cfg, plugins)
			writeSettingsOrWarn(settingsPath, cfg)
			ok("engram deshabilitado: domain queda como único sistema de memoria")
		} else {
			info("engram queda activo. vas a ver el mensaje CRITICAL FIRST ACTION en cada prompt hasta que lo deshabilites.")
		}
	}
}

// disableEngramInCfg muta cfg removiendo engram de enabledPlugins y de
// allowedTools (no toca el archivo; el caller persiste).
func disableEngramInCfg(cfg map[string]any, plugins map[string]any) {
	delete(plugins, "engram@engram")
	if mkts, ok := cfg["extraKnownMarketplaces"].(map[string]any); ok {
		delete(mkts, "engram")
	}
	if ats, ok := cfg["allowedTools"].([]any); ok {
		filtered := ats[:0]
		for _, t := range ats {
			if s, ok := t.(string); ok && !strings.HasPrefix(s, "mcp__engram__") {
				filtered = append(filtered, t)
			}
		}
		cfg["allowedTools"] = filtered
	}
}

func writeSettingsOrWarn(path string, cfg map[string]any) {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		warnL("marshal " + path + ": " + err.Error())
		return
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		warnL("write " + path + ": " + err.Error())
	}
}