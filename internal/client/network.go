package client

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"

	"hacker-chat/internal/protocol"

	"github.com/gorilla/websocket"
)

// Uplink 代表客户端与矩阵服务端的网络上行链路
type Uplink struct {
	conn *websocket.Conn

	// Receive 接收通道：从服务端收到的消息，推送到这里供 UI 层读取
	Receive chan protocol.Message

	// Send 发送通道：UI 层想要发送的消息，推送到这里由网络层发给服务端
	Send chan protocol.Message

	// done 信号通道：用于优雅地关闭后台协程
	done chan struct{}
}

// Connect 负责拨号连接矩阵中枢，并初始化链路通道
func Connect(serverAddr string, passphrase string) (*Uplink, error) {
	u := url.URL{Scheme: "ws", Host: serverAddr, Path: "/ws"}

	// +++ 构造 HTTP 头，将密码藏在 X-Matrix-Auth 中带给服务器 +++
	header := http.Header{}
	header.Add("X-Matrix-Auth", passphrase)

	// 使用带有 Header 的 Dialer 发起连接
	conn, resp, err := websocket.DefaultDialer.Dial(u.String(), header)
	if err != nil {
		// 精准捕获母体返回的 401 拒绝访问错误
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			return nil, fmt.Errorf("unauthorized")
		}
		return nil, err // 其他网络错误 (比如服务器没开)
	}

	uplink := &Uplink{
		conn: conn,
		// 设置较小的缓冲区即可，UI 层处理消息极快
		Receive: make(chan protocol.Message, 100),
		Send:    make(chan protocol.Message, 100),
		done:    make(chan struct{}),
	}

	// 连接成功后，立即启动后台的读写协程
	go uplink.readLoop()
	go uplink.writeLoop()

	return uplink, nil
}

// readLoop 是一个死循环，死死盯着服务端推过来的数据
func (u *Uplink) readLoop() {
	defer close(u.done)
	for {
		_, messageData, err := u.conn.ReadMessage()
		if err != nil {
			log.Println("[NETWORK] Connection to matrix lost.")
			return
		}

		var msg protocol.Message
		if err := json.Unmarshal(messageData, &msg); err != nil {
			continue // 忽略损坏的数据包
		}

		// 将解密后的结构体塞入接收通道，等待 UI 层拿走去渲染
		u.Receive <- msg
	}
}

// writeLoop 是一个死循环，等待 UI 层下达发送指令
func (u *Uplink) writeLoop() {
	for {
		select {
		case msg := <-u.Send:
			// UI 层发来了一条消息，序列化为 JSON
			payload, err := json.Marshal(msg)
			if err != nil {
				continue
			}
			// 通过 WebSocket 发射给服务端
			err = u.conn.WriteMessage(websocket.TextMessage, payload)
			if err != nil {
				log.Println("[NETWORK] Uplink transmission failed.")
				return
			}
		case <-u.done:
			// 收到断开信号，安全退出协程
			return
		}
	}
}

// Disconnect 用于安全切断与矩阵的连接
func (u *Uplink) Disconnect() {
	// 发送标准的 Close 帧给服务端，体面退场
	u.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	u.conn.Close()
}
