# LinyapsManager

[English](README_EN.md) | 简体中文

<div align="center">

**为容器化应用提供安全的宿主机命令执行能力**

[![License](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.19-blue.svg)](go.mod)

</div>

---

## 📖 项目简介

LinyapsManager 是一个基于 D-Bus 的命令代理服务，专为**玲珑应用商店**等容器化环境设计。它允许容器内的应用通过 D-Bus 接口安全地调用宿主机的特定命令（如 `ll-cli`、`killall`、`pkexec` 等），同时通过**白名单机制**和**参数校验**确保系统安全。

### 核心特性

- 🔒 **安全第一**：白名单机制 + 参数严格校验，防止命令注入
- 🔌 **插件式规则**：每个命令独立的验证规则，易于扩展
- 📡 **流式输出**：通过 D-Bus 信号实时传输命令输出（stdout/stderr）
- 🎭 **透明调用**：客户端通过符号链接伪装成目标命令，用户无感知
- 🌍 **环境注入**：自动抓取会话环境变量（DISPLAY、DBUS_SESSION 等）
- 🔄 **容器友好**：通过 xdg-dbus-proxy 提供套接字，容器可直接访问

---

## 🏗️ 架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                    容器内应用                                 │
│  (玲珑应用商店 / 其他容器化应用)                                │
└────────────────────┬────────────────────────────────────────┘
                     │ 调用符号链接
                     │ ./ll-cli install app
                     ↓
┌─────────────────────────────────────────────────────────────┐
│              客户端 (linyapsctl)                             │
│  • 识别程序名 (ll-cli/killall/pkexec)                         │
│  • 通过 D-Bus 调用 ExecuteCommand(cmd, args)                  │
└────────────────────┬────────────────────────────────────────┘
                     │ D-Bus 通信
                     │ org.linglong_store.LinyapsManager
                     ↓
┌─────────────────────────────────────────────────────────────┐
│              服务端 (linyaps-dbus-server)                    │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  1. 命令白名单验证 (cmdwhitelist)                     │    │
│  │     • 检查命令是否允许执行                             │   │
│  │     • 调用对应的 Rule 进行参数校验                      │   │
│  └─────────────────────────────────────────────────────┘   │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  2. 环境准备                                         │   │
│  │     • 注入会话环境变量 (DISPLAY/DBUS_SESSION)          │   │
│  │     • 强制英文 locale (确保输出可解析)                  │   │
│  └─────────────────────────────────────────────────────┘   │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  3. 流式执行 (streaming)                             │   │
│  │     • 启动命令并获取 operationID                      │   │
│  │     • 通过 D-Bus 信号流式发送输出                      │   │
│  │     • Output(opID, data, isStderr)                  │   │
│  │     • Complete(opID, exitCode, errorMsg)            │   │
│  └─────────────────────────────────────────────────────┘   │
└────────────────────┬────────────────────────────────────────┘
                     │ 执行实际命令
                     ↓
              ┌──────────────┐
              │ 宿主机命令     │
              │ ll-cli       │
              │ killall      │
              │ pkexec       │
              └──────────────┘
```

### D-Bus 接口

**服务名称**: `org.linglong_store.LinyapsManager`  
**对象路径**: `/org/linglong_store/LinyapsManager`  
**接口名称**: `org.linglong_store.LinyapsManager`

#### 方法

- **ExecuteCommand**(command: `string`, args: `[]string`) → operationID: `string`
  - 验证并执行白名单命令
  - 返回操作 ID，用于接收流式输出

- **Ping**() → `string`
  - 健康检查，返回 "pong"

- **Quit**()
  - 优雅退出服务（用于更新/重启）

#### 信号

- **Output**(operationID: `string`, data: `string`, isStderr: `bool`)
  - 流式输出信号，data 为命令输出片段

- **Complete**(operationID: `string`, exitCode: `int32`, errorMsg: `string`)
  - 命令完成信号，包含退出码和错误信息

---

## 🔐 安全模型

### 白名单机制

所有命令必须通过**插件式规则注册**才能执行。每个命令规则独立定义在 `internal/cmdwhitelist/rules/` 目录：

```
internal/cmdwhitelist/rules/
├── doc.go          # 规则系统文档
├── llcli.go        # ll-cli 命令规则
├── killall.go      # killall 命令规则
└── pkexec.go       # pkexec 命令规则
```

### 命令验证流程

```go
// 1. 检查命令是否在白名单
rule := cmdwhitelist.GetRule("ll-cli")
if rule == nil {
    return error("command not allowed")
}

// 2. 调用规则的 Validate 方法校验参数
validatedArgs, err := rule.Validate([]string{"install", "app"})
if err != nil {
    return error("validation failed")
}

// 3. 获取实际程序路径并执行
program := rule.Program()  // 例如: "ll-cli" 或 "/usr/bin/killall"
exec(program, validatedArgs...)
```

### 现有命令的安全策略

#### 1. ll-cli（玲珑命令行工具）

```go
// 允许的子命令
allowedSubcmds: [
    "list", "search", "info", "install", "uninstall",
    "run", "kill", "exec", "ps", "repo", "content", "prune"
]

// 最大参数数量：20
// 需要环境注入：是（DISPLAY、DBUS_SESSION 等）
```

#### 2. killall（批量终止进程）

```go
// 允许的目标进程
allowedTargets: ["ll-cli"]

// 允许的信号
allowedSignals: ["-15", "-SIGTERM", "-TERM"]

// 明确禁止的参数
blockedArgs: ["-u", "--user"]  // 防止跨用户操作
```

#### 3. pkexec（权限提升）

```go
// 递归验证：pkexec 后的命令也必须在白名单内
// 例如: pkexec ll-cli install app
//   → 验证 ll-cli 是否允许
//   → 验证 install 子命令是否允许
//   → 将 ll-cli 替换为实际路径 /usr/bin/ll-cli

// 最大参数数量：30
```

---

## ⚙️ 自定义命令规则（核心功能）

### 🎯 为什么要自定义规则？

LinyapsManager 的设计理念是**可扩展性优先**。不同的应用场景需要不同的命令支持：

- 🎮 游戏商店可能需要 `steam` 命令
- 📦 包管理器可能需要 `dpkg`、`rpm` 命令
- 🔧 系统工具可能需要 `systemctl`、`journalctl` 命令

**通过插件式规则系统，你可以轻松添加任何命令，而无需修改核心代码。**

---

## 📚 添加新命令：完整指南

### 方式一：快速添加（简单命令）

如果你的命令**不需要复杂参数校验**，可以用最简单的规则模板：

#### 步骤 1：创建规则文件

创建 `internal/cmdwhitelist/rules/mycommand.go`：

```go
package rules

import "linyapsmanager/internal/cmdwhitelist"

func init() {
    cmdwhitelist.Register(&myCommandRule{})
}

type myCommandRule struct{}

func (r *myCommandRule) Name() string {
    return "mycommand"  // 命令名称
}

func (r *myCommandRule) Program() string {
    return "/usr/bin/mycommand"  // 实际可执行文件路径
}

func (r *myCommandRule) NeedsEnv() bool {
    return false  // 是否需要注入会话环境变量（DISPLAY等）
}

func (r *myCommandRule) Validate(args []string) ([]string, error) {
    // 简单验证：允许所有参数
    return args, nil
}
```

#### 步骤 2：添加符号链接

编辑 `Makefile`，在 `SYMLINKS` 变量中添加命令名：

```makefile
SYMLINKS := ll-cli killall pkexec mycommand
```

#### 步骤 3：重新构建

```bash
make clean
make
```

#### 步骤 4：测试

```bash
# 启动服务端
./build/linyaps-dbus-server

# 在另一个终端测试
./build/mycommand --help
```

---

### 方式二：高级验证（带安全校验）

对于需要**严格参数控制**的命令（如 `rm`、`systemctl`），需要实现详细的 `Validate` 方法：

#### 示例：添加 `systemctl` 命令

创建 `internal/cmdwhitelist/rules/systemctl.go`：

```go
package rules

import (
    "fmt"
    "linyapsmanager/internal/cmdwhitelist"
)

func init() {
    cmdwhitelist.Register(&systemctlRule{})
}

type systemctlRule struct{}

func (r *systemctlRule) Name() string {
    return "systemctl"
}

func (r *systemctlRule) Program() string {
    return "/usr/bin/systemctl"
}

func (r *systemctlRule) NeedsEnv() bool {
    return false  // systemctl 不需要图形环境
}

// 允许的子命令白名单
var systemctlAllowedSubcmds = map[string]bool{
    "status":  true,
    "start":   true,
    "stop":    true,
    "restart": true,
    "enable":  true,
    "disable": true,
    "is-active": true,
}

// 允许操作的服务白名单
var systemctlAllowedServices = map[string]bool{
    "nginx":     true,
    "postgresql": true,
    "redis":     true,
    // 添加更多允许的服务...
}

// 最大参数数量
const systemctlMaxArgs = 10

func (r *systemctlRule) Validate(args []string) ([]string, error) {
    // 1. 检查参数数量
    if len(args) == 0 {
        return nil, fmt.Errorf("systemctl requires at least one argument")
    }
    if len(args) > systemctlMaxArgs {
        return nil, fmt.Errorf("too many arguments: max %d, got %d", 
            systemctlMaxArgs, len(args))
    }

    // 2. 提取子命令（第一个非选项参数）
    var subcmd string
    var serviceName string
    
    for i, arg := range args {
        // 跳过选项（如 --user, --system）
        if arg[0] == '-' {
            continue
        }
        
        if subcmd == "" {
            subcmd = arg
        } else if serviceName == "" {
            serviceName = arg
            break
        }
    }

    // 3. 验证子命令
    if subcmd == "" {
        return nil, fmt.Errorf("no subcommand found")
    }
    if !systemctlAllowedSubcmds[subcmd] {
        return nil, fmt.Errorf("subcommand %q is not allowed", subcmd)
    }

    // 4. 验证服务名（如果有）
    if serviceName != "" && !systemctlAllowedServices[serviceName] {
        return nil, fmt.Errorf("service %q is not allowed", serviceName)
    }

    // 5. 验证通过，返回原始参数
    return args, nil
}
```

#### 高级技巧：参数清理

有些场景下，你可能需要**修改参数**而不是直接拒绝：

```go
func (r *myRule) Validate(args []string) ([]string, error) {
    cleaned := make([]string, 0, len(args))
    
    for _, arg := range args {
        // 移除危险参数
        if arg == "--force" || arg == "-f" {
            continue  // 跳过强制选项
        }
        
        // 替换危险路径
        if strings.Contains(arg, "/etc/") {
            return nil, fmt.Errorf("cannot access /etc/")
        }
        
        cleaned = append(cleaned, arg)
    }
    
    return cleaned, nil
}
```

---

### 方式三：嵌套命令验证（pkexec 模式）

对于像 `pkexec`、`sudo` 这样的**包装命令**，需要递归验证内部命令：

```go
func (r *pkexecRule) Validate(args []string) ([]string, error) {
    if len(args) == 0 {
        return nil, fmt.Errorf("pkexec requires a command")
    }

    // 提取嵌套命令
    nestedCmd := args[0]
    nestedArgs := args[1:]

    // 递归验证嵌套命令
    nestedRule := cmdwhitelist.GetRule(nestedCmd)
    if nestedRule == nil {
        return nil, fmt.Errorf("nested command %q not allowed", nestedCmd)
    }

    // 验证嵌套命令的参数
    validatedNestedArgs, err := nestedRule.Validate(nestedArgs)
    if err != nil {
        return nil, fmt.Errorf("nested command invalid: %w", err)
    }

    // 替换命令名为完整路径
    result := []string{nestedRule.Program()}
    result = append(result, validatedNestedArgs...)
    
    return result, nil
}
```

---

## 🧪 测试你的规则

### 单元测试模板

创建 `internal/cmdwhitelist/rules/mycommand_test.go`：

```go
package rules_test

import (
    "testing"
    "linyapsmanager/internal/cmdwhitelist"
    _ "linyapsmanager/internal/cmdwhitelist/rules"
)

func TestMyCommandRule(t *testing.T) {
    tests := []struct {
        name    string
        args    []string
        wantErr bool
    }{
        {"valid args", []string{"--help"}, false},
        {"empty args", []string{}, false},
        {"dangerous args", []string{"--force"}, true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, _, err := cmdwhitelist.ValidateCommand("mycommand", tt.args)
            if (err != nil) != tt.wantErr {
                t.Errorf("got error %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

运行测试：

```bash
go test ./internal/cmdwhitelist/...
```

### 手动测试流程

```bash
# 1. 构建项目
make

# 2. 启动服务端（查看日志）
./build/linyaps-dbus-server

# 3. 在另一个终端测试客户端
./build/mycommand arg1 arg2

# 4. 检查服务端日志输出
# [INFO] ExecuteCommand command=mycommand args=[arg1 arg2]
# [INFO] command started: opID=op-12345-1
```

---

## 🚀 快速开始

### 环境要求

- Go 1.19+
- D-Bus（系统已安装）
- xdg-dbus-proxy（用于容器代理）

### 构建

```bash
# 克隆仓库
git clone https://github.com/guanzi008/org.linglong-store.LinyapsManager.git
cd org.linglong-store.LinyapsManager

# 构建所有组件
make

# 查看构建结果
ls build/
# linyaps-dbus-server   # 服务端
# linyapsctl            # 客户端主程序
# ll-cli -> linyapsctl  # 符号链接
# killall -> linyapsctl
# pkexec -> linyapsctl
```

### 运行

```bash
# 终端 1：启动服务端
./build/linyaps-dbus-server

# 终端 2：测试命令
./build/ll-cli list
./build/killall ll-cli
```

---

## 📦 安装部署

### 方式一：手动安装

```bash
# 构建
make

# 复制文件
sudo mkdir -p /usr/local/bin
sudo cp build/linyaps-dbus-server /usr/local/bin/
sudo cp build/linyapsctl /usr/local/bin/

# 创建符号链接
cd /usr/local/bin
sudo ln -s linyapsctl ll-cli
sudo ln -s linyapsctl killall
sudo ln -s linyapsctl pkexec

# 配置 D-Bus 权限
sudo cp debian/dbus/org.linglong_store.LinyapsManager.conf \
    /etc/dbus-1/system.d/
```

### 方式二：Systemd 用户服务

```bash
# 复制服务文件
mkdir -p ~/.config/systemd/user
cp debian/org.linglong-store.linyapsmanager.service \
   ~/.config/systemd/user/

# 启动服务
systemctl --user daemon-reload
systemctl --user enable linyaps-dbus-server
systemctl --user start linyaps-dbus-server

# 查看状态
systemctl --user status linyaps-dbus-server
```

### 方式三：Debian 打包

```bash
# 构建 deb 包
dpkg-buildpackage -b -uc -us

# 安装
sudo dpkg -i ../org.linglong-store.linyapsmanager_*.deb
```

---

## 🔧 配置说明

### 运行时目录结构

LinyapsManager 使用以下目录存储运行时文件：

```
/tmp/linglong-runtime-<uid>/
├── linglong/
│   ├── linyaps-proxy.sock          # 系统总线代理套接字
│   ├── linyaps-session-proxy.sock  # 会话总线代理套接字
│   └── linyaps.env                 # 可选的环境变量文件
└── dconf/                          # dconf 配置目录（容器可见）
```

或者：

```
$XDG_RUNTIME_DIR/linglong/           # 优先使用 XDG 规范目录
/run/user/<uid>/linglong/            # 备选目录
```

### 环境变量注入

服务端执行命令时会自动注入以下环境变量（针对 `NeedsEnv() == true` 的命令）：

1. **会话环境变量**（从现有用户进程抓取）：
   - `DISPLAY`
   - `WAYLAND_DISPLAY`
   - `XAUTHORITY`
   - `DBUS_SESSION_BUS_ADDRESS`
   - `XDG_RUNTIME_DIR`

2. **强制英文 locale**（确保输出可解析）：
   - `LC_ALL=C.UTF-8`
   - `LANG=C.UTF-8`
   - `LANGUAGE=en_US`

3. **容器代理地址**（如果启动了代理）：
   - `DBUS_SYSTEM_BUS_ADDRESS=unix:path=/tmp/linglong-runtime-<uid>/linglong/linyaps-proxy.sock`

### 自定义环境变量

创建 `$RUNTIME_DIR/linglong/linyaps.env` 文件：

```bash
# 自定义变量（一行一个 KEY=VALUE）
CUSTOM_VAR=value
APP_CONFIG=/path/to/config
```

---

## 🐛 故障排查

### 常见问题

#### 1. D-Bus 连接失败

```
Error: failed to connect to D-Bus: connection refused
```

**解决方案**：
- 检查服务端是否运行：`pgrep linyaps-dbus-server`
- 检查 D-Bus 配置：`cat /etc/dbus-1/system.d/org.linglong_store.LinyapsManager.conf`
- 查看系统日志：`journalctl -u dbus`

#### 2. 命令不在白名单

```
Error: command "xxx" validation failed: command not in whitelist
```

**解决方案**：
- 查看已注册命令：修改客户端调用 `cmdwhitelist.ListCommands()`
- 添加命令规则（参见上文"自定义命令规则"章节）

#### 3. 参数验证失败

```
Error: command "ll-cli" validation failed: subcommand "xxx" is not allowed
```

**解决方案**：
- 检查命令规则的 `Validate` 方法实现
- 查看 `internal/cmdwhitelist/rules/<command>.go` 中的白名单配置

#### 4. 代理套接字不存在

```
WARN: failed to spawn proxy: exec: "xdg-dbus-proxy": executable file not found
```

**解决方案**：
```bash
# Debian/Ubuntu
sudo apt install xdg-dbus-proxy

# Fedora/RHEL
sudo dnf install xdg-dbus-proxy

# Arch
sudo pacman -S xdg-dbus-proxy
```

### 查看日志

```bash
# Systemd 用户服务日志
journalctl --user -u linyaps-dbus-server -f

# 手动运行查看详细输出
./build/linyaps-dbus-server
# [INFO] D-Bus service started: name=org.linglong_store.LinyapsManager
# [INFO] proxy socket ready at /tmp/linglong-runtime-1000/linglong/linyaps-proxy.sock
```

---

## 🤝 贡献指南

### 代码规范

- 遵循 [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
- 使用 `gofmt` 格式化代码
- 为新功能添加单元测试
- 更新相关文档

### 提交新命令规则

1. **Fork 本仓库**

2. **创建规则文件**：`internal/cmdwhitelist/rules/yourcommand.go`

3. **添加测试**：`internal/cmdwhitelist/rules/yourcommand_test.go`

4. **更新 Makefile**：在 `SYMLINKS` 中添加命令名

5. **提交 Pull Request**，包含：
   - 规则文件
   - 单元测试
   - 使用示例

### 开发流程

```bash
# 1. 创建分支
git checkout -b feature/add-mycommand

# 2. 添加规则文件
vim internal/cmdwhitelist/rules/mycommand.go

# 3. 运行测试
make test

# 4. 提交代码
git add .
git commit -m "feat: add mycommand rule with validation"
git push origin feature/add-mycommand
```

---

## 📄 许可证

本项目采用 [GPLv3](LICENSE) 许可证。

---

## 🔗 相关链接

- [玲珑官方文档](https://linglong.dev/)
- [D-Bus 规范](https://dbus.freedesktop.org/doc/dbus-specification.html)
- [问题反馈](https://github.com/guanzi008/org.linglong-store.LinyapsManager/issues)

---

<div align="center">

**Made with ❤️ for the Linglong Ecosystem**

</div>
