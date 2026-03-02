package client

import (
	"fmt"
	"math/rand"
	"time"
)

// ==========================================
// 模块 1：启动仪式感 (Boot Sequence)
// ==========================================

// RunBootSequence 在终端启动前打印一系列极客风的自检日志
func RunBootSequence() {
	fmt.Println("[SYSTEM] Initializing secure terminal...")
	time.Sleep(400 * time.Millisecond)
	fmt.Println("[SYSTEM] Loading cryptographic modules... [OK]")
	time.Sleep(300 * time.Millisecond)
	fmt.Println("[SYSTEM] Bypassing local firewall... [OK]")
	time.Sleep(400 * time.Millisecond)
	fmt.Println("[SYSTEM] Connecting to Matrix Hub ...")
	time.Sleep(600 * time.Millisecond)
}

// ==========================================
// 模块 2：乱码解密特效 (Decryption Effects)
// ==========================================

// GetScrambleChar 随机抽取一个极客风符号作为未解密占位符
func GetScrambleChar() string {
	chars := []rune("!@#$%^&*<>?{}[]\\|/XYZ01")
	return string(chars[rand.Intn(len(chars))])
}

// CalculateScrambleCycles 决定下一个字符需要闪烁多少次乱码才会解密
// 把它抽离出来，方便以后在这里统一调整解密速度
func CalculateScrambleCycles() int {
	return rand.Intn(5) + 1 // 随机 1 到 5 次
}
