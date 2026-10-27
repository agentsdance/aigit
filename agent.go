package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/zzxwill/aigit/llm"
)

const maxAgentMessageBytes = 1 << 20

type agentRequest struct {
	BaseTree   string `json:"base_tree"`
	StagedTree string `json:"staged_tree"`
	Language   string `json:"language"`
	Emoji      bool   `json:"emoji"`
	Truncated  bool   `json:"truncated"`
	Prompt     string `json:"prompt"`
}

type agentCommit struct {
	BaseTree   string `json:"base_tree"`
	StagedTree string `json:"staged_tree"`
	llm.CommitData
}

func isAgentCommand(cmd *cobra.Command) bool {
	return cmd.Name() == "prepare" || cmd.Name() == "commit" && cmd.Flags().Changed("from-agent")
}

func newPrepareCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prepare",
		Short: "Print a commit-generation request for the calling coding agent",
		Long: `Print JSON containing the staged diff prompt and its Git tree IDs.
The calling agent generates the message in its current conversation; no API or
agent process is started. Add base_tree and staged_tree from this request to the
generated commit JSON, then pass it to aigit commit --from-agent <file|->.
This command does not stage files and skips update checks.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			language, _ := cmd.Flags().GetString("language")
			if language != "" && language != "en" && language != "zh" {
				return fmt.Errorf("language must be en or zh")
			}
			config, err := loadAgentConfig()
			if err != nil {
				return err
			}
			if language != "" {
				config.Language = language
			}
			request, err := prepareAgentRequest(config)
			if err != nil {
				return err
			}
			encoder := json.NewEncoder(cmd.OutOrStdout())
			encoder.SetIndent("", "  ")
			return encoder.Encode(request)
		},
	}
	cmd.Flags().StringP("language", "l", "", "Language for commit message: en or zh (overrides config file)")
	return cmd
}

func loadAgentConfig() (*llm.Config, error) {
	config := llm.NewConfig()
	if err := config.Load(); err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	return config, nil
}

func prepareAgentRequest(config *llm.Config) (*agentRequest, error) {
	stagedTree, err := gitTree("write-tree")
	if err != nil {
		return nil, err
	}
	baseTree, err := currentBaseTree()
	if err != nil {
		return nil, err
	}
	// Diff immutable trees so concurrent index edits cannot mix two snapshots.
	diff, err := agentGitOutput("diff", "--no-ext-diff", "--no-textconv", "--no-color", baseTree, stagedTree, "--")
	if err != nil {
		return nil, err
	}
	if len(diff) == 0 {
		return nil, fmt.Errorf("no staged changes; stage the intended files with git add first")
	}
	diff = processBinaryDiff(diff)
	language := config.Language
	if language == "" {
		language = "en"
	}
	return &agentRequest{
		BaseTree:   baseTree,
		StagedTree: stagedTree,
		Language:   language,
		Emoji:      config.Emoji,
		Truncated:  len(diff) > maxDiffBytes,
		Prompt:     llm.BuildCommitPrompt(string(truncateDiff(diff)), language, config.Emoji),
	}, nil
}

func runAgentCommit(cmd *cobra.Command) error {
	filename, _ := cmd.Flags().GetString("from-agent")
	if filename == "" {
		return fmt.Errorf("--from-agent requires a JSON file path or - for stdin")
	}
	var reader io.Reader = cmd.InOrStdin()
	if filename != "-" {
		file, err := os.Open(filename)
		if err != nil {
			return fmt.Errorf("opening agent response: %w", err)
		}
		defer file.Close()
		reader = file
	}
	input, err := readAgentCommit(reader)
	if err != nil {
		return err
	}
	config, err := loadAgentConfig()
	if err != nil {
		return err
	}
	message, err := llm.FormatAgentCommit(input.CommitData, config.Emoji)
	if err != nil {
		return fmt.Errorf("invalid agent message: %w", err)
	}
	if err := checkAgentSnapshot(input); err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), message)
	if dryRun, _ := cmd.Flags().GetBool("dry-run"); dryRun {
		return nil
	}
	yes, _ := cmd.Flags().GetBool("yes")
	if !yes {
		prompt := promptui.Select{
			Label: "Use the calling agent's commit message",
			Items: []string{"Commit this message", "Cancel"},
			Size:  2,
		}
		choice, _, err := prompt.Run()
		if err != nil {
			return fmt.Errorf("confirming commit (use --yes for an authorized noninteractive commit): %w", err)
		}
		if choice != 0 {
			return nil
		}
	}
	return commitAgentMessage(input, message, cmd.OutOrStdout(), cmd.ErrOrStderr())
}

func readAgentCommit(reader io.Reader) (*agentCommit, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maxAgentMessageBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading agent response: %w", err)
	}
	if len(data) > maxAgentMessageBytes {
		return nil, fmt.Errorf("agent response exceeds %d bytes", maxAgentMessageBytes)
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	var input agentCommit
	if err := decoder.Decode(&input); err != nil {
		return nil, fmt.Errorf("invalid agent JSON: %w", err)
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("agent response must contain exactly one JSON object")
	}
	if input.BaseTree == "" || input.StagedTree == "" {
		return nil, fmt.Errorf("base_tree and staged_tree from aigit prepare are required")
	}
	if input.Body == nil {
		return nil, fmt.Errorf("body must be an array of strings; use [] for an empty body")
	}
	return &input, nil
}

func checkAgentSnapshot(input *agentCommit) error {
	stagedTree, err := gitTree("write-tree")
	if err != nil {
		return err
	}
	baseTree, err := currentBaseTree()
	if err != nil {
		return err
	}
	if stagedTree != input.StagedTree || baseTree != input.BaseTree {
		return fmt.Errorf("staged changes or HEAD changed; run aigit prepare again and regenerate the message")
	}
	if stagedTree == baseTree {
		return fmt.Errorf("no staged changes; stage the intended files with git add first")
	}
	return nil
}

func commitAgentMessage(input *agentCommit, message string, stdout, stderr io.Writer) error {
	// Check again after any interactive confirmation, which may take a while.
	if err := checkAgentSnapshot(input); err != nil {
		return err
	}
	cmd := exec.Command("git", "commit", "-F", "-")
	cmd.Stdin = strings.NewReader(message)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("committing changes: %w", err)
	}
	return nil
}

func currentBaseTree() (string, error) {
	tree, err := gitTree("rev-parse", "--verify", "HEAD^{tree}")
	if err == nil {
		return tree, nil
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return "", err
	}
	// An unborn branch has no HEAD; Git computes the right empty tree for the
	// repository's object format (SHA-1 or SHA-256).
	return gitTree("hash-object", "-w", "-t", "tree", "--stdin")
}

func gitTree(args ...string) (string, error) {
	output, err := agentGitOutput(args...)
	return strings.TrimSpace(string(output)), err
}

func agentGitOutput(args ...string) ([]byte, error) {
	output, err := exec.Command("git", args...).Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return output, nil
}
