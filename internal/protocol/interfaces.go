package protocol

import "time"

// 共享常量
const (
	MaxPacketSize    = 512    // 最大数据包大小
	MaxHandshakeSize = 512    // 握手包最大大小
	MaxStringLen     = 128    // 最大字符串长度
	MaxVarIntValue   = 100000 // 最大 VarInt 值
	ReadTimeout      = 5      // 读取超时秒数
)

// RateLimiter 限流器接口
type RateLimiter interface {
	Allow(ip string) bool
	CalculateDelay(ip string) time.Duration
	GetIPFrequency(ip string) float64
}

// HandshakeInfo 握手包信息
type HandshakeInfo struct {
	ProtocolVersion int
	ServerAddress   string
	ServerPort      uint16
	NextState       int
}
