package dbusconsts

// Shared D-Bus identifiers so server and client stay aligned.
const (
	// Hyphens are not allowed in well-known bus names, so we normalize the Debian
	// package name org.linglong-store.LinyapsManager to a D-Bus-safe variant.
	BusName    = "org.linglong_store.LinyapsManager"
	ObjectPath = "/org/linglong_store/LinyapsManager"
	Interface  = "org.linglong_store.LinyapsManager"

	// Signal names for streaming output
	SignalOutput   = "Output"   // Emitted for each chunk of output (operationID, data string, isStderr bool)
	SignalComplete = "Complete" // Emitted when operation completes (operationID, exitCode int, errorMsg string)
)
