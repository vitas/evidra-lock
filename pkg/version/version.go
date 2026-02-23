package version

var (
	// Version is the build/runtime version string for Evidra binaries.
	Version = "0.1.0-dev"
	// Commit describes the revision or commit hash used to build the binary.
	Commit = "dev"
	// Date stores the build timestamp.
	Date = "dev"
)

// String returns the version string.
func String() string {
	return Version
}
