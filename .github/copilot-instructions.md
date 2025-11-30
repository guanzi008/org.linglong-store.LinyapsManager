
本文件为 GitHub Copilot 的仓库级说明，帮助 Copilot 和贡献者在本项目中生成更贴合的代码。请严格遵循下列约定与约束。

## 重点（极其重要）
在接到用户的任务的时候，先不要着急开始修改代码，要先分析需求，分析代码，列举解决方案，
详细的向用户说明你的思路，和你打算如何实现这个需求。
**只有在用户确认你的方案后，才开始动手写代码。**

## 概述
这个项目是一个dbus转发命令行输出的项目，目的是为了让玲珑应用商店在容器中能够调用外部的ll-cli命令行工具。
client致力于完整还原ll-cli的命令行交互体验，server负责调用ll-cli并通过dbus转发输出。


## 架构

- **服务 (Server):** `cmd/server/main.go`
  - 在 `/org/linglong_store/LinyapsManager` 暴露 D-Bus 接口 `org.linglong_store.LinyapsManager`。
  - 封装 `ll-cli` 命令（安装、运行、列表等）。
  - 管理沙箱应用的环境变量和代理套接字。
- **客户端 (CLI):** `cmd/client/main.go`
  - `linyapsctl` 通过 D-Bus 与服务通信。
  - 支持标准命令和长运行操作的流式输出。
- **流式传输 (Streaming):** `internal/streaming`
  - 实现了一个自定义协议，使用 `operationID` 通过 D-Bus 信号 (`Output`, `Complete`) 流式传输命令输出 (stdout/stderr)。

## 关键组件与模式

### D-Bus 通信
- **库:** 使用 `github.com/godbus/dbus/v5`。
- **常量:** 定义在 `internal/dbusconsts/consts.go` 中。
- **错误处理:** 服务端方法必须返回 `*dbus.Error` (例如 `dbus.MakeFailedError(err)`)。
- **连接:** `internal/dbusutil` 处理连接逻辑，遵循 `LINYAPS_DBUS_ADDRESS` 环境变量。

### 流式协议
对于长运行操作（例如 `InstallStream`），使用流式模式：
1. **服务端:** 调用 `streaming.RunCommandStreaming`。立即返回一个 `operationID`。
2. **服务端:** 后台 goroutine 发送 `Output` 信号，最后发送 `Complete` 信号。
3. **客户端:** 调用方法获取 `operationID`，然后使用 `streaming.Receiver` 等待信号。

### 环境与代理
- **环境注入:** `cmd/server/main.go` 中的 `internal/envgrab` 和 `buildLinyapsEnv` 将关键变量 (`DBUS_SESSION_BUS_ADDRESS`, `DISPLAY`) 注入到 `ll-cli` 进程中。
- **代理:** `internal/proxy` 管理套接字代理，允许容器化应用访问主机总线。

### 验证
- **AppID/Version:** 在执行命令前，使用 `validateAppID` 和 `validateVersion` 正则表达式模式验证输入。

## 开发工作流

### 构建
```bash
go build -o build/linyaps-dbus-server ./cmd/server
go build -o build/linyapsctl ./cmd/client
```

### 运行
服务端需要 `ll-cli` 可用或被 mock。
```bash
# 运行服务端 (根据需要调整环境变量)
export LINYAPS_DBUS_ADDRESS=unix:path=/tmp/linyaps.sock
./build/linyaps-dbus-server

# 运行客户端
export LINYAPS_DBUS_ADDRESS=unix:path=/tmp/linyaps.sock
./build/linyapsctl list
```

### 测试
- 运行单元测试: `go test ./...`
- 验证流式传输: `linyapsctl test` 触发服务端的 `TestStream` 方法。

## 常见任务

- **添加新命令:**
  1. 在 `cmd/server/main.go` 的 `LinyapsManager` 结构体中添加方法。
  2. 实现 `ll-cli` 封装逻辑。
  3. 在 `cmd/client/main.go` 中添加 CLI 命令处理。
  4. 如果是长运行任务，使用 `InstallStream` 模式。
