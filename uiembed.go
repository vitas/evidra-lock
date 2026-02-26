package evidra

import "io/fs"

// UIDistFS is the embedded UI filesystem. It is nil when the binary is built
// without the embed_ui tag (e.g., during go test or non-UI builds).
//
// Build with UI embedded:
//
//	cd ui && npm run build
//	cd .. && go build -tags embed_ui ./cmd/evidra-api
var UIDistFS fs.FS
