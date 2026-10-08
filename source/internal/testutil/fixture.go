// Package testutil provides a synthetic opencode SQLite fixture for tests.
package testutil

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// Fixture returns the filesystem path to a freshly created opencode.db that
// mirrors the real relational schema used by the extractor.
func Fixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "opencode.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer db.Close()

	schema := []string{
		`CREATE TABLE project (id text PRIMARY KEY, worktree text NOT NULL, vcs text, name text,
			time_created integer NOT NULL, time_updated integer NOT NULL)`,
		`CREATE TABLE session (id text PRIMARY KEY, project_id text NOT NULL, workspace_id text,
			parent_id text, slug text NOT NULL, directory text NOT NULL, path text, title text NOT NULL,
			version text NOT NULL, agent text, model text, cost real DEFAULT 0 NOT NULL,
			tokens_input integer DEFAULT 0 NOT NULL, tokens_output integer DEFAULT 0 NOT NULL,
			tokens_reasoning integer DEFAULT 0 NOT NULL, tokens_cache_read integer DEFAULT 0 NOT NULL,
			tokens_cache_write integer DEFAULT 0 NOT NULL, summary_additions integer,
			summary_deletions integer, summary_files integer, time_created integer NOT NULL,
			time_updated integer NOT NULL)`,
		`CREATE TABLE message (id text PRIMARY KEY, session_id text NOT NULL, time_created integer NOT NULL,
			time_updated integer NOT NULL, data text NOT NULL)`,
		`CREATE TABLE part (id text PRIMARY KEY, message_id text NOT NULL, session_id text NOT NULL,
			time_created integer NOT NULL, time_updated integer NOT NULL, data text NOT NULL)`,
	}
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("schema: %v", err)
		}
	}

	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("exec %s: %v", q, err)
		}
	}

	exec(`INSERT INTO project VALUES ('p1','/work/exocomp','git','exocomp',1,1)`)
	exec(`INSERT INTO session (id,project_id,parent_id,slug,directory,title,version,agent,model,cost,
		tokens_input,tokens_output,tokens_reasoning,tokens_cache_read,tokens_cache_write,
		time_created,time_updated)
		VALUES ('ses_root','p1',NULL,'neon-knight','/work/exocomp','Fix CVE-2026-75650 RCE exploit',
		'1.18.29','build','{"id":"deepseek-v4-pro","providerID":"deepseek"}',0.5,100,20,50,0,0,1000,2000)`)
	exec(`INSERT INTO session (id,project_id,parent_id,slug,directory,title,version,agent,model,cost,
		tokens_input,tokens_output,tokens_reasoning,tokens_cache_read,tokens_cache_write,
		time_created,time_updated)
		VALUES ('ses_child','p1','ses_root','calm-wolf','/work/exocomp','explore parser (@explore subagent)',
		'1.18.29','explore','{"id":"deepseek-flash","providerID":"deepseek"}',0.1,30,5,10,0,0,1500,1600)`)

	msg := func(id, sess, role string, created int64, data string) {
		exec(`INSERT INTO message VALUES (?,?,?,?,?)`, id, sess, created, created, data)
	}
	part := func(id, mid, sess string, created int64, data string) {
		exec(`INSERT INTO part VALUES (?,?,?,?,?,?)`, id, mid, sess, created, created, data)
	}

	// Root session: user -> assistant(reasoning+text+tool) -> tool result -> assistant(reasoning+text).
	msg("m1", "ses_root", "user", 1000, `{"role":"user","time":{"created":1000},"agent":"build"}`)
	part("pt1", "m1", "ses_root", 1000, `{"type":"text","text":"Exploit the parser with payload secret=\"supersecretvalue123\""}`)

	msg("m2", "ses_root", "assistant", 1010, `{"parentID":"m1","role":"assistant","agent":"build","modelID":"deepseek-v4-pro","providerID":"deepseek","finish":"tool-calls","tokens":{"input":50,"output":10,"reasoning":5}}`)
	part("pt2", "m2", "ses_root", 1010, `{"type":"step-start"}`)
	part("pt3", "m2", "ses_root", 1011, `{"type":"reasoning","text":"Let me read the parser."}`)
	part("pt4", "m2", "ses_root", 1012, `{"type":"tool","tool":"read","callID":"call_1","state":{"status":"completed","input":{"filePath":"/work/exocomp/parser.go"},"output":"package parser","metadata":{"truncated":false}}}`)
	part("pt5", "m2", "ses_root", 1013, `{"type":"step-finish","reason":"tool-calls"}`)

	msg("m3", "ses_root", "assistant", 1020, `{"parentID":"m1","role":"assistant","agent":"build","modelID":"deepseek-v4-pro","providerID":"deepseek","finish":"stop","tokens":{"input":60,"output":15,"reasoning":8}}`)
	part("pt6", "m3", "ses_root", 1020, `{"type":"step-start"}`)
	part("pt7", "m3", "ses_root", 1021, `{"type":"reasoning","text":"Found the injection point."}`)
	part("pt8", "m3", "ses_root", 1022, `{"type":"text","text":"Patched the buffer overflow in parser.go."}`)
	part("pt9", "m3", "ses_root", 1023, `{"type":"patch","hash":"abc","files":["/work/exocomp/parser.go"]}`)
	part("pt10", "m3", "ses_root", 1024, `{"type":"step-finish","reason":"stop"}`)

	// Child session.
	msg("c1", "ses_child", "user", 1500, `{"role":"user","time":{"created":1500},"agent":"explore"}`)
	part("ct1", "c1", "ses_child", 1500, `{"type":"text","text":"Review the linux networking article for factual errors."}`)
	msg("c2", "ses_child", "assistant", 1510, `{"role":"assistant","agent":"explore","modelID":"deepseek-flash","providerID":"deepseek","finish":"stop"}`)
	part("ct2", "c2", "ses_child", 1510, `{"type":"reasoning","text":"Checking iptables flags."}`)
	part("ct3", "c2", "ses_child", 1511, `{"type":"text","text":"The article is correct."}`)

	if err := db.Close(); err != nil {
		t.Fatalf("close fixture: %v", err)
	}
	return path
}

// FixtureDir returns a directory containing the fixture database.
func FixtureDir(t *testing.T) string {
	t.Helper()
	p := Fixture(t)
	return filepath.Dir(p)
}
