








package install

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// maxStderrBytes limita cuanto stderr capturamos (8KB).
const maxStderrBytes = 8192

// InstallRunner es la funcion que corre el install real, streameando
// cada línea de output via onLine. Tests lo mockean con SetInstallRunner.
//
// Retorna (error, stderr). El stderr es util cuando err != nil.
type InstallRunner func(ctx context.Context, flags []string, onLine func(string)) (error, string)

var installRunner InstallRunner = defaultInstallRunner

// SetInstallRunner reemplaza el runner (solo para tests).
func SetInstallRunner(fn InstallRunner) {
	installRunner = fn
}

// runInstallStreaming corre el install con los flags dados, streameando
// el output línea a línea.
func runInstallStreaming(ctx context.Context, flags []string, onLine func(string)) (error, string) {
	return installRunner(ctx, flags, onLine)
}

func defaultInstallRunner(ctx context.Context, flags []string, onLine func(string)) (error, string) {
	bin, err := exec.LookPath("domain")
	if err != nil {

		if _, statErr := os.Stat("./bin/domain"); statErr == nil {
			bin, _ = filepath.Abs("./bin/domain")
		} else {
			return fmt.Errorf("domain binary not found in PATH or ./bin/: %w", err), ""
		}
	}
	args := append([]string{"install"}, flags...)
	cmd := exec.CommandContext(ctx, bin, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err, ""
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err, ""
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start install sub-process: %w", err), ""
	}

	var stderrBuf bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		streamLines(stdout, onLine, nil)
	}()
	go func() {
		defer wg.Done()
		streamLines(stderr, onLine, &stderrBuf)
	}()
	wg.Wait()

	err = cmd.Wait()
	stderrStr := stderrBuf.String()
	if err != nil {
		return fmt.Errorf("install sub-process failed: %w", err), stderrStr
	}
	return nil, stderrStr
}

// streamLines lee r línea a línea: invoca onLine por cada una y,
// opcionalmente, acumula en buf (con cap maxStderrBytes).
func streamLines(r io.Reader, onLine func(string), buf *bytes.Buffer) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if onLine != nil {
			onLine(line)
		}
		if buf != nil && buf.Len() < maxStderrBytes {
			remaining := maxStderrBytes - buf.Len()
			if len(line)+1 > remaining {
				buf.WriteString(line[:remaining])
			} else {
				buf.WriteString(line)
				buf.WriteByte('\n')
			}
		}
	}
}
