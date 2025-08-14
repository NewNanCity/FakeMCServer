package network

import (
	"sync"
	"sync/atomic"
	"time"
)

// ConnectionManager 高性能连接管理器
type ConnectionManager struct {
	// 使用分片锁减少锁竞争
	shards    []*connectionShard
	shardMask uint64
	count     atomic.Int64
}

// connectionShard 连接分片
type connectionShard struct {
	mu          sync.RWMutex
	connections map[string]*Connection
}

// NewConnectionManager 创建连接管理器
func NewConnectionManager(shardCount int) *ConnectionManager {
	// 确保分片数量是2的幂
	if shardCount <= 0 {
		shardCount = 16
	}
	
	// 找到最接近的2的幂
	actualShardCount := 1
	for actualShardCount < shardCount {
		actualShardCount <<= 1
	}
	
	shards := make([]*connectionShard, actualShardCount)
	for i := range shards {
		shards[i] = &connectionShard{
			connections: make(map[string]*Connection),
		}
	}
	
	return &ConnectionManager{
		shards:    shards,
		shardMask: uint64(actualShardCount - 1),
	}
}

// getShard 获取连接对应的分片
func (cm *ConnectionManager) getShard(connID string) *connectionShard {
	hash := fnv1aHash(connID)
	return cm.shards[hash&cm.shardMask]
}

// Store 存储连接
func (cm *ConnectionManager) Store(connID string, conn *Connection) {
	shard := cm.getShard(connID)
	shard.mu.Lock()
	shard.connections[connID] = conn
	shard.mu.Unlock()
	cm.count.Add(1)
}

// Load 加载连接
func (cm *ConnectionManager) Load(connID string) (*Connection, bool) {
	shard := cm.getShard(connID)
	shard.mu.RLock()
	conn, exists := shard.connections[connID]
	shard.mu.RUnlock()
	return conn, exists
}

// Delete 删除连接
func (cm *ConnectionManager) Delete(connID string) {
	shard := cm.getShard(connID)
	shard.mu.Lock()
	if _, exists := shard.connections[connID]; exists {
		delete(shard.connections, connID)
		cm.count.Add(-1)
	}
	shard.mu.Unlock()
}

// Count 获取连接数量
func (cm *ConnectionManager) Count() int64 {
	return cm.count.Load()
}

// Range 遍历所有连接
func (cm *ConnectionManager) Range(fn func(connID string, conn *Connection) bool) {
	for _, shard := range cm.shards {
		shard.mu.RLock()
		for id, conn := range shard.connections {
			if !fn(id, conn) {
				shard.mu.RUnlock()
				return
			}
		}
		shard.mu.RUnlock()
	}
}

// CleanupExpired 清理过期连接
func (cm *ConnectionManager) CleanupExpired(maxIdleTime time.Duration) int {
	now := time.Now()
	cleaned := 0
	
	for _, shard := range cm.shards {
		shard.mu.Lock()
		for id, conn := range shard.connections {
			if now.Sub(conn.StartTime) > maxIdleTime {
				conn.Close()
				delete(shard.connections, id)
				cleaned++
			}
		}
		shard.mu.Unlock()
	}
	
	cm.count.Add(int64(-cleaned))
	return cleaned
}

// fnv1aHash FNV-1a 哈希函数
func fnv1aHash(s string) uint64 {
	const (
		offset64 = 14695981039346656037
		prime64  = 1099511628211
	)
	
	hash := uint64(offset64)
	for i := 0; i < len(s); i++ {
		hash ^= uint64(s[i])
		hash *= prime64
	}
	return hash
}
