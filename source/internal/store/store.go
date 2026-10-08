package store

import (
	"fmt"
	"os"
	"path/filepath"

	"opencode-reasoning-extractor/internal/model"
)

// Store provides read access to an opencode data directory.
type Store interface {
	// Root is the directory that contained the database. Used to resolve
	// tool-output file references.
	Root() string
	// Projects returns all projects keyed by project id.
	Projects() (map[string]model.Project, error)
	// Sessions returns all sessions in a stable order.
	Sessions() ([]model.Session, error)
	// Messages returns all messages for a session ordered by time and id.
	Messages(sessionID string) ([]model.Message, error)
	// Parts returns all parts for a session ordered by time and id.
	Parts(sessionID string) ([]model.Part, error)
	Close() error
}

// Open locates and opens the opencode database under the given path. The path
// may be either a data directory (containing opencode.db) or the database file
// itself.
func Open(path string) (Store, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}

	root := path
	dbPath := path
	if info.IsDir() {
		dbPath = filepath.Join(path, "opencode.db")
		if _, err := os.Stat(dbPath); err != nil {
			return nil, fmt.Errorf("no opencode.db found in %s", path)
		}
	} else {
		root = filepath.Dir(path)
	}

	return openSQLite(root, dbPath)
}
