package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/devantler-tech/data-product-controller/internal/demoproduct"
	"github.com/devantler-tech/data-product-controller/internal/httpserver"
)

func main() {
	if err := run(); err != nil {
		slog.Error("run demo data product", "error", err)
		os.Exit(1)
	}
}

// run validates publication settings before starting the sample API and independent release browser.
func run() error {
	address := flag.String("listen-address", ":8080", "Address for the example data product.")
	flag.Parse()
	handler, err := demoproduct.NewHandlerWithOptions(demoproduct.HandlerOptions{
		PublicBaseURL: os.Getenv("PUBLIC_BASE_URL"),
	})
	if err != nil {
		return err
	}
	server := &http.Server{
		Addr:              *address,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	stopContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	slog.Info("starting demo data product", "address", *address)
	return httpserver.Run(stopContext, server, server.ListenAndServe, 10*time.Second)
}
