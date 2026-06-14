// Package installer — auto-detect OS + distro + package manager
// + auto-install dependencies with confirm (HU-01.11).
//
// Diseño: el binario `domain` chequea que Go/git/docker esten
// disponibles antes de empezar el wizard de install. Si falta algo
// crítico, ofrece auto-instalar via el package manager nativo del
// sistema, pidiendo confirmacion previa.
//
// Tests-friendly: Install() acepta un callback `withConfirm` para
// que la TUI/CLI decidan como pedir confirm. No se acopla a bubbletea.

package installer

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// OS representa el sistema operativo detectado.
type OS string

const (
	OSLinux   OS = "linux"
	OSDarwin  OS = "darwin"
	OSWindows OS = "windows"
	OSUnknown OS = "unknown"
)

// Distro representa la distribución Linux (o "" para no-linux).
type Distro string

const (
	DistroDebian Distro = "debian"
	DistroUbuntu Distro = "ubuntu"
	DistroFedora Distro = "fedora"
	DistroArch   Distro = "arch"
	DistroAlpine Distro = "alpine"
	DistroMacOS  Distro = "macos"
	DistroWin    Distro = "windows"
	DistroOther  Distro = "other"
)

// PkgManager representa el package manager a usar.
type PkgManager string

const (
	PkgApt    PkgManager = "apt"
	PkgDnf    PkgManager = "dnf"
	PkgYum    PkgManager = "yum"
	PkgPacman PkgManager = "pacman"
	PkgApk    PkgManager = "apk"
	PkgBrew   PkgManager = "brew"
	PkgChoco  PkgManager = "choco"
	PkgWinget PkgManager = "winget"
	PkgNone   PkgManager = "none"
)

// Platform agrupa OS + Distro + PkgManager.
type Platform struct {
	OS      OS
	Distro  Distro
	PkgMgr  PkgManager
	Version string // version del SO (e.g., "22.04" para ubuntu)
}

// ErrUnsupportedOS retorna cuando el OS no es linux/darwin/windows.
var ErrUnsupportedOS = errors.New("unsupported OS")

// DetectPlatform detecta el OS + distro + package manager.
// Lee /etc/os-release en linux, usa runtime.GOOS para darwin/windows.
// Retorna error solo si el OS no es soportado.
func DetectPlatform() (Platform, error) {
	p := Platform{OS: detectOS()}

	switch p.OS {
	case OSLinux:
		distro, version, err := readOSRelease()
		if err != nil {
			p.Distro = DistroOther
		} else {
			p.Distro = mapDistroID(distro)
			p.Version = version
		}
		p.PkgMgr = pkgMgrForDistro(p.Distro)
	case OSDarwin:
		p.Distro = DistroMacOS
		p.PkgMgr = PkgBrew
	case OSWindows:
		p.Distro = DistroWin
		// Preferimos choco (más simple); fallback a winget.
		if _, err := exec.LookPath("choco"); err == nil {
			p.PkgMgr = PkgChoco
		} else {
			p.PkgMgr = PkgWinget
		}
	default:
		return p, fmt.Errorf("%w: %s", ErrUnsupportedOS, p.OS)
	}
	return p, nil
}

func detectOS() OS {
	switch runtime.GOOS {
	case "linux":
		return OSLinux
	case "darwin":
		return OSDarwin
	case "windows":
		return OSWindows
	}
	return OSUnknown
}

// readOSRelease lee /etc/os-release y retorna (ID, VERSION_ID, err).
// Solo linux; en otros OS la implementacion de os_other.go retorna
// error. No definimos el cuerpo aca para evitar conflicto con build tags.

func mapDistroID(id string) Distro {
	switch strings.ToLower(id) {
	case "debian":
		return DistroDebian
	case "ubuntu", "pop", "elementary", "linuxmint":
		return DistroUbuntu
	case "fedora", "rhel", "centos", "rocky", "almalinux":
		return DistroFedora
	case "arch", "manjaro", "endeavouros":
		return DistroArch
	case "alpine":
		return DistroAlpine
	}
	return DistroOther
}

func pkgMgrForDistro(d Distro) PkgManager {
	switch d {
	case DistroDebian, DistroUbuntu:
		return PkgApt
	case DistroFedora:
		return PkgDnf
	case DistroArch:
		return PkgPacman
	case DistroAlpine:
		return PkgApk
	}
	return PkgNone
}
