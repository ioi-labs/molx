package obscura

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"nexora-crawl/models"
)

// Fetcher is the minimal surface callers need from an Obscura client.
// ponytail: interface keeps handlers unit-testable without a real binary.
type Fetcher interface {
	Fetch(ctx context.Context, req models.FetchRequest) ([]byte, error)
}

// Client wraps execution of the Obscura CLI binary.
type Client struct {
	binaryPath string
	timeout    time.Duration
}

func NewClient(binaryPath string, timeout time.Duration) *Client {
	return &Client{
		binaryPath: binaryPath,
		timeout:    timeout,
	}
}

// Fetch runs `obscura fetch` and returns the raw stdout.
// ponytail: keep raw []byte API for callers; JSON dump parsing can be added later if needed.
func (c *Client) Fetch(ctx context.Context, req models.FetchRequest) ([]byte, error) {
	args := []string{"fetch", req.URL}

	dump := strings.ToLower(req.Dump)
	if dump == "" {
		dump = "html"
	}
	args = append(args, "--dump", dump)
	if req.Selector != "" {
		args = append(args, "--selector", req.Selector)
	}
	if req.Wait > 0 {
		args = append(args, "--wait", strconv.Itoa(req.Wait))
	}
	if req.Timeout > 0 {
		args = append(args, "--timeout", strconv.Itoa(req.Timeout))
	}
	if req.WaitUntil != "" {
		args = append(args, "--wait-until", req.WaitUntil)
	}
	if req.Eval != "" {
		args = append(args, "--eval", req.Eval)
	}
	if req.Proxy != "" {
		args = append(args, "--proxy", req.Proxy)
	}
	if req.Stealth {
		args = append(args, "--stealth")
	}
	if req.UserAgent != "" {
		args = append(args, "--user-agent", req.UserAgent)
	}
	if req.StorageDir != "" {
		args = append(args, "--storage-dir", req.StorageDir)
	}

	return c.run(ctx, args)
}

func (c *Client) run(ctx context.Context, args []string) ([]byte, error) {
	if err := c.verifyExecutable(); err != nil {
		return nil, err
	}

	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, c.binaryPath, args...)
	cmd.Env = append(cmd.Env, "OBSCURA_ALLOW_PRIVATE_NETWORK=0")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		msg := err.Error()
		if stderr.Len() > 0 {
			msg = fmt.Sprintf("%s: %s", msg, stderr.String())
		}
		return stdout.Bytes(), fmt.Errorf("obscura failed: %s", msg)
	}

	return stdout.Bytes(), nil
}

func (c *Client) verifyExecutable() error {
	info, err := os.Stat(c.binaryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("obscura binary not found at %s", c.binaryPath)
		}
		return fmt.Errorf("obscura binary stat failed: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("obscura path is a directory: %s", c.binaryPath)
	}
	if info.Mode()&0o111 == 0 {
		return fmt.Errorf("obscura binary is not executable (run chmod +x %s)", c.binaryPath)
	}
	return nil
}

