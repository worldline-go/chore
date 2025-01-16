package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"

	"github.com/worldline-go/auth/pkg/authecho"
	"github.com/worldline-go/chore/internal/server/middlewares"
	"github.com/worldline-go/chore/internal/utils"
	"github.com/worldline-go/chore/pkg/flow"
	"github.com/worldline-go/chore/pkg/models"
	"github.com/worldline-go/chore/pkg/models/apimodels"
	"github.com/worldline-go/chore/pkg/registry"
)

// @Summary Send run the control; methods depending in control
// @Description Send request with bind id or name
// @Security ApiKeyAuth
// @Tags run
// @Router /send [post]
// @Router /send [get]
// @Param endpoint query string true "set endpoint"
// @Param control query string true "set control"
// @Param payload body string false "send key values" SchemaExample()
// @Accept plain
// @Success 200 {object} interface{} "respond from related server"
// @failure 400 {object} apimodels.Error{}
// @failure 405 {object} apimodels.Error{}
// @failure 409 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func send(c echo.Context) error {
	endpoint, _ := c.Get("endpoint").(string)
	name, _ := c.Get("control").(string)

	control := models.Control{}

	ctx := utils.Context(c)
	query := registry.Reg.DB.WithContext(ctx).Where("name = ?", name)
	result := query.First(&control)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return c.JSON(
			http.StatusNotFound,
			apimodels.Error{
				Error: result.Error.Error(),
			},
		)
	}

	if result.Error != nil {
		return c.JSON(
			http.StatusInternalServerError,
			apimodels.Error{
				Error: result.Error.Error(),
			},
		)
	}

	// file, err := c.FormFile("document")
	// if err != nil {
	// 	return c.Status(http.StatusInternalServerError).JSON(
	// 		apimodels.Error{
	// 			Error: err.Error(),
	// 		},
	// 	)
	// }

	content, err := base64.StdEncoding.DecodeString(control.Content)
	if err != nil {
		return c.JSON(
			http.StatusInternalServerError,
			apimodels.Error{
				Error: result.Error.Error(),
			},
		)
	}

	// update logCtx
	logControl := log.Ctx(ctx).With().
		Str("control", control.Name).
		Str("endpoint", endpoint).
		Str("method", c.Request().Method).
		Logger()
	// replace context.Background() with own context
	ctx = logControl.WithContext(ctx)

	logControl.Info().Msg("new call")

	bodyReader := c.Request().Body
	body, _ := io.ReadAll(bodyReader)

	var bodyCopy []byte
	if len(body) > 0 {
		bodyCopy = body
	}

	nodesReg, err := flow.StartFlow(ctx, registry.Reg.WG, control.Name, endpoint, c.Request().Method, content, registry.Reg, bodyCopy)
	if errors.Is(err, flow.ErrEndpointNotFound) {
		return c.JSON(
			http.StatusNotFound,
			apimodels.Error{
				Error: err.Error(),
			},
		)
	}

	if err != nil {
		return c.JSON(
			http.StatusPreconditionFailed,
			apimodels.Error{
				Error: err.Error(),
			},
		)
	}

	respondChan := nodesReg.GetChan()
	if respondChan == nil {
		return c.String(http.StatusAccepted, http.StatusText(http.StatusAccepted))
	}

	// caller context canceled, process is still running in background
	select {
	case <-c.Request().Context().Done():
		nodesReg.SetChanInactive()

		return c.String(http.StatusRequestTimeout, http.StatusText(http.StatusRequestTimeout))
	case valueChan := <-respondChan:
		for k, v := range valueChan.Header {
			c.Response().Header().Set(k, fmt.Sprint(v))
		}

		if valueChan.IsError {
			return c.JSON(
				http.StatusPreconditionFailed,
				apimodels.Error{
					// prevent to marshal base64
					Error: string(valueChan.Data),
				},
			)
		}

		return c.Blob(valueChan.Status, echo.MIMETextPlainCharsetUTF8, valueChan.Data)
	}
}

// endpointCheck middleware is checking endpoint.
func endpointCheck(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		endpoint := c.QueryParam("endpoint")
		name := c.QueryParam("control")

		if endpoint == "" || name == "" {
			return c.JSON(
				http.StatusBadRequest,
				apimodels.Error{
					Error: "endpoint and control parameters cannot be empty",
				},
			)
		}

		c.Set("endpoint", endpoint)
		c.Set("control", name)

		v := models.Endpoints{}

		query := registry.Reg.DB.WithContext(c.Request().Context()).Model(&models.Control{}).Where("name = ?", name)
		result := query.First(&v)

		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return c.JSON(
				http.StatusNotFound,
				apimodels.Error{
					Error: result.Error.Error(),
				},
			)
		}

		if result.Error != nil {
			return c.JSON(
				http.StatusInternalServerError,
				apimodels.Error{
					Error: result.Error.Error(),
				},
			)
		}

		endpoints := make(map[string]models.ControlEndpoint)
		if err := json.Unmarshal(v.Endpoints, &endpoints); err != nil {
			return c.JSON(
				http.StatusInternalServerError,
				apimodels.Error{
					Error: err.Error(),
				},
			)
		}

		endpointSpec, ok := endpoints[endpoint]
		if !ok {
			return c.JSON(
				http.StatusNotFound,
				apimodels.Error{
					Error: fmt.Sprintf("endpoint %s not found", endpoint),
				},
			)
		}

		// method check
		allowMethod := false

		reqMethod := c.Request().Method
		for _, endpointMethod := range endpointSpec.Methods {
			if endpointMethod == reqMethod {
				allowMethod = true

				break
			}
		}

		if !allowMethod {
			c.Response().Header().Set("Allow", strings.ToUpper(strings.Join(endpointSpec.Methods, ", ")))

			return c.JSON(
				http.StatusMethodNotAllowed,
				apimodels.Error{
					Error: fmt.Sprintf("method %s not allowed", reqMethod),
				},
			)
		}

		// public check
		if !endpointSpec.Public {
			return next(c)
		}

		c.Set(authecho.DisableRoleCheckKey, true)
		c.Set(authecho.DisableScopeCheckKey, true)
		c.Set(authecho.DisableControlCheckKey, true)
		c.Set(middlewares.DisableTokenCheck, true)

		return next(c)
	}
}

func Send(e *echo.Group, authMiddleware echo.MiddlewareFunc) {
	e.Any("/send", send, endpointCheck, authMiddleware, middlewares.UserRole, middlewares.PatToken)
}
