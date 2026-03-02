# hacker-chat

```text
 _               _                  _           _
| |__   __ _  ___| | _____ _ __     | |__   __ _| |_
| '_ \ / _` |/ __| |/ / _ \ '__|____| '_ \ / _` | __|
| | | | (_| | (__|   <  __/ | |_____| | | | (_| | |_
|_| |_|\__,_|\___|_|\_\___|_|       |_| |_|\__,_|\__|
```

一个基于 Go + WebSocket + Bubble Tea 的终端黑客风聊天室。

支持：
- 公聊广播
- 私聊（精准路由）
- 指令系统（`/whoami`、`/ping`、`/newname`、`/msg`、`/clear`）
- SQLite 消息持久化
- 新客户端连入自动下发最近历史（瞬时渲染）
- 终端打字机/乱码解密特效

## 项目结构

```text
hacker-chat/
├── cmd/
│   ├── client/main.go         # 客户端入口
│   └── server/main.go         # 服务端入口
├── internal/
│   ├── config/config.go       # 全局配置常量（地址/端口/口令）
│   ├── protocol/message.go    # C/S 共享消息协议
│   ├── server/
│   │   ├── hub.go             # 连接管理与广播路由
│   │   ├── client_conn.go     # 单连接读写/命令解析
│   │   └── db.go              # SQLite 持久化与历史查询
│   └── client/
│       ├── network.go         # 客户端网络层
│       ├── ui.go              # Bubble Tea UI 状态机
│       ├── style.go           # Lipgloss 样式
│       └── effects.go         # 启动与视觉特效
├── go.mod
└── README.md
```

## 技术栈

- Go 1.24+
- `github.com/gorilla/websocket`
- `github.com/charmbracelet/bubbletea`
- `github.com/charmbracelet/bubbles`
- `github.com/charmbracelet/lipgloss`
- `modernc.org/sqlite`（纯 Go SQLite 驱动，无 CGO）

## 消息协议

消息结构位于 `internal/protocol/message.go`：

- `type`：消息类型
  - `SYSTEM`：系统消息
  - `CHAT`：公聊消息
  - `COMMAND`：命令消息（客户端上行）
  - `PRIVATE`：私聊消息
  - `HISTORY`：历史档案下发消息（服务端→客户端）
- `sender`：发送者代号
- `target`：接收者代号（私聊时使用）
- `content`：正文内容
- `timestamp`：时间戳

## 数据库设计

服务端启动时会初始化 SQLite 数据库文件 `matrix.db`。

表：`messages`

- `id INTEGER PRIMARY KEY AUTOINCREMENT`
- `msg_type TEXT NOT NULL`
- `sender TEXT NOT NULL`
- `target TEXT`
- `content TEXT NOT NULL`
- `created_at DATETIME DEFAULT CURRENT_TIMESTAMP`

读取历史时按 `id DESC LIMIT 50` 获取最近消息，再反转回时间正序。
默认会过滤私聊记录（`msg_type != 'PRIVATE'`）。

## 配置方式

当前版本使用代码内置配置，不依赖 `json` 配置文件。

统一配置入口：`internal/config/config.go`

- `ServerAddr`：客户端默认连接地址
- `ServerListenAddr`：服务端默认监听端口
- `Passphrase`：客户端默认口令/服务端校验口令

仓库默认只放模板值（安全占位），不放真实数据。

如果你想本地使用真实配置但不提交到 Git：

1. 复制模板文件：

```bash
cp internal/config/config.example.go.txt internal/config/config.go
```

2. 把 `internal/config/config.go` 改成你的真实地址和口令。

3. 该文件已在 `.gitignore` 中，`git push` 时不会被提交。

4. 如果仓库历史里曾追踪过 `internal/config/config.go`，先执行一次：

```bash
git rm --cached internal/config/config.go
```

修改配置后重新编译即可生效。

## 运行方式

### 1) 启动服务端

```bash
go run cmd/server/main.go
```

可自定义端口：

```bash
go run cmd/server/main.go -addr=:8089
```

### 2) 启动客户端

```bash
go run cmd/client/main.go
```

可指定服务端地址：

```bash
go run cmd/client/main.go -server=127.0.0.1:8089
```

可指定口令：

```bash
go run cmd/client/main.go -pass=Ciallo
```

## 聊天命令

客户端本地命令：

- `/clear`：仅清空本地视图，不影响服务端历史

服务端解析命令：

- `/whoami`：查看当前节点代号
- `/ping`：链路探测
- `/newname <alias>`：改名（长度上限 16）
- `/msg <target_alias> <secret_message>`：私聊指定节点

## 消息流说明

1. 客户端输入普通文本 → 发送 `CHAT` 消息。
2. 客户端输入 `/xxx` → 发送 `COMMAND` 消息。
3. 服务端 `client_conn` 解析命令：
	- 生成系统反馈，或构建 `PRIVATE` 消息投入 Hub。
4. Hub 广播时：
	- 所有消息先落盘（`SaveMessage`）
	- 若为 `PRIVATE`，仅投递给发送者和目标接收者。
5. 新客户端连接时：
	- 服务端查询最近 50 条公开历史
	- 用 `HISTORY` 打包下发
	- 客户端收到后瞬时渲染（跳过打字机队列）。

## UI 与特效

- Bubble Tea 负责终端状态机与事件循环
- Viewport 展示消息，TextInput 负责输入
- Lipgloss 管理“矩阵绿”主题与私聊高亮
- 打字机逻辑：逐 rune 输出 + 乱码解密过渡
- 历史消息：批量导入后立即渲染，不逐字打印

## 构建

全量编译检查：

```bash
go build ./...
```

### 🖥️ 1. 编译母体服务端 (Server)

通常你的云服务器都是 Linux 系统（64位），只需要编译这一个版本即可部署上云。

Linux (普通云服务器 / AMD64 架构)

```powershell
$env:GOOS="linux"; $env:GOARCH="amd64"; go build -o matrix-server ./cmd/server
```

### 💻 2. 编译特工终端 (Client)

根据你朋友使用的电脑或设备，选择对应的命令进行编译打包。

Windows (64位 PC)

```powershell
$env:GOOS="windows"; $env:GOARCH="amd64"; go build -o hacker-chat.exe ./cmd/client
```

macOS (Apple Silicon 芯片 - M1/M2/M3)

```powershell
$env:GOOS="darwin"; $env:GOARCH="arm64"; go build -o hacker-chat-mac-m1 ./cmd/client
```

macOS (老款 Intel 芯片)

```powershell
$env:GOOS="darwin"; $env:GOARCH="amd64"; go build -o hacker-chat-mac-intel ./cmd/client
```

Linux (普通桌面版 - Ubuntu/Arch 等)

```powershell
$env:GOOS="linux"; $env:GOARCH="amd64"; go build -o hacker-chat-linux ./cmd/client
```

Linux ARM64 (树莓派 / 安卓手机 Termux 渗透)

```powershell
$env:GOOS="linux"; $env:GOARCH="arm64"; go build -o hacker-chat-linux-arm ./cmd/client
```

## 部署（Linux 可选）

```
# 赋予执行权限
chmod +x matrix-server

# 使用 nohup 在后台运行，并将日志输出到 matrix.log
nohup ./matrix-server -addr=":8080" > matrix.log 2>&1 &

# 查看一下启动日志，确认它活过来了
tail -f matrix.log
```
## 常见问题

### 服务端启动失败（exit code 1）

优先检查：
- 端口是否被占用（默认 `:8089`）
- 是否有当前目录写权限（需要创建 `matrix.db`）
- 是否在项目根目录执行命令

### 客户端连接不上

确认：
- 服务端已启动
- 客户端 `-server` 地址与端口正确
- 防火墙/安全组放行端口

## License

当前仓库未声明开源许可证；如需开源发布，请补充 `LICENSE` 文件。

