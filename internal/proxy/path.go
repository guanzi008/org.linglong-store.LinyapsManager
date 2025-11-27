package proxy

import (
	"os"
	"path/filepath"
	"strconv"
)

// runtimeBase selects a directory visible to both host and container.
// Preference order:
// 1) /tmp/linglong-runtime-<uid>/linglong if exists or can be created.
// 2) $XDG_RUNTIME_DIR/linglong
// 3) /run/user/<uid>/linglong
func runtimeBase() string {
	uid := os.Getuid()
	candidate := filepath.Join("/tmp", "linglong-runtime-"+strconv.Itoa(uid), "linglong")
	if ensureDir(candidate) == nil {
		return candidate
	}
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		candidate = filepath.Join(xdg, "linglong")
		if ensureDir(candidate) == nil {
			return candidate
		}
	}
	candidate = filepath.Join("/run/user", strconv.Itoa(uid), "linglong")
	_ = ensureDir(candidate)
	return candidate
}

func ensureDir(path string) error {
	return os.MkdirAll(path, 0o700)
}

// EnsureDconfDir makes sure /tmp/linglong-runtime-<uid>/dconf (or nearest root) exists.
// Returns the path and any error encountered.
func EnsureDconfDir() (string, error) {
	base := runtimeBase()
	root := filepath.Dir(base) // e.g., /tmp/linglong-runtime-<uid>
	dconfPath := filepath.Join(root, "dconf")
	if err := ensureDir(dconfPath); err != nil {
		return dconfPath, err
	}
	return dconfPath, nil
}

// RuntimeBase exposes the selected base path.
func RuntimeBase() string {
	return runtimeBase()
}
