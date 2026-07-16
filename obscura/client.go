package obscura

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"nexora-crawl/models"
)

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
func (c *Client) Fetch(ctx context.Context, req models.FetchRequest) ([]byte, error) {
	args := []string{"fetch", req.URL}

	dump := normalizeDump(req.Dump)
	if dump != "" {
		args = append(args, "--dump", dump)
	}
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

func normalizeDump(d string) string {
	switch strings.ToLower(d) {
	case "html", "text", "links", "markdown", "original", "assets", "cookies":
		return strings.ToLower(d)
	case "":
		return "html"
	default:
		return strings.ToLower(d)
	}
}
