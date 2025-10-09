# FakeMCServer - GoMC版本

## 概述

这是基于[go-mc](https://github.com/Tnze/go-mc)库重新实现的FakeMCServer版本。go-mc是一个成熟的Minecraft协议库，提供了完整的服务器框架实现。

## 为什么需要GoMC版本？

原始的自实现版本在处理某些非标准的MC状态查询工具时存在兼容性问题：
- "packet too small: 1" 错误
- "connection has been closed when write" 错误
- 某些在线MC服务器状态查询网站无法正确识别

GoMC版本使用go-mc库的标准服务器框架，提供：
- ✅ **更好的兼容性**：完全符合Minecraft协议标准
- ✅ **更可靠的连接处理**：使用go-mc的成熟网络层
- ✅ **更简洁的代码**：利用go-mc提供的高级抽象
- ✅ **更好的维护性**：跟随go-mc库的更新

## 架构设计

### 核心组件

```
GoMCHandler (internal/protocol/gomc_handler.go)
├── HandleConnection()      # 主处理入口
├── handleHandshake()       # 握手处理（使用go-mc）
├── handleStatusQuery()     # 状态查询（使用go-mc）
└── handleLogin()           # 登录处理（使用go-mc）
```

### 与go-mc的集成

```go
// 使用go-mc的net.Conn
mcConn := net.Conn{Socket: conn.Conn}

// 使用go-mc的包处理
var p pk.Packet
mcConn.ReadPacket(&p)
mcConn.WritePacket(pk.Marshal(...))
```

## 编译和运行

### 编译GoMC版本

```bash
# Windows
go build -o fake-mc-server-gomc.exe cmd/server-gomc/main.go

# Linux/Mac
go build -o fake-mc-server-gomc cmd/server-gomc/main.go
```

### 运行

```bash
# Windows
./fake-mc-server-gomc.exe -config config/config.yml

# Linux/Mac
./fake-mc-server-gomc -config config/config.yml
```

### 查看版本信息

```bash
./fake-mc-server-gomc -version
```

## 配置

GoMC版本使用与原版相同的配置文件（`config/config.yml`），无需额外配置。

## 功能对比

| 功能 | 原版 | GoMC版本 |
|------|------|----------|
| 状态查询 | ✅ | ✅ |
| Ping/Pong | ✅ | ✅ |
| 登录拒绝 | ✅ | ✅ |
| 上游同步 | ✅ | ✅ |
| 限流延迟 | ✅ | ✅ |
| 蜜罐日志 | ✅ | ✅ |
| 协议兼容性 | ⚠️ 部分 | ✅ 完全 |
| 连接稳定性 | ⚠️ 一般 | ✅ 优秀 |
| 代码复杂度 | 高 | 低 |

## 技术细节

### 握手处理

使用go-mc的标准握手包解析：

```go
var (
    protocolVersion pk.VarInt
    serverAddress   pk.String
    serverPort      pk.UnsignedShort
    nextState       pk.VarInt
)
p.Scan(&protocolVersion, &serverAddress, &serverPort, &nextState)
```

### 状态查询

使用go-mc的包ID常量和标准响应格式：

```go
mcConn.WritePacket(pk.Marshal(
    packetid.ClientboundStatusStatusResponse,
    pk.String(statusJSON),
))
```

### 登录处理

使用go-mc的chat.Message和标准断开连接包：

```go
kickMessage := chat.Message{Text: h.config.Messages.KickMessage}
mcConn.WritePacket(pk.Marshal(
    packetid.ClientboundLoginLoginDisconnect,
    kickMessage,
))
```

## 测试

### 本地测试

1. 启动服务器：
```bash
./fake-mc-server-gomc -config config/config.yml
```

2. 使用Minecraft客户端连接：
   - 添加服务器：`localhost:25565`
   - 查看服务器状态（应该正常显示）
   - 尝试登录（应该被拒绝并显示kick消息）

### 在线测试

使用各种MC服务器状态查询网站测试：
- https://mcsrvstat.us/
- https://mcstatus.io/
- https://minecraft-server-list.com/

## 故障排除

### 编译错误

如果遇到go-mc相关的编译错误：

```bash
go mod tidy
go mod download
```

### 运行时错误

查看日志输出，GoMC版本提供更详细的错误信息。

## 迁移指南

### 从原版迁移到GoMC版本

1. 停止原版服务器
2. 编译GoMC版本
3. 使用相同的配置文件启动GoMC版本
4. 观察日志，确认正常运行

### 回退到原版

如果GoMC版本出现问题，可以随时回退：

1. 停止GoMC版本
2. 启动原版服务器
3. 配置文件无需修改

## 性能

GoMC版本的性能与原版相当或更好：
- 内存使用：相似
- CPU使用：相似或更低
- 网络延迟：相似或更低
- 连接处理：更稳定

## 未来计划

- [ ] 添加更多go-mc特性支持
- [ ] 优化性能
- [ ] 添加更多测试
- [ ] 完善文档

## 贡献

欢迎提交Issue和Pull Request！

## 许可证

与主项目相同的许可证。

