//go:build !windows

package main

import (
	"fmt"
	"os"
)

// The installer is a Windows program; elsewhere it builds only so its logic can be tested.
func main() {
	fmt.Fprintln(os.Stderr, "SkillIssues Installer runs on Windows. Get the app for your system at https://skillissues.dizziedbliss.lol/download")
	os.Exit(1)
}
