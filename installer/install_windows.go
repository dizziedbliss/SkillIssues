//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

// Installed is SkillIssues as Windows' "Apps" list records it.
type Installed struct {
	Version    string `json:"version"`
	Location   string `json:"location"`
	PerMachine bool   `json:"perMachine"`
	uninstall  string // the UninstallString, quoted path then arguments
}

const uninstallKey = `Software\Microsoft\Windows\CurrentVersion\Uninstall`

// findInstalled looks for the app's uninstall entry, for this user first,
// then for everyone. electron-builder names the entry by a GUID, so it is
// found by its DisplayName.
func findInstalled() (*Installed, error) {
	for _, root := range []struct {
		key        registry.Key
		perMachine bool
	}{{registry.CURRENT_USER, false}, {registry.LOCAL_MACHINE, true}} {
		parent, err := registry.OpenKey(root.key, uninstallKey, registry.ENUMERATE_SUB_KEYS|registry.WOW64_64KEY)
		if err != nil {
			continue // no such key on this machine
		}
		names, err := parent.ReadSubKeyNames(-1)
		parent.Close()
		if err != nil {
			return nil, fmt.Errorf("reading installed apps: %w", err)
		}
		for _, name := range names {
			k, err := registry.OpenKey(root.key, uninstallKey+`\`+name, registry.QUERY_VALUE|registry.WOW64_64KEY)
			if err != nil {
				continue
			}
			display, _, _ := k.GetStringValue("DisplayName")
			if display != "SkillIssues" && !strings.HasPrefix(display, "SkillIssues ") {
				k.Close()
				continue
			}
			in := &Installed{PerMachine: root.perMachine}
			in.Version, _, _ = k.GetStringValue("DisplayVersion")
			in.Location, _, _ = k.GetStringValue("InstallLocation")
			in.uninstall, _, _ = k.GetStringValue("UninstallString")
			k.Close()
			if in.Location == "" {
				if exe, _ := splitCommand(in.uninstall); exe != "" {
					in.Location = filepath.Dir(exe)
				}
			}
			return in, nil
		}
	}
	return nil, nil
}

// defaultDir is where electron-builder's installer puts the app by default.
func defaultDir(perMachine bool) string {
	if perMachine {
		return filepath.Join(os.Getenv("ProgramFiles"), "SkillIssues")
	}
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "SkillIssues")
}

// runInstaller runs electron-builder's installer silently: for this user or
// everyone (which asks Windows for permission), into dir. NSIS wants /D last
// and unquoted.
func runInstaller(path string, perMachine bool, dir string) error {
	scope := "/currentuser"
	if perMachine {
		scope = "/allusers"
	}
	args := []string{"/S", scope}
	if dir != "" {
		if !filepath.IsAbs(dir) {
			return errors.New("the folder must be a full path, like C:\\Apps\\SkillIssues")
		}
		args = append(args, "/D="+filepath.Clean(dir))
	}
	return run(path, args...)
}

// runUninstaller runs the recorded uninstaller silently.
func runUninstaller(in *Installed) error {
	exe, args := splitCommand(in.uninstall)
	if exe == "" {
		return errors.New("Windows has no uninstaller recorded for SkillIssues")
	}
	return run(exe, append(args, "/S")...)
}

// launch starts the installed app without waiting for it.
func launch(in *Installed) error {
	if in == nil || in.Location == "" {
		return errors.New("SkillIssues isn't installed")
	}
	cmd := exec.Command(filepath.Join(in.Location, "SkillIssues.exe"))
	cmd.Dir = in.Location
	return cmd.Start()
}

// run starts an installer and waits for it. An exit code other than 0 is an error.
func run(exe string, args ...string) error {
	cmd := exec.Command(exe, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			if exit.ExitCode() == 1223 { // ERROR_CANCELLED: the permission prompt was declined
				return errors.New("Windows didn't give permission to install for everyone")
			}
			return fmt.Errorf("the installer stopped with code %d", exit.ExitCode())
		}
		return err
	}
	return nil
}

// splitCommand splits an UninstallString: a path, quoted if it has spaces,
// then arguments separated by spaces.
func splitCommand(s string) (string, []string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	var exe, rest string
	if strings.HasPrefix(s, `"`) {
		end := strings.Index(s[1:], `"`)
		if end < 0 {
			return "", nil
		}
		exe, rest = s[1:1+end], s[2+end:]
	} else {
		exe, rest, _ = strings.Cut(s, " ")
	}
	return exe, strings.Fields(rest)
}
