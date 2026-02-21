package policysource

import pkgpolicysource "samebits.com/evidra-mcp/pkg/policysource"

// TODO(monorepo-split): move core/policysource implementations into standalone core module.

type LocalFileSource = pkgpolicysource.LocalFileSource

var NewLocalFileSource = pkgpolicysource.NewLocalFileSource
