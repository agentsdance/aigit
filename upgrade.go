package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const repoURL = "https://github.com/agentsdance/aigit"

// selfUpgrade replaces the running binary with the given release version.
// Homebrew installs are delegated to brew; everything else is built from the
// release tag with the local Go toolchain and swapped in place.
func selfUpgrade(version string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locating current binary: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	if strings.Contains(exe, "/Cellar/") || strings.Contains(exe, "/linuxbrew/") {
		cmd := exec.Command("brew", "upgrade", "aigit")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	goBin, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("go toolchain not found; cannot build %s from source", version)
	}

	tmpDir, err := os.MkdirTemp("", "aigit-upgrade-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	srcDir := filepath.Join(tmpDir, "src")
	if err := runQuiet(exec.Command("git", "clone", "--quiet", "--depth", "1", "--branch", version, repoURL, srcDir)); err != nil {
		return fmt.Errorf("downloading %s: %w", version, err)
	}

	newBin := filepath.Join(tmpDir, "aigit")
	build := exec.Command(goBin, "build", "-ldflags", "-s -w -X main.Version="+version, "-o", newBin, ".")
	build.Dir = srcDir
	if err := runQuiet(build); err != nil {
		return fmt.Errorf("building %s: %w", version, err)
	}

	return replaceBinary(newBin, exe)
}

// replaceBinary swaps dst for newBin via a rename from a sibling temp file,
// so the running executable is replaced atomically rather than truncated.
func replaceBinary(newBin, dst string) error {
	data, err := os.ReadFile(newBin)
	if err != nil {
		return err
	}
	staged := dst + ".new"
	if err := os.WriteFile(staged, data, 0o755); err != nil {
		return fmt.Errorf("staging new binary (try upgrading with elevated permissions): %w", err)
	}
	if err := os.Rename(staged, dst); err != nil {
		os.Remove(staged)
		return fmt.Errorf("installing new binary: %w", err)
	}
	return nil
}

func runQuiet(cmd *exec.Cmd) error {
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
