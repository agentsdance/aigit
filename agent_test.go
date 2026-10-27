package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/zzxwill/aigit/llm"
)

func agentTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(dir, "unused-gitconfig"))
	agentTestGit(t, "init", "--quiet")
	agentTestGit(t, "config", "user.name", "aigit test")
	agentTestGit(t, "config", "user.email", "aigit@example.test")
	return dir
}

func agentTestGit(t *testing.T, args ...string) string {
	t.Helper()
	output, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

func agentTestWrite(t *testing.T, name, content string) {
	t.Helper()
	if err := os.WriteFile(name, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func agentTestStageChange(t *testing.T) *agentRequest {
	t.Helper()
	agentTestWrite(t, "tracked.txt", "original\n")
	agentTestGit(t, "add", "tracked.txt")
	agentTestGit(t, "commit", "--quiet", "-m", "initial")
	agentTestWrite(t, "tracked.txt", "staged-value\n")
	agentTestGit(t, "add", "tracked.txt")
	request, err := prepareAgentRequest(llm.NewConfig())
	if err != nil {
		t.Fatal(err)
	}
	return request
}

func TestPrepareAgentRequestUsesOnlyStagedChanges(t *testing.T) {
	agentTestRepo(t)
	request := agentTestStageChange(t)
	agentTestWrite(t, "tracked.txt", "unstaged-value\n")
	agentTestWrite(t, "untracked.txt", "untracked-value\n")
	before := agentTestGit(t, "status", "--porcelain")
	config := llm.NewConfig()
	config.Language = "zh"
	config.Emoji = false
	prepared, err := prepareAgentRequest(config)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prepared.Prompt, "+staged-value") ||
		strings.Contains(prepared.Prompt, "unstaged-value") ||
		strings.Contains(prepared.Prompt, "untracked-value") {
		t.Fatalf("prompt did not preserve the staged-only scope: %s", prepared.Prompt)
	}
	if prepared.BaseTree != request.BaseTree || prepared.StagedTree != request.StagedTree {
		t.Fatal("unstaged edits changed the prepared snapshot")
	}
	if prepared.Language != "zh" || prepared.Emoji || !strings.Contains(prepared.Prompt, "必须用中文") {
		t.Fatal("language or emoji preferences were not retained")
	}
	if after := agentTestGit(t, "status", "--porcelain"); after != before {
		t.Fatalf("prepare changed the index or worktree: before=%q after=%q", before, after)
	}
}

func TestPrepareAgentRequestSupportsInitialCommit(t *testing.T) {
	agentTestRepo(t)
	agentTestWrite(t, "new.txt", "first change\n")
	agentTestGit(t, "add", "new.txt")
	request, err := prepareAgentRequest(llm.NewConfig())
	if err != nil {
		t.Fatal(err)
	}
	if request.BaseTree == request.StagedTree || !strings.Contains(request.Prompt, "+first change") {
		t.Fatal("initial staged changes were not captured")
	}
	input := &agentCommit{BaseTree: request.BaseTree, StagedTree: request.StagedTree}
	if err := commitAgentMessage(input, "feat: add first change", io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	if got := agentTestGit(t, "show", "HEAD:new.txt"); got != "first change" {
		t.Fatalf("wrong initial commit contents: %q", got)
	}
}

func TestPrepareAgentRequestDoesNotStageFiles(t *testing.T) {
	agentTestRepo(t)
	agentTestWrite(t, "untracked.txt", "keep unstaged\n")
	if _, err := prepareAgentRequest(llm.NewConfig()); err == nil || !strings.Contains(err.Error(), "no staged changes") {
		t.Fatalf("expected no-staged-changes error, got %v", err)
	}
	if got := agentTestGit(t, "ls-files"); got != "" {
		t.Fatalf("prepare staged files: %s", got)
	}
}

func TestPrepareAgentRequestReportsTruncation(t *testing.T) {
	agentTestRepo(t)
	agentTestWrite(t, "large.txt", strings.Repeat("large change\n", maxDiffBytes/10))
	agentTestGit(t, "add", "large.txt")
	request, err := prepareAgentRequest(llm.NewConfig())
	if err != nil {
		t.Fatal(err)
	}
	if !request.Truncated || !strings.Contains(request.Prompt, "diff truncated") {
		t.Fatal("truncated diff was not reported")
	}
}

func TestCommitAgentMessagePreservesUnstagedChanges(t *testing.T) {
	agentTestRepo(t)
	request := agentTestStageChange(t)
	agentTestWrite(t, "tracked.txt", "unstaged-value\n")
	agentTestWrite(t, "untracked.txt", "untracked-value\n")
	message := "feat(test): update staged value\n\n- Explain the intended behavior"
	input := &agentCommit{BaseTree: request.BaseTree, StagedTree: request.StagedTree}
	if err := commitAgentMessage(input, message, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	if got := agentTestGit(t, "show", "HEAD:tracked.txt"); got != "staged-value" {
		t.Fatalf("committed unstaged content: %q", got)
	}
	if got := agentTestGit(t, "log", "-1", "--format=%B"); got != message {
		t.Fatalf("commit message changed: %q", got)
	}
	if got := agentTestGit(t, "status", "--porcelain"); !strings.Contains(got, "M tracked.txt") || !strings.Contains(got, "?? untracked.txt") {
		t.Fatalf("unrelated changes were not preserved: %q", got)
	}
}

func TestCommitAgentMessageRejectsSnapshotChanges(t *testing.T) {
	for _, change := range []string{"index", "HEAD"} {
		t.Run(change, func(t *testing.T) {
			agentTestRepo(t)
			request := agentTestStageChange(t)
			if change == "index" {
				agentTestWrite(t, "tracked.txt", "later staged content\n")
				agentTestGit(t, "add", "tracked.txt")
			} else {
				// Advance HEAD without touching the index: only base_tree changes.
				commit := agentTestGit(t, "commit-tree", request.StagedTree, "-p", "HEAD", "-m", "parallel commit")
				agentTestGit(t, "update-ref", "HEAD", commit)
			}
			head := agentTestGit(t, "rev-parse", "HEAD")
			input := &agentCommit{BaseTree: request.BaseTree, StagedTree: request.StagedTree}
			err := commitAgentMessage(input, "feat: stale message", io.Discard, io.Discard)
			if err == nil || !strings.Contains(err.Error(), "run aigit prepare again") {
				t.Fatalf("expected snapshot rejection, got %v", err)
			}
			if got := agentTestGit(t, "rev-parse", "HEAD"); got != head {
				t.Fatal("stale message created a commit")
			}
		})
	}
}

func TestCommitAgentMessagePreservesGitHooks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell hook fixture requires a POSIX shell")
	}
	dir := agentTestRepo(t)
	request := agentTestStageChange(t)
	head := agentTestGit(t, "rev-parse", "HEAD")
	hook := filepath.Join(dir, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\nprintf 'hook rejected commit' >&2\nexit 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	input := &agentCommit{BaseTree: request.BaseTree, StagedTree: request.StagedTree}
	if err := commitAgentMessage(input, "feat: staged change", io.Discard, &stderr); err == nil {
		t.Fatal("commit bypassed the rejecting hook")
	}
	if !strings.Contains(stderr.String(), "hook rejected commit") || agentTestGit(t, "rev-parse", "HEAD") != head {
		t.Fatal("hook output or unchanged HEAD was not preserved")
	}
}

func TestReadAgentCommitRejectsInvalidResponses(t *testing.T) {
	valid := `{"base_tree":"base","staged_tree":"staged","type":"feat","description":"add a feature","body":["Explain why"]}`
	for name, response := range map[string]string{
		"empty":          "",
		"missing trees":  `{"type":"feat","description":"missing snapshot"}`,
		"wrong body":     `{"base_tree":"base","staged_tree":"staged","body":"not a list"}`,
		"null body":      `{"base_tree":"base","staged_tree":"staged","body":null}`,
		"unknown field":  `{"base_tree":"base","staged_tree":"staged","command":"git add ."}`,
		"extra object":   valid + `{}`,
		"trailing prose": valid + "done",
		"markdown":       "```json\n" + valid + "\n```",
		"oversized":      strings.Repeat(" ", maxAgentMessageBytes+1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := readAgentCommit(strings.NewReader(response)); err == nil {
				t.Fatal("invalid response was accepted")
			}
		})
	}
	input, err := readAgentCommit(strings.NewReader(valid))
	if err != nil {
		t.Fatal(err)
	}
	if input.Type != "feat" || len(input.Body) != 1 {
		t.Fatal("valid commit fields were not decoded")
	}
}

func TestAgentCommandsRejectInvalidFlagsAndJSON(t *testing.T) {
	for _, args := range [][]string{
		{"prepare", "--language", "fr"},
		{"prepare", "unexpected-argument"},
		{"commit", "--dry-run", "--yes"},
		{"commit", "--from-agent", ""},
		{"commit", "--from-agent", "-", "--language", "en"},
		{"commit", "--from-agent", "-", "--yes"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			cmd := newRootCommand()
			cmd.SetArgs(args)
			cmd.SetIn(strings.NewReader(`{"description":"incomplete"}`))
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			if err := cmd.Execute(); err == nil {
				t.Fatal("invalid command was accepted")
			}
		})
	}
}
