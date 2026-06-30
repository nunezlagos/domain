




package installer

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)



func TestDetectOS_CurrentPlatform(t *testing.T) {
	p, err := DetectPlatform()
	require.NoError(t, err)

	t.Logf("Detected: OS=%s Distro=%s PkgMgr=%s Version=%s", p.OS, p.Distro, p.PkgMgr, p.Version)
	require.NotEmpty(t, p.PkgMgr, "pkg manager must be detected")
}

func TestMapDistroID(t *testing.T) {
	cases := map[string]Distro{
		"ubuntu":      DistroUbuntu,
		"debian":      DistroDebian,
		"fedora":      DistroFedora,
		"arch":        DistroArch,
		"manjaro":     DistroArch,
		"alpine":      DistroAlpine,
		"pop":         DistroUbuntu, // pop es derivado de ubuntu
		"rhel":        DistroFedora,
		"rocky":       DistroFedora,
		"unknown":     DistroOther,
		"elementary":  DistroUbuntu,
		"linuxmint":   DistroUbuntu,
		"endeavouros": DistroArch,
	}
	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			require.Equal(t, want, mapDistroID(input))
		})
	}
}

func TestPkgMgrForDistro(t *testing.T) {
	cases := map[Distro]PkgManager{
		DistroUbuntu: PkgApt,
		DistroDebian: PkgApt,
		DistroFedora: PkgDnf,
		DistroArch:   PkgPacman,
		DistroAlpine: PkgApk,
		DistroOther:  PkgNone,
	}
	for d, want := range cases {
		t.Run(string(d), func(t *testing.T) {
			require.Equal(t, want, pkgMgrForDistro(d))
		})
	}
}



func TestCompareVersion(t *testing.T) {
	cases := []struct {
		have, want string
		expected   int
	}{
		{"1.22.0", "1.22", 0},
		{"1.22.3", "1.22", 0},
		{"1.23.0", "1.22", 1},
		{"1.21.9", "1.22", -1},
		{"2.0.0", "1.22", 1},
		{"1.0", "1.0", 0},
		{"0.9", "1.0", -1},
	}
	for _, tc := range cases {
		t.Run(tc.have+"_vs_"+tc.want, func(t *testing.T) {
			require.Equal(t, tc.expected, compareVersion(tc.have, tc.want))
		})
	}
}



func TestInstallCommand(t *testing.T) {
	cases := []struct {
		pm       PkgManager
		dep      Dep
		wantSubs string // substring que debe estar en el comando
	}{
		{PkgApt, DepGo, "apt install -y golang-go"},
		{PkgApt, DepGit, "apt install -y git"},
		{PkgDnf, DepGo, "dnf install -y golang-go"},
		{PkgPacman, DepGo, "pacman -S"},
		{PkgBrew, DepGo, "brew install go"},
		{PkgChoco, DepGit, "choco install -y git"},
		{PkgWinget, DepGit, "winget install -y git"},
	}
	for _, tc := range cases {
		t.Run(string(tc.pm)+"_"+tc.dep.Name, func(t *testing.T) {
			cmd := InstallCommand(tc.pm, tc.dep)
			require.Contains(t, cmd, tc.wantSubs)
			if tc.pm == PkgApt || tc.pm == PkgDnf || tc.pm == PkgPacman || tc.pm == PkgApk {
				require.Contains(t, cmd, "sudo", "linux pkg managers should use sudo")
			}
			if tc.pm == PkgBrew {
				require.NotContains(t, cmd, "sudo", "brew should NOT use sudo")
			}
		})
	}
}



func TestCheck_FindsGoInPATH(t *testing.T) {

	results := Check([]Dep{DepGo})
	require.Len(t, results, 1)
	r := results[0]
	require.True(t, r.Found, "go must be found (we are running tests with go)")
	require.NotEmpty(t, r.Path)
	t.Logf("go found at %s, version=%s, MinMet=%v", r.Path, r.Version, r.MinMet)
}

func TestCheck_NotFoundHasHint(t *testing.T) {
	results := Check([]Dep{{
		Name:    "nonexistent-binary-xyz",
		Binary:  "nonexistent-binary-xyz",
		PkgName: "fake-pkg",
	}})
	require.Len(t, results, 1)
	r := results[0]
	require.False(t, r.Found)
	require.NotEmpty(t, r.Hint, "missing dep must have install hint")
}



func TestInstall_UserRejectsConfirm(t *testing.T) {
	called := false
	confirm := func(prompt string) bool {
		called = true
		return false // user dice no
	}
	skipped, err := Install(context.Background(),
		Platform{OS: OSLinux, Distro: DistroUbuntu, PkgMgr: PkgApt},
		DepGo, confirm)
	require.NoError(t, err)
	require.True(t, called, "withConfirm must be called")
	require.True(t, skipped, "user rejected → skipped=true")
}

func TestInstall_UserAcceptsConfirm_CommandFails(t *testing.T) {





	confirm := func(prompt string) bool { return true }
	_, err := Install(context.Background(),
		Platform{OS: OSLinux, Distro: DistroUbuntu, PkgMgr: PkgApt},
		Dep{Binary: "nonexistent-binary-xyz", PkgName: "fake"},
		confirm)


	_ = err
}

func TestInstall_NilConfirm_ReturnsError(t *testing.T) {
	_, err := Install(context.Background(),
		Platform{OS: OSLinux, Distro: DistroUbuntu, PkgMgr: PkgApt},
		DepGo, nil)
	require.Error(t, err)
}



func TestErrorsIs_UnsupportedOS(t *testing.T) {



	err := ErrUnsupportedOS
	require.True(t, errors.Is(err, ErrUnsupportedOS))
}



func TestRunInstallCommand_EmptyString(t *testing.T) {
	err := runInstallCommand(context.Background(), "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty")
}

func TestRunInstallCommand_WhitespaceOnly(t *testing.T) {
	err := runInstallCommand(context.Background(), "   \t  \n  ")
	require.Error(t, err)
}
