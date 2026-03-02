package client

import "github.com/charmbracelet/lipgloss"

// 定义矩阵核心色板 (Hex Codes)
const (
	ColorMatrixGreen = "#00FF41"
	ColorDarkGreen   = "#008F11"
	ColorSystemCyan  = "#00FFFF"
	ColorWarningRed  = "#FF0000"
	ColorPrivatePink = "#FF00FF"
)

var (
	// StyleSender：发送者代号样式（加粗的暗绿色）
	StyleSender = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorDarkGreen)).
			Bold(true)

	// StyleChatMsg：普通聊天内容（经典的矩阵亮绿）
	StyleChatMsg = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorMatrixGreen))

	// StyleSystemMsg：系统广播消息（耀眼的青色，带中括号）
	StyleSystemMsg = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorSystemCyan)).
			Italic(true)

	// StyleInputPrompt：输入框前的提示符样式
	StyleInputPrompt = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorMatrixGreen)).
				Bold(true)

	// StyleBorder：上下分屏的边界线
	StyleBorder = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorDarkGreen))

	// StylePrivateMsg：私聊消息样式 (神秘的暗紫色 / 洋红)
	StylePrivateMsg = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorPrivatePink)).
			Italic(true)
)
