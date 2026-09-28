package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func repo(t *testing.T, branch string) string {
	t.Helper()
	dir := t.TempDir()
	command := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	command("init", "-q", "-b", "main")
	command("config", "user.name", "Test")
	command("config", "user.email", "test@example.invalid")
	command("config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("before\n"), 0600); err != nil {
		t.Fatal(err)
	}
	command("add", "file.txt")
	command("commit", "-q", "-m", "initial")
	if branch != "main" {
		command("checkout", "-q", "-b", branch)
	}
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("after\n"), 0600); err != nil {
		t.Fatal(err)
	}
	command("add", "file.txt")
	return dir
}

func ai(t *testing.T, content string, status int) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected AI path: %s", r.URL.Path)
		}
		w.WriteHeader(status)
		if status == http.StatusOK {
			_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": content}}}})
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func invoke(dir, endpoint string, args ...string) (string, error) {
	var out bytes.Buffer
	env := func(key string) string {
		switch key {
		case "COMMITGEN_ENDPOINT":
			return endpoint + "/v1"
		case "COMMITGEN_MODEL":
			return "test-model"
		default:
			return ""
		}
	}
	err := run(context.Background(), args, dir, env, &out, &out)
	return out.String(), err
}

func subject(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "log", "-1", "--format=%s")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

func TestCommitTicketAndMain(t *testing.T) {
	server := ai(t, `{"description":"update login","body":"Explain change."}`, http.StatusOK)
	for _, tc := range []struct{ branch, expected string }{
		{"feature/ABC-123-login", "ABC-123 update login"},
		{"main", "update login"},
	} {
		t.Run(tc.branch, func(t *testing.T) {
			dir := repo(t, tc.branch)
			out, err := invoke(dir, server.URL)
			if err != nil || !strings.Contains(out, tc.expected) || subject(t, dir) != tc.expected {
				t.Fatalf("out=%q err=%v subject=%q", out, err, subject(t, dir))
			}
		})
	}
}

func TestPreviewAndHook(t *testing.T) {
	server := ai(t, `{"description":"change file"}`, http.StatusOK)
	dir := repo(t, "main")
	for _, args := range [][]string{{"--commit=false"}, {"--dry-run"}} {
		if out, err := invoke(dir, server.URL, args...); err != nil || !strings.Contains(out, "change file") || subject(t, dir) != "initial" {
			t.Fatalf("preview: out=%q err=%v", out, err)
		}
	}
	path := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
	if _, err := invoke(dir, server.URL, "--hook", path); err != nil {
		t.Fatal(err)
	}
	if text, err := os.ReadFile(path); err != nil || string(text) != "change file\n" || subject(t, dir) != "initial" {
		t.Fatalf("hook: text=%q err=%v", text, err)
	}
}

func TestHooksDefaultAndOptOut(t *testing.T) {
	server := ai(t, `{"description":"change file"}`, http.StatusOK)
	dir := repo(t, "main")
	hook := filepath.Join(dir, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\necho blocked >&2\nexit 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := invoke(dir, server.URL); err == nil || !strings.Contains(err.Error(), "blocked") || subject(t, dir) != "initial" {
		t.Fatalf("expected hook rejection, got %v", err)
	}
	if _, err := invoke(dir, server.URL, "--no-verify"); err != nil || subject(t, dir) != "change file" {
		t.Fatalf("explicit bypass: %v", err)
	}
}

func TestHelpDoesNotExposeConfig(t *testing.T) {
	var out bytes.Buffer
	env := func(key string) string { return "private-" + key }
	err := run(context.Background(), []string{"--help"}, ".", env, &out, &out)
	if !errors.Is(err, flag.ErrHelp) || strings.Contains(out.String(), "private-") {
		t.Fatalf("help exposed config: err=%v output=%q", err, out.String())
	}
}

func TestRedirectDoesNotForwardDiff(t *testing.T) {
	requests := 0
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests++ }))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()
	dir := repo(t, "main")
	if _, err := invoke(dir, redirect.URL); err == nil || requests != 0 || subject(t, dir) != "initial" {
		t.Fatalf("redirect followed or commit created: err=%v requests=%d", err, requests)
	}
}

func TestFailuresDoNotCommit(t *testing.T) {
	dir := repo(t, "main")
	server := ai(t, `not json`, http.StatusOK)
	if _, err := invoke(dir, server.URL); err == nil || subject(t, dir) != "initial" {
		t.Fatalf("invalid AI response: %v", err)
	}
	server = ai(t, "", http.StatusUnauthorized)
	if _, err := invoke(dir, server.URL); err == nil || subject(t, dir) != "initial" {
		t.Fatalf("AI failure: %v", err)
	}
	if _, err := invoke(dir, server.URL, "--max-bytes=0"); err == nil {
		t.Fatal("invalid diff cap accepted")
	}
	var out bytes.Buffer
	if err := run(context.Background(), nil, dir, func(string) string { return "" }, &out, &out); err == nil || subject(t, dir) != "initial" {
		t.Fatalf("missing config: %v", err)
	}
	cmd := exec.Command("git", "reset", "-q")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	if _, err := invoke(dir, server.URL); err == nil || !strings.Contains(err.Error(), "no staged changes") {
		t.Fatalf("no staged changes: %v", err)
	}
}
