package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"hacker-chat/internal/client"
	"hacker-chat/internal/config"

	tea "github.com/charmbracelet/bubbletea"
)

var serverAddr = flag.String("server", config.ServerAddr, "Matrix hub address")
var passphrase = flag.String("pass", config.Passphrase, "Matrix passphrase")

func main() {
	flag.Parse()

	fmt.Println("[SYSTEM] INITIATING UPLINK. AUTHORIZATION REQUIRED.")

	uplink, err := client.Connect(*serverAddr, *passphrase)
	if err != nil {
		if err.Error() == "unauthorized" {
			time.Sleep(800 * time.Millisecond)
			fmt.Println("[WARN] ACCESS DENIED. INVALID SIGNATURE.")
			os.Exit(1)
		}
		log.Fatalf("\n[FATAL] Network failure: %v\n", err)
	}

	time.Sleep(500 * time.Millisecond)
	fmt.Println("[SYSTEM] DECRYPTION SUCCESSFUL. UPLINK ESTABLISHED.")
	time.Sleep(500 * time.Millisecond)

	// 验证通过，运行启动伪装日志 (假装加载模块)
	client.RunBootSequence()

	defer uplink.Disconnect()

	p := tea.NewProgram(
		client.InitialModel(uplink),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		log.Fatalf("[FATAL] Terminal UI crashed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("[SYSTEM] Secure connection terminated. See you, space cowboy...")
}
