<div align="center">

# 🎮 FakeMCServer

<p align="center">
  <strong>高性能 Minecraft 协议代理服务器</strong><br>
  用于网络安全研究和攻击行为分析的专业工具
</p>

<p align="center">
  <a href="#-快速开始">快速开始</a> •
  <a href="#-部署方式">部署指南</a> •
  <a href="#-配置说明">配置文档</a> •
  <a href="#-故障排除">故障排除</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go Version" />
  <img src="https://img.shields.io/badge/Platform-Windows%20%7C%20Linux%20%7C%20macOS-lightgrey?style=for-the-badge" alt="Platform" />
  <img src="https://img.shields.io/badge/Docker-Supported-2496ED?style=for-the-badge&logo=docker&logoColor=white" alt="Docker" />
  <img src="https://img.shields.io/badge/License-MIT-green?style=for-the-badge" alt="License" />
</p>

<p align="center">
  <img src="https://img.shields.io/github/stars/NewNanCity/FakeMCServer?style=social" alt="GitHub stars" />
  <img src="https://img.shields.io/github/forks/NewNanCity/FakeMCServer?style=social" alt="GitHub forks" />
</p>

</div>

---

## ✨ 主要功能

- 🎮 **Minecraft 协议兼容**: 完整支持 Minecraft 服务器状态查询和登录流程
- 🔄 **上游服务器同步**: 自动同步真实 Minecraft 服务器的状态信息
- 🛡️ **智能限流防护**: IP 级别和全局限流，有效防止攻击
- 📊 **详细监控记录**: 记录所有连接和攻击行为，便于分析
- ⚡ **高性能架构**: 支持大量并发连接
- 🔧 **灵活配置**: 简单的 YAML 配置文件

### 🚀 性能表现

| 特性           | 指标                    |
| -------------- | ----------------------- |
| 🔗 **并发连接** | 10,000+                 |
| ⚡ **响应时间** | < 100ms                 |
| 💾 **内存占用** | < 50MB                  |
| 🌐 **平台支持** | Windows / Linux / macOS |

</div>

### 📋 环境要求

- **Go**: 1.21 或更高版本
- **操作系统**: Linux、macOS、Windows
- **端口**: 25565 (Minecraft 默认端口)
- **内存**: 至少 256MB 可用内存

## 🚀 部署方式

### 📦 方式一：二进制部署

<details>
<summary>🔧 点击展开详细步骤</summary>

#### 1️⃣ 下载和编译

```bash
# 克隆项目
git clone <repository-url>
cd FakeMCServer

# 安装依赖
go mod tidy

# 编译（Windows）
go build -o fake-mc-server.exe cmd/server/main.go

# 编译（Linux/macOS）
go build -o fake-mc-server cmd/server/main.go
```

#### 2️⃣ 配置

复制示例配置文件：
```bash
# Windows
copy config\example.config.yml config\config.yml

# Linux/macOS
cp config/example.config.yml config/config.yml
```

编辑 `config/config.yml` 文件，重点配置以下项目：
```yaml
server:
  host: "0.0.0.0"    # 监听地址
  port: 25565        # Minecraft 标准端口

upstream:
  enabled: true                # 是否启用上游同步
  address: "mc.hypixel.net"   # 要模拟的真实服务器
```

#### 3️⃣ 运行

```bash
# Windows
fake-mc-server.exe -config config/config.yml

# Linux/macOS
./fake-mc-server -config config/config.yml
```

</details>

### 🐳 方式二：Docker 部署

<details>
<summary>🚢 点击展开 Docker 部署选项</summary>

#### 🔨 选项 1: 直接构建

```bash
# 构建镜像
docker build -t fake-mc-server .

# 运行容器
docker run -d \
  --name fake-mc-server \
  -p 25565:25565 \
  -v $(pwd)/config:/app/config \
  fake-mc-server
```

#### 🐙 选项 2: Docker Compose

创建 `docker-compose.yml` 文件：
```yaml
version: '3.8'
services:
  fake-mc-server:
    build: .
    ports:
      - "25565:25565"
    volumes:
      - ./config:/app/config
    restart: unless-stopped
```

运行：
```bash
docker-compose up -d
```

</details>

> **💡 提示**: 推荐使用 Docker 部署方式，更加简单且易于管理！

## ⚙️ 配置说明

<details>
<summary>📝 点击展开详细配置说明</summary>

### 🔧 基础配置

```yaml
# 🖥️ 服务器配置
server:
  host: "0.0.0.0"          # 🌐 监听地址，0.0.0.0 表示监听所有网卡
  port: 25565              # 🔌 监听端口，Minecraft 默认端口
  max_connections: 10000   # 🔗 最大连接数

# 🔄 上游服务器配置
upstream:
  enabled: true                    # ✅ 是否启用上游同步
  address: "mc.hypixel.net"       # 🎯 要模拟的真实服务器地址
  sync_interval: "10s"            # ⏱️ 状态同步间隔

# 🛡️ 限流配置
rate_limit:
  ip_limit: 5              # 🚦 单个 IP 每秒最大连接数
  global_limit: 100        # 🌍 全局每秒最大连接数

# 📝 日志配置
logging:
  level: "info"            # 📊 日志级别: debug, info, warn, error
  format: "console"        # 🖨️ 日志格式: json, console
```

</details>

### 🌐 支持的服务器地址格式

<div align="center">

| 格式类型       | 示例                | 说明              |
| -------------- | ------------------- | ----------------- |
| 🔢 **IP 地址**  | `192.168.1.1:25565` | 直接 IP 连接      |
| 🌍 **域名**     | `example.com:25565` | 域名解析          |
| 📋 **SRV 记录** | `mc.example.com`    | 自动 DNS SRV 查询 |

</div>

## 📱 使用说明

### ✅ 启动后的效果

服务器启动后，Minecraft 客户端可以：
- ✨ 在服务器列表中看到这个服务器
- 📊 看到与真实服务器相同的 MOTD、在线人数等信息
- 🔒 尝试连接时会被记录但无法真正进入游戏

### 📊 日志监控

服务器会记录以下信息：
- 🔍 所有连接尝试的 IP 地址
- 👤 玩家尝试使用的用户名
- 📈 连接频率和攻击模式
- 🚨 限流触发情况

<details>
<summary>📋 点击查看日志示例</summary>

```bash
2024/08/14 10:30:45 [INFO] 新连接: 192.168.1.100
2024/08/14 10:30:45 [WARN] 登录尝试: ip=192.168.1.100 username=admin
2024/08/14 10:30:46 [WARN] 限流触发: ip=192.168.1.100 超过连接限制
```

</details>

## 🚀 生产环境部署

### 🐧 Linux 系统服务

<details>
<summary>📝 点击展开 systemd 配置</summary>

创建 systemd 服务文件 `/etc/systemd/system/fake-mc-server.service`：
```ini
[Unit]
Description=Fake Minecraft Server
After=network.target

[Service]
Type=simple
User=minecraft
Group=minecraft
WorkingDirectory=/opt/fake-mc-server
ExecStart=/opt/fake-mc-server/fake-mc-server -config /opt/fake-mc-server/config/config.yml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

启用服务：
```bash
sudo systemctl enable fake-mc-server
sudo systemctl start fake-mc-server
sudo systemctl status fake-mc-server
```

</details>

### 🐳 Docker 生产部署

<details>
<summary>🔧 点击展开高级 Docker 配置</summary>

使用持久化存储和日志：
```bash
docker run -d \
  --name fake-mc-server \
  --restart unless-stopped \
  -p 25565:25565 \
  -v /opt/fake-mc-server/config:/app/config \
  --memory="256m" \
  --cpus="1.0" \
  fake-mc-server
```

</details>

### 🔒 安全建议

<div align="center">

| 安全措施         | 重要性 | 说明                   |
| ---------------- | ------ | ---------------------- |
| 🔥 **防火墙配置** | 🔴 高   | 只开放必要端口 (25565) |
| 📁 **日志轮转**   | 🟡 中   | 避免磁盘空间占满       |
| 📊 **监控告警**   | 🟠 中   | 异常连接数量告警       |
| 🔍 **定期分析**   | 🟢 低   | 分析攻击模式和来源     |

</div>

## 🔧 故障排除

<details>
<summary>❓ 常见问题解答</summary>

### 🚨 **Q: 服务器无法启动，提示端口被占用**
**💡 A:** 检查 25565 端口是否被其他程序占用，使用 `netstat -an | grep 25565` 查看

### 🌐 **Q: 客户端无法连接**
**💡 A:** 检查防火墙设置，确保 25565 端口已开放

### 🔄 **Q: 看不到上游服务器信息**
**💡 A:** 检查 `upstream.address` 配置是否正确，确保网络能够访问目标服务器

### 💾 **Q: 日志过多占用磁盘空间**
**💡 A:** 配置日志轮转或调整日志级别为 "warn" 或 "error"

</details>

### 🐛 调试模式

启用调试日志查看更多信息：
```yaml
logging:
  level: "debug"
```

---

<div align="center">

## 📈 项目信息

| 项目信息       | 详情                    |
| -------------- | ----------------------- |
| 🚀 **当前版本** | v1.0.0                  |
| 📅 **最后更新** | 2024年8月14日           |
| 🌍 **支持平台** | Windows / Linux / macOS |
| 🔧 **Go 版本**  | 1.21+                   |
| 📦 **依赖数量** | 最小化依赖              |

![Language](https://img.shields.io/badge/Language-Go-00ADD8?style=flat-square&logo=go)
![Platform](https://img.shields.io/badge/Platform-Cross--Platform-lightgrey?style=flat-square)
![License](https://img.shields.io/badge/License-MIT-green?style=flat-square)

</div>

## 🤝 贡献

我们欢迎所有形式的贡献！

<p align="center">
  <a href="https://github.com/NewNanCity/FakeMCServer/issues/new?template=bug_report.md">
    <img src="https://img.shields.io/badge/🐛_报告_Bug-red?style=for-the-badge" alt="Report Bug" />
  </a>
  <a href="https://github.com/NewNanCity/FakeMCServer/issues/new?template=feature_request.md">
    <img src="https://img.shields.io/badge/💡_功能_建议-blue?style=for-the-badge" alt="Request Feature" />
  </a>
  <a href="https://github.com/NewNanCity/FakeMCServer/fork">
    <img src="https://img.shields.io/badge/🔀_Fork_项目-green?style=for-the-badge" alt="Fork" />
  </a>
</p>

### 📋 贡献步骤

1. 🍴 Fork 这个项目
2. 🌿 创建你的功能分支 (`git checkout -b feature/AmazingFeature`)
3. 💾 提交你的更改 (`git commit -m 'Add some AmazingFeature'`)
4. 📤 推送到分支 (`git push origin feature/AmazingFeature`)
5. 🔄 开启一个 Pull Request

## 📄 许可证

<div align="center">

该项目采用 MIT 许可证 - 查看 [LICENSE](LICENSE) 文件了解详细信息。

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

</div>

## 💌 支持与反馈

<div align="center">

<p>
  <strong>喜欢这个项目吗？给我们一个 ⭐️ 支持一下！</strong>
</p>

<p>
  <a href="https://github.com/NewNanCity/FakeMCServer/discussions">💬 讨论</a> •
  <a href="https://github.com/NewNanCity/FakeMCServer/issues">🐛 问题</a>
</p>

</div>

---

<div align="center">
  <sub>© 2024 NewNanCity Team. All rights reserved.</sub>
</div>
