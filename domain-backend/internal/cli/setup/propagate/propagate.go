package propagate

import (
	"fmt"
	"os/exec"
)

func Propagate(selected []ProjectInfo, dryRun bool) (success, failed int, errs []error) {
	if dryRun {
		return 0, 0, nil
	}

	for _, info := range selected {
		cmd := exec.Command("domain", "setup", "auto-detect", info.Path, "--quiet")
		output, err := cmd.CombinedOutput()
		if err != nil {
			failed++
			errs = append(errs, fmt.Errorf("%s: %w\n%s", info.Name, err, string(output)))
		} else {
			success++
		}
	}
	return
}
