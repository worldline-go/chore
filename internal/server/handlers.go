package server

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog/log"
	"github.com/rytsh/mugo/pkg/fstore"
	"github.com/rytsh/mugo/pkg/templatex"
	"github.com/worldline-go/auth"
	"github.com/worldline-go/auth/pkg/authecho"
	"github.com/worldline-go/logz"
	"github.com/worldline-go/logz/logecho"
	"github.com/worldline-go/tell/metric/metricecho"
	"github.com/ziflex/lecho/v3"
	"gorm.io/gorm"

	jwtgo "github.com/golang-jwt/jwt/v5"
	"github.com/worldline-go/chore/internal/api"
	"github.com/worldline-go/chore/internal/api/run"
	"github.com/worldline-go/chore/internal/config"
	"github.com/worldline-go/chore/internal/server/claims"
	"github.com/worldline-go/chore/internal/server/middlewares"
	"github.com/worldline-go/chore/pkg/registry"
	"github.com/worldline-go/chore/pkg/request"
)

// @description Storage and Send API
// @description First login with user and use authorization as "Bearer JWTTOKEN"
// @BasePath /api/v1
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization
// @securityDefinitions.basic BasicAuth
// @in header
// @name Authorization
func setHandlers(e *echo.Group, authMiddleware echo.MiddlewareFunc) error {
	apiPath := "/api/v1"
	v1 := e.Group(apiPath)

	// set swagger
	if err := routerSwagger(v1, apiPath); err != nil {
		return err
	}

	// set routers
	api.Auth(v1, authMiddleware)
	api.Template(v1, authMiddleware)
	api.User(v1, authMiddleware)
	api.Login(v1)
	api.Token(v1, authMiddleware)
	api.Control(v1, authMiddleware)
	api.Settings(v1, authMiddleware)
	api.Info(v1)
	run.API(v1, authMiddleware)

	// testing
	// apitest.Test(v1Router)

	// set send api
	api.Send(v1, authMiddleware)

	return nil
}

func Set(ctx context.Context, wg *sync.WaitGroup, db *gorm.DB) (*echo.Echo, error) {
	serverJWT, err := auth.NewJWT(
		auth.WithExpFunc(
			func() int64 {
				return time.Now().Add(time.Hour).Unix()
			},
		),
		auth.WithSecretByte([]byte(config.Application.Secret)),
		auth.WithKID(auth.GenerateKeyID([]byte(config.Application.Secret))),
		auth.WithMethod(jwtgo.SigningMethodHS256),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create jwt: %w", err)
	}

	providers := make([]auth.InfProviderCert, 0, len(config.Application.AuthProviders))
	for i := range config.Application.AuthProviders {
		providers = append(providers, &auth.ProviderExtra{InfProvider: config.Application.AuthProviders[i]})
	}

	jwksMulti, err := auth.MultiJWTKeyFunc(providers, auth.WithContext(ctx), auth.WithKeyFunc(serverJWT.Jwks()))
	if err != nil {
		return nil, fmt.Errorf("failed to create jwks: %w", err)
	}

	authMiddleware := authecho.MiddlewareJWT(
		authecho.WithKeyFunc(jwksMulti.Keyfunc),
		authecho.WithClaims(claims.NewClaims),
		authecho.WithSkipper(func(c echo.Context) bool {
			if v, ok := c.Get(middlewares.DisableTokenCheck).(bool); ok && v {
				return true
			}

			return false
		}),
	)

	e := echo.New()

	registry.Init(&registry.Registry{
		DB: db,
		Template: templatex.New(templatex.WithAddFuncsTpl(
			fstore.FuncMapTpl(
				fstore.WithLog(logz.AdapterKV{Log: log.With().Str("component", "template").Logger()}),
				fstore.WithTrust(config.Application.Template.Trust),
			),
		)),
		Server: e,
		JWT: registry.JWT{
			JWT:    serverJWT,
			Parser: auth.JwkKeyFuncParse{KeyFunc: jwksMulti.Keyfunc},
		},
		WG:            wg,
		AuthProviders: config.Application.AuthProviders,
	})

	request.InitGlobalRegistry(ctx).Start(wg)

	e.HideBanner = true

	e.Logger = lecho.From(log.With().Str("component", "server").Logger())

	// middlewares
	e.Use(metricecho.HTTPMetrics(nil))
	e.Use(
		middleware.Recover(),
		middleware.CORS(),
	)

	e.Use(
		middleware.RequestID(),
		middleware.RequestLoggerWithConfig(logecho.RequestLoggerConfig()),
		logecho.ZerologLogger(),
	)

	e.Use(
		middleware.Gzip(),
	)

	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("context", ctx)
			c.Response().Header().Set(echo.HeaderServer, config.AppName+":"+config.AppVersion)

			return next(c)
		}
	})

	if config.Application.BasePath != "" {
		config.Application.BasePath = "/" + strings.Trim(config.Application.BasePath, "/")
		log.Info().Msgf("application BasePath: %s", config.Application.BasePath)
	}

	baseGroup := e.Group(config.Application.BasePath)
	if err := setHandlers(baseGroup, authMiddleware); err != nil {
		return nil, err
	}

	setFileHandler(baseGroup)

	return e, nil
}
