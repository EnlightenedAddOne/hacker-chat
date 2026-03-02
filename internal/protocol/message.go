package protocol

import "time"

// 定义消息类型的枚举常量
// 这样可以避免在代码里硬编码字符串，减少拼写错误
const (
	TypeSystem  = "SYSTEM"  // 系统通知（例如：[SYS] Node Trinity connected.）
	TypeChat    = "CHAT"    // 普通终端聊天信息
	TypeCommand = "COMMAND" // 预留给未来：处理特殊指令（如 /clear, /whoami）
	TypePrivate = "PRIVATE" // +++ 新增：私有加密频道类型
	TypeHistory = "HISTORY" // 历史档案下发
)

// Message 是在 WebSocket 中传输的核心数据包
type Message struct {
	// Type 决定了客户端/服务端收到消息后该怎么处理它
	Type string `json:"type"`

	// Sender 是用户的代号（比如 "Neo" 或 "192.168.0.1"）
	// 系统消息的 Sender 通常设定为 "SERVER" 或 "MATRIX"
	Sender string `json:"sender"`

	Target string `json:"target,omitempty"` // +++ 新增：目标节点的代号 (omitempty表示如果没有目标则忽略)

	// Content 是真正的文字内容
	Content string `json:"content"`

	// Timestamp 用于客户端排序或在消息前缀显示时间戳
	Timestamp time.Time `json:"timestamp"`
}

// NewSystemMessage 是一个辅助函数，用来快速生成系统消息
func NewSystemMessage(content string) Message {
	return Message{
		Type:      TypeSystem,
		Sender:    "MATRIX_HUB",
		Content:   content,
		Timestamp: time.Now(),
	}
}

// NewChatMessage 是一个辅助函数，用来快速生成聊天消息
func NewChatMessage(sender, content string) Message {
	return Message{
		Type:      TypeChat,
		Sender:    sender,
		Content:   content,
		Timestamp: time.Now(),
	}
}
