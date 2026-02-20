package version

// Version is the build/runtime version string for Evidra binaries.
// Override at build time with:
// go build -ldflags "-X samebits.com/evidra-mcp/pkg/version.Version=0.1.0"
var Version = "0.1.0-dev"
