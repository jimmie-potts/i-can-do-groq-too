// Package service owns the FastGate process lifecycle and operational health handler.
package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	healthPath             = "/healthz"
	healthBody             = "ok\n"
	maximumAddressLength   = 255
	maximumMaxHeaderBytes  = 1 << 20
	defaultMaxHeaderBytes  = 16 << 10
	defaultShutdownTimeout = 5 * time.Second
)

// Config contains the bounded startup settings for the operational HTTP server.
type Config struct {
	ListenAddress     string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
	MaxHeaderBytes    int
}

// DefaultConfig returns the reviewed local-development settings for FastGate.
func DefaultConfig() Config {
	return Config{
		ListenAddress:     "127.0.0.1:8080",
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		ShutdownTimeout:   defaultShutdownTimeout,
		MaxHeaderBytes:    defaultMaxHeaderBytes,
	}
}

type managedServer interface {
	Serve(net.Listener) error
	Shutdown(context.Context) error
	Close() error
}

// Service owns one HTTP server from listener admission through shutdown and join.
// A Service is single-use: call Serve at most once.
type Service struct {
	server          managedServer
	shutdownTimeout time.Duration
}

// New validates config and constructs a FastGate service without opening a listener.
func New(config Config) (*Service, error) {
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	server := &http.Server{
		Addr:              config.ListenAddress,
		Handler:           newHealthHandler(),
		ReadHeaderTimeout: config.ReadHeaderTimeout,
		ReadTimeout:       config.ReadTimeout,
		WriteTimeout:      config.WriteTimeout,
		IdleTimeout:       config.IdleTimeout,
		MaxHeaderBytes:    config.MaxHeaderBytes,
	}
	return &Service{server: server, shutdownTimeout: config.ShutdownTimeout}, nil
}

// Serve admits requests on listener until serving fails or ctx is canceled.
// Either terminal event starts bounded cleanup; a failed drain forces closure before Serve returns.
func (service *Service) Serve(ctx context.Context, listener net.Listener) error {
	if ctx == nil {
		return errors.New("serve FastGate: context is required")
	}
	if listener == nil {
		return errors.New("serve FastGate: listener is required")
	}

	serveResult := make(chan error, 1)
	go func() {
		serveResult <- service.server.Serve(listener)
	}()

	var serveErr error
	serveFinished := false
	select {
	case serveErr = <-serveResult:
		serveFinished = true
	case <-ctx.Done():
	}

	shutdownErr := service.stop(ctx)
	if !serveFinished {
		serveErr = <-serveResult
	}
	return errors.Join(normalizeServeError(serveErr), shutdownErr)
}

func (service *Service) stop(ctx context.Context) error {
	shutdownCtx, cancelShutdown := context.WithTimeout(
		context.WithoutCancel(ctx),
		service.shutdownTimeout,
	)
	defer cancelShutdown()

	shutdownErr := service.server.Shutdown(shutdownCtx)
	if shutdownErr == nil {
		return nil
	}

	closeErr := service.server.Close()
	return errors.Join(
		fmt.Errorf("graceful FastGate shutdown: %w", shutdownErr),
		wrapError("force-close FastGate server", closeErr),
	)
}

func validateConfig(config Config) error {
	if config.ListenAddress == "" {
		return errors.New("listen address is required")
	}
	if strings.TrimSpace(config.ListenAddress) != config.ListenAddress {
		return errors.New("listen address must not contain surrounding whitespace")
	}
	if len(config.ListenAddress) > maximumAddressLength {
		return fmt.Errorf("listen address exceeds %d bytes", maximumAddressLength)
	}

	timeouts := []struct {
		name  string
		value time.Duration
	}{
		{name: "read header timeout", value: config.ReadHeaderTimeout},
		{name: "read timeout", value: config.ReadTimeout},
		{name: "write timeout", value: config.WriteTimeout},
		{name: "idle timeout", value: config.IdleTimeout},
		{name: "shutdown timeout", value: config.ShutdownTimeout},
	}
	for _, timeout := range timeouts {
		if timeout.value <= 0 {
			return fmt.Errorf("%s must be positive", timeout.name)
		}
	}

	if config.MaxHeaderBytes <= 0 {
		return errors.New("maximum header bytes must be positive")
	}
	if config.MaxHeaderBytes > maximumMaxHeaderBytes {
		return fmt.Errorf("maximum header bytes exceeds %d", maximumMaxHeaderBytes)
	}
	return nil
}

func newHealthHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+healthPath, func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Cache-Control", "no-store")
		response.Header().Set("Content-Type", "text/plain; charset=utf-8")
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write([]byte(healthBody))
	})
	return mux
}

func normalizeServeError(err error) error {
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return fmt.Errorf("serve FastGate: %w", err)
}

func wrapError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", operation, err)
}
