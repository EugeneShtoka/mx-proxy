package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	configPath := flag.String("config", "", "path to config file (required)")
	flag.Parse()

	if *configPath == "" {
		log.Fatal("--config is required")
	}

	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	tmpl, err := CompileTemplate(cfg.Processor.SendTemplate)
	if err != nil {
		log.Fatalf("compile send_template: %v", err)
	}

	transport, err := NewTransport(cfg.Processor)
	if err != nil {
		log.Fatalf("init transport: %v", err)
	}

	router := NewRouter(cfg)
	processor := NewProcessor(cfg.Processor, transport, tmpl)
	csServer := &http.Server{
		Addr:    cfg.Listen.CS,
		Handler: newCSHandler(cfg.Upstream.Homeserver, processor, router),
	}
	asServer := &http.Server{
		Addr:    cfg.Listen.AS,
		Handler: newASHandler(cfg, processor, router),
	}

	log.Printf("mx-proxy: CS listening on %s", cfg.Listen.CS)
	log.Printf("mx-proxy: AS listening on %s", cfg.Listen.AS)
	log.Printf("mx-proxy: transport=%s endpoint=%s", cfg.Processor.Transport, cfg.Processor.Endpoint)

	errCh := make(chan error, 2)
	go func() { errCh <- csServer.ListenAndServe() }()
	go func() { errCh <- asServer.ListenAndServe() }()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		log.Fatalf("server error: %v", err)
	case sig := <-sigCh:
		log.Printf("mx-proxy: received %s, shutting down", sig)
	}

	transport.Close()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	csServer.Shutdown(shutdownCtx)
	asServer.Shutdown(shutdownCtx)
}
