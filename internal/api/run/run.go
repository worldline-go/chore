package run

import (
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/worldline-go/chore/internal/server/middlewares"
	"github.com/worldline-go/chore/pkg/models/apimodels"
	"github.com/worldline-go/chore/pkg/registry"
	"github.com/worldline-go/chore/pkg/script/js"
)

// @Summary Run JS script
// @Tags run
// @Description Run JS script with scripts and input values
// @Security ApiKeyAuth
// @Router /run/js [post]
// @Param payload body runModel true "Script and inputs"
// @Accept plain
// @Success 200 {object} string "result of the script"
// @failure 400 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func postJS(c echo.Context) error {
	body := defaultRunModel()
	if err := c.Bind(&body); err != nil {
		return c.JSON(
			http.StatusBadRequest,
			apimodels.Error{
				Error: err.Error(),
			},
		)
	}

	if body.Settings.Timeout != "" {
		duration, err := time.ParseDuration(body.Settings.Timeout)
		if err != nil {
			return c.JSON(
				http.StatusBadRequest,
				apimodels.Error{
					Error: err.Error(),
				},
			)
		}

		body.Settings.TimeoutDuration = duration
	}

	ctx := c.Request().Context()
	log.Ctx(ctx).Debug().Msgf("script run with API")

	runtime := js.NewGoja()
	parsedInputs := js.ParseInputs(body.Inputs)

	if body.Settings.Async {
		wg := registry.Reg.WG

		wg.Add(1)
		go func() {
			defer wg.Done()

			result, err := runtime.RunScript(ctx, string(body.Script), parsedInputs)
			if err != nil {
				log.Ctx(ctx).Error().Err(err).Msg("error while running script")

				return
			}

			log.Ctx(ctx).Info().Msgf("%s", result)
		}()

		return c.String(http.StatusAccepted, http.StatusText(http.StatusAccepted))
	}

	result, err := runtime.RunScript(ctx, string(body.Script), parsedInputs)
	if err != nil && !errors.Is(err, js.ErrThrow) {
		return c.JSON(
			http.StatusBadRequest,
			apimodels.Error{
				Error: err.Error(),
			},
		)
	}

	// return recorded data's id
	return c.Blob(http.StatusOK, "text/plain", result)
}

// @Summary Render template
// @Tags run
// @Description Render template with input values
// @Security ApiKeyAuth
// @Router /run/template [post]
// @Param payload body string false "send key values" SchemaExample()
// @Accept plain
// @Success 200 {object} apimodels.Data{data=apimodels.ID{}}
// @failure 400 {object} apimodels.Error{}
// @failure 409 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func postTemplate(c echo.Context) error {
	return c.JSON(
		http.StatusNotImplemented,
		apimodels.Error{
			Error: "waiting implementation",
		},
	)
}

func API(e *echo.Group, authMiddleware echo.MiddlewareFunc) {
	e.POST("/run/js", postJS, authMiddleware, middlewares.UserRole, middlewares.PatToken)
	e.POST("/run/template", postTemplate, authMiddleware, middlewares.UserRole, middlewares.PatToken)
}
