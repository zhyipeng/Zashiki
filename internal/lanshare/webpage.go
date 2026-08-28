package lanshare

import _ "embed"

// receiverPageHTML is the browser-facing receiver workbench: a single
// self-contained page (list/preview/download/zip/upload/text) with no build
// step, served under the token path.
//
//go:embed web/index.html
var receiverPageHTML string
