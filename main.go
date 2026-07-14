package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime/debug"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"

	"github.com/zzxwill/aigit/llm"
)

var Version = "dev"

func main() {
	var updateNotice <-chan string
	rootCmd := &cobra.Command{
		Use:   "aigit",
		Short: "Generate git commit message including title and body",
		Long:  `AI Git Commi streamlines the git commit process by automatically generating meaningful and standardized commit messages.`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			debug, _ := cmd.Flags().GetBool("debug")
			if debug {
				llm.Debug = true
			}
			updateNotice = startUpdateCheck(Version)
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			select {
			case latest := <-updateNotice:
				if latest == "" {
					return
				}
				fmt.Printf("\n%s %s → %s\n",
					color.YellowString("A new release of aigit is available:"),
					strings.TrimPrefix(Version, "v"), strings.TrimPrefix(latest, "v"))
				prompt := promptui.Select{
					Label: "Upgrade now",
					Items: []string{"Yes", "No"},
					Size:  2,
				}
				// Non-interactive runs fail the prompt; skip silently.
				choice, _, err := prompt.Run()
				if err != nil || choice != 0 {
					return
				}
				fmt.Println("⬆️  Upgrading aigit to", latest, "...")
				if err := selfUpgrade(latest); err != nil {
					color.Red("Upgrade failed: %v", err)
					return
				}
				color.Green("✅ aigit upgraded to %s", latest)
			// Slightly longer than the update check's HTTP timeout so the
			// once-per-day refresh can finish and persist its state file;
			// cached runs deliver instantly.
			case <-time.After(3 * time.Second):
			}
		},
	}

	authCmd := &cobra.Command{
		Use:                   "auth",
		Short:                 "Manage LLM providers and API keys",
		Long:                  `Manage Language Model providers and their API keys. Use subcommands to list, add, or select providers.`,
		DisableFlagsInUseLine: true,
	}

	authListCmd := &cobra.Command{
		Use:                   "list",
		Aliases:               []string{"ls"},
		Short:                 "List configured LLM providers",
		DisableFlagsInUseLine: true,
		Run: func(cmd *cobra.Command, args []string) {
			config := llm.NewConfig()
			if err := config.Load(); err != nil {
				fmt.Printf("Error reading config: %v\n", err)
				exit(ExitConfigLoad)
			}

			fmt.Println("Configured providers:")
			for _, provider := range config.ListProviders() {
				if provider == config.CurrentProvider {
					fmt.Printf("* %s (current)\n", provider)
				} else {
					fmt.Printf("  %s\n", provider)
				}
			}
		},
	}

	authAddCmd := &cobra.Command{
		Use:                   "add <provider> <api_key> [model]",
		Short:                 "Add or update API key for a provider",
		Long:                  "Add or update API key for a provider. Supported providers: openai, gemini, doubao, deepseek, qwen, openai-compatible. For openai-compatible, both model and --base-url are required.",
		DisableFlagsInUseLine: true,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) < 2 {
				color.Red("Not enough arguments")
				color.Red(cmd.Long)
				color.Red("\nUsage: aigit auth add <provider> <api_key> [endpoint_id]")
				exit(ExitInvalidArgs)
			}

			provider := strings.ToLower(args[0])
			apiKey := strings.TrimSpace(args[1])

			config := llm.NewConfig()
			if err := config.Load(); err != nil {
				fmt.Printf("Error reading config: %v\n", err)
				exit(ExitConfigLoad)
			}

			// Validate provider
			switch provider {
			case llm.ProviderOpenAI, llm.ProviderGemini, llm.ProviderDeepseek, llm.ProviderQwen, llm.ProviderDoubao, llm.ProviderOpenAICompatible:
				if err := config.AddProvider(provider, apiKey, args[2:]...); err != nil {
					fmt.Printf("Error saving config: %v\n", err)
					exit(ExitConfigSave)
				}
			default:
				fmt.Printf("Unsupported provider: %s\nSupported providers are: openai, gemini, doubao, deepseek, qwen, openai-compatible\n", provider)
				exit(ExitUnsupportedProvider)
			}

			color.Green("Successfully added API key for %s", provider)

			if provider == llm.ProviderOpenAICompatible {
				baseURL, _ := cmd.Flags().GetString("base-url")
				if baseURL == "" {
					color.Red("--base-url is required for openai-compatible provider")
					exit(ExitUnsupportedProvider)
				}
				p := config.Providers[provider]
				p.BaseURL = baseURL
				config.Providers[provider] = p
				if err := config.Save(); err != nil {
					fmt.Printf("Error saving config: %v\n", err)
					exit(ExitConfigSave)
				}
			}
		},
	}

	authAddCmd.Flags().String("base-url", "", "API base URL (required for openai-compatible provider)")

	authUseCmd := &cobra.Command{
		Use:                   "use [provider]",
		Short:                 "Set the current LLM provider",
		Args:                  cobra.ExactArgs(1),
		DisableFlagsInUseLine: true,
		Run: func(cmd *cobra.Command, args []string) {
			provider := strings.ToLower(args[0])

			config := llm.NewConfig()
			if err := config.Load(); err != nil {
				fmt.Printf("Error reading config: %v\n", err)
				exit(ExitConfigLoad)
			}

			if err := config.UseProvider(provider); err != nil {
				fmt.Printf("Error: %v\n", err)
				exit(ExitUnsupportedProvider)
			}

			color.Green("Now using %s as the current provider", provider)
		},
	}

	authCmd.AddCommand(authListCmd)
	authCmd.AddCommand(authAddCmd)
	authCmd.AddCommand(authUseCmd)
	rootCmd.AddCommand(authCmd)

	commitCmd := &cobra.Command{
		Use:   "commit",
		Short: "Generate git commit message including title and body",
		Run: func(cmd *cobra.Command, args []string) {
			yes, _ := cmd.Flags().GetBool("yes")

			// Execute git diff --cached command
			diffOutput, err := exec.Command("git", "diff", "--cached").Output()
			if err != nil {
				fmt.Printf("Error getting git diff: %v\n", err)
				exit(ExitGitDiff)
			}

			// If there are no staged changes
			if len(diffOutput) == 0 {
				if yes {
					if err := exec.Command("git", "add", ".").Run(); err != nil {
						fmt.Printf("Error staging changes: %v\n", err)
						exit(ExitGitStage)
					}
					color.Green("All changes staged successfully!")
					diffOutput, err = exec.Command("git", "diff", "--cached").Output()
					if err != nil {
						fmt.Printf("Error getting git diff: %v\n", err)
						exit(ExitGitDiff)
					}
				} else {
					color.Yellow("No staged changes found.")
					stagePrompt := promptui.Select{
						Label: "Would you like to run 'git add .' to stage all changes?",
						Items: []string{"Yes", "No"},
						Size:  2,
					}

					_, stageChoice, err := stagePrompt.Run()
					if err != nil {
						fmt.Printf("Error with prompt: %v\n", err)
						exit(ExitPrompt)
					}

					if stageChoice == "Yes" {
						cmd := exec.Command("git", "add", ".")
						if err := cmd.Run(); err != nil {
							fmt.Printf("Error staging changes: %v\n", err)
							exit(ExitGitStage)
						}
						color.Green("All changes staged successfully!")

						diffOutput, err = exec.Command("git", "diff", "--cached").Output()
						if err != nil {
							fmt.Printf("Error getting git diff: %v\n", err)
							exit(ExitGitDiff)
						}
					} else {
						color.Red("No changes staged. Please use 'git add' to stage your changes.")
						exit(ExitUserAbort)
					}
				}
			}

			config := llm.NewConfig()
			if err := config.Load(); err != nil {
				fmt.Printf("Error reading config: %v\n", err)
				exit(ExitConfigLoad)
			}

			var provider string
			if config.CurrentProvider == "" {
				provider = llm.ProviderDoubao
			} else {
				provider = config.CurrentProvider
			}

			// First message generation
			fmt.Println("\n🤖 Generating commit message by", provider)
			var commitMessage string
			commitMessage, err = generateMessage(config, diffOutput)
			if err != nil {
				fmt.Printf("Error generating commit message: %v\n", err)
				exit(ExitLLM)
			}

			if yes {
				if llm.IsRawCommitJSON(commitMessage) {
					color.Yellow("⚠️ Template parsing failed, using raw response")
				}
				cmd := exec.Command("git", "commit", "-m", commitMessage)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					fmt.Printf("Error committing changes: %v\n", err)
					exit(ExitGitCommit)
				}
				fmt.Println("\n" + commitMessage)
				color.Green("✅ Successfully committed changes!")
				return
			}

			for {
				isRaw := llm.IsRawCommitJSON(commitMessage)
				if isRaw {
					color.Yellow("\n⚠️ Template parsing failed, showing raw response:")
				} else {
					fmt.Println("\n📝 Generated commit message:")
				}
				fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
				fmt.Println(commitMessage)
				fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

				items := []string{"Commit this message", "Regenerate message"}
				if isRaw {
					items = []string{"Use this anyway", "Regenerate message"}
				}
				prompt := promptui.Select{
					Label: "Choose an action",
					Items: items,
					Size:  2,
				}

				commitChoice, _, err := prompt.Run()
				if err != nil {
					fmt.Printf("Error with prompt: %v\n", err)
					exit(ExitPrompt)
				}

				switch commitChoice {
				case 0:
					cmd := exec.Command("git", "commit", "-m", commitMessage)
					if err := cmd.Run(); err != nil {
						fmt.Printf("Error committing changes: %v\n", err)
						exit(ExitGitCommit)
					}
					color.Green("\n✅ Successfully committed changes!")

					pushPrompt := promptui.Select{
						Label: "Would you like to push these changes to the remote repository?",
						Items: []string{"No", "Yes"},
						Size:  2,
					}

					_, pushChoice, err := pushPrompt.Run()
					if err != nil {
						fmt.Printf("Error with prompt: %v\n", err)
						exit(ExitPrompt)
					}

					if pushChoice == "Yes" {
						cmd := exec.Command("git", "push", "origin", "HEAD")
						output, err := cmd.CombinedOutput()
						if err != nil {
							color.Red("Error pushing changes: %v\n%s", err, output)
							exit(ExitGitPush)
						}
						fmt.Printf("%s", output)
						color.Green("✅ Successfully pushed changes to remote repository!")
					} else {
						color.Yellow("Changes committed locally. Remember to push when ready.")
					}
					return
				case 1:
					fmt.Println("\n🤖 Regenerating commit message...")
					commitMessage, err = generateMessage(config, diffOutput)
					if err != nil {
						fmt.Printf("Error generating commit message: %v\n", err)
						exit(ExitLLM)
					}
					continue
				default:
					color.Red("Invalid choice")
				}
			}
		},
	}

	commitCmd.Flags().BoolP("yes", "y", false, "Skip all confirmations and commit directly")

	rootCmd.AddCommand(commitCmd)

	versionCmd := &cobra.Command{
		Use:     "version",
		Aliases: []string{"v", "-v", "-version", "--version"},
		Short:   "Print the version of aigit",
		Long:    "Print the current version of the aigit CLI tool.",
		Run: func(cmd *cobra.Command, args []string) {
			if Version != "dev" {
				fmt.Println(Version)
				return
			}

			if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
				fmt.Println(info.Main.Version)
				return
			}

			version, err := exec.Command("git", "describe", "--tags").Output()
			if err != nil {
				fmt.Println("dev")
				return
			}
			fmt.Printf("%s\n", strings.TrimSpace(string(version)))
		},
	}

	rootCmd.PersistentFlags().Bool("debug", false, "enable debug logging")
	rootCmd.AddCommand(versionCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		exit(ExitCommand)
	}
}

func generateMessage(config *llm.Config, diffOutput []byte) (string, error) {
	diff := processBinaryDiff(diffOutput)
	truncated := truncateDiff(diff)

	if llm.Debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] diff: original=%d bytes, after binary removal=%d, after truncation=%d\n",
			len(diffOutput), len(diff), len(truncated))
	}

	generator, err := config.GetMessageGenerator()
	if err != nil {
		return "", fmt.Errorf("error getting message generator: %w", err)
	}
	return generator.GenerateCommitMessage(string(truncated))
}
