package ipc

import (
	"bufio"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kushanparikh/personalizationSetup_ClaudeCode/sidecar/internal/paths"
)

func shortDir(t *testing.T) string {
	t.Helper()
	d, err := os.MkdirTemp("", "pcs")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(d) })
	return d
}

type server struct {
	socket string
	lines  chan string
	stop   chan struct{}
	done   chan error
}

func startServe(t *testing.T, socket string) *server {
	t.Helper()
	pr, pw := io.Pipe()
	s := &server{socket: socket, lines: make(chan string, 16), stop: make(chan struct{}), done: make(chan error, 1)}
	go func() {
		sc := bufio.NewScanner(pr)
		for sc.Scan() {
			s.lines <- sc.Text()
		}
	}()
	go func() {
		err := Serve(socket, pw, s.stop)
		pw.Close()
		s.done <- err
	}()
	t.Cleanup(func() {
		select {
		case <-s.stop:
		default:
			close(s.stop)
		}
		<-s.done
	})
	return s
}

func (s *server) next(t *testing.T) string {
	t.Helper()
	select {
	case l := <-s.lines:
		return l
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for line")
		return ""
	}
}

func (s *server) none(t *testing.T) {
	t.Helper()
	select {
	case l := <-s.lines:
		t.Fatalf("unexpected line %q", l)
	case <-time.After(150 * time.Millisecond):
	}
}

func rawSend(t *testing.T, socket, data string) {
	t.Helper()
	c, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.Write([]byte(data)) // errors tolerated: server may drop early
}

const readyLine = `{"v":1,"kind":"ready","focus":false}`
const panelLine = `{"v":1,"kind":"panel","sessionId":"s1","title":"T","message":"hi","sound":"Ping","focus":true}`

func TestServeForwardsValid(t *testing.T) {
	sock := filepath.Join(shortDir(t), "run", "a.sock")
	s := startServe(t, sock)
	if got := s.next(t); got != readyLine {
		t.Fatalf("ready = %q", got)
	}
	c := &Client{Socket: sock, DialTimeout: time.Second}
	e := Event{V: 1, Kind: KindPanel, SessionID: "s1", Title: "T", Message: "hi", Sound: "Ping", Focus: true}
	if err := c.Send(e, false); err != nil {
		t.Fatal(err)
	}
	if got := s.next(t); got != panelLine {
		t.Fatalf("got %q", got)
	}
}

func TestServeDropsInvalid(t *testing.T) {
	sock := filepath.Join(shortDir(t), "a.sock")
	s := startServe(t, sock)
	s.next(t)
	rawSend(t, sock, `{"v":1,"kind":"x","focus":false}`+"\n")
	rawSend(t, sock, `{"v":1,"kind":"toast","message":"`+strings.Repeat("a", 70*1024)+`","focus":false}`+"\n")
	rawSend(t, sock, "not json\n")
	rawSend(t, sock, `{"v":1,"kind":"ready","focus":false}`+"\n")
	s.none(t)
	rawSend(t, sock, panelLine+"\n")
	if got := s.next(t); got != panelLine {
		t.Fatalf("got %q", got)
	}
}

func TestValidate(t *testing.T) {
	bad := []Event{
		{V: 2, Kind: KindToast},
		{V: 1, Kind: KindReady},
		{V: 1, Kind: KindPanel},
		{V: 1, Kind: KindMovedOn},
		{V: 1, Kind: KindToast, Message: strings.Repeat("a", 1001)},
		{V: 1, Kind: KindToast, Tone: "purple"},
		{V: 1, Kind: KindToast, Cause: "Stop"},
		{V: 1, Kind: KindPanel, SessionID: "s", Cause: "Stop"},
		{V: 1, Kind: KindMovedOn, SessionID: "s", Cause: strings.Repeat("a", 41)},
	}
	for _, e := range bad {
		if e.Validate() == nil {
			t.Errorf("expected error for %+v", e)
		}
	}
	good := []Event{
		{V: 1, Kind: KindToast},
		{V: 1, Kind: KindPanel, SessionID: "s"},
		{V: 1, Kind: KindMovedOn, SessionID: "s"},
		{V: 1, Kind: KindMovedOn, SessionID: "s", Cause: "Stop"},
		{V: 1, Kind: KindMovedOn, SessionID: "s", Cause: strings.Repeat("é", 40)},
		{V: 1, Kind: KindToast, Message: strings.Repeat("é", 1000)},
		{V: 1, Kind: KindToast, Tone: ToneWait},
		{V: 1, Kind: KindToast, Tone: ToneDone},
		{V: 1, Kind: KindToast, Tone: ToneDanger},
	}
	for _, e := range good {
		if err := e.Validate(); err != nil {
			t.Errorf("unexpected error for %+v: %v", e, err)
		}
	}
}

func TestSocketMode0600(t *testing.T) {
	dir := filepath.Join(shortDir(t), "run")
	sock := filepath.Join(dir, "a.sock")
	s := startServe(t, sock)
	s.next(t)
	fi, err := os.Stat(sock)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("socket mode %v", fi.Mode().Perm())
	}
	di, _ := os.Stat(dir)
	if di.Mode().Perm() != 0o700 {
		t.Fatalf("dir mode %v", di.Mode().Perm())
	}
}

func TestStaleSocketReplaced(t *testing.T) {
	sock := filepath.Join(shortDir(t), "a.sock")
	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	l.(*net.UnixListener).SetUnlinkOnClose(false)
	l.Close() // leaves a stale file
	if _, err := os.Stat(sock); err != nil {
		t.Fatal("stale file missing")
	}
	s := startServe(t, sock)
	if got := s.next(t); got != readyLine {
		t.Fatalf("got %q", got)
	}
}

func TestSecondServeAlreadyRunning(t *testing.T) {
	sock := filepath.Join(shortDir(t), "a.sock")
	s := startServe(t, sock)
	s.next(t)
	err := Serve(sock, io.Discard, make(chan struct{}))
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("err = %v", err)
	}
}

func TestServeStopRemovesSocket(t *testing.T) {
	sock := filepath.Join(shortDir(t), "a.sock")
	s := startServe(t, sock)
	s.next(t)
	close(s.stop)
	if err := <-s.done; err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sock); !os.IsNotExist(err) {
		t.Fatalf("socket still there: %v", err)
	}
	s.done <- nil // satisfy cleanup
}

func TestSendLaunchesOnceThenDelivers(t *testing.T) {
	sock := filepath.Join(shortDir(t), "a.sock")
	var calls atomic.Int32
	var s *server
	started := make(chan struct{})
	c := &Client{
		Socket:      sock,
		DialTimeout: 100 * time.Millisecond,
		LaunchWait:  3 * time.Second,
		Launch: func() error {
			calls.Add(1)
			go func() {
				time.Sleep(250 * time.Millisecond)
				s = startServe(t, sock)
				close(started)
			}()
			return nil
		},
	}
	e := Event{V: 1, Kind: KindToast, Title: "T", Message: "Finished working"}
	if err := c.Send(e, true); err != nil {
		t.Fatal(err)
	}
	<-started
	s.next(t) // ready
	want := `{"v":1,"kind":"toast","title":"T","message":"Finished working","focus":false}`
	if got := s.next(t); got != want {
		t.Fatalf("got %q", got)
	}
	if calls.Load() != 1 {
		t.Fatalf("Launch called %d times", calls.Load())
	}
}

func TestSendNoLaunchFailsFast(t *testing.T) {
	sock := filepath.Join(shortDir(t), "a.sock")
	var calls atomic.Int32
	c := &Client{Socket: sock, DialTimeout: 200 * time.Millisecond, LaunchWait: 3 * time.Second,
		Launch: func() error { calls.Add(1); return nil }}
	start := time.Now()
	err := c.Send(Event{V: 1, Kind: KindToast}, false)
	if err == nil {
		t.Fatal("expected error")
	}
	if d := time.Since(start); d > 300*time.Millisecond {
		t.Fatalf("took %v", d)
	}
	if calls.Load() != 0 {
		t.Fatal("Launch called")
	}
}

func TestSendLaunchErrorWrappedWhenStillDown(t *testing.T) {
	sock := filepath.Join(shortDir(t), "a.sock")
	boom := errors.New("boom")
	c := &Client{Socket: sock, DialTimeout: 50 * time.Millisecond, LaunchWait: 300 * time.Millisecond,
		Launch: func() error { return boom }}
	err := c.Send(Event{V: 1, Kind: KindToast}, true)
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
}

type tempErr struct{}

func (tempErr) Error() string   { return "temp" }
func (tempErr) Temporary() bool { return true }

type fakeLn struct {
	net.Listener
	errs []error
}

func (f *fakeLn) Accept() (net.Conn, error) {
	e := f.errs[0]
	f.errs = f.errs[1:]
	return nil, e
}

func TestAcceptLoopRetriesTemporaryThenSurfacesPermanent(t *testing.T) {
	perm := errors.New("listener broke")
	ln := &fakeLn{errs: []error{tempErr{}, tempErr{}, perm}}
	err := acceptLoop(ln, make(chan struct{}), func(net.Conn) {})
	if !errors.Is(err, perm) {
		t.Fatalf("err = %v", err)
	}
}

func TestAcceptLoopStopIsNil(t *testing.T) {
	stop := make(chan struct{})
	close(stop)
	ln := &fakeLn{errs: []error{errors.New("closed")}}
	if err := acceptLoop(ln, stop, func(net.Conn) {}); err != nil {
		t.Fatal(err)
	}
}

func TestSendLaunchBounded(t *testing.T) {
	sock := filepath.Join(shortDir(t), "a.sock")
	var wg sync.WaitGroup
	wg.Add(1)
	c := &Client{Socket: sock, DialTimeout: 100 * time.Millisecond, LaunchWait: 300 * time.Millisecond,
		Launch: func() error { defer wg.Done(); time.Sleep(5 * time.Second); return nil }}
	start := time.Now()
	if err := c.Send(Event{V: 1, Kind: KindToast}, true); err == nil {
		t.Fatal("expected error")
	}
	if d := time.Since(start); d > 1200*time.Millisecond {
		t.Fatalf("took %v", d)
	}
}

// serveErr runs Serve and returns its error, failing if it doesn't return
// within a second.
func serveErr(t *testing.T, socket string) error {
	t.Helper()
	stop := make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- Serve(socket, io.Discard, stop) }()
	select {
	case err := <-done:
		return err
	case <-time.After(time.Second):
		close(stop)
		<-done
		t.Fatal("Serve started instead of refusing")
		return nil
	}
}

func TestServeTightensExistingRunDir(t *testing.T) {
	dir := filepath.Join(shortDir(t), "run")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	s := startServe(t, filepath.Join(dir, "a.sock"))
	if got := s.next(t); got != readyLine {
		t.Fatalf("got %q", got)
	}
	di, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if di.Mode().Perm() != 0o700 {
		t.Fatalf("dir mode %v, want 0700", di.Mode().Perm())
	}
}

func TestServeRefusesNonSocketAtPath(t *testing.T) {
	sock := filepath.Join(shortDir(t), "a.sock")
	if err := os.WriteFile(sock, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := serveErr(t, sock); err == nil {
		t.Fatal("Serve replaced a regular file")
	}
	b, err := os.ReadFile(sock)
	if err != nil || string(b) != "keep" {
		t.Fatalf("file changed: %q %v", b, err)
	}
}

func TestServeRefusesSymlinkedRunDir(t *testing.T) {
	base := shortDir(t)
	real := filepath.Join(base, "real")
	if err := os.Mkdir(real, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "run")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if err := serveErr(t, filepath.Join(link, "a.sock")); err == nil {
		t.Fatal("Serve accepted a symlinked run dir")
	}
}

func TestDefaultLaunchArgs(t *testing.T) {
	want := []string{"-g", "-j", "-a", paths.AppPath(), "--args", "--background"}
	if got := launchArgs(); !reflect.DeepEqual(got, want) {
		t.Errorf("launchArgs() = %v, want %v", got, want)
	}
}
