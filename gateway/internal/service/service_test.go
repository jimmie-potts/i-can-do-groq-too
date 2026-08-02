package service

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDefaultConfigIsValid(t *testing.T) {
	t.Parallel()

	if _, err := New(DefaultConfig()); err != nil {
		t.Fatalf("New(DefaultConfig()) returned an error: %v", err)
	}
}

func TestNewRejectsInvalidConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		change func(*Config)
		want   string
	}{
		{
			name: "empty listen address",
			change: func(config *Config) {
				config.ListenAddress = ""
			},
			want: "listen address is required",
		},
		{
			name: "non-positive read header timeout",
			change: func(config *Config) {
				config.ReadHeaderTimeout = 0
			},
			want: "read header timeout must be positive",
		},
		{
			name: "non-positive shutdown timeout",
			change: func(config *Config) {
				config.ShutdownTimeout = 0
			},
			want: "shutdown timeout must be positive",
		},
		{
			name: "unbounded headers",
			change: func(config *Config) {
				config.MaxHeaderBytes = maximumMaxHeaderBytes + 1
			},
			want: "maximum header bytes exceeds",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			config := DefaultConfig()
			test.change(&config)

			_, err := New(config)

			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("New() error = %v, want text %q", err, test.want)
			}
		})
	}
}

func TestHealthHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		method      string
		path        string
		wantStatus  int
		wantBody    string
		wantHeaders map[string]string
	}{
		{
			name:       "healthy",
			method:     http.MethodGet,
			path:       healthPath,
			wantStatus: http.StatusOK,
			wantBody:   healthBody,
			wantHeaders: map[string]string{
				"Cache-Control": "no-store",
				"Content-Type":  "text/plain; charset=utf-8",
			},
		},
		{
			name:       "method rejected",
			method:     http.MethodPost,
			path:       healthPath,
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "path rejected",
			method:     http.MethodGet,
			path:       "/missing",
			wantStatus: http.StatusNotFound,
		},
	}

	handler := newHealthHandler()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequest(test.method, test.path, nil)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if test.wantBody != "" && response.Body.String() != test.wantBody {
				t.Fatalf("body = %q, want %q", response.Body.String(), test.wantBody)
			}
			for name, want := range test.wantHeaders {
				if got := response.Header().Get(name); got != want {
					t.Errorf("header %s = %q, want %q", name, got, want)
				}
			}
		})
	}
}

func TestServiceServesHealthAndStopsOnCancellation(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() returned an error: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	config := DefaultConfig()
	config.ListenAddress = listener.Addr().String()
	application, err := New(config)
	if err != nil {
		t.Fatalf("New() returned an error: %v", err)
	}

	serveCtx, cancelServe := context.WithCancel(context.Background())
	defer cancelServe()
	serveResult := make(chan error, 1)
	go func() {
		serveResult <- application.Serve(serveCtx, listener)
	}()

	requestCtx, cancelRequest := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelRequest()
	request, err := http.NewRequestWithContext(
		requestCtx,
		http.MethodGet,
		"http://"+listener.Addr().String()+healthPath,
		nil,
	)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext() returned an error: %v", err)
	}
	transport := &http.Transport{Proxy: nil}
	t.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{Transport: transport}

	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("health request returned an error: %v", err)
	}
	body, readErr := io.ReadAll(io.LimitReader(response.Body, 32))
	closeErr := response.Body.Close()
	if readErr != nil {
		t.Fatalf("read health response: %v", readErr)
	}
	if closeErr != nil {
		t.Fatalf("close health response: %v", closeErr)
	}
	if response.StatusCode != http.StatusOK || string(body) != healthBody {
		t.Fatalf("health response = (%d, %q), want (%d, %q)", response.StatusCode, body, http.StatusOK, healthBody)
	}
	if got := response.Header.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want %q", got, "no-store")
	}
	if got := response.Header.Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want %q", got, "text/plain; charset=utf-8")
	}

	cancelServe()
	if err := receiveResult(t, serveResult); err != nil {
		t.Fatalf("Serve() returned an error during graceful shutdown: %v", err)
	}
}

func TestServiceServeFailureRunsBoundedCleanup(t *testing.T) {
	t.Parallel()

	serveFailure := errors.New("accept failed")
	fake := newControlledServer(serveFailure)
	shutdownObserved := make(chan bool, 1)
	fake.shutdown = func(ctx context.Context) error {
		_, bounded := ctx.Deadline()
		shutdownObserved <- bounded
		return nil
	}
	fake.close = func() error { return nil }
	fake.release()
	application := &Service{server: fake, shutdownTimeout: time.Second}

	err := application.Serve(context.Background(), inertListener{})

	if !errors.Is(err, serveFailure) {
		t.Fatalf("Serve() error = %v, want original serve failure", err)
	}
	if bounded := <-shutdownObserved; !bounded {
		t.Fatal("Shutdown() after a serve failure did not receive a deadline-bounded context")
	}
	select {
	case <-fake.closed:
		t.Fatal("Serve() force-closed after graceful cleanup succeeded")
	default:
	}
}

func TestServiceShutdownFailureForcesCloseAndJoins(t *testing.T) {
	t.Parallel()

	fake := newControlledServer(http.ErrServerClosed)
	shutdownObserved := make(chan bool, 1)
	fake.shutdown = func(ctx context.Context) error {
		_, bounded := ctx.Deadline()
		shutdownObserved <- bounded
		return context.DeadlineExceeded
	}
	fake.close = func() error {
		fake.release()
		return nil
	}
	application := &Service{server: fake, shutdownTimeout: time.Second}

	serveCtx, cancelServe := context.WithCancel(context.Background())
	defer cancelServe()
	serveResult := make(chan error, 1)
	go func() {
		serveResult <- application.Serve(serveCtx, inertListener{})
	}()
	<-fake.started

	cancelServe()
	err := receiveResult(t, serveResult)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Serve() error = %v, want context deadline exceeded", err)
	}
	if bounded := <-shutdownObserved; !bounded {
		t.Fatal("Shutdown() did not receive a deadline-bounded context")
	}
	select {
	case <-fake.closed:
	default:
		t.Fatal("Serve() returned without force-closing the server")
	}
}

func receiveResult(t *testing.T, result <-chan error) error {
	t.Helper()
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()

	select {
	case err := <-result:
		return err
	case <-timer.C:
		t.Fatal("timed out waiting for lifecycle result")
		return nil
	}
}

type controlledServer struct {
	started  chan struct{}
	closed   chan struct{}
	releaseC chan struct{}
	once     sync.Once
	serveErr error
	shutdown func(context.Context) error
	close    func() error
}

func newControlledServer(serveErr error) *controlledServer {
	return &controlledServer{
		started:  make(chan struct{}),
		closed:   make(chan struct{}),
		releaseC: make(chan struct{}),
		serveErr: serveErr,
	}
}

func (server *controlledServer) Serve(net.Listener) error {
	close(server.started)
	<-server.releaseC
	return server.serveErr
}

func (server *controlledServer) Shutdown(ctx context.Context) error {
	return server.shutdown(ctx)
}

func (server *controlledServer) Close() error {
	close(server.closed)
	return server.close()
}

func (server *controlledServer) release() {
	server.once.Do(func() { close(server.releaseC) })
}

type inertListener struct{}

func (inertListener) Accept() (net.Conn, error) { return nil, errors.New("not implemented") }
func (inertListener) Close() error              { return nil }
func (inertListener) Addr() net.Addr            { return inertAddress("inert") }

type inertAddress string

func (address inertAddress) Network() string { return string(address) }
func (address inertAddress) String() string  { return string(address) }
