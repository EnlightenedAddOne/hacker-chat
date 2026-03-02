package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"hacker-chat/internal/config"
	"hacker-chat/internal/server"
)

// flag 允许我们在启动时通过命令行参数自定义端口
// 例如：go run cmd/server/main.go -addr=":9999"
var addr = flag.String("addr", config.ServerListenAddr, "The matrix listening address")

func main() {
	flag.Parse()

	// 1. 打印极客风的启动序列 (装X必须有)
	log.Println("[SYSTEM] Booting Matrix Protocol...")
	time.Sleep(500 * time.Millisecond)
	log.Println("[SYSTEM] Loading cryptographic modules... [OK]")
	time.Sleep(500 * time.Millisecond)

	// 2. 挂载 SQLite 记忆核心
	server.InitDB()

	// 3. 实例化矩阵中枢 (Hub)
	hub := server.NewHub()

	// 4. 在独立的 Goroutine 中启动中枢的事件循环
	go hub.Run()

	// 5. 配置网络路由
	// 当有节点请求 /ws 路径时，将其移交给 ServeWs 进行 WebSocket 升级和接管
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		server.ServeWs(hub, w, r)
	})

	// 6. 启动 HTTP 监听服务器
	log.Printf("[SYSTEM] Matrix C2 Server is strictly listening on %s\n", *addr)
	log.Println("[SYSTEM] Awaiting incoming secure connections...")

	// ListenAndServe 会一直阻塞在这里，直到程序被强行终止或发生致命错误
	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		log.Fatalf("[FATAL] Matrix server crashed: %v", err)
	}
}
