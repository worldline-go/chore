package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/worldline-go/chore/internal/config"
)

var shutdownTimeout = 5 * time.Second

func Start(e *echo.Echo) error {
	// custom host
	hostPort := config.Application.Host + ":" + config.Application.Port
	if err := e.Start(hostPort); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func Stop(e *echo.Echo) error {
	if e == nil {
		return nil
	}

	ctxShutdown, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := e.Shutdown(ctxShutdown); err != nil {
		return fmt.Errorf("failed to shutdown server: %w", err)
	}

	return nil
}
