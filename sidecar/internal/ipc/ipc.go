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
	KindBanner  = "banner"
	KindPanel   = "panel"
	KindMovedOn = "moved_on"
	KindReady   = "ready" // emitted by Serve only

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
	Focus     bool   `json:"focus"`
}

// Validate checks an event received from a client.
func (e Event) Validate() error {
	if e.V != 1 {
		return fmt.Errorf("unsupported version %d", e.V)
	}
	switch e.Kind {
	case KindBanner:
	case KindPanel, KindMovedOn:
		if e.SessionID == "" {
			return fmt.Errorf("kind %q requires sessionId", e.Kind)
		}
	default:
		return fmt.Errorf("invalid kind %q", e.Kind)
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

// DefaultClient targets the app socket and launches the app with `open`.
func DefaultClient() *Client {
	return &Client{
		Socket: paths.SocketPath(),
		Launch: func() error {
			return exec.Command("open", "-g", "-j", "-a", paths.AppPath()).Run()
		},
		DialTimeout: 200 * time.Millisecond,
		LaunchWait:  3 * time.Second,
	}
}

func (c *Client) dial() (net.Conn, error) {
	return net.DialTimeout("unix", c.Socket, c.DialTimeout)
}

// Send delivers e as one JSON line. If the app is down and launchIfDown is
// set, Launch is called once and the dial is retried until LaunchWait elapses.
func (c *Client) Send(e Event, launchIfDown bool) error {
	conn, err := c.dial()
	if err != nil {
		if !launchIfDown {
			return err
		}
		var launchErr error
		if c.Launch != nil {
			launchErr = c.Launch()
		}
		deadline := time.Now().Add(c.LaunchWait)
		for {
			if conn, err = c.dial(); err == nil {
				break
			}
			if !time.Now().Before(deadline) {
				if launchErr != nil {
					return fmt.Errorf("launch failed: %w; dial failed: %w", launchErr, err)
				}
				return err
			}
			time.Sleep(redialEvery)
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
	if err := os.MkdirAll(filepath.Dir(socket), 0o700); err != nil {
		return err
	}
	if c, err := net.DialTimeout("unix", socket, 200*time.Millisecond); err == nil {
		c.Close()
		return ErrAlreadyRunning
	}
	if err := os.Remove(socket); err != nil && !os.IsNotExist(err) {
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
	go func() {
		<-stop
		ln.Close()
	}()
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-stop:
			default:
				slog.Default().Error("ipc accept failed", "err", err)
			}
			break
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer conn.Close()
			handle(conn, emit)
		}()
	}
	ln.Close()
	wg.Wait()
	os.Remove(socket)
	return nil
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
