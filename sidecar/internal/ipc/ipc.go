// Package ipc carries hook events from short-lived hook processes to the
// long-lived `pcs serve` process over a unix socket. The JSON line format of
// Event is the contract the app shell reads.
package ipc

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/paths"
)

const (
	KindToast   = "toast"
	KindPanel   = "panel"
	KindMovedOn = "moved_on"
	KindReady   = "ready" // emitted by Serve only

	// Tone is the look of an alert (lamp colour); Kind is its behaviour.
	// "" means the app picks a default for the kind.
	ToneWait   = "wait"
	ToneDone   = "done"
	ToneDanger = "danger"

	maxLine        = 64 * 1024
	maxMessageRune = 1000
	readDeadline   = 2 * time.Second
	writeDeadline  = 500 * time.Millisecond
	redialEvery    = 100 * time.Millisecond
)

// ErrAlreadyRunning is returned by Serve when a live server owns the socket.
var ErrAlreadyRunning = errors.New("ipc: server already running")

// Event is one line on the wire. Sound "" means silent.
type Event struct {
	V         int    `json:"v"`
	Kind      string `json:"kind"`
	SessionID string `json:"sessionId,omitempty"`
	Title     string `json:"title,omitempty"`
	Message   string `json:"message,omitempty"`
	Sound     string `json:"sound,omitempty"`
	TTY       string `json:"tty,omitempty"`
	BundleID  string `json:"bundleId,omitempty"`
	Tone      string `json:"tone,omitempty"`
	Focus     bool   `json:"focus"`
}

// Validate checks an event received from a client.
func (e Event) Validate() error {
	if e.V != 1 {
		return fmt.Errorf("unsupported version %d", e.V)
	}
	switch e.Kind {
	case KindToast:
	case KindPanel, KindMovedOn:
		if e.SessionID == "" {
			return fmt.Errorf("kind %q requires sessionId", e.Kind)
		}
	default:
		return fmt.Errorf("invalid kind %q", e.Kind)
	}
	switch e.Tone {
	case "", ToneWait, ToneDone, ToneDanger:
	default:
		return fmt.Errorf("invalid tone %q", e.Tone)
	}
	if utf8.RuneCountInString(e.Message) > maxMessageRune {
		return errors.New("message too long")
	}
	return nil
}

// Client sends events to the app.
type Client struct {
	Socket      string
	Launch      func() error
	DialTimeout time.Duration
	LaunchWait  time.Duration
}

// launchArgs is the argv for `open`: start the app hidden and in the background.
func launchArgs() []string {
	return []string{"-g", "-j", "-a", paths.AppPath(), "--args", "--background"}
}

// DefaultClient targets the app socket and launches the app with `open`.
func DefaultClient() *Client {
	return &Client{
		Socket: paths.SocketPath(),
		Launch: func() error {
			cmd := exec.Command("open", launchArgs()...)
			if err := cmd.Start(); err != nil {
				return err
			}
			go cmd.Wait() // reap; do not block the caller
			return nil
		},
		DialTimeout: 200 * time.Millisecond,
		LaunchWait:  3 * time.Second,
	}
}

func (c *Client) dial(timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout("unix", c.Socket, timeout)
}

// Send delivers e as one JSON line. If the app is down and launchIfDown is
// set, Launch is called once (in the background, so a slow Launch cannot extend
// the wait) and the dial is retried until LaunchWait, measured from before
// Launch, elapses.
func (c *Client) Send(e Event, launchIfDown bool) error {
	conn, err := c.dial(c.DialTimeout)
	if err != nil {
		if !launchIfDown {
			return err
		}
		deadline := time.Now().Add(c.LaunchWait)
		launched := make(chan error, 1)
		go func() {
			if c.Launch == nil {
				launched <- nil
				return
			}
			launched <- c.Launch()
		}()
		for {
			if conn, err = c.dial(min(c.DialTimeout, max(time.Until(deadline), time.Millisecond))); err == nil {
				break
			}
			if !time.Now().Before(deadline) {
				select {
				case launchErr := <-launched:
					if launchErr != nil {
						return fmt.Errorf("launch failed: %w; dial failed: %w", launchErr, err)
					}
				default:
				}
				return err
			}
			time.Sleep(min(redialEvery, max(time.Until(deadline), 0)))
		}
	}
	defer conn.Close()
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	conn.SetWriteDeadline(time.Now().Add(writeDeadline))
	_, err = conn.Write(append(b, '\n'))
	return err
}

// Serve listens on socket and forwards each valid event as one JSON line on
// out until stop closes.
func Serve(socket string, out io.Writer, stop <-chan struct{}) error {
	if err := prepareDir(filepath.Dir(socket)); err != nil {
		return err
	}
	if c, err := net.DialTimeout("unix", socket, 200*time.Millisecond); err == nil {
		c.Close()
		return ErrAlreadyRunning
	}
	if err := removeStale(socket); err != nil {
		return err
	}
	ln, err := net.Listen("unix", socket)
	if err != nil {
		return err
	}
	if err := os.Chmod(socket, 0o600); err != nil {
		ln.Close()
		return err
	}

	var mu sync.Mutex
	emit := func(e Event) error {
		b, err := json.Marshal(e)
		if err != nil {
			return err
		}
		mu.Lock()
		defer mu.Unlock()
		_, err = out.Write(append(b, '\n'))
		return err
	}
	if err := emit(Event{V: 1, Kind: KindReady}); err != nil {
		ln.Close()
		os.Remove(socket)
		return err
	}

	var wg sync.WaitGroup
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-stop:
		case <-done:
		}
		ln.Close()
	}()
	err = acceptLoop(ln, stop, func(conn net.Conn) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer conn.Close()
			handle(conn, emit)
		}()
	})
	ln.Close()
	wg.Wait()
	os.Remove(socket)
	return err
}

// prepareDir creates the socket's directory and makes sure it is a real
// directory (not a symlink) owned by this user with mode 0700.
func prepareDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	fi, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !fi.IsDir() || !ownedByMe(fi) {
		return fmt.Errorf("ipc: %s isn't a directory owned by you; not listening there", dir)
	}
	return os.Chmod(dir, 0o700)
}

// removeStale removes a leftover socket at path. Anything else there is
// left alone and reported.
func removeStale(path string) error {
	fi, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if fi.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("ipc: %s exists and isn't a socket; not replacing it", path)
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// acceptLoop accepts until stop closes (returns nil) or a permanent error
// occurs (returned wrapped). Temporary errors are retried with backoff.
func acceptLoop(ln net.Listener, stop <-chan struct{}, spawn func(net.Conn)) error {
	var delay time.Duration
	for {
		conn, err := ln.Accept()
		if err == nil {
			delay = 0
			spawn(conn)
			continue
		}
		select {
		case <-stop:
			return nil
		default:
		}
		var ne interface{ Temporary() bool }
		if errors.As(err, &ne) && ne.Temporary() {
			if delay == 0 {
				delay = 5 * time.Millisecond
			} else if delay *= 2; delay > time.Second {
				delay = time.Second
			}
			slog.Default().Warn("ipc accept failed, retrying", "err", err, "delay", delay)
			select {
			case <-time.After(delay):
			case <-stop:
				return nil
			}
			continue
		}
		return fmt.Errorf("ipc accept: %w", err)
	}
}

func handle(conn net.Conn, emit func(Event) error) {
	conn.SetReadDeadline(time.Now().Add(readDeadline))
	r := bufio.NewReader(io.LimitReader(conn, maxLine+1))
	line, err := r.ReadBytes('\n')
	if len(line) > maxLine || (err != nil && err != io.EOF) {
		slog.Default().Warn("ipc dropped event", "reason", "oversize or unreadable", "err", err)
		return
	}
	var e Event
	if err := json.Unmarshal(line, &e); err != nil {
		slog.Default().Warn("ipc dropped event", "reason", "bad json", "err", err)
		return
	}
	if err := e.Validate(); err != nil {
		slog.Default().Warn("ipc dropped event", "reason", err.Error())
		return
	}
	if err := emit(e); err != nil {
		slog.Default().Error("ipc write failed", "err", err)
	}
}
