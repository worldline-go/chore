package api

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/worldline-go/chore/internal/config"
	"github.com/worldline-go/chore/pkg/registry"
)

type information struct {
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Environment string    `json:"environment"`
	BuildDate   string    `json:"buildDate"`
	BuildCommit string    `json:"buildCommit"`
	StartDate   time.Time `json:"startDate"`
	ServerDate  time.Time `json:"serverDate"`
	Providers   []string  `json:"providers"`
}

// @Summary Information
// @Description Get information of the server
// @Tags public
// @Router /info [get]
// @Success 200 {object} information{}
func getInfo(c echo.Context) error {
	providers := make([]string, 0, len(registry.Reg.AuthProviders))
	for i := range registry.Reg.AuthProviders {
		providers = append(providers, i)
	}

	return c.JSON(
		http.StatusOK,
		information{
			Name:        config.AppName,
			Version:     config.AppVersion,
			Environment: config.Application.Env,
			BuildDate:   config.AppBuildDate,
			BuildCommit: config.AppBuildCommit,
			StartDate:   config.StartDate,
			ServerDate:  time.Now(),
			Providers:   providers,
		},
	)
}

// @Summary Ping server
// @Description Check server is active
// @Tags public
// @Router /ping [get]
// @Success 200
func getPing(c echo.Context) error {
	return c.String(http.StatusOK, http.StatusText(http.StatusOK))
}

func Info(e *echo.Group) {
	e.GET("/info", getInfo)
	e.GET("/ping", getPing)
}
