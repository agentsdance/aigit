package main

import "os"

type ExitCode int

const (
	// 成功退出
	ExitOK ExitCode = 0
	// 通用错误（未分类）
	ExitError ExitCode = 1
	// 配置文件加载失败
	ExitConfigLoad ExitCode = 2
	// 配置文件保存失败
	ExitConfigSave ExitCode = 3
	// 命令行参数无效（数量不足、格式错误等）
	ExitInvalidArgs ExitCode = 4
	// 不支持的 provider 或缺少必要参数
	ExitUnsupportedProvider ExitCode = 5
	// git diff 执行失败
	ExitGitDiff ExitCode = 6
	// git add 执行失败
	ExitGitStage ExitCode = 7
	// git commit 执行失败
	ExitGitCommit ExitCode = 8
	// git push 执行失败
	ExitGitPush ExitCode = 9
	// LLM API 调用失败
	ExitLLM ExitCode = 10
	// 交互式 prompt 出错
	ExitPrompt ExitCode = 11
	// 用户主动取消或拒绝操作
	ExitUserAbort ExitCode = 12
	// cobra 命令执行错误
	ExitCommand ExitCode = 13
)

func exit(code ExitCode) {
	os.Exit(int(code))
}
