package lanshare

import "time"

// ShareItemKind marks the type of a shared item.
type ShareItemKind string

const (
	ShareItemKindFile ShareItemKind = "file"
	ShareItemKindText ShareItemKind = "text"
)

// ShareItem is one entry of the share list. It is delivered both to the
// desktop UI and to the browser receiver page, therefore it must never
// contain local paths: file downloads go through opaque IDs only.
type ShareItem struct {
	ID       string        `json:"id"`
	Kind     ShareItemKind `json:"kind"`
	Name     string        `json:"name"`
	Size     int64         `json:"size"`
	IsImage  bool          `json:"isImage,omitempty"`
	MIMEType string        `json:"mimeType,omitempty"`
	Text     string        `json:"text,omitempty"`
	AddedAt  time.Time     `json:"addedAt"`
}

// TransferDirection marks whether a transfer is a browser pulling a shared
// file (download) or a browser pushing a file to Zashiki (upload).
type TransferDirection string

const (
	TransferDirectionDownload TransferDirection = "download"
	TransferDirectionUpload   TransferDirection = "upload"
)

// TransferProgressPhase is the lifecycle phase of a single transfer.
type TransferProgressPhase string

const (
	TransferPhaseRun   TransferProgressPhase = "run"
	TransferPhaseDone  TransferProgressPhase = "done"
	TransferPhaseError TransferProgressPhase = "error"
)

// TransferProgress is emitted to the desktop UI while a browser is
// downloading shared files or uploading files to Zashiki.
type TransferProgress struct {
	TransferID string                `json:"transferId"`
	Direction  TransferDirection     `json:"direction"`
	RemoteAddr string                `json:"remoteAddr"`
	Name       string                `json:"name"`
	TotalBytes int64                 `json:"totalBytes"` // -1 when unknown
	DoneBytes  int64                 `json:"doneBytes"`
	Phase      TransferProgressPhase `json:"phase"`
	Error      string                `json:"error,omitempty"`
}

// TextReceived is emitted when a browser sends a text message to Zashiki.
type TextReceived struct {
	ID         string    `json:"id"`
	Text       string    `json:"text"`
	RemoteAddr string    `json:"remoteAddr"`
	At         time.Time `json:"at"`
}

// ItemsChanged carries the full share list after any change.
type ItemsChanged struct {
	Items []ShareItem `json:"items"`
}

// ServerStatus describes the share server for the desktop UI.
type ServerStatus struct {
	Running   bool     `json:"running"`
	Port      int      `json:"port"`
	Token     string   `json:"token,omitempty"`
	URLs      []string `json:"urls"`
	ItemCount int      `json:"itemCount"`
}
