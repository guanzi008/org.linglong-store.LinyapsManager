# LinyapsManager

English | [简体中文](README.md)

<div align="center">

**Secure Host Command Execution for Containerized Applications**

[![License](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.19-blue.svg)](go.mod)

</div>

---

## 📖 Overview

LinyapsManager is a D-Bus-based command proxy service designed for containerized environments like **Linglong App Store**. It allows applications running inside containers to securely execute specific host commands (such as `ll-cli`, `killall`, `pkexec`) via D-Bus interfaces, while ensuring system security through **whitelist mechanism** and **argument validation**.

### Key Features

- 🔒 **Security First**: Whitelist mechanism + strict argument validation to prevent command injection
- 🔌 **Plugin-based Rules**: Independent validation rules for each command, easy to extend
- 📡 **Streaming Output**: Real-time command output transmission (stdout/stderr) via D-Bus signals
- 🎭 **Transparent Invocation**: Client masquerades as target commands via symlinks, transparent to users
- 🌍 **Environment Injection**: Auto-capture session environment variables (DISPLAY, DBUS_SESSION, etc.)
- 🔄 **Container Friendly**: Provides sockets via xdg-dbus-proxy for direct container access

---

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────────┐
│              Containerized Application                       │
│     (Linglong App Store / Other Container Apps)             │
└────────────────────┬────────────────────────────────────────┘
                     │ Invoke via symlink
                     │ ./ll-cli install app
                     ↓
┌─────────────────────────────────────────────────────────────┐
│              Client (linyapsctl)                             │
│  • Recognize program name (ll-cli/killall/pkexec)           │
│  • Call ExecuteCommand(cmd, args) via D-Bus                 │
└────────────────────┬────────────────────────────────────────┘
                     │ D-Bus Communication
                     │ org.linglong_store.LinyapsManager
                     ↓
┌─────────────────────────────────────────────────────────────┐
│              Server (linyaps-dbus-server)                    │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  1. Command Whitelist Validation (cmdwhitelist)     │   │
│  │     • Check if command is allowed                   │   │
│  │     • Invoke corresponding Rule for validation      │   │
│  └─────────────────────────────────────────────────────┘   │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  2. Environment Preparation                         │   │
│  │     • Inject session env vars (DISPLAY/DBUS)        │   │
│  │     • Force English locale (ensure parsable output) │   │
│  └─────────────────────────────────────────────────────┘   │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  3. Streaming Execution                             │   │
│  │     • Start command and get operationID             │   │
│  │     • Stream output via D-Bus signals               │   │
│  │     • Output(opID, data, isStderr)                  │   │
│  │     • Complete(opID, exitCode, errorMsg)            │   │
│  └─────────────────────────────────────────────────────┘   │
└────────────────────┬────────────────────────────────────────┘
                     │ Execute actual command
                     ↓
              ┌──────────────┐
              │ Host Commands│
              │ ll-cli       │
              │ killall      │
              │ pkexec       │
              └──────────────┘
```

### D-Bus Interface

**Service Name**: `org.linglong_store.LinyapsManager`  
**Object Path**: `/org/linglong_store/LinyapsManager`  
**Interface**: `org.linglong_store.LinyapsManager`

#### Methods

- **ExecuteCommand**(command: `string`, args: `[]string`) → operationID: `string`
  - Validate and execute whitelisted command
  - Returns operation ID for receiving streaming output

- **Ping**() → `string`
  - Health check, returns "pong"

- **Quit**()
  - Graceful shutdown (for updates/restarts)

#### Signals

- **Output**(operationID: `string`, data: `string`, isStderr: `bool`)
  - Streaming output signal, data contains command output chunk

- **Complete**(operationID: `string`, exitCode: `int32`, errorMsg: `string`)
  - Command completion signal with exit code and error message

---

## 🔐 Security Model

### Whitelist Mechanism

All commands must be registered via **plugin-based rules** before execution. Each command rule is independently defined in `internal/cmdwhitelist/rules/`:

```
internal/cmdwhitelist/rules/
├── doc.go          # Rule system documentation
├── llcli.go        # ll-cli command rule
├── killall.go      # killall command rule
└── pkexec.go       # pkexec command rule
```

### Command Validation Flow

```go
// 1. Check if command is whitelisted
rule := cmdwhitelist.GetRule("ll-cli")
if rule == nil {
    return error("command not allowed")
}

// 2. Call rule's Validate method to check arguments
validatedArgs, err := rule.Validate([]string{"install", "app"})
if err != nil {
    return error("validation failed")
}

// 3. Get actual program path and execute
program := rule.Program()  // e.g., "ll-cli" or "/usr/bin/killall"
exec(program, validatedArgs...)
```

### Security Policies of Built-in Commands

#### 1. ll-cli (Linglong CLI Tool)

```go
// Allowed subcommands
allowedSubcmds: [
    "list", "search", "info", "install", "uninstall",
    "run", "kill", "exec", "ps", "repo", "content", "prune"
]

// Max arguments: 20
// Needs environment injection: Yes (DISPLAY, DBUS_SESSION, etc.)
```

#### 2. killall (Batch Process Termination)

```go
// Allowed target processes
allowedTargets: ["ll-cli"]

// Allowed signals
allowedSignals: ["-15", "-SIGTERM", "-TERM"]

// Explicitly blocked arguments
blockedArgs: ["-u", "--user"]  // Prevent cross-user operations
```

#### 3. pkexec (Privilege Escalation)

```go
// Recursive validation: Command after pkexec must also be whitelisted
// Example: pkexec ll-cli install app
//   → Validate if ll-cli is allowed
//   → Validate if install subcommand is allowed
//   → Replace ll-cli with actual path /usr/bin/ll-cli

// Max arguments: 30
```

---

## ⚙️ Custom Command Rules (Core Feature)

### 🎯 Why Custom Rules?

LinyapsManager is designed with **extensibility as a priority**. Different application scenarios require different command support:

- 🎮 Gaming stores may need `steam` command
- 📦 Package managers may need `dpkg`, `rpm` commands
- 🔧 System tools may need `systemctl`, `journalctl` commands

**With the plugin-based rule system, you can easily add any command without modifying core code.**

---

## 📚 Adding New Commands: Complete Guide

### Method 1: Quick Addition (Simple Commands)

If your command **doesn't require complex argument validation**, use the simplest rule template:

#### Step 1: Create Rule File

Create `internal/cmdwhitelist/rules/mycommand.go`:

```go
package rules

import "linyapsmanager/internal/cmdwhitelist"

func init() {
    cmdwhitelist.Register(&myCommandRule{})
}

type myCommandRule struct{}

func (r *myCommandRule) Name() string {
    return "mycommand"  // Command name
}

func (r *myCommandRule) Program() string {
    return "/usr/bin/mycommand"  // Actual executable path
}

func (r *myCommandRule) NeedsEnv() bool {
    return false  // Whether to inject session env vars (DISPLAY, etc.)
}

func (r *myCommandRule) Validate(args []string) ([]string, error) {
    // Simple validation: allow all arguments
    return args, nil
}
```

#### Step 2: Add Symlink

Edit `Makefile`, add command name to `SYMLINKS` variable:

```makefile
SYMLINKS := ll-cli killall pkexec mycommand
```

#### Step 3: Rebuild

```bash
make clean
make
```

#### Step 4: Test

```bash
# Start server
./build/linyaps-dbus-server

# Test in another terminal
./build/mycommand --help
```

---

### Method 2: Advanced Validation (With Security Checks)

For commands requiring **strict argument control** (like `rm`, `systemctl`), implement detailed `Validate` method:

#### Example: Adding `systemctl` Command

Create `internal/cmdwhitelist/rules/systemctl.go`:

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
    return false  // systemctl doesn't need graphical environment
}

// Allowed subcommands whitelist
var systemctlAllowedSubcmds = map[string]bool{
    "status":  true,
    "start":   true,
    "stop":    true,
    "restart": true,
    "enable":  true,
    "disable": true,
    "is-active": true,
}

// Allowed services whitelist
var systemctlAllowedServices = map[string]bool{
    "nginx":     true,
    "postgresql": true,
    "redis":     true,
    // Add more allowed services...
}

// Max argument count
const systemctlMaxArgs = 10

func (r *systemctlRule) Validate(args []string) ([]string, error) {
    // 1. Check argument count
    if len(args) == 0 {
        return nil, fmt.Errorf("systemctl requires at least one argument")
    }
    if len(args) > systemctlMaxArgs {
        return nil, fmt.Errorf("too many arguments: max %d, got %d", 
            systemctlMaxArgs, len(args))
    }

    // 2. Extract subcommand (first non-option argument)
    var subcmd string
    var serviceName string
    
    for i, arg := range args {
        // Skip options (like --user, --system)
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

    // 3. Validate subcommand
    if subcmd == "" {
        return nil, fmt.Errorf("no subcommand found")
    }
    if !systemctlAllowedSubcmds[subcmd] {
        return nil, fmt.Errorf("subcommand %q is not allowed", subcmd)
    }

    // 4. Validate service name (if present)
    if serviceName != "" && !systemctlAllowedServices[serviceName] {
        return nil, fmt.Errorf("service %q is not allowed", serviceName)
    }

    // 5. Validation passed, return original arguments
    return args, nil
}
```

#### Advanced Tip: Argument Sanitization

In some scenarios, you may need to **modify arguments** instead of outright rejection:

```go
func (r *myRule) Validate(args []string) ([]string, error) {
    cleaned := make([]string, 0, len(args))
    
    for _, arg := range args {
        // Remove dangerous arguments
        if arg == "--force" || arg == "-f" {
            continue  // Skip force option
        }
        
        // Block dangerous paths
        if strings.Contains(arg, "/etc/") {
            return nil, fmt.Errorf("cannot access /etc/")
        }
        
        cleaned = append(cleaned, arg)
    }
    
    return cleaned, nil
}
```

---

### Method 3: Nested Command Validation (pkexec Mode)

For **wrapper commands** like `pkexec`, `sudo`, recursive validation of inner commands is needed:

```go
func (r *pkexecRule) Validate(args []string) ([]string, error) {
    if len(args) == 0 {
        return nil, fmt.Errorf("pkexec requires a command")
    }

    // Extract nested command
    nestedCmd := args[0]
    nestedArgs := args[1:]

    // Recursively validate nested command
    nestedRule := cmdwhitelist.GetRule(nestedCmd)
    if nestedRule == nil {
        return nil, fmt.Errorf("nested command %q not allowed", nestedCmd)
    }

    // Validate nested command's arguments
    validatedNestedArgs, err := nestedRule.Validate(nestedArgs)
    if err != nil {
        return nil, fmt.Errorf("nested command invalid: %w", err)
    }

    // Replace command name with full path
    result := []string{nestedRule.Program()}
    result = append(result, validatedNestedArgs...)
    
    return result, nil
}
```

---

## 🧪 Testing Your Rules

### Unit Test Template

Create `internal/cmdwhitelist/rules/mycommand_test.go`:

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

Run tests:

```bash
go test ./internal/cmdwhitelist/...
```

### Manual Testing Workflow

```bash
# 1. Build project
make

# 2. Start server (watch logs)
./build/linyaps-dbus-server

# 3. Test client in another terminal
./build/mycommand arg1 arg2

# 4. Check server log output
# [INFO] ExecuteCommand command=mycommand args=[arg1 arg2]
# [INFO] command started: opID=op-12345-1
```

---

## 🚀 Quick Start

### Requirements

- Go 1.19+
- D-Bus (system installed)
- xdg-dbus-proxy (for container proxy)

### Build

```bash
# Clone repository
git clone https://github.com/guanzi008/org.linglong-store.LinyapsManager.git
cd org.linglong-store.LinyapsManager

# Build all components
make

# Check build results
ls build/
# linyaps-dbus-server   # Server
# linyapsctl            # Client main program
# ll-cli -> linyapsctl  # Symlinks
# killall -> linyapsctl
# pkexec -> linyapsctl
```

### Run

```bash
# Terminal 1: Start server
./build/linyaps-dbus-server

# Terminal 2: Test commands
./build/ll-cli list
./build/killall ll-cli
```

---

## 📦 Installation & Deployment

### Method 1: Manual Installation

```bash
# Build
make

# Copy files
sudo mkdir -p /usr/local/bin
sudo cp build/linyaps-dbus-server /usr/local/bin/
sudo cp build/linyapsctl /usr/local/bin/

# Create symlinks
cd /usr/local/bin
sudo ln -s linyapsctl ll-cli
sudo ln -s linyapsctl killall
sudo ln -s linyapsctl pkexec

# Configure D-Bus permissions
sudo cp debian/dbus/org.linglong_store.LinyapsManager.conf \
    /etc/dbus-1/system.d/
```

### Method 2: Systemd User Service

```bash
# Copy service file
mkdir -p ~/.config/systemd/user
cp debian/org.linglong-store.linyapsmanager.service \
   ~/.config/systemd/user/

# Start service
systemctl --user daemon-reload
systemctl --user enable linyaps-dbus-server
systemctl --user start linyaps-dbus-server

# Check status
systemctl --user status linyaps-dbus-server
```

### Method 3: Debian Packaging

```bash
# Build deb package
dpkg-buildpackage -b -uc -us

# Install
sudo dpkg -i ../org.linglong-store.linyapsmanager_*.deb
```

---

## 🔧 Configuration

### Runtime Directory Structure

LinyapsManager uses the following directories for runtime files:

```
/tmp/linglong-runtime-<uid>/
├── linglong/
│   ├── linyaps-proxy.sock          # System bus proxy socket
│   ├── linyaps-session-proxy.sock  # Session bus proxy socket
│   └── linyaps.env                 # Optional environment variable file
└── dconf/                          # dconf config dir (visible to container)
```

Or:

```
$XDG_RUNTIME_DIR/linglong/           # Prefer XDG standard directory
/run/user/<uid>/linglong/            # Fallback directory
```

### Environment Variable Injection

Server automatically injects the following environment variables when executing commands (for `NeedsEnv() == true` commands):

1. **Session environment variables** (captured from existing user processes):
   - `DISPLAY`
   - `WAYLAND_DISPLAY`
   - `XAUTHORITY`
   - `DBUS_SESSION_BUS_ADDRESS`
   - `XDG_RUNTIME_DIR`

2. **Force English locale** (ensure parsable output):
   - `LC_ALL=C.UTF-8`
   - `LANG=C.UTF-8`
   - `LANGUAGE=en_US`

3. **Container proxy address** (if proxy is started):
   - `DBUS_SYSTEM_BUS_ADDRESS=unix:path=/tmp/linglong-runtime-<uid>/linglong/linyaps-proxy.sock`

### Custom Environment Variables

Create `$RUNTIME_DIR/linglong/linyaps.env` file:

```bash
# Custom variables (one KEY=VALUE per line)
CUSTOM_VAR=value
APP_CONFIG=/path/to/config
```

---

## 🐛 Troubleshooting

### Common Issues

#### 1. D-Bus Connection Failed

```
Error: failed to connect to D-Bus: connection refused
```

**Solution**:
- Check if server is running: `pgrep linyaps-dbus-server`
- Check D-Bus config: `cat /etc/dbus-1/system.d/org.linglong_store.LinyapsManager.conf`
- View system logs: `journalctl -u dbus`

#### 2. Command Not Whitelisted

```
Error: command "xxx" validation failed: command not in whitelist
```

**Solution**:
- View registered commands: modify client to call `cmdwhitelist.ListCommands()`
- Add command rule (see "Custom Command Rules" section above)

#### 3. Argument Validation Failed

```
Error: command "ll-cli" validation failed: subcommand "xxx" is not allowed
```

**Solution**:
- Check command rule's `Validate` method implementation
- Review whitelist configuration in `internal/cmdwhitelist/rules/<command>.go`

#### 4. Proxy Socket Not Found

```
WARN: failed to spawn proxy: exec: "xdg-dbus-proxy": executable file not found
```

**Solution**:
```bash
# Debian/Ubuntu
sudo apt install xdg-dbus-proxy

# Fedora/RHEL
sudo dnf install xdg-dbus-proxy

# Arch
sudo pacman -S xdg-dbus-proxy
```

### View Logs

```bash
# Systemd user service logs
journalctl --user -u linyaps-dbus-server -f

# Manual run with verbose output
./build/linyaps-dbus-server
# [INFO] D-Bus service started: name=org.linglong_store.LinyapsManager
# [INFO] proxy socket ready at /tmp/linglong-runtime-1000/linglong/linyaps-proxy.sock
```

---

## 🤝 Contributing

### Code Standards

- Follow [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
- Format code with `gofmt`
- Add unit tests for new features
- Update relevant documentation

### Submitting New Command Rules

1. **Fork this repository**

2. **Create rule file**: `internal/cmdwhitelist/rules/yourcommand.go`

3. **Add tests**: `internal/cmdwhitelist/rules/yourcommand_test.go`

4. **Update Makefile**: Add command name to `SYMLINKS`

5. **Submit Pull Request**, including:
   - Rule file
   - Unit tests
   - Usage examples

### Development Workflow

```bash
# 1. Create branch
git checkout -b feature/add-mycommand

# 2. Add rule file
vim internal/cmdwhitelist/rules/mycommand.go

# 3. Run tests
make test

# 4. Commit code
git add .
git commit -m "feat: add mycommand rule with validation"
git push origin feature/add-mycommand
```

---

## 📄 License

This project is licensed under the [GPLv3](LICENSE) license.

---

## 🔗 Related Links

- [Linglong Official Documentation](https://linglong.dev/)
- [D-Bus Specification](https://dbus.freedesktop.org/doc/dbus-specification.html)
- [Issue Tracker](https://github.com/guanzi008/org.linglong-store.LinyapsManager/issues)

---

<div align="center">

**Made with ❤️ for the Linglong Ecosystem**

</div>
