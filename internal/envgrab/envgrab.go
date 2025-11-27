package envgrab

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// CaptureSessionEnv tries to collect session-like environment variables from
// another process of the same uid that already has DISPLAY/DBUS session set,
// so that GUI apps started by the service behave like user-launched ones.
// Returns a slice of "KEY=VALUE". Best-effort; returns nil on failure.
func CaptureSessionEnv() []string {
	uid := os.Getuid()
	procEntries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}

	for _, e := range procEntries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid <= 1 {
			continue
		}
		if !sameUID(pid, uid) {
			continue
		}
		env, err := readEnviron(pid)
		if err != nil || len(env) == 0 {
			continue
		}
		if hasDisplay(env) {
			return filterInteresting(env)
		}
	}
	return nil
}

func sameUID(pid, uid int) bool {
	statusPath := filepath.Join("/proc", strconv.Itoa(pid), "status")
	data, err := os.ReadFile(statusPath)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "Uid:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 && fields[1] == fmt.Sprint(uid) {
				return true
			}
			return false
		}
	}
	return false
}

func readEnviron(pid int) ([]string, error) {
	envPath := filepath.Join("/proc", strconv.Itoa(pid), "environ")
	data, err := os.ReadFile(envPath)
	if err != nil {
		return nil, err
	}
	raw := strings.Split(string(data), "\x00")
	var out []string
	for _, kv := range raw {
		if kv == "" || !strings.Contains(kv, "=") {
			continue
		}
		out = append(out, kv)
	}
	return out, nil
}

func hasDisplay(env []string) bool {
	for _, kv := range env {
		if strings.HasPrefix(kv, "DISPLAY=") || strings.HasPrefix(kv, "WAYLAND_DISPLAY=") {
			return true
		}
	}
	return false
}

func filterInteresting(env []string) []string {
	keep := map[string]bool{
		"DISPLAY":                  true,
		"WAYLAND_DISPLAY":          true,
		"XAUTHORITY":               true,
		"DBUS_SESSION_BUS_ADDRESS": true,
		"DBUS_SYSTEM_BUS_ADDRESS":  true,
		"XDG_RUNTIME_DIR":          true,
		"LANG":                     true,
		"LC_ALL":                   true,
		"PATH":                     true,
		"QT_IM_MODULE":             true,
		"GTK_IM_MODULE":            true,
		"XMODIFIERS":               true,
		"HOME":                     true,
	}
	var out []string
	for _, kv := range env {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			continue
		}
		if keep[parts[0]] {
			out = append(out, kv)
		}
	}
	return out
}
