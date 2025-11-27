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
	defaultProxyName = "linyaps-proxy.sock"
)

// SpawnSystemProxy starts xdg-dbus-proxy to forward org.linglong_store.LinyapsManager
// from the system bus to a unix socket that containers can access. It returns
// the proxy path and a cleanup func. If xdg-dbus-proxy is not available, it
// returns empty path and nil cleanup.
func SpawnSystemProxy(busAddress string) (string, func(), error) {
	if busAddress == "" {
		busAddress = "unix:path=/var/run/dbus/system_bus_socket"
	}
	bin, err := exec.LookPath("xdg-dbus-proxy")
	if err != nil {
		return "", nil, nil
	}

	proxyPath := defaultProxyPath()
	if err := os.MkdirAll(filepath.Dir(proxyPath), 0o700); err != nil {
		return "", nil, fmt.Errorf("create proxy dir: %w", err)
	}
	_ = os.Remove(proxyPath)

	// Note: xdg-dbus-proxy expects the address/path first, then options.
	cmd := exec.Command(
		bin,
		busAddress,
		proxyPath,
		"--talk=org.linglong_store.LinyapsManager",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return "", nil, fmt.Errorf("start proxy: %w", err)
	}

	// Wait briefly for the socket to appear.
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

func defaultProxyPath() string {
	return filepath.Join(runtimeBase(), defaultProxyName)
}

func waitForSocket(p string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(p); err == nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("proxy socket %s not created in time", p)
}
