package globals

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

type HTTPServer struct {
	Host              string        `env:"HTTP_HOST" help:"Host to bind the server to" default:"0.0.0.0"`
	Port              int           `env:"HTTP_PORT" help:"Port to bind the server to" default:"8080"`
	ReadTimeout       time.Duration `env:"HTTP_READ_TIMEOUT" help:"Max duration for reading the entire request" default:"30s"`
	ReadHeaderTimeout time.Duration `env:"HTTP_READHEADER_TIMEOUT" help:"Max duration for reading request headers" default:"5s"`
	WriteTimeout      time.Duration `env:"HTTP_WRITE_TIMEOUT" help:"Max duration for writing the response" default:"30s"`
	IdleTimeout       time.Duration `env:"HTTP_IDLE_TIMEOUT" help:"Max duration to wait for the next request when keep-alives are enabled" default:"120s"`
	MaxHeaderBytes    int           `env:"HTTP_MAX_HEADER_BYTES" help:"Max number of bytes to read parsing request headers" default:"1048576"`
	GracefulPeriod    time.Duration `env:"HTTP_GRACEFUL_PERIOD" help:"Period to wait for graceful shutdown" default:"5s"`
}

func (srv *HTTPServer) Server(h http.Handler) *http.Server {
	return &http.Server{
		Addr:              srv.Addr(),
		Handler:           h,
		ReadTimeout:       srv.ReadTimeout,
		ReadHeaderTimeout: srv.ReadHeaderTimeout,
		WriteTimeout:      srv.WriteTimeout,
		IdleTimeout:       srv.IdleTimeout,
		MaxHeaderBytes:    srv.MaxHeaderBytes,
	}
}

func (srv *HTTPServer) Addr() string {
	return fmt.Sprintf("%s:%d", srv.Host, srv.Port)
}

func (srv *HTTPServer) Start(ctx context.Context, l *zap.Logger, router http.Handler) func() error {
	return func() error {
		server := srv.Server(router)

		l.Info("HTTP server listening", zap.String("address", srv.Addr()))
		if listenErr := server.ListenAndServe(); listenErr != nil && !errors.Is(listenErr, http.ErrServerClosed) {
			return listenErr
		}

		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), srv.GracefulPeriod)
		defer cancel()

		if shutdownErr := server.Shutdown(shutdownCtx); shutdownErr != nil {
			l.Error("failed to shutdown HTTP server", zap.Error(shutdownErr))
			return shutdownErr
		}

		return nil
	}
}
