// Package plugins defines compile-time tool plugin contracts.
//
// Status: experimental/future. Level 2 plugins are not the primary extension
// mechanism in v0.1 (Tool Packs are primary) and this API may change.
//
// Level 2 plugins provide tool definitions to the registry and are compiled
// into the binary. Plugins are explicitly registered in cmd wiring and are
// never auto-registered via init().
package plugins
