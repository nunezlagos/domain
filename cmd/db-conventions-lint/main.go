// Command db-conventions-lint — HU-25.13 entrypoint CLI.
//
// Uso:
//
//	db-conventions-lint <archivo.sql>...
//	db-conventions-lint --dir internal/migrate/migrations
//	db-conventions-lint --baseline 28 --dir internal/migrate/migrations
//
// Exit code: 0 si limpio, 1 si hay issues.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"nunezlagos/domain/internal/dbconvlint"
)

func main() {
	dir := flag.String("dir", "", "directorio de migrations (recursivo no, solo *.up.sql)")
	baseline := flag.Int("baseline", 0, "ignora migrations cuyo número sea <= baseline (default 0 = sin baseline)")
	flag.Parse()

	files := flag.Args()
	if *dir != "" {
		entries, err := os.ReadDir(*dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read dir: %v\n", err)
			os.Exit(2)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".up.sql") {
				continue
			}
			files = append(files, filepath.Join(*dir, e.Name()))
		}
	}
	sort.Strings(files)

	reMigrationNum := regexp.MustCompile(`^(\d{6})`)
	allIssues := 0
	for _, f := range files {
		name := filepath.Base(f)
		if *baseline > 0 {
			if m := reMigrationNum.FindStringSubmatch(name); m != nil {
				if n, _ := strconv.Atoi(m[1]); n <= *baseline {
					continue
				}
			}
		}
		data, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read %s: %v\n", f, err)
			os.Exit(2)
		}
		issues := dbconvlint.Lint(f, string(data))
		for _, is := range issues {
			fmt.Println(is.String())
		}
		allIssues += len(issues)
	}
	if allIssues > 0 {
		fmt.Fprintf(os.Stderr, "\n%d issue(s) found\n", allIssues)
		os.Exit(1)
	}
	fmt.Println("db-conventions-lint: OK")
}
