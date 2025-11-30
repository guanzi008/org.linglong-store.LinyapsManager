package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"

	"linyapsmanager/internal/dbusconsts"
	"linyapsmanager/internal/dbusutil"
	"linyapsmanager/internal/envgrab"
	"linyapsmanager/internal/proxy"
	"linyapsmanager/internal/streaming"
)

const (
	linyapsCmd = "ll-cli"
	cmdTimeout = 2 * time.Minute
)

const (
	envFileName = "linyaps.env"
)

var (
	appIDPattern       = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,64}$`)
	versionPattern     = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,64}$`)
	containerIDPattern = regexp.MustCompile(`^[a-fA-F0-9]{6,64}$`)
)

func validateAppID(id string) error {
	if !appIDPattern.MatchString(id) {
		return fmt.Errorf("invalid appid: %q", id)
	}
	return nil
}

func validateVersion(v string) error {
	if v == "" {
		return fmt.Errorf("version cannot be empty")
	}
	if !versionPattern.MatchString(v) {
		return fmt.Errorf("invalid version: %q", v)
	}
	return nil
}

func appRef(appID, version string) (string, error) {
	if err := validateAppID(appID); err != nil {
		return "", err
	}
	if err := validateVersion(version); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%s", appID, version), nil
}

// buildLinyapsEnv builds the environment for running ll-cli commands.
func buildLinyapsEnv() []string {
	env := os.Environ()
	env = append(env, sessionEnv()...)
	env = append(env, loadUserEnv()...)
	if p := os.Getenv("LINYAPS_DBUS_ADDRESS"); p != "" {
		env = append(env, "DBUS_SYSTEM_BUS_ADDRESS="+p)
	} else if p := os.Getenv("DBUS_SYSTEM_BUS_ADDRESS"); p != "" {
		env = append(env, "DBUS_SYSTEM_BUS_ADDRESS="+p)
	} else if p := dbusutil.DefaultProxyPath(); fileExists(p) {
		env = append(env, "DBUS_SYSTEM_BUS_ADDRESS=unix:path="+p)
	}
	if p := os.Getenv("LINYAPS_SESSION_BUS_ADDRESS"); p != "" {
		env = append(env, "DBUS_SESSION_BUS_ADDRESS="+p)
	} else if p := os.Getenv("DBUS_SESSION_BUS_ADDRESS"); p != "" {
		env = append(env, "DBUS_SESSION_BUS_ADDRESS="+p)
	} else if p := proxy.DefaultSessionProxyPath(); fileExists(p) {
		env = append(env, "DBUS_SESSION_BUS_ADDRESS=unix:path="+p)
	}
	return env
}

func runLinyaps(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, linyapsCmd, args...)
	cmd.Env = buildLinyapsEnv()

	out, err := cmd.CombinedOutput()
	output := string(out)
	if err != nil {
		return output, fmt.Errorf("command %v failed: %w, output=%s", args, err, output)
	}
	return output, nil
}

type LinyapsManager struct {
	emitter *streaming.Emitter
}

// Help -> ll-cli --help
func (m *LinyapsManager) Help() (string, *dbus.Error) {
	log.Printf("[INFO] Help")
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	out, err := runLinyaps(ctx, "--help")
	if err != nil {
		return out, dbus.MakeFailedError(err)
	}
	return out, nil
}

func (m *LinyapsManager) GetVersion(json bool) (string, *dbus.Error) {
	log.Printf("[INFO] GetVersion json=%v", json)
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	args := []string{}
	if json {
		args = append(args, "--json")
	}
	args = append(args, "--version")

	out, err := runLinyaps(ctx, args...)
	if err != nil {
		return out, dbus.MakeFailedError(err)
	}
	return out, nil
}

func (m *LinyapsManager) RepoShow(json bool) (string, *dbus.Error) {
	log.Printf("[INFO] RepoShow json=%v", json)
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	args := []string{}
	if json {
		args = append(args, "--json")
	}
	args = append(args, "repo", "show")

	out, err := runLinyaps(ctx, args...)
	if err != nil {
		return out, dbus.MakeFailedError(err)
	}
	return out, nil
}

func (m *LinyapsManager) ListAll(json bool) (string, *dbus.Error) {
	log.Printf("[INFO] ListAll json=%v", json)
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	args := []string{}
	if json {
		args = append(args, "--json")
	}
	args = append(args, "list", "--type=all")

	out, err := runLinyaps(ctx, args...)
	if err != nil {
		return out, dbus.MakeFailedError(err)
	}
	return out, nil
}

func (m *LinyapsManager) ListUpgradable(json bool) (string, *dbus.Error) {
	log.Printf("[INFO] ListUpgradable json=%v", json)
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	args := []string{}
	if json {
		args = append(args, "--json")
	}
	args = append(args, "list", "--upgradable")

	out, err := runLinyaps(ctx, args...)
	if err != nil {
		return out, dbus.MakeFailedError(err)
	}
	return out, nil
}

func (m *LinyapsManager) ListUpgradableApp(json bool) (string, *dbus.Error) {
	log.Printf("[INFO] ListUpgradableApp json=%v", json)
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	args := []string{}
	if json {
		args = append(args, "--json")
	}
	args = append(args, "list", "--upgradable", "--type=app")

	out, err := runLinyaps(ctx, args...)
	if err != nil {
		return out, dbus.MakeFailedError(err)
	}
	return out, nil
}

func (m *LinyapsManager) Search(keyword string, json bool) (string, *dbus.Error) {
	log.Printf("[INFO] Search keyword=%s json=%v", keyword, json)
	if keyword == "" {
		return "", dbus.MakeFailedError(fmt.Errorf("keyword cannot be empty"))
	}
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	args := []string{"search", keyword}
	if json {
		args = append(args, "--json")
	}

	out, err := runLinyaps(ctx, args...)
	if err != nil {
		return out, dbus.MakeFailedError(err)
	}
	return out, nil
}

func (m *LinyapsManager) Info(appID string) (string, *dbus.Error) {
	log.Printf("[INFO] Info appID=%s", appID)
	if err := validateAppID(appID); err != nil {
		return "", dbus.MakeFailedError(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	out, err := runLinyaps(ctx, "info", appID)
	if err != nil {
		return out, dbus.MakeFailedError(err)
	}
	return out, nil
}

func (m *LinyapsManager) Ps(json bool) (string, *dbus.Error) {
	log.Printf("[INFO] Ps json=%v", json)
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	args := []string{}
	if json {
		args = append(args, "--json")
	}
	args = append(args, "ps")

	out, err := runLinyaps(ctx, args...)
	if err != nil {
		return out, dbus.MakeFailedError(err)
	}
	return out, nil
}

func (m *LinyapsManager) Install(appID, version string, force bool) (string, *dbus.Error) {
	log.Printf("[INFO] Install appID=%s version=%s force=%v", appID, version, force)
	var ref string
	if version == "" {
		if err := validateAppID(appID); err != nil {
			return "", dbus.MakeFailedError(err)
		}
		ref = appID
	} else {
		r, err := appRef(appID, version)
		if err != nil {
			return "", dbus.MakeFailedError(err)
		}
		ref = r
	}

	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	args := []string{"install", ref, "-y"}
	if force {
		args = append(args, "--force")
	}

	out, err := runLinyaps(ctx, args...)
	if err != nil {
		return out, dbus.MakeFailedError(err)
	}
	return out, nil
}

// InstallStream starts an install operation and streams output via D-Bus signals.
// Returns an operationID; subscribe to Output and Complete signals to receive data.
func (m *LinyapsManager) InstallStream(appID, version string, force bool) (string, *dbus.Error) {
	log.Printf("[INFO] InstallStream appID=%s version=%s force=%v", appID, version, force)
	var ref string
	if version == "" {
		if err := validateAppID(appID); err != nil {
			return "", dbus.MakeFailedError(err)
		}
		ref = appID
	} else {
		r, err := appRef(appID, version)
		if err != nil {
			return "", dbus.MakeFailedError(err)
		}
		ref = r
	}

	args := []string{"install", ref, "-y"}
	if force {
		args = append(args, "--force")
	}

	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	opID, err := streaming.RunCommandStreamingPTY(ctx, m.emitter, buildLinyapsEnv(), linyapsCmd, args...)
	if err != nil {
		cancel()
		return "", dbus.MakeFailedError(err)
	}
	go func() {
		<-ctx.Done()
		cancel()
	}()
	return opID, nil
}

// 验证流式内容传输。验证通过。
func (m *LinyapsManager) TestStream() (string, *dbus.Error) {
	operationID := streaming.GenerateOperationID()

	// Stream asynchronously so the D-Bus call returns immediately and the client can
	// consume Output signals while they arrive.
	go func(operationID string) {
		log.Printf("[INFO] 测试流式输出")
		emitter := m.emitter
		for i := 0; i < 30; i++ {
			log.Printf("[INFO] 流式输出第 %d 行", i)
			time.Sleep(1 * time.Second) // 模拟输出延迟
			if err := emitter.EmitOutput(operationID, strconv.Itoa(i)+"\n", false); err != nil {
				fmt.Fprintf(os.Stderr, "[streaming] failed to emit output: %v\n", err)
			}
		}
		if err := emitter.EmitComplete(operationID, 0, ""); err != nil {
			fmt.Fprintf(os.Stderr, "[streaming] failed to emit complete: %v\n", err)
		}
		log.Printf("[INFO] 流式输出完成")
	}(operationID)

	return operationID, nil
}

func (m *LinyapsManager) Uninstall(appID, version string) (string, *dbus.Error) {
	log.Printf("[INFO] Uninstall appID=%s version=%s", appID, version)
	var ref string
	if version == "" {
		if err := validateAppID(appID); err != nil {
			return "", dbus.MakeFailedError(err)
		}
		ref = appID
	} else {
		r, err := appRef(appID, version)
		if err != nil {
			return "", dbus.MakeFailedError(err)
		}
		ref = r
	}

	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	out, err := runLinyaps(ctx, "uninstall", ref)
	if err != nil {
		return out, dbus.MakeFailedError(err)
	}
	return out, nil
}

func (m *LinyapsManager) Run(appID, version string) (string, *dbus.Error) {
	log.Printf("[INFO] Run appID=%s version=%s", appID, version)
	var args []string
	if version == "" {
		if err := validateAppID(appID); err != nil {
			return "", dbus.MakeFailedError(err)
		}
		args = []string{"run", appID}
	} else {
		ref, err := appRef(appID, version)
		if err != nil {
			return "", dbus.MakeFailedError(err)
		}
		args = []string{"run", ref}
	}
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	out, err := runLinyaps(ctx, args...)
	if err != nil {
		return out, dbus.MakeFailedError(err)
	}
	return out, nil
}

func (m *LinyapsManager) Kill(appID string) (string, *dbus.Error) {
	log.Printf("[INFO] Kill appID=%s", appID)
	if err := validateAppID(appID); err != nil {
		return "", dbus.MakeFailedError(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	out, err := runLinyaps(ctx, "kill", appID)
	if err != nil {
		return out, dbus.MakeFailedError(err)
	}
	return out, nil
}

func (m *LinyapsManager) Prune() (string, *dbus.Error) {
	log.Printf("[INFO] Prune")
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	out, err := runLinyaps(ctx, "prune")
	if err != nil {
		return out, dbus.MakeFailedError(err)
	}
	return out, nil
}

// Exec -> ll-cli <container> -- <args...>
func (m *LinyapsManager) Exec(container string, args []string) (string, *dbus.Error) {
	log.Printf("[INFO] Exec container=%s args=%v", container, args)
	if container == "" {
		return "", dbus.MakeFailedError(fmt.Errorf("container cannot be empty"))
	}
	if len(args) == 0 {
		return "", dbus.MakeFailedError(fmt.Errorf("args cannot be empty"))
	}
	if err := validateAppID(container); err != nil {
		if !containerIDPattern.MatchString(container) {
			return "", dbus.MakeFailedError(err)
		}
	}

	all := []string{container, "--"}
	all = append(all, args...)

	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	out, err := runLinyaps(ctx, all...)
	if err != nil {
		return out, dbus.MakeFailedError(err)
	}
	return out, nil
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	busAddr := os.Getenv("LINYAPS_DBUS_ADDRESS")
	conn, err := dbusutil.Connect(busAddr)
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

	// Optionally spawn a session-bus proxy for apps that need it (e.g., WeChat).
	if p, cleanup, err := proxy.SpawnSessionProxy(""); err != nil {
		log.Printf("[WARN] failed to spawn session proxy: %v", err)
	} else if p != "" {
		log.Printf("[INFO] session proxy socket ready at %s (auto-injected into ll-cli env)", p)
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

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// sessionEnv grabs session-like env (DISPLAY/DBUS_SESSION/etc.) from an existing
// user process each time we spawn ll-cli, so we can pick up a session that started
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
