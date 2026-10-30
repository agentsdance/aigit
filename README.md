# aigit

[中文文档 (Chinese Documentation)](./README_CN.md) | English Documentation

The most powerful git commit assistant ever!

It's a command-line tool that streamlines the git commit process by automatically generating meaningful and standardized commit messages, including title and body.

`aigit commit` is as simple as `git commit`.

Use your existing LLM API provider, or let the coding agent you're already using
generate the message in its current conversation with the
[aigit skill](skills/aigit/SKILL.md).

## Supported 🤖 AI Providers

| Provider | Default model |
|---|---|
| [OpenAI](https://openai.com/) | `gpt-5.4-mini` |
| [DeepSeek](https://deepseek.com/) | `deepseek-v4-flash-260425` |
| [Doubao (豆包)](https://www.volcengine.com/product/doubao) - Built-in, you don't need to bring your own key | `doubao-seed-2-0-lite-260428` |
| [Gemini](https://gemini.google.com/) | `gemini-3.5-flash` |
| [Qwen (通义千问)](https://www.aliyun.com/product/tongyi) | `qwen-plus` |

## Getting Started

### Installation

#### Option 1: npm (Recommended)

Install the published [@agentsdance/aigit](https://www.npmjs.com/package/@agentsdance/aigit) CLI:

```shell
npm install -g @agentsdance/aigit
aigit version
```

Or run without a global installation:

```shell
npx @agentsdance/aigit commit
```

Requires Node.js 20+ and Git. The npm package downloads the matching release
binary and verifies its SHA256 checksum; no Go toolchain is required. Supports
macOS (Apple Silicon and Intel), Linux x64, and Windows x64. If installation
scripts are disabled, the binary downloads on first use. See [updating aigit](#updating-aigit)
for upgrade instructions.

#### Option 2: Homebrew

```shell
# Add the repository as a tap (use full URL)
brew tap agentsdance/aigit https://github.com/agentsdance/aigit.git

# Install stable version (from releases)
brew install aigit

# Install the current master branch
brew install --HEAD aigit

# Alternative: Install from local formula file
# curl -O https://raw.githubusercontent.com/agentsdance/aigit/master/Formula/aigit.rb
# brew install --formula aigit.rb
```

#### Option 3: Go

Requires Go 1.27.1 or newer.

```shell
go install github.com/zzxwill/aigit@latest
aigit version
```

#### Option 4: Download a Binary

Download the binary for your platform from the [latest release](https://github.com/agentsdance/aigit/releases/latest).
On macOS or Linux, rename it to `aigit` and move it to a directory on your `PATH`, for example:

```shell
chmod +x aigit && sudo mv aigit /usr/local/bin/aigit
```

On Windows, rename `aigit_windows_amd64.exe` to `aigit.exe` and place it in a directory on your `PATH`.

#### Option 5: Build from Source

```shell
git clone https://github.com/agentsdance/aigit.git
cd aigit
go build -o aigit .
sudo mv aigit /usr/local/bin/aigit
```

### Generate commit message

```shell
$ aigit commit

🤖 Generating commit message...

📝 Generated commit message:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
feat(llm): add support for volcengine-go-sdk

This commit adds support for the volcengine-go-sdk for integrating with Doubao LLM service.

The following changes were made:

- Provider type and APIKey field were added to the llm.Config struct.
- generateDoubaoCommitMessage function was updated to use the volcengine-go-sdk.
- The client is initialized with the apiKey and endpointId.
- A prompt is constructed and sent to the CreateChatCompletion API.
- The first choice's message is returned as the commit message.
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🤔 What would you like to do?
1. Commit this message (default)
2. Regenerate message

Enter your choice (press Enter for default):

✅ Successfully committed changes!
```

### Generate with your current coding agent

After [installing the CLI](#installation), install the skill for your coding agent using the
[Skills CLI](https://github.com/vercel-labs/skills):

```shell
npx skills add agentsdance/aigit --skill aigit
```

For a local checkout, use `npx skills add . --skill aigit` instead. The
installer lets you choose the target agents; add `-g` for installation across
projects. The skill installs instructions for your coding agent. Ask your agent to
"Use aigit to generate a commit message" or "Use aigit to commit these changes."

The calling agent generates the message using its current model and session.
aigit collects the staged changes and formats/applies the result. This flow makes
no LLM API calls and starts no additional agent process. Your agent's existing
account and usage limits apply. Any agent that can read the skill and run the CLI
can use the same workflow; no agent-specific adapter or caller detection is needed.
Existing API providers, defaults, and the ordinary `aigit commit` flow still work.

The CLI contract is also available directly:

```shell
git add path/to/intended-file
aigit prepare --language en
```

`prepare` prints JSON with `prompt`, `base_tree`, `staged_tree`, `language`, `emoji`,
and `truncated`. The prompt uses the same commit conventions as API generation.
Only staged changes are included. When `truncated` is true, inspect the remaining
staged diff before describing it. The agent generates a JSON response such as:

```json
{
  "base_tree": "<copy base_tree from prepare>",
  "staged_tree": "<copy staged_tree from prepare>",
  "type": "feat",
  "scope": "auth",
  "description": "add session expiration",
  "body": ["Expire inactive sessions to limit credential reuse"]
}
```

Save the response outside the repository, then preview or apply it:

```shell
aigit commit --from-agent /path/to/response.json --dry-run
aigit commit --from-agent /path/to/response.json --yes
# Use --from-agent - to read the same JSON from stdin.
```

Without `--yes` or `--dry-run`, aigit asks for commit confirmation. `body` must be
an array of strings. If HEAD's tree or the staged changes differ from the prepared
snapshot, aigit refuses the response: run `prepare` again and regenerate it. Agent
mode never stages or pushes, preserves normal Git hooks, and skips automatic
update checks. Select the message language during `prepare`; rendering respects
the configured emoji preference.

### Generate commit message with your own AI API Key

```shell
$ aigit auth add gemini AIzaSyCb56bjWn02e2v4s_TxHMDnHbSJQSx_tu8
Successfully added API key for gemini

$ aigit auth add doubao 6e3e438c-a380-4ed5-b597-e01cb82bc4df
Successfully added API key for doubao

$ aigit auth ls
Configured providers:
  gemini(gemini-3.5-flash) *default
  doubao(doubao-seed-2-0-lite-260428)

$ aigit commit

🤖 Generating commit message...

📝 Generated commit message:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
feat(llm): add support for volcengine-go-sdk

This commit adds support for the volcengine-go-sdk for integrating with Doubao LLM service.

The following changes were made:

- Provider type and APIKey field were added to the llm.Config struct.
- generateDoubaoCommitMessage function was updated to use the volcengine-go-sdk.
- The client is initialized with the apiKey and endpointId.
- A prompt is constructed and sent to the CreateChatCompletion API.
- The first choice's message is returned as the commit message.
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🤔 What would you like to do?
1. Commit this message (default)
2. Regenerate message

Enter your choice (press Enter for default): 2

🤖 Regenerating commit message...

📝 Generated commit message:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
feat(llm): add support for volcengine-go-sdk

This commit adds support for the volcengine-go-sdk for integrating with Doubao LLM service.

The following changes were made:

- Provider type and APIKey field were added to the llm.Config struct.
- generateDoubaoCommitMessage function was updated to use the volcengine-go-sdk.
- The client is initialized with the apiKey and endpointId.
- A prompt is constructed and sent to the CreateChatCompletion API.
- The first choice's message is returned as the commit message.
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🤔 What would you like to do?
1. Commit this message (default)
2. Regenerate message

Enter your choice (press Enter for default): 1

✅ Successfully committed changes!

```

For Doubao, the provider uses the built-in default model. You can optionally pass an
[Ark inference endpoint ID](https://www.volcengine.com/docs/82379/1099522) (`ep-xxx`) or a model ID to override it:

```shell
$ aigit auth add doubao <api_key> ep-20250110202503-fdkgq
```

## Updating aigit

### npm installations

```shell
npm install -g @agentsdance/aigit@latest
aigit version
```

npm manages updates for this installation. The npm launcher disables the CLI's
built-in update prompt, which otherwise requires Homebrew or a Go toolchain.

### Homebrew and other installations

aigit checks for new releases in the background (at most once every 24 hours, cached
in `~/.aigit/update-check.json`). When a newer version is available, it asks after the
command finishes whether to upgrade; if you agree, aigit upgrades itself in place
(via Homebrew for brew installs, otherwise by building the release from source).
Set `AIGIT_NO_UPDATE_CHECK=1` to disable the check.

## Contributors

Thanks to everyone who has contributed to aigit.

<table>
  <tr>
    <td align="center"><a href="https://github.com/zzxwill"><img src="https://avatars.githubusercontent.com/u/2805315?v=4" width="80" alt="zzxwill" /><br />zzxwill</a></td>
    <td align="center"><a href="https://github.com/double12gzh"><img src="https://avatars.githubusercontent.com/u/2534467?v=4" width="80" alt="Jeffrey Guan" /><br />Jeffrey Guan</a></td>
    <td align="center"><a href="https://github.com/bolechen"><img src="https://avatars.githubusercontent.com/u/195015?v=4" width="80" alt="Bole Chen" /><br />Bole Chen</a></td>
    <td align="center"><a href="https://github.com/YouCD"><img src="https://avatars.githubusercontent.com/u/24710351?v=4" width="80" alt="You ChangDing" /><br />You ChangDing</a></td>
    <td align="center"><a href="https://github.com/sanjishi"><img src="https://avatars.githubusercontent.com/u/42719702?v=4" width="80" alt="sanjishi" /><br />sanjishi</a></td>
  </tr>
</table>
