package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/dcm-project/service-provider-manager/internal/api/server"
	apiserver "github.com/dcm-project/service-provider-manager/internal/api_server"
	"github.com/dcm-project/service-provider-manager/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	listener, err := net.Listen("tcp", cfg.Service.Address)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	// TODO: Replace with real handler implementation
	handler := &stubHandler{}

	srv := apiserver.New(cfg, listener, handler)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	log.Printf("Starting server on %s", listener.Addr().String())
	if err := srv.Run(ctx); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// stubHandler implements server.StrictServerInterface with stub responses
type stubHandler struct{}

func (s *stubHandler) GetHealth(ctx context.Context, request server.GetHealthRequestObject) (server.GetHealthResponseObject, error) {
	return server.GetHealth200JSONResponse{Status: ptr("ok")}, nil
}

func (s *stubHandler) ListProviders(ctx context.Context, request server.ListProvidersRequestObject) (server.ListProvidersResponseObject, error) {
	return notImplemented(), nil
}

func (s *stubHandler) CreateProvider(ctx context.Context, request server.CreateProviderRequestObject) (server.CreateProviderResponseObject, error) {
	return notImplemented(), nil
}

func (s *stubHandler) DeleteProvider(ctx context.Context, request server.DeleteProviderRequestObject) (server.DeleteProviderResponseObject, error) {
	return notImplemented(), nil
}

func (s *stubHandler) GetProvider(ctx context.Context, request server.GetProviderRequestObject) (server.GetProviderResponseObject, error) {
	return notImplemented(), nil
}

func (s *stubHandler) ApplyProvider(ctx context.Context, request server.ApplyProviderRequestObject) (server.ApplyProviderResponseObject, error) {
	return notImplemented(), nil
}

func ptr(s string) *string { return &s }

type notImplementedResponse struct{}

func (notImplementedResponse) VisitListProvidersResponse(w http.ResponseWriter) error {
	w.WriteHeader(http.StatusNotImplemented)
	return nil
}

func (notImplementedResponse) VisitCreateProviderResponse(w http.ResponseWriter) error {
	w.WriteHeader(http.StatusNotImplemented)
	return nil
}

func (notImplementedResponse) VisitDeleteProviderResponse(w http.ResponseWriter) error {
	w.WriteHeader(http.StatusNotImplemented)
	return nil
}

func (notImplementedResponse) VisitGetProviderResponse(w http.ResponseWriter) error {
	w.WriteHeader(http.StatusNotImplemented)
	return nil
}

func (notImplementedResponse) VisitApplyProviderResponse(w http.ResponseWriter) error {
	w.WriteHeader(http.StatusNotImplemented)
	return nil
}

func notImplemented() notImplementedResponse { return notImplementedResponse{} }
