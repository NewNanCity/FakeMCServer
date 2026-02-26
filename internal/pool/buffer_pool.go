package pool

import (
	"sync"
)

// BufferPool 缓冲区对象池
type BufferPool struct {
	pool sync.Pool
	size int
}

// NewBufferPool 创建缓冲区池
func NewBufferPool(size int) *BufferPool {
	return &BufferPool{
		pool: sync.Pool{
			New: func() any {
				return make([]byte, size)
			},
		},
		size: size,
	}
}

// Get 获取缓冲区
func (p *BufferPool) Get() []byte {
	return p.pool.Get().([]byte)
}

// Put 归还缓冲区
func (p *BufferPool) Put(buf []byte) {
	if cap(buf) == p.size {
		// 重置长度但保留容量
		buf = buf[:p.size]
		// 清零缓冲区（可选，用于安全性）
		for i := range buf {
			buf[i] = 0
		}
		p.pool.Put(buf)
	}
}

// ResponsePool 响应对象池
type ResponsePool struct {
	pool sync.Pool
}

// NewResponsePool 创建响应池
func NewResponsePool() *ResponsePool {
	return &ResponsePool{
		pool: sync.Pool{
			New: func() any {
				return make([]byte, 0, 4096) // 预分配4KB容量
			},
		},
	}
}

// Get 获取响应缓冲区
func (p *ResponsePool) Get() []byte {
	buf := p.pool.Get().([]byte)
	return buf[:0] // 重置长度但保留容量
}

// Put 归还响应缓冲区
func (p *ResponsePool) Put(buf []byte) {
	if cap(buf) <= 8192 { // 限制最大容量避免内存泄漏
		p.pool.Put(buf)
	}
}
