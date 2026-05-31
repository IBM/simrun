package config

// PackType represents the type of pack.
type PackType string

const (
	// PackTypeLocal is a local binary pack referenced by filesystem path.
	PackTypeLocal PackType = "local"
	// PackTypeRemote is a pack downloaded from a GitHub release.
	PackTypeRemote PackType = "remote"
	// PackTypeUpload is a binary pack uploaded via the web UI.
	PackTypeUpload PackType = "upload"
)

// PackConfig is the in-memory shape used by the parser and runner factory
// to look up pack metadata. Source is interpreted based on Type:
//   - local: filesystem path to the pack binary
//   - remote: GitHub repository (e.g., github.com/org/repo)
//   - upload: filesystem path to a UI-uploaded binary
type PackConfig struct {
	Name       string         `yaml:"name"`
	Type       PackType       `yaml:"type"`
	Source     string         `yaml:"source"`
	Version    string         `yaml:"version,omitempty"`
	Parameters map[string]any `yaml:"-" json:"-"`
}

// IsLocal returns true if this is a local binary pack.
func (p PackConfig) IsLocal() bool { return p.Type == PackTypeLocal }

// IsRemote returns true if this is a remote (GitHub) binary pack.
func (p PackConfig) IsRemote() bool { return p.Type == PackTypeRemote }
