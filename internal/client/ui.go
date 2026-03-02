package client

import (
	"encoding/json"
	"fmt"
	"hacker-chat/internal/protocol"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

type msgFromUplink protocol.Message
type tickMsg time.Time

// TerminalModel 是整个终端 UI 的状态机。
type TerminalModel struct {
	uplink    *Uplink         // 网络上行链路
	viewport  viewport.Model  // 上方消息滚动区域
	textInput textinput.Model // 下方输入区域

	messages     []string // 已完整打印的历史消息
	pendingRunes []rune   // 等待逐字打印的字符队列
	typingLine   string   // 当前正在逐字输出的行
	err          error

	// +++ 新增特效控制字段 +++
	inEscapeSeq    bool // 标记当前是否正在读取不可见的颜色代码
	scrambleCycles int  // 当前字符还要经历几次乱码闪烁才能解密
}

// InitialModel 初始化客户端终端界面状态。
func InitialModel(u *Uplink) TerminalModel {
	// 1) 初始化输入框
	ti := textinput.New()
	ti.Placeholder = "Enter to transmit..."
	ti.Prompt = StyleInputPrompt.Render("[SECURE_LINK] >_ ")
	ti.PromptStyle = StyleInputPrompt
	ti.TextStyle = StyleChatMsg
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 50

	// 2) 初始化视口（会在 WindowSizeMsg 中自适应）
	vp := viewport.New(80, 20)
	vp.SetContent(StyleSystemMsg.Render("[SYSTEM] Terminal initialized. Waiting for uplink..."))

	return TerminalModel{
		uplink:       u,
		textInput:    ti,
		viewport:     vp,
		messages:     []string{},
		pendingRunes: []rune{},
	}
}

// Init 在程序启动时执行，注册输入闪烁、网络监听和打字机时钟。
func (m TerminalModel) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		waitForUplink(m.uplink),
		tick(),
	)
}

// Update 是核心事件循环：处理按键、网络消息和定时器滴答。
func (m TerminalModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
		cmds  []tea.Cmd
	)

	switch msg := msg.(type) {
	// 1) 键盘输入事件
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			// 安全断开网络并退出
			m.uplink.Disconnect()
			return m, tea.Quit
		case tea.KeyEnter:
			val := strings.TrimSpace(m.textInput.Value())
			if val == "" {
				return m, nil
			}
			m.textInput.Reset()

			// [核心逻辑] 判断是否为指令 (以 '/' 开头)
			if strings.HasPrefix(val, "/") {
				// 按空格将输入切分为两部分，例如 ["/alias", "Neo"]
				parts := strings.SplitN(val, " ", 2)
				cmdName := strings.ToLower(parts[0]) // 指令本身统一转小写进行匹配

				// 1. 本地指令拦截
				if cmdName == "/clear" {
					m.messages = []string{}
					m.typingLine = ""
					m.viewport.SetContent(StyleSystemMsg.Render("[SYSTEM] Local viewport memory wiped."))
					m.viewport.GotoBottom()
					return m, nil
				}

				// 2. 远程指令转发
				// 把开头的 '/' 去掉后，原封不动地发给服务器（保留了参数的大小写）
				m.uplink.Send <- protocol.Message{
					Type:      protocol.TypeCommand,
					Content:   strings.TrimPrefix(val, "/"),
					Timestamp: time.Now(),
				}
				return m, nil
			}

			// 普通文本消息
			m.uplink.Send <- protocol.NewChatMessage("", val)
		}

	// 2 窗口大小变化，自适应布局
	case tea.WindowSizeMsg:
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 3
		m.viewport.GotoBottom()

	// 3. 处理从服务器推过来的网络消息
	case msgFromUplink:
		// 历史档案批量导入：瞬时渲染，不进入打字机队列
		if msg.Type == protocol.TypeHistory {
			var pastMessages []protocol.Message
			if err := json.Unmarshal([]byte(msg.Content), &pastMessages); err == nil {
				for _, pm := range pastMessages {
					var line string
					if pm.Type == protocol.TypeSystem {
						line = StyleSystemMsg.Render(fmt.Sprintf("[%s]", pm.Content))
					} else {
						sender := StyleSender.Render(fmt.Sprintf("<%s>", pm.Sender))
						content := StyleChatMsg.Render(pm.Content)
						line = fmt.Sprintf("%s %s", sender, content)
					}
					m.messages = append(m.messages, line)
				}
				m.viewport.SetContent(strings.Join(m.messages, "\n"))
				m.viewport.GotoBottom()
			}

			cmds = append(cmds, waitForUplink(m.uplink))
			return m, tea.Batch(cmds...)
		}

		var formattedMsg string

		switch msg.Type {
		case protocol.TypeSystem:
			formattedMsg = StyleSystemMsg.Render(fmt.Sprintf("[%s]", msg.Content))

		// +++ 新增：私聊消息的独特排版 +++
		case protocol.TypePrivate:
			sender := StyleSender.Render(fmt.Sprintf("<%s>", msg.Sender))
			target := StyleSender.Render(fmt.Sprintf("@%s", msg.Target))
			content := StylePrivateMsg.Render(fmt.Sprintf("[ENCRYPTED] %s", msg.Content))

			// 组装格式： <发送者> -> @接收者: [ENCRYPTED] 内容
			formattedMsg = fmt.Sprintf("%s -> %s: %s", sender, target, content)
		// ++++++++++++++++++++++++++++++

		default:
			sender := StyleSender.Render(fmt.Sprintf("<%s>", msg.Sender))
			content := StyleChatMsg.Render(msg.Content)
			formattedMsg = fmt.Sprintf("%s %s", sender, content)
		}

		// 不立即显示，拆成 rune 放入队列，由 tick 逐字打印
		m.pendingRunes = append(m.pendingRunes, []rune(formattedMsg+"\n")...)
		cmds = append(cmds, waitForUplink(m.uplink))

	// 4) 打字机定时器滴答
	// 4. 处理打字机定时器滴答 (灵魂特效)
	case tickMsg:
		// [第一阶段：跳过并吞噬 ANSI 颜色转义字符]
		for len(m.pendingRunes) > 0 {
			char := m.pendingRunes[0]
			if char == '\x1b' {
				m.inEscapeSeq = true // 发现隐藏的颜色代码开头！
			}

			if m.inEscapeSeq {
				// 如果在颜色代码中，直接上屏，不延迟，不乱码
				m.typingLine += string(char)
				m.pendingRunes = m.pendingRunes[1:]
				if char == 'm' {
					m.inEscapeSeq = false // 颜色代码结束，恢复拦截
				}
			} else {
				// 遇到普通可见字符，跳出循环，交给下方的解密引擎处理
				break
			}
		}

		// [第二阶段：对可见字符进行乱码解密特效]
		if len(m.pendingRunes) > 0 {
			char := m.pendingRunes[0]

			if char == '\n' || char == ' ' {
				// 空格和换行不需要解密，直接上屏
				m.typingLine += string(char)
				m.pendingRunes = m.pendingRunes[1:]
				if char == '\n' {
					m.messages = append(m.messages, m.typingLine)
					m.typingLine = ""
				}
				m.scrambleCycles = 0
			} else {
				// 正常字符，进入解密周期
				if m.scrambleCycles > 0 {
					// 1. 还在解密中，递减周期
					m.scrambleCycles--

					// 2. 拔出一个随机乱码符号拼在最后面 (但不真正存入 typingLine)
					randomChar := GetScrambleChar()

					displayContent := strings.Join(m.messages, "\n")
					if m.typingLine != "" || randomChar != "" {
						displayContent += "\n" + m.typingLine + randomChar
					}
					m.viewport.SetContent(displayContent)
					m.viewport.GotoBottom()
				} else {
					// 1. 解密周期结束，真身显现！
					m.typingLine += string(char)
					m.pendingRunes = m.pendingRunes[1:]

					// 2. 为下一个字符随机设定 1~3 次的乱码周期
					m.scrambleCycles = CalculateScrambleCycles()

					displayContent := strings.Join(m.messages, "\n")
					if m.typingLine != "" {
						displayContent += "\n" + m.typingLine
					}
					m.viewport.SetContent(displayContent)
					m.viewport.GotoBottom()
				}
			}
		}
		cmds = append(cmds, tick())
	}

	// 更新子组件状态
	m.textInput, tiCmd = m.textInput.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	cmds = append(cmds, tiCmd, vpCmd)
	return m, tea.Batch(cmds...)
}

// View 将当前状态渲染为终端可显示字符串。
func (m TerminalModel) View() string {
	// 与视口同宽的分隔线
	border := StyleBorder.Render(strings.Repeat("-", m.viewport.Width))

	return fmt.Sprintf(
		"%s\n%s\n%s",
		m.viewport.View(),
		border,
		m.textInput.View(),
	)
}

// waitForUplink 将阻塞的 channel 读取包装成 Bubble Tea 可处理的 Cmd。
func waitForUplink(u *Uplink) tea.Cmd {
	return func() tea.Msg {
		msg := <-u.Receive
		return msgFromUplink(msg)
	}
}

// tick 每 30ms 触发一次打字机滴答事件。
func tick() tea.Cmd {
	return tea.Tick(time.Millisecond*15, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
