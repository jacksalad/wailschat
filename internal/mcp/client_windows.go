//go:build windows

package mcp

import (
	"os/exec"
	"syscall"
)

// buildCommand 构建执行命令 (Windows版本)
func (c *Client) buildCommand(command string, env map[string]string) *exec.Cmd {
	parts := parseCommandLine(command)
	if len(parts) == 0 {
		return nil
	}

	var cmd *exec.Cmd
	if len(parts) > 1 {
		cmd = exec.Command(parts[0], parts[1:]...)
	} else {
		cmd = exec.Command(parts[0])
	}

	// 在Windows上隐藏控制台窗口
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}

	// 添加环境变量
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	return cmd
}
