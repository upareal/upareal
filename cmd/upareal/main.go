// story: e01s01

// Command upareal runs the upareal web app.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // embed zone data so America/Sao_Paulo loads in distroless

	"github.com/upareal/upareal/internal/config"
	"github.com/upareal/upareal/internal/db"
	"github.com/upareal/upareal/web"
)

const (
	listenAddr    = ":8080"
	shutdownGrace = 10 * time.Second
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		if err := healthcheck(healthcheckURL); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ln, err := net.Listen("tcp", listenAddr)
	if err == nil {
		err = run(ctx, os.Getenv, ln, os.Stdout)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "upareal:", err)
		os.Exit(1)
	}
}

// run serves the app on ln until ctx is cancelled, then drains in-flight
// requests (up to shutdownGrace) and closes the database.
func run(ctx context.Context, getenv func(string) string, ln net.Listener, logOut io.Writer) error {
	defer func() { _ = ln.Close() }() // already closed by Shutdown on the normal path
	logger := slog.New(slog.NewJSONHandler(logOut, nil))

	cfg, err := config.Load(getenv)
	if err != nil {
		return err
	}
	conn, err := db.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := conn.Close(); err != nil {
			logger.Error("close database", "err", err)
		}
	}()

	srv := &http.Server{
		Handler:           web.LogRequests(logger, web.NewHandler(conn)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()
	logger.Info("listening", "addr", ln.Addr().String())

	select {
	case err := <-serveErr:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve: %w", err)
	}
	logger.Info("shutdown complete")
	return nil
}
