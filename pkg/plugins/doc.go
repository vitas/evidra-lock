// Package plugins defines compile-time tool plugin contracts.
//
// Level 2 plugins provide tool definitions to the registry and are compiled
// into the binary. Plugins are explicitly registered in cmd wiring and are
// never auto-registered via init().
package plugins
