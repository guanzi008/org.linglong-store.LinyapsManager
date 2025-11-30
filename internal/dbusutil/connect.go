package dbusutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/godbus/dbus/v5"
)

const (
	defaultProxyName = "linyaps-proxy.sock"
)

// DefaultProxyPath returns a proxy path under a runtime directory visible to the container.
func DefaultProxyPath() string {
	return filepath.Join(proxyRuntimeBase(), defaultProxyName)
}

// Connect returns a D-Bus connection using an explicit address if provided.
// If addr is empty, it falls back to DBUS_SYSTEM_BUS_ADDRESS and finally the
// default proxy path (if present) and finally the default system bus.
func Connect(addr string) (*dbus.Conn, error) {
	triedProxy := false
	if addr == "" {
		addr = os.Getenv("DBUS_SYSTEM_BUS_ADDRESS")
	}
	if addr == "" {
		if p := DefaultProxyPath(); fileExists(p) {
			addr = "unix:path=" + p
			triedProxy = true
		}
	}
	if addr != "" && !strings.HasPrefix(addr, "unix:path=") && !strings.HasPrefix(addr, "tcp:") {
		// Normalize bare paths to unix:path=
		if fileExists(addr) {
			addr = "unix:path=" + addr
		}
	}
	if addr != "" {
		conn, err := dialAndAuth(addr)
		if err != nil {
			// If we tried to reuse a stale proxy socket, drop it and fall back to the system bus.
			if triedProxy && errors.Is(err, syscall.ECONNREFUSED) {
				if p := DefaultProxyPath(); p != "" {
					_ = os.Remove(p)
				}
				return dbus.ConnectSystemBus()
			}
			return nil, err
		}
		return conn, nil
	}
	return dbus.ConnectSystemBus()
}

func dialAndAuth(addr string) (*dbus.Conn, error) {
	conn, err := dbus.Dial(addr)
	if err != nil {
		return nil, fmt.Errorf("dial bus %q: %w", addr, err)
	}
	// Perform auth and hello sequence because Dial skips it.
	if err := conn.Auth(nil); err != nil {
		conn.Close()
		return nil, fmt.Errorf("auth bus %q: %w", addr, err)
	}
	if err := conn.Hello(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("hello bus %q: %w", addr, err)
	}
	return conn, nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// proxyRuntimeBase mirrors the logic in proxy.runtimeBase without import cycle.
func proxyRuntimeBase() string {
	uid := os.Getuid()
	candidate := filepath.Join("/tmp", "linglong-runtime-"+strconv.Itoa(uid), "linglong")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		candidate = filepath.Join(xdg, "linglong")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return filepath.Join("/run/user", strconv.Itoa(uid), "linglong")
}
