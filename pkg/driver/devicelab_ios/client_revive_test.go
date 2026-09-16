package devicelab_ios

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// okServer starts an httptest server on 127.0.0.1 that answers every command
// with a success envelope, and returns it plus its port.
func okServer(t *testing.T, hits *atomic.Int32) (*httptest.Server, int) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits != nil {
			hits.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	port := srv.Listener.Addr().(*net.TCPAddr).Port
	return srv, port
}

// TestReviveRetriesReadOnlyCommand: a read-only command against a dead runner
// relaunches (via the reviver) and re-sends on the new port, succeeding.
func TestReviveRetriesReadOnlyCommand(t *testing.T) {
	var deadHits, liveHits atomic.Int32
	dead, deadPort := okServer(t, &deadHits)
	dead.Close() // runner is gone before the call — connection refused

	live, livePort := okServer(t, &liveHits)
	defer live.Close()

	c := NewClient("127.0.0.1", deadPort)
	var revives atomic.Int32
	c.SetReviver(func(_ context.Context, failedPort int) (int, error) {
		revives.Add(1)
		if failedPort != deadPort {
			t.Errorf("reviver got failedPort %d, want dead port %d", failedPort, deadPort)
		}
		return livePort, nil
	})

	data, err := c.Call(context.Background(), Command{Command: CmdSnapshot})
	if err != nil {
		t.Fatalf("read-only Call after revive should succeed, got %v", err)
	}
	if data == nil {
		// success envelope has no data; that's fine, but Call must not error
		_ = data
	}
	if revives.Load() != 1 {
		t.Errorf("expected exactly 1 revive, got %d", revives.Load())
	}
	if liveHits.Load() != 1 {
		t.Errorf("expected the retry to hit the relaunched runner once, got %d", liveHits.Load())
	}
	if c.Port() != livePort {
		t.Errorf("client should now target the relaunched port %d, got %d", livePort, c.Port())
	}
}

// TestReviveDoesNotReplayAction: an action against a dead runner relaunches
// (so the next command works) but is NOT re-sent, and the original transport
// error surfaces for this step.
func TestReviveDoesNotReplayAction(t *testing.T) {
	var liveHits atomic.Int32
	dead, deadPort := okServer(t, nil)
	dead.Close()

	live, livePort := okServer(t, &liveHits)
	defer live.Close()

	c := NewClient("127.0.0.1", deadPort)
	var revives atomic.Int32
	c.SetReviver(func(_ context.Context, _ int) (int, error) {
		revives.Add(1)
		return livePort, nil
	})

	_, err := c.Call(context.Background(), Command{Command: CmdTap})
	if err == nil {
		t.Fatal("an action against a dead runner must not be replayed; expected an error")
	}
	if revives.Load() != 1 {
		t.Errorf("expected the runner to be relaunched once even for an action, got %d revives", revives.Load())
	}
	if liveHits.Load() != 0 {
		t.Errorf("action must NOT be re-sent to the relaunched runner, but it got %d hits", liveHits.Load())
	}
	if c.Port() != livePort {
		t.Errorf("client should be re-pointed at %d for the next command, got %d", livePort, c.Port())
	}
}

// TestNoReviverReturnsTransportError: without a reviver, a dead runner is just
// an error — the pre-restart behaviour is unchanged.
func TestNoReviverReturnsTransportError(t *testing.T) {
	dead, deadPort := okServer(t, nil)
	dead.Close()
	c := NewClient("127.0.0.1", deadPort)
	if _, err := c.Call(context.Background(), Command{Command: CmdSnapshot}); err == nil {
		t.Fatal("expected a transport error with no reviver installed")
	}
}

// TestReviveFailureSurfacesOriginalError: when the relaunch itself fails, the
// caller gets an error rather than a retry.
func TestReviveFailureSurfacesOriginalError(t *testing.T) {
	dead, deadPort := okServer(t, nil)
	dead.Close()
	c := NewClient("127.0.0.1", deadPort)
	c.SetReviver(func(_ context.Context, _ int) (int, error) {
		return 0, context.DeadlineExceeded
	})
	if _, err := c.Call(context.Background(), Command{Command: CmdSnapshot}); err == nil {
		t.Fatal("expected an error when relaunch fails")
	}
	if c.Port() != deadPort {
		t.Errorf("port should be unchanged when relaunch fails, got %d", c.Port())
	}
}
