package server

import (
	"hacker-chat/internal/protocol"
	"log"
)

// Hub 维护所有活跃的客户端连接，并处理消息的广播分发
type Hub struct {
	// clients 存储所有在线的客户端。
	// 使用 map 的键来存储指针，布尔值仅作为占位符，保证 O(1) 的超快增删查。
	clients map[*Client]bool

	// broadcast 是广播通道。当 Hub 从这个通道收到消息时，会立刻推给所有 clients。
	broadcast chan protocol.Message

	// register 是注册通道。新客户端连接时，通过此通道通知 Hub。
	register chan *Client

	// unregister 是注销通道。客户端断开连接时，通过此通道通知 Hub 清理资源。
	unregister chan *Client
}

// NewHub 实例化并返回一个准备就绪的矩阵中枢
func NewHub() *Hub {
	return &Hub{
		broadcast:  make(chan protocol.Message),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

// Run 是 Hub 的引擎，必须在一个独立的 Goroutine 中永远运行 (死循环)
func (h *Hub) Run() {
	log.Println("[SYSTEM] Matrix C2 Hub Engine Initialized. Awaiting node connections...")

	for {
		// select 是 Go 并发编程的魔法。
		// 它会阻塞在这里，直到下面三个 channel 中有任何一个收到了数据。
		// 因为所有对 clients map 的操作都在这一个单独的 goroutine 里串行执行，
		// 所以我们完全不需要使用 sync.Mutex 加锁，天生并发安全！
		select {

		case client := <-h.register:
			// 触发：有新连接加入
			h.clients[client] = true
			log.Printf("[HUB] Secure node linked. Active connections: %d\n", len(h.clients))

		case client := <-h.unregister:
			// 触发：有连接断开（主动退出或网络异常）
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send) // 关闭该客户端的消息发送通道，防止内存泄漏
				log.Printf("[HUB] Node connection lost. Active connections: %d\n", len(h.clients))
			}

		case message := <-h.broadcast:
			// 广播前先将消息归档到 SQLite
			SaveMessage(message)

			// 遍历所有在线客户端
			for client := range h.clients {

				// +++ 新增：精准路由拦截 +++
				// 如果这是一条私聊消息，且当前遍历到的客户端 既不是发送者 也不是接收者，则跳过不发
				if message.Type == protocol.TypePrivate {
					if client.alias != message.Target && client.alias != message.Sender {
						continue
					}
				}
				// +++++++++++++++++++++++++

				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}
