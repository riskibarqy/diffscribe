package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var ticketPattern = regexp.MustCompile(`[A-Za-z]+-\d+`)

type options struct {
	endpoint, model, key, hook       string
	commit, dryRun, noVerify, review bool
	maxBytes                         int
	timeout                          time.Duration
}

func main() {
	if err := run(context.Background(), os.Args[1:], ".", os.Getenv, os.Stdout, os.Stderr); err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(os.Stderr, "cmg:", err)
			os.Exit(1)
		}
	}
}

func run(ctx context.Context, args []string, dir string, getenv func(string) string, out, errOut io.Writer) error {
	fs := flag.NewFlagSet("cmg", flag.ContinueOnError)
	fs.SetOutput(errOut)
	endpoint := fs.String("endpoint", "", "OpenAI-compatible base URL (or COMMITGEN_ENDPOINT)")
	model := fs.String("model", "", "AI model (or COMMITGEN_MODEL)")
	commit := fs.Bool("commit", true, "Create a commit (set to false to preview)")
	dryRun := fs.Bool("dry-run", false, "Generate a message without committing")
	noVerify := fs.Bool("no-verify", false, "Skip Git pre-commit and commit-msg hooks")
	review := fs.Bool("review", false, "Request an AI review before generation")
	hook := fs.String("hook", "", "Write the message to this file without committing")
	maxBytes := fs.Int("max-bytes", 32000, "Maximum staged diff bytes sent to AI")
	timeout := fs.Duration("timeout", 30*time.Second, "Total command timeout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}
	if *endpoint == "" {
		*endpoint = getenv("COMMITGEN_ENDPOINT")
	}
	if *model == "" {
		*model = getenv("COMMITGEN_MODEL")
	}
	cfg := options{strings.TrimSpace(*endpoint), strings.TrimSpace(*model), getenv("COMMITGEN_API_KEY"), *hook, *commit, *dryRun, *noVerify, *review, *maxBytes, *timeout}
	if cfg.endpoint == "" || cfg.model == "" {
		return errors.New("set COMMITGEN_ENDPOINT and COMMITGEN_MODEL (or use --endpoint and --model)")
	}
	u, err := url.Parse(cfg.endpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("endpoint must be an http(s) base URL without credentials, query, or fragment")
	}
	if cfg.maxBytes <= 0 || cfg.timeout <= 0 {
		return errors.New("--max-bytes and --timeout must be positive")
	}
	ctx, cancel := context.WithTimeout(ctx, cfg.timeout)
	defer cancel()

	diff, err := git(ctx, dir, "diff", "--staged", "-U0", "-M")
	if err != nil {
		return err
	}
	if strings.TrimSpace(diff) == "" {
		return errors.New("no staged changes detected")
	}
	if len(diff) > cfg.maxBytes {
		diff = diff[:cfg.maxBytes]
		for !utf8.ValidString(diff) {
			diff = diff[:len(diff)-1]
		}
		if i := strings.LastIndexByte(diff, '\n'); i > 0 {
			diff = diff[:i]
		}
	}
	branch, err := git(ctx, dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return err
	}
	branch = strings.TrimSpace(branch)
	if branch == "HEAD" {
		branch, err = git(ctx, dir, "rev-parse", "--short", "HEAD")
		if err != nil {
			return err
		}
		branch = strings.TrimSpace(branch)
	}

	if cfg.review {
		result, err := generate(ctx, cfg, "Review this staged diff for correctness, security, and missing tests. List issues briefly, or say 'No blocking issues found'.\n\n"+diff)
		if err != nil {
			fmt.Fprintln(errOut, "review failed:", err)
		} else if strings.TrimSpace(result) != "" {
			fmt.Fprintln(out, "Review findings:\n"+strings.TrimSpace(result)+"\n")
		}
	}
	result, err := generate(ctx, cfg, "Write a Git commit message for this staged diff. Return only JSON with string fields description, summary, body. Description: concise imperative summary, maximum 72 characters, no ticket prefix. Body: brief rationale.\nBranch: "+branch+"\nDiff:\n"+diff)
	if err != nil {
		return fmt.Errorf("generate message: %w", err)
	}
	start, end := strings.IndexByte(result, '{'), strings.LastIndexByte(result, '}')
	if start < 0 || end < start {
		return errors.New("AI response missing JSON commit message")
	}
	var message struct{ Description, Summary, Body string }
	if err := json.Unmarshal([]byte(result[start:end+1]), &message); err != nil {
		return errors.New("AI response contains invalid JSON commit message")
	}
	headline := strings.Join(strings.Fields(message.Description), " ")
	if headline == "" || utf8.RuneCountInString(headline) > 72 || strings.ContainsAny(headline, "\x00\r\n") {
		return errors.New("AI response contains invalid commit description")
	}
	headline = strings.TrimRight(headline, ".")
	if headline == "" {
		return errors.New("AI response contains invalid commit description")
	}
	if i := strings.LastIndex(branch, "/"); i >= 0 {
		branch = branch[i+1:]
	}
	if ticket := ticketPattern.FindString(branch); ticket != "" {
		headline = ticket + " " + headline
	}
	body := strings.TrimSpace(message.Body)
	if body == "" {
		body = strings.TrimSpace(message.Summary)
	}
	combined := headline
	if body != "" {
		combined += "\n\n" + body
	}
	if cfg.hook != "" {
		if err := os.WriteFile(cfg.hook, []byte(combined+"\n"), 0600); err != nil {
			return fmt.Errorf("write hook message: %w", err)
		}
	} else if cfg.commit && !cfg.dryRun {
		gitArgs := []string{"commit"}
		if cfg.noVerify {
			gitArgs = append(gitArgs, "--no-verify")
		}
		gitArgs = append(gitArgs, "-m", headline)
		if body != "" {
			gitArgs = append(gitArgs, "-m", body)
		}
		if _, err := git(ctx, dir, gitArgs...); err != nil {
			return fmt.Errorf("commit failed: %w", err)
		}
	}
	fmt.Fprintln(out, combined)
	return nil
}

func git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func generate(ctx context.Context, cfg options, prompt string) (string, error) {
	payload, err := json.Marshal(map[string]any{
		"model": cfg.model, "messages": []map[string]string{{"role": "user", "content": prompt}}, "stream": false,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(cfg.endpoint, "/")+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", errors.New("invalid AI endpoint")
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.key != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.key)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("AI request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("AI request failed: HTTP %d", resp.StatusCode)
	}
	var completion struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&completion); err != nil || len(completion.Choices) == 0 || strings.TrimSpace(completion.Choices[0].Message.Content) == "" {
		return "", errors.New("AI returned no usable response")
	}
	return completion.Choices[0].Message.Content, nil
}
