# aigit

[English Documentation](README.md) | 中文文档

最强大的 Git 提交助手！

既可以继续使用现有的大模型 API，也可以通过
[aigit 技能](skills/aigit/SKILL.md)，让当前使用的代码智能体在当前对话中生成提交信息。

## 支持的大模型/AI

| 提供商 | 默认模型 |
|---|---|
| [OpenAI](https://openai.com/) | `gpt-5.4-mini` |
| [DeepSeek](https://deepseek.com/) | `deepseek-v4-flash-260425` |
| [Doubao (豆包)](https://www.volcengine.com/product/doubao) - 内置，您不需要自己携带 API Key | `doubao-seed-2-0-lite-260428` |
| [Gemini](https://gemini.google.com/) | `gemini-3.5-flash` |
| [Qwen (通义千问)](https://www.aliyun.com/product/tongyi) | `qwen-plus` |

## 快速开始

### 安装

#### 选项 1：npm（推荐）

安装已发布的 [@agentsdance/aigit](https://www.npmjs.com/package/@agentsdance/aigit) 命令行工具：

```shell
npm install -g @agentsdance/aigit
aigit version
```

无需全局安装也可以运行：

```shell
npx @agentsdance/aigit commit
```

需要 Node.js 20+ 和 Git，无需安装 Go。npm 包会下载对应版本的预编译程序并
校验 SHA256，支持 macOS（Apple Silicon 和 Intel）、Linux x64 和 Windows x64。
禁用安装脚本时，会在首次运行时下载。升级方式见[更新 aigit](#更新-aigit)。

#### 选项 2：Homebrew

```shell
# 将仓库添加为tap（使用完整URL）
brew tap agentsdance/aigit https://github.com/agentsdance/aigit.git

# 安装稳定版本（从releases）
brew install aigit

# 安装 master 分支的当前代码
brew install --HEAD aigit

# 备选方案：从本地formula文件安装
# curl -O https://raw.githubusercontent.com/agentsdance/aigit/master/Formula/aigit.rb
# brew install --formula aigit.rb
```

#### 选项 3：Go

需要 Go 1.27.1 或更新版本。

```shell
go install github.com/zzxwill/aigit@latest
aigit version
```

#### 选项 4：下载二进制文件

前往[最新发布页面](https://github.com/agentsdance/aigit/releases/latest)下载适合您平台的二进制文件。
在 macOS 或 Linux 上，将其重命名为 `aigit`，并移到 `PATH` 中的目录，例如：

```shell
chmod +x aigit && sudo mv aigit /usr/local/bin/aigit
```

在 Windows 上，将 `aigit_windows_amd64.exe` 重命名为 `aigit.exe`，并放入 `PATH` 中的目录。

#### 选项 5：从源码构建

```shell
git clone https://github.com/agentsdance/aigit.git
cd aigit
go build -o aigit .
sudo mv aigit /usr/local/bin/aigit
```

### 生成提交信息

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

### 使用当前代码智能体生成提交信息

[安装 aigit CLI](#安装) 后，通过 [Skills CLI](https://github.com/vercel-labs/skills) 为代码智能体安装技能：

```shell
npx skills add agentsdance/aigit --skill aigit
```

本地仓库可以使用 `npx skills add . --skill aigit`。安装器会让您选择目标智能体，
加上 `-g` 可以跨项目使用。技能会安装供代码智能体使用的指令。接着告诉智能体：
“使用 aigit 生成提交信息”或“使用 aigit 提交这些修改”。

技能由调用它的智能体在当前会话中生成信息，使用该智能体现有的模型、账号和额度。
aigit 负责收集暂存区差异、格式化信息并执行提交，不会调用大模型 API，也不会启动另一个智能体。
能读取技能并执行 CLI 的智能体都可以使用这套流程，无需识别调用方或配置专用适配器。
原有 API 提供商、默认配置和 `aigit commit` 流程继续保留。

也可以直接使用 CLI 协议：

```shell
git add path/to/intended-file
aigit prepare --language zh
```

`prepare` 输出 JSON，包含 `prompt`、`base_tree`、`staged_tree`、`language`、`emoji`
和 `truncated`。提示词沿用现有提交规范，只包含已暂存的修改。若 `truncated` 为 true，
智能体应先检查被截断的暂存区差异。智能体按提示词生成下面的 JSON，并原样复制两个 tree 字段：

```json
{
  "base_tree": "<prepare 返回的 base_tree>",
  "staged_tree": "<prepare 返回的 staged_tree>",
  "type": "feat",
  "scope": "auth",
  "description": "添加会话过期机制",
  "body": ["让不活跃的会话过期，限制凭据重复使用"]
}
```

将响应保存在仓库之外，然后预览或提交：

```shell
aigit commit --from-agent /path/to/response.json --dry-run
aigit commit --from-agent /path/to/response.json --yes
# 使用 --from-agent - 从标准输入读取同样的 JSON。
```

不加 `--yes` 或 `--dry-run` 时会询问是否提交。`body` 必须为字符串数组。
若 HEAD 的 tree 或暂存区快照发生变化，aigit 会拒绝旧响应，需重新执行 `prepare` 并生成信息。
智能体模式不会自动暂存或推送，会保留 Git hooks，并跳过自动更新检查。
语言通过 `prepare` 选择，格式化时沿用已配置的 emoji 偏好。

### 使用自己的 AI API Key 生成提交信息

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

豆包提供商默认使用内置模型。您也可以选择传入[方舟推理接入点 ID](https://www.volcengine.com/docs/82379/1099522)（`ep-xxx`）或模型 ID 来覆盖默认模型：

```shell
$ aigit auth add doubao <api_key> ep-20250110202503-fdkgq
```

## 更新 aigit

### npm 安装

```shell
npm install -g @agentsdance/aigit@latest
aigit version
```

通过 npm 管理此安装方式的更新。npm 启动器会禁用 CLI 内置的升级提示，
该提示原本会通过 Homebrew 或 Go 工具链完成升级。

### Homebrew 和其他安装方式

aigit 会在后台检查新版本（每 24 小时最多一次，结果缓存在 `~/.aigit/update-check.json`）。
当有新版本可用时，会在命令执行结束后询问是否升级；如果您同意，aigit 会自动完成升级
（Homebrew 安装的版本通过 brew 升级，其他安装方式则从源码构建新版本并原地替换）。
设置 `AIGIT_NO_UPDATE_CHECK=1` 可以禁用该功能。

## 贡献者

感谢所有为 aigit 做出贡献的人。

<table>
  <tr>
    <td align="center"><a href="https://github.com/zzxwill"><img src="https://avatars.githubusercontent.com/u/2805315?v=4" width="80" alt="zzxwill" /><br />zzxwill</a></td>
    <td align="center"><a href="https://github.com/double12gzh"><img src="https://avatars.githubusercontent.com/u/2534467?v=4" width="80" alt="Jeffrey Guan" /><br />Jeffrey Guan</a></td>
    <td align="center"><a href="https://github.com/bolechen"><img src="https://avatars.githubusercontent.com/u/195015?v=4" width="80" alt="Bole Chen" /><br />Bole Chen</a></td>
    <td align="center"><a href="https://github.com/YouCD"><img src="https://avatars.githubusercontent.com/u/24710351?v=4" width="80" alt="You ChangDing" /><br />You ChangDing</a></td>
    <td align="center"><a href="https://github.com/sanjishi"><img src="https://avatars.githubusercontent.com/u/42719702?v=4" width="80" alt="sanjishi" /><br />sanjishi</a></td>
  </tr>
</table>
