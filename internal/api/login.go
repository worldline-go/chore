package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/worldline-go/auth"
	"github.com/worldline-go/auth/request"
	"gorm.io/gorm"

	"github.com/worldline-go/chore/internal/parser"
	"github.com/worldline-go/chore/internal/server/claims"
	"github.com/worldline-go/chore/internal/server/middlewares"
	"github.com/worldline-go/chore/pkg/models"
	"github.com/worldline-go/chore/pkg/models/apimodels"
	"github.com/worldline-go/chore/pkg/registry"
	"github.com/worldline-go/chore/pkg/sec"
)

type LoginModel struct {
	Login    string `json:"login" example:"admin"`
	Password string `json:"password" example:"admin"`
}

type LoginToken struct {
	Token string `json:"token" example:"tokenJWT"`
}

// @Summary Login
// @Description Get JWT token for 1 hour
// @Tags public
// @Router /login [post]
// @Param payload body LoginModel{} false "send login object"
// @Success 200 {object} apimodels.Data{data=LoginToken{}}
// @failure 400 {object} apimodels.Error{}
// @failure 401 {object} apimodels.Error{}
// @failure 409 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func postLogin(c echo.Context) error {
	body := LoginModel{}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: err.Error()})
	}

	if body.Login == "" {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: "required username, email address or user id"})
	}

	providerName := c.QueryParam("provider")
	if providerName != "" {
		return loginAndGetTokenProvider(c, body, providerName, false)
	}

	return loginAndGetToken(c, body, false)
}

// @Summary Login with basic auth
// @Description Get JWT token for 1 hour
// @Tags public
// @Router /login [get]
// @Param raw query bool false "raw token output"
// @Param provider query string false "oauth2 provider name"
// @Security BasicAuth
// @Success 200 {object} apimodels.Data{data=LoginToken{}}
// @failure 400 {object} apimodels.Error{}
// @failure 401 {object} apimodels.Error{}
// @failure 409 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func getLogin(c echo.Context) error {
	// parse authorization basic if it is exist
	login, err := parser.GetAuthorizationBasic(c.Request())
	if err != nil {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: err.Error()})
	}

	raw, err := parser.GetQueryBool(c, "raw")
	if err != nil {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: err.Error()})
	}

	body := LoginModel{
		Login:    login.User,
		Password: login.Pass,
	}

	providerName := c.QueryParam("provider")
	if providerName != "" {
		return loginAndGetTokenProvider(c, body, providerName, raw)
	}

	return loginAndGetToken(c, body, raw)
}

func loginAndGetTokenProvider(c echo.Context, body LoginModel, providerName string, raw bool) error {
	generic := registry.Reg.AuthProviders[providerName]
	if generic == nil {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: "provider not found"})
	}

	// get token from provider with using grant type password
	tokenRaw, err := request.DefaultAuth.Password(c.Request().Context(), request.PassswordConfig{
		Username: body.Login,
		Password: body.Password,
		Scopes:   generic.Scopes,
		AuthRequestConfig: request.AuthRequestConfig{
			TokenURL: generic.TokenURL,
			ClientID: generic.ClientID,
		},
	})
	if err != nil {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: err.Error()})
	}

	token := auth.Token{}
	if err := json.Unmarshal(tokenRaw, &token); err != nil {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: err.Error()})
	}

	claim := claims.Custom{}
	_, _, err = auth.ParseUnverified(token.AccessToken, &claim)
	if err != nil {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: err.Error()})
	}

	if !claim.HasRole(middlewares.UserRoleKey) {
		return c.JSON(http.StatusUnauthorized, apimodels.Error{Error: `user need to have "chore_user" role`})
	}

	return returnToken(c, token, raw)
}

func loginAndGetToken(c echo.Context, body LoginModel, raw bool) error {
	// declare user for result
	user := UserPureID{}

	query := registry.Reg.DB.WithContext(c.Request().Context()).Model(&models.User{})
	query = query.Where("name = ?", body.Login).Or("email = ?", body.Login).Or("id = ?", body.Login)

	result := query.First(&user)

	if !sec.CheckHashPassword([]byte(user.Password), []byte(body.Password)) {
		return c.JSON(http.StatusUnauthorized, apimodels.Error{Error: "name or password wrong"})
	}

	// check write error
	if result.Error != nil && errors.Is(result.Error, gorm.ErrDuplicatedKey) {
		return c.JSON(http.StatusConflict, apimodels.Error{Error: result.Error.Error()})
	}

	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: result.Error.Error()})
	}

	var groups []string
	if user.Groups.Groups != nil {
		if err := json.Unmarshal(user.Groups.Groups, &groups); err != nil {
			return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: err.Error()})
		}
	}

	// log.Info().Msgf("user groups: %v", groups)

	tokenID, err := uuid.NewUUID()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: err.Error()})
	}

	// generate JWT token
	accessToken, err := registry.Reg.JWT.Generate(
		claims.NewMapClaims(tokenID, user.ID.ID, models.TypeAccessToken, groups),
		registry.Reg.JWT.ExpFunc())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: err.Error()})
	}

	return returnToken(c, auth.Token{
		AccessToken: accessToken,
	}, raw)
}

func returnToken(c echo.Context, token auth.Token, raw bool) error {
	if raw {
		return c.String(http.StatusOK, token.AccessToken)
	}

	return c.JSON(http.StatusOK,
		apimodels.Data{
			Data: token,
		},
	)
}

func Login(e *echo.Group) {
	e.POST("/login", postLogin)
	e.GET("/login", getLogin)
}
