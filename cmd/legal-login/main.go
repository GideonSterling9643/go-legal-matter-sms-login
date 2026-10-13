package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	legalotp "github.com/infrai-examples/legal-matter-otp-gateway"
)

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		slog.Error("INFRAI_API_KEY is required")
		os.Exit(2)
	}
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	client := &legalotp.Client{
		APIKey:     key,
		MaxRetries: 3,
		HTTP:       &http.Client{Timeout: 10 * time.Second},
	}
	handler := legalotp.NewHTTPService(legalotp.NewMatterAccess(client), logger)
	server := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	logger.Info("legal login listening", "addr", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
