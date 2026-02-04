package apiserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/dcm-project/service-provider-manager/api/v1alpha1"
	"github.com/dcm-project/service-provider-manager/api/v1alpha1/resource_manager"
	"github.com/dcm-project/service-provider-manager/internal/api/server"
	rmserver "github.com/dcm-project/service-provider-manager/internal/api/server/resource_manager"
	"github.com/dcm-project/service-provider-manager/internal/config"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

const gracefulShutdownTimeout = 5 * time.Second

type Server struct {
	cfg             *config.Config
	listener        net.Listener
	providerHandler server.StrictServerInterface
	rmHandler       rmserver.StrictServerInterface
}

func New(cfg *config.Config, listener net.Listener, providerHandler server.StrictServerInterface, rmHandler rmserver.StrictServerInterface) *Server {
	return &Server{
		cfg:             cfg,
		listener:        listener,
		providerHandler: providerHandler,
		rmHandler:       rmHandler,
	}
}

func (s *Server) Run(ctx context.Context) error {
	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)

	// Provider API
	swagger, err := v1alpha1.GetSwagger()
	if err != nil {
		return fmt.Errorf("load OpenAPI spec: %w", err)
	}
	if len(swagger.Servers) == 0 {
		return fmt.Errorf("OpenAPI spec missing servers configuration")
	}
	server.HandlerFromMuxWithBaseURL(server.NewStrictHandler(s.providerHandler, nil), router, swagger.Servers[0].URL)

	// Resource Manager API
	rmSwagger, err := resource_manager.GetSwagger()
	if err != nil {
		return fmt.Errorf("load Resource Manager OpenAPI spec: %w", err)
	}
	if len(rmSwagger.Servers) == 0 {
		return fmt.Errorf("Resource Manager OpenAPI spec missing servers configuration")
	}
	rmserver.HandlerFromMuxWithBaseURL(rmserver.NewStrictHandler(s.rmHandler, nil), router, rmSwagger.Servers[0].URL)

	srv := http.Server{Handler: router}

	go func() {
		<-ctx.Done()
		ctxTimeout, cancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
		defer cancel()
		srv.SetKeepAlivesEnabled(false)
		_ = srv.Shutdown(ctxTimeout)
	}()

	if err := srv.Serve(s.listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
