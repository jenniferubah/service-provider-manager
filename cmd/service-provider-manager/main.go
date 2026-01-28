package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	apiserver "github.com/dcm-project/service-provider-manager/internal/api_server"
	"github.com/dcm-project/service-provider-manager/internal/config"
	"github.com/dcm-project/service-provider-manager/internal/handlers"
	"github.com/dcm-project/service-provider-manager/internal/service"
	"github.com/dcm-project/service-provider-manager/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database
	db, err := store.InitDB(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Initialize store, service, and handler
	dataStore := store.NewStore(db)
	defer dataStore.Close()

	providerService := service.NewProviderService(dataStore)
	handler := handlers.NewHandler(providerService)

	// Start server
	listener, err := net.Listen("tcp", cfg.Service.Address)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	srv := apiserver.New(cfg, listener, handler)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	log.Printf("Starting server on %s", listener.Addr().String())
	if err := srv.Run(ctx); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
