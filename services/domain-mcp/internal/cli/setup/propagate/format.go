package propagate

import (
	"fmt"
	"strings"
)

func FormatTable(infos []ProjectInfo) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%-24s %-40s %-6s %s\n", "NAME", "PATH", "DOMAIN", "IA_CONFIGS"))
	b.WriteString(strings.Repeat("-", 110) + "\n")

	for _, info := range infos {
		domain := "no"
		if info.HasDomain {
			domain = "yes"
		}
		configs := strings.Join(info.IAConfigs, ", ")
		if configs == "" {
			configs = "-"
		}
		name := truncate(info.Name, 22)
		path := truncate(info.Path, 38)
		b.WriteString(fmt.Sprintf("%-24s %-40s %-6s %s\n", name, path, domain, configs))
	}

	return b.String()
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
