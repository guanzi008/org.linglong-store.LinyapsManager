package proxy

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const (
	defaultSessionProxyName = "linyaps-session-proxy.sock"
)

// SpawnSessionProxy starts xdg-dbus-proxy for the user's session bus and writes
// a proxy socket under the shared runtime dir. It returns the proxy path and a
// cleanup func. If xdg-dbus-proxy is absent or session bus address is
// unavailable, it returns empty path and nil cleanup.
func SpawnSessionProxy(sessionBusAddr string) (string, func(), error) {
	bin, err := exec.LookPath("xdg-dbus-proxy")
	if err != nil {
		return "", nil, nil
	}
	if sessionBusAddr == "" {
		sessionBusAddr = os.Getenv("DBUS_SESSION_BUS_ADDRESS")
	}
	if sessionBusAddr == "" {
		uid := os.Getuid()
		sessionBusAddr = fmt.Sprintf("unix:path=/run/user/%d/bus", uid)
	}

	proxyPath := defaultSessionProxyPath()
	if err := os.MkdirAll(filepath.Dir(proxyPath), 0o700); err != nil {
		return "", nil, fmt.Errorf("create proxy dir: %w", err)
	}
	_ = os.Remove(proxyPath)

	// Use a filtered proxy with broad but valid name globs to avoid exposing the
	// entire session bus while keeping compatibility.
	args := []string{
		sessionBusAddr,
		proxyPath,
		"--filter",
		"--see=org.freedesktop.*", "--talk=org.freedesktop.*",
		"--see=org.*", "--talk=org.*",
		"--see=com.*", "--talk=com.*",
		"--see=net.*", "--talk=net.*",
		"--see=io.*", "--talk=io.*",
		"--see=cn.*", "--talk=cn.*",
	}
	cmd := exec.Command(bin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return "", nil, fmt.Errorf("start session proxy: %w", err)
	}

	if err := waitForSocket(proxyPath, 2*time.Second); err != nil {
		_ = cmd.Process.Kill()
		return "", nil, err
	}

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		select {
		case <-ctx.Done():
		}
		_ = os.Remove(proxyPath)
	}
	return proxyPath, cleanup, nil
}

func defaultSessionProxyPath() string {
	return filepath.Join(runtimeBase(), defaultSessionProxyName)
}

// DefaultSessionProxyPath exposes the default path to other packages.
func DefaultSessionProxyPath() string {
	return defaultSessionProxyPath()
}
