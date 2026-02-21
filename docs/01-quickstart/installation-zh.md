# 安装指南

**OCG-Go (OCG)** - 快速、轻量级的 Go AI Agent 系统。

---

## 前置要求

### 系统要求

- **操作系统**: Linux (推荐 Ubuntu/Debian)、macOS、Windows (WSL2)
- **Go**: 1.22 或更高版本
- **GCC**: SQLite CGO 绑定必需
- **内存**: 最低 512MB RAM (推荐 1GB+)
- **存储**: 约 500MB (二进制文件 + 模型)

### Linux 依赖

```bash
# Ubuntu/Debian
sudo apt-get update
sudo apt-get install -y \
    golang-go \
    gcc \
    libgomp1 \
    libblas3 \
    liblapack3 \
    libopenblas0 \
    libgfortran5 \
    git \
    make

# Arch Linux
sudo pacman -S go gcc openblas lapack git make

# Fedora/RHEL
sudo dnf install -y golang gcc openblas lapack git make
```

### macOS 依赖

```bash
# 安装 Xcode 命令行工具
xcode-select --install

# 安装 Go (如未使用 Homebrew)
brew install go
```

---

## 安装方法

### 方法 1: 源码编译 (推荐)

```bash
# 克隆仓库
git clone https://github.com/gliderlab/OCG.git
cd cogate

# 构建所有组件
make build-all

# 或构建特定组件
make build-gateway
make build-agent
make build-embedding
```

**构建产物:**
- `bin/ocg` - 主 CLI 和进程管理器
- `bin/ocg-gateway` - Gateway 服务 (HTTP/WebSocket)
- `bin/ocg-agent` - Agent 服务 (RPC)
- `bin/ocg-embedding` - Embedding 服务
- `bin/llama-server` - llama.cpp embedding 服务器

### 方法 2: 预编译二进制

查看 [Releases](https://github.com/gliderlab/OCG/releases) 获取预编译二进制。

```bash
# 下载最新版本
wget https://github.com/gliderlab/OCG/releases/latest/download/ocg-linux-amd64.tar.gz
tar -xzf ocg-linux-amd64.tar.gz
sudo mv ocg /usr/local/bin/
```

---

## 验证安装

```bash
# 检查版本
./bin/ocg version

# 检查系统状态
./bin/ocg status

# 健康检查 (启动后)
curl http://localhost:55003/health
```

---

## 下一步

- [快速入门](first-steps-zh.md) - 开始使用 OCG
- [环境配置](environment-zh.md) - 配置环境变量

---

## 故障排除

### CGO 构建错误

```bash
# 安装缺失的库
sudo apt-get install -y libgomp1 libblas3 liblapack3
```

### 端口已被占用

```bash
# 检查端口占用
lsof -i :55003

# 终止进程
kill -9 <PID>
```

### 权限被拒绝

```bash
# 使二进制可执行
chmod +x bin/ocg
```
