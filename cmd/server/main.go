package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"

	"linyapsmanager/internal/cmdwhitelist"
	_ "linyapsmanager/internal/cmdwhitelist/rules" // Register command rules
	"linyapsmanager/internal/dbusconsts"
	"linyapsmanager/internal/dbusutil"
	"linyapsmanager/internal/envgrab"
	"linyapsmanager/internal/proxy"
	"linyapsmanager/internal/streaming"
)

const (
	cmdTimeout  = 5 * time.Minute
	envFileName = "linyaps.env"
)

var (
	englishLocaleEnv = []struct {
		key   string
		value string
	}{
		{"LC_ALL", "C.UTF-8"},
		{"LANG", "C.UTF-8"},
		{"LANGUAGE", "en_US"},
		{"LC_MESSAGES", "C.UTF-8"},
	}
	englishLocaleKeys = func() map[string]struct{} {
		keys := make(map[string]struct{}, len(englishLocaleEnv))
		for _, kv := range englishLocaleEnv {
			keys[kv.key] = struct{}{}
		}
		return keys
	}()
)

// LinyapsManager exposes a single D-Bus method for executing whitelisted commands.
type LinyapsManager struct {
	emitter *streaming.Emitter
}

// ExecuteCommand validates and executes a whitelisted command.
// It returns an operationID; subscribe to Output and Complete signals to receive data.
//
// Parameters:
//   - command: The command name as invoked (e.g., "ll-cli", "killall")
//   - args: Command arguments
//
// Returns:
//   - operationID: Unique ID to track this operation's output signals
func (m *LinyapsManager) ExecuteCommand(command string, args []string) (string, *dbus.Error) {
	log.Printf("[INFO] ExecuteCommand command=%s args=%v", command, args)

	// Validate command against whitelist
	program, validatedArgs, err := cmdwhitelist.ValidateCommand(command, args)
	if err != nil {
		log.Printf("[ERROR] validation failed: %v", err)
		return "", dbus.MakeFailedError(err)
	}

	// Build environment
	env := buildCommandEnv(command)

	// Execute command with streaming output
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	opID, err := streaming.RunCommand(ctx, m.emitter, env, program, validatedArgs...)
	if err != nil {
		cancel()
		log.Printf("[ERROR] failed to start command: %v", err)
		return "", dbus.MakeFailedError(err)
	}

	// Cancel context when command completes (handled by streaming)
	go func() {
		<-ctx.Done()
		cancel()
	}()

	log.Printf("[INFO] command started: opID=%s", opID)
	return opID, nil
}

// Quit causes the server to exit gracefully. This is used for updates/restarts.
func (m *LinyapsManager) Quit() *dbus.Error {
	log.Printf("[INFO] Quit requested via D-Bus, shutting down")
	// Give D-Bus a moment to send the reply
	go func() {
		time.Sleep(100 * time.Millisecond)
		os.Exit(0)
	}()
	return nil
}

// buildCommandEnv builds the environment for running commands.
func buildCommandEnv(command string) []string {
	env := os.Environ()

	// Add session environment for commands that need it (like ll-cli)
	if cmdwhitelist.NeedsSpecialEnv(command) {
		env = append(env, sessionEnv()...)
		env = append(env, loadUserEnv()...)
	}

	// Enforce English locale for stable output parsing
	return enforceEnglishLocale(env)
}

// sessionEnv grabs session-like env (DISPLAY/DBUS_SESSION/etc.) from an existing
// user process each time we spawn a command, so we can pick up a session that started
// after this service launched. Best-effort; returns nil if nothing found.
func sessionEnv() []string {
	return envgrab.CaptureSessionEnv()
}

// loadUserEnv reads an optional env file to inject user session vars (e.g., DISPLAY).
// Path: <runtimeBase>/linyaps.env (one KEY=VALUE per line).
func loadUserEnv() []string {
	base := proxy.RuntimeBase()
	path := filepath.Join(base, envFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(data), "\n")
	var env []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") || !strings.Contains(l, "=") {
			continue
		}
		env = append(env, l)
	}
	return env
}

// enforceEnglishLocale removes locale-related keys from env and appends fixed English
// values so command outputs are deterministic regardless of host locale.
func enforceEnglishLocale(env []string) []string {
	filtered := make([]string, 0, len(env)+len(englishLocaleEnv))
	for _, kv := range env {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			continue
		}
		if _, skip := englishLocaleKeys[parts[0]]; skip {
			continue
		}
		filtered = append(filtered, kv)
	}
	for _, kv := range englishLocaleEnv {
		filtered = append(filtered, kv.key+"="+kv.value)
	}
	return filtered
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	conn, err := dbusutil.Connect("")
	if err != nil {
		log.Fatalf("connect bus failed: %v", err)
	}
	defer conn.Close()

	reply, err := conn.RequestName(dbusconsts.BusName, dbus.NameFlagDoNotQueue)
	if err != nil {
		log.Fatalf("request name failed: %v", err)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		log.Fatalf("name %s already taken", dbusconsts.BusName)
	}

	emitter := streaming.NewEmitter(conn)
	mgr := &LinyapsManager{emitter: emitter}
	conn.Export(mgr, dbus.ObjectPath(dbusconsts.ObjectPath), dbusconsts.Interface)

	log.Printf("[INFO] D-Bus service started: name=%s path=%s iface=%s",
		dbusconsts.BusName, dbusconsts.ObjectPath, dbusconsts.Interface)

	// Ensure dconf dir exists for apps expecting /tmp/linglong-runtime-<uid>/dconf.
	if p, err := proxy.EnsureDconfDir(); err != nil {
		log.Printf("[WARN] failed to ensure dconf dir %s: %v", p, err)
	} else {
		log.Printf("[INFO] dconf dir ready at %s", p)
	}

	// Optionally spawn a system-bus proxy socket for containers to consume.
	if p, cleanup, err := proxy.SpawnSystemProxy(""); err != nil {
		log.Printf("[WARN] failed to spawn proxy: %v", err)
	} else if p != "" {
		log.Printf("[INFO] proxy socket ready at %s (set LINYAPS_DBUS_ADDRESS to use)", p)
		defer func() {
			if cleanup != nil {
				cleanup()
			}
		}()
	}

	// Optionally spawn a session-bus proxy for apps that need it.
	if p, cleanup, err := proxy.SpawnSessionProxy(""); err != nil {
		log.Printf("[WARN] failed to spawn session proxy: %v", err)
	} else if p != "" {
		log.Printf("[INFO] session proxy socket ready at %s (auto-injected into env)", p)
		defer func() {
			if cleanup != nil {
				cleanup()
			}
		}()
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Printf("[INFO] shutting down")
}
