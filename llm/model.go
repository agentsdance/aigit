package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/google/generative-ai-go/genai"
	"github.com/openai/openai-go"
	openaioption "github.com/openai/openai-go/option"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
	"google.golang.org/api/option"
)

const (
	ProviderOpenAI           = "openai"
	ProviderGemini           = "gemini"
	ProviderDoubao           = "doubao"
	ProviderDeepseek         = "deepseek"
	ProviderQwen             = "qwen"
	ProviderOpenAICompatible = "openai-compatible"

	// Model constants
	geminiModel   = "gemini-3.5-flash"
	deepseekModel = "deepseek-v4-flash-260425"
	openaiModel   = "gpt-5.4-mini"
	qwenModel     = "qwen-plus"
	doubaoModel   = "doubao-seed-2-0-lite-260428"
)

const (
	DefaultAPIKey   = "YzAzZDIyN2ItMWFkNS00MDNkLWJkM2YtZjgzNzczOWE4YzFj"
	DefaultEndpoint = "ZXAtMjAyNTAxMTMyMzE5NTEtOTJ4bjI="
)

const englishPrompt = `Analyze the git diff and output ONLY a valid JSON object with these fields:
- "type": commit type from the list below
- "scope": affected component (optional, empty string if none)
- "description": short imperative description (max 72 chars)
- "body": detailed explanation with bullet points if needed

Type selection (by priority):
BREAKING CHANGE: API breaking changes
fix: Bug fixes, crashes, errors, security issues
feat: New features, APIs, capabilities
perf: Performance improvements
refactor: Code restructuring without functional changes
docs: Documentation only
style: Formatting, whitespace, imports
test: Test changes
build: Build system, dependencies
ci: CI/CD changes
chore: Maintenance, version updates

Rules:
- Body should explain WHAT and WHY, not HOW
- Each line should be less than 72 characters

Example: {"type":"feat","scope":"auth","description":"add user login endpoint","body":"- implement JWT token generation\n- add password hashing"}

IMPORTANT: Output ONLY the JSON object. No other text.

Diff:`

const chinesePrompt = `分析 git diff，只输出一个有效的 JSON 对象，包含以下字段：
- "type": 提交类型，从下方列表选择
- "scope": 影响范围（可选，没有则填空字符串）
- "description": 简短的描述
- "body": 详细的说明，可以用列表

类型选择（按优先级）：
BREAKING CHANGE: API 不兼容变更
fix: 修复 bug、崩溃、错误、安全问题
feat: 新功能、API、能力
perf: 性能改进
refactor: 代码重构，无功能变化
docs: 仅文档变更
style: 格式化、空白、导入
test: 测试变更
build: 构建系统、依赖
ci: CI/CD 变更
chore: 维护、版本更新

规则：
- body 说明 WHAT 和 WHY，不是 HOW
- 每行不超过 72 个字符
- description 和 body 必须用中文

示例：{"type":"feat","scope":"auth","description":"添加用户登录接口","body":"- 实现 JWT token 生成\n- 添加密码哈希中间件"}

重要：只输出 JSON 对象，不要其他文字。

Diff:`

func getPrompt(language string, _ bool) string {
	if language == "zh" {
		return chinesePrompt
	}
	return englishPrompt
}

type bodyField []string

func (b *bodyField) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	if data[0] == '[' {
		return json.Unmarshal(data, (*[]string)(b))
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*b = []string{s}
	return nil
}

type CommitData struct {
	Type        string    `json:"type"`
	Scope       string    `json:"scope"`
	Description string    `json:"description"`
	Body        bodyField `json:"body"`
}

func emojiForType(t string) string {
	switch t {
	case "feat":
		return "✨"
	case "fix":
		return "🐛"
	case "docs":
		return "📝"
	case "style":
		return "🎨"
	case "refactor":
		return "♻️"
	case "perf":
		return "⚡️"
	case "test":
		return "✅"
	case "build":
		return "🔧"
	case "ci":
		return "🚀"
	case "chore":
		return "🔖"
	default:
		return ""
	}
}

const commitTemplateText = `{{if .Emoji}}{{.Emoji}} {{end}}{{.Type}}{{if .Scope}}({{.Scope}}){{end}}: {{.Description}}{{if .Body}}

{{.Body}}{{end}}`

type commitTemplateData struct {
	Type        string
	Scope       string
	Description string
	Body        string
	Emoji       string
}

func renderCommitMessage(data *CommitData, useEmoji bool) string {
	body := ""
	if len(data.Body) > 0 {
		body = strings.Join(data.Body, "\n")
	}
	tmplData := commitTemplateData{
		Type:        data.Type,
		Scope:       data.Scope,
		Description: data.Description,
		Body:        body,
	}
	if useEmoji {
		tmplData.Emoji = emojiForType(data.Type)
	}
	tmpl := template.Must(template.New("commit").Parse(commitTemplateText))
	var buf strings.Builder
	if err := tmpl.Execute(&buf, tmplData); err != nil {
		return data.Description
	}
	return buf.String()
}

func parseAndRenderCommit(raw string, useEmoji bool) (string, error) {
	data, _ := extractCommitData(raw)
	if data == nil {
		return strings.TrimSpace(raw), nil
	}
	return renderCommitMessage(data, useEmoji), nil
}

func extractCommitData(raw string) (*CommitData, error) {
	cleaned := raw
	if idx := strings.Index(cleaned, "```"); idx >= 0 {
		cleaned = cleaned[idx+3:]
		if endIdx := strings.LastIndex(cleaned, "```"); endIdx >= 0 {
			cleaned = cleaned[:endIdx]
		}
	}
	cleaned = strings.TrimSpace(cleaned)
	cleaned = strings.TrimPrefix(cleaned, "json")
	cleaned = strings.TrimSpace(cleaned)

	var data CommitData
	if err := json.Unmarshal([]byte(cleaned), &data); err != nil {
		return nil, err
	}
	if data.Type == "" || data.Description == "" {
		return nil, fmt.Errorf("missing required fields")
	}
	return &data, nil
}

var (
	_ MessageGenerator = (*DefaultGenerator)(nil)
	_ MessageGenerator = (*GeminiGenerator)(nil)
	_ MessageGenerator = (*OpenAIGenerator)(nil)
	_ MessageGenerator = (*DoubaoGenerator)(nil)
	_ MessageGenerator = (*DeepseekGenerator)(nil)
	_ MessageGenerator = (*QwenGenerator)(nil)
	_ MessageGenerator = (*OpenAICompatibleGenerator)(nil)
)

// MessageGenerator Define a commit message generator
type MessageGenerator interface {
	GenerateCommitMessage(diff string) (string, error)
}

type DefaultGenerator struct {
	MessageGenerator MessageGenerator
}

func NewDefauleGenerator(language ...string) (*DefaultGenerator, error) {
	apiKey, err := base64.StdEncoding.DecodeString(DefaultAPIKey)
	if err != nil {
		return nil, fmt.Errorf("error decoding API key: %w", err)
	}
	endpoint, err := base64.StdEncoding.DecodeString(DefaultEndpoint)
	if err != nil {
		return nil, fmt.Errorf("error decoding endpoint: %w", err)
	}
	lang := ""
	emoji := true
	timeout := 60
	if len(language) > 0 {
		lang = language[0]
	}
	generator := &DefaultGenerator{
		MessageGenerator: NewDoubaoGenerator(string(apiKey), string(endpoint), lang, emoji, timeout),
	}

	return generator, nil
}

func (d *DefaultGenerator) GenerateCommitMessage(diff string) (string, error) {
	return d.MessageGenerator.GenerateCommitMessage(diff)
}

// GeminiGenerator Implemention Gemini provider
type GeminiGenerator struct {
	apiKey   string
	language string
	emoji    bool
	timeout  int
}

func NewGeminiGenerator(apiKey, language string, emoji bool, timeout int) *GeminiGenerator {
	return &GeminiGenerator{apiKey: apiKey, language: language, emoji: emoji, timeout: timeout}
}

func (g *GeminiGenerator) GenerateCommitMessage(diff string) (string, error) {
	return generateGeminiCommitMessage(diff, g.apiKey, g.language, g.emoji)
}

// OpenAIGenerator Implemention OpenAI provider
type OpenAIGenerator struct {
	apiKey   string
	language string
	emoji    bool
	timeout  int
}

func NewOpenAIGenerator(apiKey, language string, emoji bool, timeout int) *OpenAIGenerator {
	return &OpenAIGenerator{apiKey: apiKey, language: language, emoji: emoji, timeout: timeout}
}

func (g *OpenAIGenerator) GenerateCommitMessage(diff string) (string, error) {
	return generateOpenAICommitMessage(diff, g.apiKey, g.language, g.emoji)
}

// DoubaoGenerator Implemention Doubao provider
type DoubaoGenerator struct {
	apiKey   string
	endpoint string
	language string
	emoji    bool
	timeout  int
}

func NewDoubaoGenerator(apiKey, endpoint, language string, emoji bool, timeout int) *DoubaoGenerator {
	return &DoubaoGenerator{apiKey: apiKey, endpoint: endpoint, language: language, emoji: emoji, timeout: timeout}
}

func (g *DoubaoGenerator) GenerateCommitMessage(diff string) (string, error) {
	return generateDoubaoCommitMessage(diff, g.apiKey, g.endpoint, g.language, g.emoji)
}

// DeepseekGenerator Implemention Deepseek provider
type DeepseekGenerator struct {
	apiKey   string
	language string
	emoji    bool
	timeout  int
}

func NewDeepseekGenerator(apiKey, language string, emoji bool, timeout int) *DeepseekGenerator {
	return &DeepseekGenerator{apiKey: apiKey, language: language, emoji: emoji, timeout: timeout}
}

func (g *DeepseekGenerator) GenerateCommitMessage(diff string) (string, error) {
	return generateDeepseekCommitMessage(diff, g.apiKey, g.language, g.emoji)
}

// QwenGenerator Implemention Qwen provider
type QwenGenerator struct {
	apiKey   string
	language string
	emoji    bool
	timeout  int
}

func NewQwenGenerator(apiKey, language string, emoji bool, timeout int) *QwenGenerator {
	return &QwenGenerator{apiKey: apiKey, language: language, emoji: emoji, timeout: timeout}
}

func (g *QwenGenerator) GenerateCommitMessage(diff string) (string, error) {
	return generateQwenCommitMessage(diff, g.apiKey, g.language, g.emoji, g.timeout)
}

// OpenAICompatibleGenerator Implementation for OpenAI-compatible providers (e.g. Groq, Together AI, OpenRouter)
type OpenAICompatibleGenerator struct {
	apiKey   string
	model    string
	baseURL  string
	language string
	emoji    bool
	timeout  int
}

func NewOpenAICompatibleGenerator(apiKey, model, baseURL, language string, emoji bool, timeout int) *OpenAICompatibleGenerator {
	return &OpenAICompatibleGenerator{apiKey: apiKey, model: model, baseURL: baseURL, language: language, emoji: emoji, timeout: timeout}
}

func (g *OpenAICompatibleGenerator) GenerateCommitMessage(diff string) (string, error) {
	return generateOpenAICompatibleCommitMessage(diff, g.apiKey, g.model, g.baseURL, g.language, g.emoji, g.timeout)
}

func generateGeminiCommitMessage(diff, apiKey, language string, emoji bool) (string, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return "", fmt.Errorf("creating Gemini client: %w", err)
	}
	defer client.Close()

	model := client.GenerativeModel(geminiModel)
	prompt := fmt.Sprintf("%s\n%s", getPrompt(language, emoji), diff)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("generating commit message: %w", err)
	}

	if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
		generatedMessage := resp.Candidates[0].Content.Parts[0].(genai.Text)
		return parseAndRenderCommit(strings.TrimSpace(string(generatedMessage)), emoji)
	}

	return "", fmt.Errorf("no commit message generated by Gemini")
}

func generateOpenAICommitMessage(diff, apiKey, language string, emoji bool) (string, error) {
	client := openai.NewClient(
		openaioption.WithAPIKey(apiKey),
	)
	ctx := context.Background()
	prompt := fmt.Sprintf("%s\n%s", getPrompt(language, emoji), diff)

	chatCompletion, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: openai.F([]openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(prompt),
		}),
		Model: openai.F(openai.ChatModel(openaiModel)),
	})
	if err != nil {
		return "", fmt.Errorf("generating commit message: %w", err)
	}

	if len(chatCompletion.Choices) > 0 {
		return parseAndRenderCommit(strings.TrimSpace(chatCompletion.Choices[0].Message.Content), emoji)
	}

	return "", fmt.Errorf("no commit message generated by OpenAI")
}

func generateDoubaoCommitMessage(diff, apiKey, endpointID, language string, emoji bool) (string, error) {
	// An Ark endpoint ID (ep-xxx) or model ID may be supplied; fall back to
	// the default model when neither is configured.
	if endpointID == "" {
		endpointID = doubaoModel
	}

	client := arkruntime.NewClientWithApiKey(
		apiKey,
	)

	ctx := context.Background()

	prompt := fmt.Sprintf("%s\n%s", getPrompt(language, emoji), diff)

	req := model.ChatCompletionRequest{
		Model: endpointID,
		Messages: []*model.ChatCompletionMessage{
			{
				Role: model.ChatMessageRoleSystem,
				Content: &model.ChatCompletionMessageContent{
					StringValue: volcengine.String("你是豆包，是由字节跳动开发的 AI 人工智能助手，你非常擅长生成 git commit message"),
				},
			},
			{
				Role: model.ChatMessageRoleUser,
				Content: &model.ChatCompletionMessageContent{
					StringValue: volcengine.String(prompt),
				},
			},
		},
	}

	resp, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", err
	}
	return parseAndRenderCommit(*resp.Choices[0].Message.Content.StringValue, emoji)
}

func generateDeepseekCommitMessage(diff, apiKey, language string, emoji bool) (string, error) {
	client := &http.Client{}
	ctx := context.Background()
	prompt := fmt.Sprintf("%s\n%s", getPrompt(language, emoji), diff)

	reqBody := map[string]any{
		"model": deepseekModel,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You are Deepseek, an AI assistant specialized in generating git commit messages.",
			},
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"stream": false,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshaling request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.deepseek.com/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decoding response: %w", err)
	}

	if choices, ok := result["choices"].([]any); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]any); ok {
			if message, ok := choice["message"].(map[string]any); ok {
				if content, ok := message["content"].(string); ok {
					return parseAndRenderCommit(content, emoji)
				}
			}
		}
	}

	return "", fmt.Errorf("invalid response format from Deepseek: %v", result)
}

func generateQwenCommitMessage(diff, apiKey, language string, emoji bool, timeout int) (string, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		apiKey = os.Getenv("DASHSCOPE_API_KEY")
	}

	if apiKey == "" {
		return "", fmt.Errorf("API key is empty: please provide an API key via configuration or DASHSCOPE_API_KEY environment variable")
	}

	client := openai.NewClient(
		openaioption.WithAPIKey(apiKey),
		openaioption.WithBaseURL("https://dashscope.aliyuncs.com/compatible-mode/v1/"),
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	prompt := fmt.Sprintf("%s\n%s", getPrompt(language, emoji), diff)

	chatCompletion, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: openai.F([]openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(prompt),
		}),
		Model: openai.F(qwenModel),
	})
	if err != nil {
		return "", fmt.Errorf("generating commit message: %w", err)
	}

	if len(chatCompletion.Choices) > 0 {
		return parseAndRenderCommit(strings.TrimSpace(chatCompletion.Choices[0].Message.Content), emoji)
	}

	return "", fmt.Errorf("no commit message generated by Qwen")
}

func generateOpenAICompatibleCommitMessage(diff, apiKey, model, baseURL, language string, emoji bool, timeout int) (string, error) {
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	client := openai.NewClient(
		openaioption.WithAPIKey(apiKey),
		openaioption.WithBaseURL(baseURL),
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	prompt := fmt.Sprintf("%s\n%s", getPrompt(language, emoji), diff)

	chatCompletion, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: openai.F([]openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(prompt),
		}),
		Model: openai.F(openai.ChatModel(model)),
	})
	if err != nil {
		return "", fmt.Errorf("generating commit message: %w", err)
	}

	if len(chatCompletion.Choices) > 0 {
		return parseAndRenderCommit(strings.TrimSpace(chatCompletion.Choices[0].Message.Content), emoji)
	}

	return "", fmt.Errorf("no commit message generated by OpenAI-compatible provider")
}
