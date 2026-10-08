package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"

	"opencode-reasoning-extractor/internal/model"

	_ "modernc.org/sqlite"
)

type sqliteStore struct {
	root string
	db   *sql.DB
}

// openSQLite opens the database in strict read-only mode. It only ever touches
// the small relational tables (session/message/part/project); the multi-gigabyte
// event log is never queried.
func openSQLite(root, dbPath string) (Store, error) {
	abs, err := absoluteURL(dbPath)
	if err != nil {
		return nil, err
	}
	dsn := fmt.Sprintf("file:%s?mode=ro&_pragma=query_only(1)&_pragma=busy_timeout(5000)", abs)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return &sqliteStore{root: root, db: db}, nil
}

func absoluteURL(path string) (string, error) {
	u := &url.URL{Path: path}
	return u.String(), nil
}

func (s *sqliteStore) Root() string { return s.root }

func (s *sqliteStore) Close() error { return s.db.Close() }

func (s *sqliteStore) Projects() (map[string]model.Project, error) {
	rows, err := s.db.Query(`SELECT id, worktree, COALESCE(name,''), COALESCE(vcs,''), time_created, time_updated FROM project`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]model.Project)
	for rows.Next() {
		var p model.Project
		if err := rows.Scan(&p.ID, &p.Worktree, &p.Name, &p.VCS, &p.TimeCreated, &p.TimeUpdated); err != nil {
			return nil, err
		}
		out[p.ID] = p
	}
	return out, rows.Err()
}

func (s *sqliteStore) Sessions() ([]model.Session, error) {
	const q = `SELECT id, project_id, COALESCE(workspace_id,''), COALESCE(parent_id,''), slug,
		directory, COALESCE(path,''), title, version, COALESCE(agent,''), COALESCE(model,''),
		cost, tokens_input, tokens_output, tokens_reasoning, tokens_cache_read, tokens_cache_write,
		COALESCE(summary_additions,0), COALESCE(summary_deletions,0), COALESCE(summary_files,0),
		time_created, time_updated
		FROM session ORDER BY time_created, id`
	rows, err := s.db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.Session
	for rows.Next() {
		var (
			sess    model.Session
			modelJS string
		)
		if err := rows.Scan(
			&sess.ID, &sess.ProjectID, &sess.WorkspaceID, &sess.ParentID, &sess.Slug,
			&sess.Directory, &sess.Path, &sess.Title, &sess.Version, &sess.Agent, &modelJS,
			&sess.Cost, &sess.Tokens.Input, &sess.Tokens.Output, &sess.Tokens.Reasoning,
			&sess.Tokens.CacheRead, &sess.Tokens.CacheWrite,
			&sess.SummaryAdditions, &sess.SummaryDeletions, &sess.SummaryFiles,
			&sess.TimeCreated, &sess.TimeUpdated,
		); err != nil {
			return nil, err
		}
		if modelJS != "" {
			_ = json.Unmarshal([]byte(modelJS), &sess.Model)
		}
		out = append(out, sess)
	}
	return out, rows.Err()
}

func (s *sqliteStore) Messages(sessionID string) ([]model.Message, error) {
	const q = `SELECT id, data, time_created, time_updated FROM message
		WHERE session_id = ? ORDER BY time_created, id`
	rows, err := s.db.Query(q, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.Message
	for rows.Next() {
		var (
			id      string
			data    string
			created int64
			updated int64
		)
		if err := rows.Scan(&id, &data, &created, &updated); err != nil {
			return nil, err
		}
		m, err := model.ParseMessage(id, sessionID, created, updated, []byte(data))
		if err != nil {
			return nil, fmt.Errorf("parse message %s: %w", id, err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *sqliteStore) Parts(sessionID string) ([]model.Part, error) {
	const q = `SELECT id, message_id, data, time_created, time_updated FROM part
		WHERE session_id = ? ORDER BY time_created, id`
	rows, err := s.db.Query(q, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.Part
	for rows.Next() {
		var (
			id        string
			messageID string
			data      string
			created   int64
			updated   int64
		)
		if err := rows.Scan(&id, &messageID, &data, &created, &updated); err != nil {
			return nil, err
		}
		p, err := model.ParsePart(id, messageID, sessionID, created, updated, []byte(data))
		if err != nil {
			return nil, fmt.Errorf("parse part %s: %w", id, err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
