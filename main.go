// Command portfolio serves the static portfolio site.
//
// It is deliberately not configurable: there are no flags and no config file.
// The site is a handful of embedded files, so the only things an operator ever
// needs to change are the listen address and port, both read from the
// environment so that a Kubernetes Deployment can set them.
//
//	ADDR   listen address (default ":8080")
//	PORT   port, used when ADDR carries no port (default "8080")
package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// shutdownTimeout is how long in-flight requests get to finish after a
// SIGTERM. Kubernetes sends SIGTERM and then waits gracePeriodSeconds, so this
// should stay comfortably below that.
const shutdownTimeout = 10 * time.Second

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	addr := listenAddr()

	srv := &http.Server{
		Addr:              addr,
		Handler:           newHandler(logger),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errc := make(chan error, 1)
	go func() {
		logger.Info("portfolio listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
	}()

	select {
	case err := <-errc:
		logger.Error("server failed", "err", err)
		os.Exit(1)
	case <-ctx.Done():
		logger.Info("shutdown signal received, draining")
	}

	sctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(sctx); err != nil {
		logger.Error("graceful shutdown failed, closing connections", "err", err)
		_ = srv.Close()
		os.Exit(1)
	}
	logger.Info("stopped cleanly")
}

// listenAddr resolves the bind address from the environment. ADDR wins when set;
// otherwise PORT is applied to a wildcard host so the pod listens on all
// interfaces, which is what a Service needs in order to reach it.
func listenAddr() string {
	if a := strings.TrimSpace(os.Getenv("ADDR")); a != "" {
		if _, _, err := net.SplitHostPort(a); err == nil {
			return a
		}
		return net.JoinHostPort(a, port())
	}
	return ":" + port()
}

func port() string {
	p := strings.TrimSpace(os.Getenv("PORT"))
	if p == "" {
		return "8080"
	}
	if _, err := strconv.Atoi(p); err != nil || p == "0" {
		return "8080"
	}
	return p
}
