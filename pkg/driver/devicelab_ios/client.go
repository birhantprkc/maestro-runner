package devicelab_ios

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Client is the HTTP transport to the on-device runner. agent-device's
// runner accepts JSON-bodied POST requests at any path (the transport
// matches on Content-Length and decodes the body as a Command). There is
// no separate /health endpoint — readiness is probed by sending an
// `uptime` command, which is the runner's built-in lightweight ping.
type Client struct {
	baseURL    string
	host       string
	port       int
	httpClient *http.Client

	// reviveMu serialises revive attempts so a burst of failed calls
	// relaunches the runner at most once between successes.
	reviveMu sync.Mutex
	// reviver relaunches a dead runner and returns the port the new one
	// listens on. Set by the supervisor (SetReviver); nil in tests and on
	// the readiness-probe client, where a transport error is just an error.
	// failedPort is the port the failing call used, so the reviver can
	// no-op when another call already relaunched onto a new port.
	reviver func(ctx context.Context, failedPort int) (newPort int, err error)
}

// NewClient builds a Client targeting `host:port`. host is typically
// 127.0.0.1 for simulator and tunneled-device flows.
func NewClient(host string, port int) *Client {
	return &Client{
		baseURL: fmt.Sprintf("http://%s:%d", host, port),
		host:    host,
		port:    port,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        4,
				MaxIdleConnsPerHost: 4,
				IdleConnTimeout:     90 * time.Second,
				DisableCompression:  true,
			},
		},
	}
}

// SetReviver installs the callback that relaunches a dead runner. The
// supervisor wires this after Setup; without it the client behaves exactly
// as before (a transport error is returned to the caller).
func (c *Client) SetReviver(fn func(ctx context.Context, failedPort int) (int, error)) {
	c.reviver = fn
}

// Port reports the port the client is currently targeting (it changes when
// the runner is relaunched onto a fresh port).
func (c *Client) Port() int { return c.port }

// setPort re-points the client at a relaunched runner.
func (c *Client) setPort(port int) {
	c.port = port
	c.baseURL = fmt.Sprintf("http://%s:%d", c.host, port)
}

// Ping probes the runner by sending an `uptime` command. Used by setup
// to wait until the runner is listening and dispatching.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.Call(ctx, Command{Command: CmdUptime})
	return err
}

// CallRaw sends an arbitrary JSON-serializable body (instead of a typed
// Command). Used for the one-off case where a handler needs to emit a
// field that the typed Command would `omitempty` away (e.g. eraseText
// needs `"text": ""` to survive the marshal).
func (c *Client) CallRaw(ctx context.Context, body any) (*ResponseData, error) {
	rawBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal raw command: %w", err)
	}
	// A raw body is opaque here, so we cannot tell whether replaying it is
	// safe; relaunch a dead runner but never re-send the command.
	return c.doRequest(ctx, rawBody, false)
}

// readOnlyCommands are the commands with no side effect on the device, so a
// transport failure can safely re-send them on a relaunched runner. An
// action (tap, type, drag, …) is NOT replayed: a runner that died mid-action
// may have executed it, and — more importantly — the action that KILLED the
// runner would otherwise re-run and re-crash it on a loop. Those still get a
// relaunch (so the next command works), just not a retry.
var readOnlyCommands = map[CommandType]bool{
	CmdSnapshot:         true,
	CmdScreenshot:       true,
	CmdQuerySelector:    true,
	CmdFindText:         true,
	CmdReadText:         true,
	CmdUptime:           true,
	CmdInteractionFrame: true,
	CmdIdleCheck:        true,
	CmdAppearance:       true,
}

// Call sends a command and decodes the response envelope. Errors from the
// runner (`ok: false`) are returned as RunnerError so callers can branch on
// the structured error code.
func (c *Client) Call(ctx context.Context, cmd Command) (*ResponseData, error) {
	body, err := json.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("marshal command: %w", err)
	}
	return c.doRequest(ctx, body, readOnlyCommands[cmd.Command])
}

// doRequest sends one command. On a transport failure (the runner process is
// gone — connection refused, reset, EOF) it asks the reviver to relaunch the
// runner, re-points at the new port, and then re-sends the command once, but
// only when retryOnRevive is set (a read-only command). An action is not
// re-sent; the relaunch alone keeps the next command working.
func (c *Client) doRequest(ctx context.Context, body []byte, retryOnRevive bool) (*ResponseData, error) {
	data, err := c.sendOnce(ctx, body)
	if err == nil || !isTransportError(err) || c.reviver == nil {
		return data, err
	}
	if !c.revive(ctx) {
		return data, err
	}
	if !retryOnRevive {
		// Runner is back for the next command, but this one is not safe to
		// replay. Surface the original failure for this step.
		return data, err
	}
	return c.sendOnce(ctx, body)
}

// revive relaunches the runner at most once per burst of failures. When two
// calls fail against the same dead runner, the first relaunches and the
// second sees the port has already moved and simply re-points.
func (c *Client) revive(ctx context.Context) bool {
	c.reviveMu.Lock()
	defer c.reviveMu.Unlock()
	failedPort := c.port
	newPort, err := c.reviver(ctx, failedPort)
	if err != nil || newPort <= 0 {
		return false
	}
	c.setPort(newPort)
	return true
}

// isTransportError reports whether err came from the HTTP transport (the
// runner was unreachable), as opposed to a decode error or a RunnerError
// (the runner answered). Only transport errors warrant a relaunch.
func isTransportError(err error) bool {
	if err == nil {
		return false
	}
	var re *RunnerError
	if errors.As(err, &re) {
		return false
	}
	var transportErr transportError
	return errors.As(err, &transportErr)
}

type transportError struct{ err error }

func (e transportError) Error() string { return "runner request failed: " + e.err.Error() }
func (e transportError) Unwrap() error { return e.err }

func (c *Client) sendOnce(ctx context.Context, body []byte) (*ResponseData, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/command", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, transportError{err}
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read runner response: %w", err)
	}

	var envelope Response
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("decode runner response: %w (body=%q)", err, string(raw))
	}
	if !envelope.Ok {
		code := ""
		msg := "unknown runner error"
		if envelope.Error != nil {
			code = envelope.Error.Code
			msg = envelope.Error.Message
		}
		return envelope.Data, &RunnerError{Code: code, Message: msg}
	}
	return envelope.Data, nil
}

// RunnerError is the typed error returned when the runner responds with
// `ok: false`. Driver code can check Code against the ErrXxx constants to
// branch on specific failure modes (ELEMENT_NOT_FOUND vs APP_NOT_RUNNING).
type RunnerError struct {
	Code    string
	Message string
}

func (e *RunnerError) Error() string {
	return fmt.Sprintf("runner: %s: %s", e.Code, e.Message)
}

// IsRunnerError unwraps an error and reports its code, or "" if not a
// RunnerError. Convenience for branching on the error code.
func IsRunnerError(err error) (*RunnerError, bool) {
	if err == nil {
		return nil, false
	}
	re, ok := err.(*RunnerError)
	return re, ok
}
