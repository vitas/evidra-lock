package policy

import pkgpolicy "samebits.com/evidra-mcp/pkg/policy"

// TODO(monorepo-split): move core/policy engine implementation to its own module.

type Decision = pkgpolicy.Decision
type Engine = pkgpolicy.Engine

var NewOPAEngine = pkgpolicy.NewOPAEngine
