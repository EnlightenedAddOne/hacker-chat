package server

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"hacker-chat/internal/config"
	"hacker-chat/internal/protocol"

	"github.com/gorilla/websocket"
)

var ServerSecret = config.Passphrase // WebSocket 握手认证密钥

const (
	// 定义一些极客风的超时常量
	writeWait      = 10 * time.Second    // 写入数据的超时时间
	pongWait       = 60 * time.Second    // 等待客户端心跳响应的超时时间
	pingPeriod     = (pongWait * 9) / 10 // 发送心跳包的周期（必须小于 pongWait）
	maxMessageSize = 1024                // 限制单个 Payload 大小，防止恶意 DDOS 攻击
)

// upgrader 用于将普通的 HTTP 请求“跃迁”为 WebSocket 全双工通道
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// 在开发阶段，允许所有跨域请求连入矩阵
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Client 代表一个连接到矩阵的终端节点
type Client struct {
	hub   *Hub
	conn  *websocket.Conn
	send  chan protocol.Message // 缓冲通道：存放即将发送给该节点的加密数据包
	alias string                // 分配给该节点的随机代号 (例如: Node-0x4F)
}

// readPump 监听客户端发来的上行数据 (Uplink)
func (c *Client) readPump() {
	// 当协程退出时，断开连接并通知 Hub 注销该节点
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	// 设置安全限制
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, messageData, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[ERROR] Node %s abnormal disconnect: %v", c.alias, err)
			}
			break // 退出死循环，触发 defer 清理
		}

		// 解析 JSON 数据包
		var msg protocol.Message
		if err := json.Unmarshal(messageData, &msg); err != nil {
			log.Printf("[WARN] Node %s sent invalid payload: %v", c.alias, err)
			continue
		}

		// 指令处理引擎
		if msg.Type == protocol.TypeCommand {
			// 按空格拆分出指令和参数（例如 "newname Neo" -> ["newname", "Neo"]）
			parts := strings.SplitN(msg.Content, " ", 2)
			commandName := strings.ToLower(parts[0])

			switch commandName {
			case "whoami":
				c.send <- protocol.NewSystemMessage(fmt.Sprintf("Your authenticated identity is: %s", c.alias))
			case "ping":
				c.send <- protocol.NewSystemMessage("PONG - Uplink latency is negligible.")

			// === 新增：私聊指令解析 ===
			case "msg":
				// 按空格拆分出 3 部分 (例如 "/msg Node-1A2B Hello" -> ["msg", "Node-1A2B", "Hello"])
				parts := strings.SplitN(msg.Content, " ", 3)
				if len(parts) < 3 {
					c.send <- protocol.NewSystemMessage("ERR: Syntax is /msg <target_alias> <secret_message>")
				} else {
					targetAlias := parts[1]
					privateContent := parts[2]

					// 构建私聊专属数据包，投入 Hub 进行精准路由
					privMsg := protocol.Message{
						Type:      protocol.TypePrivate,
						Sender:    c.alias,
						Target:    targetAlias,
						Content:   privateContent,
						Timestamp: time.Now(),
					}
					c.hub.broadcast <- privMsg
				}

			// 改名指令
			case "newname":
				if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
					// 没提供新名字时的错误提示
					c.send <- protocol.NewSystemMessage("ERR: Syntax is /alias <new_name>")
				} else {
					oldAlias := c.alias
					newAlias := strings.TrimSpace(parts[1])

					// [极客防御] 限制名字长度，防止有人发超长字符串破坏 UI 排版
					if len(newAlias) > 16 {
						newAlias = newAlias[:16]
					}

					// 1. 修改服务器内存中该节点的代号
					c.alias = newAlias

					// 2. 单独给他自己发一条确认消息
					c.send <- protocol.NewSystemMessage(fmt.Sprintf("Identity updated. You are now known as [%s].", c.alias))

					// 3. 向全网广播改名通知 (放入广播通道)
					c.hub.broadcast <- protocol.NewSystemMessage(fmt.Sprintf("Node [%s] has spoofed their identity to [%s].", oldAlias, c.alias))
				}

			default:
				c.send <- protocol.NewSystemMessage(fmt.Sprintf("ERR: Unknown command directive '%s'", commandName))
			}
			continue
		}

		// 强制覆盖发送者代号，防止客户端伪造身份
		msg.Sender = c.alias
		msg.Timestamp = time.Now()

		// 将解析后的数据包推入矩阵中枢进行广播
		c.hub.broadcast <- msg
	}
}

// writePump 将矩阵的下行数据推给客户端 (Downlink)
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			// 每次向 WebSocket 写入前，设定超时时间
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub 关闭了 channel，说明这个节点被判定为无响应，踢下线
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// 将数据结构序列化回 JSON 字节流
			payload, err := json.Marshal(message)
			if err != nil {
				continue
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				return // 写入失败，退出协程
			}

		case <-ticker.C:
			// 定期发送 Ping 探针，维持长连接存活
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ServeWs 是开放给外部的 HTTP 接口，处理客户端的接入请求
func ServeWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	// ==========================================
	// 绝对防御层：服务端 Header 拦截
	// ==========================================
	authHeader := r.Header.Get("X-Matrix-Auth")
	if authHeader != ServerSecret {
		// 密码错误，直接打回 401 状态码，拒绝 WebSocket 升级
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("ACCESS DENIED"))
		log.Printf("[SECURITY] Unauthorized access attempt blocked from IP: %s", r.RemoteAddr)
		return
	}
	// 1. 协议跃迁：HTTP -> WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("[ERROR] Protocol upgrade failed:", err)
		return
	}

	// 2. 生成随机黑客代号 (例如 Node-A7B2)
	alias := fmt.Sprintf("Node-%04X", rand.Intn(0xFFFF))

	// 3. 实例化节点
	client := &Client{
		hub:   hub,
		conn:  conn,
		send:  make(chan protocol.Message, 256), // 设定缓冲区大小，防止瞬间消息洪峰
		alias: alias,
	}

	// 4. 将节点注册到矩阵中枢
	client.hub.register <- client

	// 5. 启动读写泵 (独立协程)
	go client.writePump()
	go client.readPump()

	// 6. 新节点连入时先下发历史档案（瞬时加载）
	historyRecord := GetHistory()
	if len(historyRecord) > 0 {
		historyJSON, err := json.Marshal(historyRecord)
		if err == nil {
			client.send <- protocol.Message{
				Type:    protocol.TypeHistory,
				Sender:  "MATRIX_ARCHIVE",
				Content: string(historyJSON),
			}
		}
	}

	// 7. 发送系统广播：欢迎新节点加入
	sysMsg := protocol.NewSystemMessage(fmt.Sprintf("Secure channel established. %s joined the matrix.", alias))
	hub.broadcast <- sysMsg
}
