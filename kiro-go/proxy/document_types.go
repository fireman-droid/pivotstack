package proxy

const (
	docMaxChars       = 100_000
	docMaxPerRequest  = 8
	docMaxBytesDecode = 50 << 20
	docZipMaxFileSize = 50 << 20
	docZipMaxEntries  = 1000
)

type KiroDoc struct {
	Filename  string
	MimeType  string
	Text      string
	Truncated bool
	Pages     int
	ErrMsg    string
}

var plainTextExts = map[string]bool{
	"txt": true, "md": true, "markdown": true, "rst": true,
	"csv": true, "tsv": true,
	"json": true, "xml": true, "yaml": true, "yml": true,
	"toml": true, "ini": true, "conf": true, "cfg": true, "env": true,
	"log": true,
	"go":  true, "py": true, "js": true, "ts": true,
	"tsx": true, "jsx": true, "mjs": true, "cjs": true,
	"java": true, "c": true, "cc": true, "cpp": true, "cxx": true,
	"h": true, "hpp": true,
	"rs": true, "rb": true, "php": true,
	"sh": true, "bash": true, "zsh": true, "fish": true, "ps1": true, "bat": true,
	"swift": true, "kt": true, "kts": true, "scala": true, "groovy": true,
	"lua": true, "pl": true, "r": true, "dart": true, "ex": true, "exs": true,
	"html": true, "htm": true, "css": true, "scss": true, "sass": true, "less": true,
	"sql": true, "graphql": true, "gql": true, "proto": true,
	"vue": true, "svelte": true,
}
