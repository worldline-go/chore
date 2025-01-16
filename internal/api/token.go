package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"

	"github.com/worldline-go/auth"
	"github.com/worldline-go/auth/pkg/authecho"
	"github.com/worldline-go/auth/request"
	"github.com/worldline-go/chore/internal/server/claims"
	"github.com/worldline-go/chore/internal/server/middlewares"
	"github.com/worldline-go/chore/internal/utils"
	"github.com/worldline-go/chore/pkg/models"
	"github.com/worldline-go/chore/pkg/models/apimodels"
	"github.com/worldline-go/chore/pkg/registry"
)

type TokenDataByID struct {
	models.TokenDataBy
	apimodels.ID
}

// @Summary List tokens
// @Tags token
// @Description Get list of the tokens
// @Security ApiKeyAuth
// @Router /tokens [get]
// @Param limit query int false "set the limit, default is 20"
// @Param offset query int false "set the offset, default is 0"
// @Param search query string string "search item"
// @Success 200 {object} apimodels.DataMeta{data=[]TokenDataByID{},meta=apimodels.Meta{}}
// @failure 400 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func listTokens(c echo.Context) error {
	meta := apimodels.Meta{Limit: apimodels.Limit}

	if err := c.Bind(&meta); err != nil {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: err.Error()})
	}

	tokens := []TokenDataByID{}

	query := registry.Reg.DB.WithContext(c.Request().Context()).Model(&models.Token{}).Limit(meta.Limit).Offset(meta.Offset)

	if meta.Search != "" {
		query = query.Where("name LIKE ?", meta.Search+"%")
	}

	if result := query.Find(&tokens); result.Error != nil {
		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: result.Error.Error()})
	}

	// get counts
	query = registry.Reg.DB.WithContext(c.Request().Context()).Model(&models.Token{})
	if meta.Search != "" {
		query = query.Where("name LIKE ?", meta.Search+"%")
	}

	query.Count(&meta.Count)

	return c.JSON(http.StatusOK,
		apimodels.DataMeta{
			Meta: meta,
			Data: apimodels.Data{Data: tokens},
		},
	)
}

// @Summary Get token
// @Tags token
// @Description Get token with token id
// @Security ApiKeyAuth
// @Router /token [get]
// @Param id query string false "get by token id"
// @Success 200 {object} apimodels.Data{data=models.Token{}}
// @failure 400 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func getToken(c echo.Context) error {
	id := c.QueryParam("id")

	if id == "" {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: "id is required"})
	}

	token := models.Token{}
	result := registry.Reg.DB.WithContext(c.Request().Context()).Where("id = ?", id).Find(&token)

	if result.RowsAffected == 0 {
		return c.JSON(http.StatusNotFound, apimodels.Error{Error: "not found any releated data"})
	}

	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: result.Error.Error()})
	}

	return c.JSON(http.StatusOK, apimodels.Data{Data: token})
}

// @Summary New token
// @Tags token
// @Description Send and record PAT token
// @Security ApiKeyAuth
// @Router /token [post]
// @Param payload body models.TokenData{} false "token parameters"
// @Success 200 {object} apimodels.Data{data=models.Token{}}
// @failure 400 {object} apimodels.Error{}
// @failure 409 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func postToken(c echo.Context) error {
	body := new(models.TokenData)
	if err := c.Bind(body); err != nil {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: err.Error()})
	}

	if body.Name == "" {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: "name is empty"})
	}

	var groups []string
	if body.Groups.Groups != nil {
		if err := json.Unmarshal(body.Groups.Groups, &groups); err != nil {
			return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: err.Error()})
		}
	}

	tokenID, err := uuid.NewUUID()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: err.Error()})
	}

	var unixLastDate int64
	if body.Date != nil && !body.Date.IsZero() {
		unixLastDate = body.Date.Unix()

		if unixLastDate-time.Now().Unix() <= 0 {
			return c.JSON(http.StatusBadRequest, apimodels.Error{Error: "date should be older than now"})
		}
	}

	userID, err := uuid.Parse("00000000-0000-0000-0000-000000000000")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: err.Error()})
	}

	claim, _ := c.Get(authecho.KeyClaims).(*claims.Custom)
	if claim != nil {
		if claim.Subject != "" {
			userIDParsed, err := uuid.Parse(claim.Subject)
			if err != nil {
				log.Warn().Err(err).Msgf("cannot parse user id: %s", claim.Subject)
			}

			userID = userIDParsed
		}
	}

	// generate new PAT token
	token, err := registry.Reg.JWT.Generate(
		claims.NewMapClaims(tokenID, userID, models.TypePersonalAccessToken, groups), unixLastDate)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: err.Error()})
	}

	createToken := models.Token{
		ModelC: apimodels.ModelC{
			ID: apimodels.ID{ID: tokenID},
		},
		TokenPure: models.TokenPure{
			TokenDataBy: models.TokenDataBy{
				TokenData: *body,
				CreatedBy: userID,
			},
			TokenPrivate: models.TokenPrivate{
				Token: token,
			},
		},
	}

	ctx := utils.Context(c)
	result := registry.Reg.DB.WithContext(ctx).Create(&createToken)

	// check write error
	if result.Error != nil && errors.Is(result.Error, gorm.ErrDuplicatedKey) {
		return c.JSON(http.StatusConflict, apimodels.Error{Error: result.Error.Error()})
	}

	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: result.Error.Error()})
	}

	// return recorded data's id
	return c.JSON(http.StatusOK, apimodels.Data{Data: createToken})
}

// @Summary Delete token
// @Tags token
// @Description Delete with token
// @Security ApiKeyAuth
// @Router /token [delete]
// @Param id query string false "get by token id"
// @Success 204 "No Content"
// @failure 400 {object} apimodels.Error{}
// @failure 404 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func deleteToken(c echo.Context) error {
	id := c.QueryParam("id")

	if id == "" {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: "id is required"})
	}

	// delete directly in DB
	ctx := utils.Context(c)
	result := registry.Reg.DB.WithContext(ctx).Where("id = ?", id).Unscoped().Delete(&models.Token{})

	if result.RowsAffected == 0 {
		return c.JSON(http.StatusNotFound, apimodels.Error{Error: "not found any releated data"})
	}

	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: result.Error.Error()})
	}

	//nolint:wrapcheck // checking before
	return c.NoContent(http.StatusNoContent)
}

// @Summary Check token
// @Tags token
// @Description Send token to check validation, if not valid return 401
// @Router /token/check [post]
// @Param payload body models.TokenPrivate{} false "send a token"
// @Success 204 "No Content"
// @failure 400 {object} apimodels.Error{}
// @failure 401 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func postTokenCheck(c echo.Context) error {
	body := new(models.TokenPrivate)
	if err := c.Bind(body); err != nil {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: err.Error()})
	}

	if body.Token == "" {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: "token is required"})
	}

	// check token is valid
	claims := jwt.MapClaims{}
	if _, err := registry.Reg.JWT.Parser.ParseWithClaims(body.Token, &claims); err != nil {
		return c.JSON(http.StatusUnauthorized, apimodels.Error{Error: fmt.Sprintf("token not valid: %v", err.Error())})
	}

	// return recorded data's id
	return c.NoContent(http.StatusNoContent)
}

// @Summary Renew token
// @Tags token
// @Description Get new token based on old token
// @Router /token/renew [get]
// @Param payload body models.TokenPrivate{} false "send a token"
// @Param provider query string false "oauth2 provider name"
// @Success 200 {object} apimodels.Data{data=models.TokenPrivate{}}
// @failure 400 {object} apimodels.Error{}
// @failure 401 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func postTokenRenew(c echo.Context) error {
	// check token is valid
	body := new(models.TokenPrivate)
	if err := c.Bind(body); err != nil {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: err.Error()})
	}

	providerName := c.QueryParam("provider")
	if providerName != "" {
		generic := registry.Reg.AuthProviders[providerName]
		if generic == nil {
			return c.JSON(http.StatusBadRequest, apimodels.Error{Error: "provider not found"})
		}

		tokenRaw, err := request.DefaultAuth.RefreshToken(c.Request().Context(), request.RefreshTokenConfig{
			RefreshToken: body.Token,
			AuthRequestConfig: request.AuthRequestConfig{
				TokenURL:     generic.TokenURL,
				ClientID:     generic.ClientID,
				ClientSecret: generic.ClientSecret,
				Scopes:       generic.Scopes,
			},
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, apimodels.Error{Error: err.Error()})
		}

		token := auth.Token{}
		if err := json.Unmarshal(tokenRaw, &token); err != nil {
			return c.JSON(http.StatusBadRequest, apimodels.Error{Error: err.Error()})
		}

		return c.JSON(http.StatusOK, apimodels.Data{Data: token})
	}

	token, err := registry.Reg.JWT.Renew(body.Token, registry.Reg.JWT.ExpFunc())
	if err != nil {
		return c.JSON(http.StatusUnauthorized, apimodels.Error{Error: fmt.Sprintf("renew failed: %v", err.Error())})
	}

	// return recorded data's id
	return c.JSON(http.StatusOK, apimodels.Data{Data: auth.Token{AccessToken: token}})
}

func Token(e *echo.Group, authMiddleware echo.MiddlewareFunc) {
	e.POST("/token/check", postTokenCheck)
	e.POST("/token/renew", postTokenRenew)
	e.GET("/tokens", listTokens, authMiddleware, middlewares.UserRole, middlewares.PatToken)
	e.GET("/token", getToken, authMiddleware, middlewares.UserRole, middlewares.PatToken)
	e.POST("/token", postToken, authMiddleware, middlewares.UserRole, middlewares.PatToken)
	e.DELETE("/token", deleteToken, authMiddleware, middlewares.UserRole, middlewares.PatToken)
}
