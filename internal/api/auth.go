package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/worldline-go/chore/internal/parser"
	"github.com/worldline-go/chore/internal/server/middlewares"
	"github.com/worldline-go/chore/internal/utils"
	"github.com/worldline-go/chore/pkg/models"
	"github.com/worldline-go/chore/pkg/models/apimodels"
	"github.com/worldline-go/chore/pkg/registry"
)

type AuthPureID struct {
	models.AuthPure
	apimodels.ID
}

// @Summary List auths
// @Tags auth
// @Description Get list of the auths
// @Security ApiKeyAuth
// @Router /auths [get]
// @Param limit query int false "set the limit, default is 20"
// @Param offset query int false "set the offset, default is 0"
// @Param search query string false "search item"
// @Success 200 {object} apimodels.DataMeta{data=[]AuthPureID{},meta=apimodels.Meta{}}
// @failure 400 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func listAuths(c echo.Context) error {
	auths := []AuthPureID{}

	meta := &apimodels.Meta{Limit: apimodels.Limit}

	if err := c.Bind(meta); err != nil {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: err.Error()})
	}

	query := registry.Reg.DB.WithContext(c.Request().Context()).Model(&models.Auth{}).Limit(meta.Limit).Offset(meta.Offset)

	if meta.Search != "" {
		query = query.Where("name LIKE ?", meta.Search+"%")
	}

	result := query.Find(&auths)

	// check write error
	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: result.Error.Error()})
	}

	// get counts
	query = registry.Reg.DB.WithContext(c.Request().Context()).Model(&models.Auth{})
	if meta.Search != "" {
		query = query.Where("name LIKE ?", meta.Search+"%")
	}

	query.Count(&meta.Count)

	return c.JSON(http.StatusOK,
		apimodels.DataMeta{
			Meta: meta,
			Data: apimodels.Data{Data: auths},
		},
	)
}

// @Summary Get auth
// @Tags auth
// @Description Get one auth with id or name
// @Security ApiKeyAuth
// @Router /auth [get]
// @Param id query string false "get by id"
// @Param name query string false "get by name"
// @Param dump query bool false "get for record values"
// @Param pretty query bool false "pretty output for dump"
// @Success 200 {object} apimodels.Data{data=AuthPureID{}}
// @failure 400 {object} apimodels.Error{}
// @failure 404 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func getAuth(c echo.Context) error {
	id := c.QueryParam("id")
	name := c.QueryParam("name")

	if id == "" && name == "" {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: apimodels.ErrRequiredIDName.Error()})
	}

	dump, err := parser.GetQueryBool(c, "dump")
	if err != nil {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: err.Error()})
	}

	pretty, err := parser.GetQueryBool(c, "pretty")
	if err != nil {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: err.Error()})
	}

	getData := new(AuthPureID)

	query := registry.Reg.DB.WithContext(c.Request().Context()).Model(&models.Auth{})
	if id != "" {
		query = query.Where("id = ?", id)
	}

	if name != "" {
		query = query.Where("name = ?", name)
	}

	result := query.First(&getData)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return c.JSON(http.StatusNotFound, apimodels.Error{Error: result.Error.Error()})
	}

	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: result.Error.Error()})
	}

	if dump {
		if pretty {
			return c.JSONPretty(http.StatusOK, getData, "  ")
		}

		return c.JSON(http.StatusOK, getData)
	}

	return c.JSON(http.StatusOK,
		apimodels.Data{
			Data: getData,
		},
	)
}

// @Summary New or Update auth
// @Tags auth
// @Description Send and record auth
// @Security ApiKeyAuth
// @Router /auth [put]
// @Param payload body AuthPureID{} false "send auth object"
// @Success 204 "No Content"
// @failure 400 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func putAuth(c echo.Context) error {
	var body AuthPureID
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: err.Error()})
	}

	id := body.ID.ID
	if id.String() == "00000000-0000-0000-0000-000000000000" {
		if body.Name == "" {
			return c.JSON(http.StatusBadRequest, apimodels.Error{Error: apimodels.ErrRequiredName.Error()})
		}

		var err error
		id, err = uuid.NewUUID()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: err.Error()})
		}
	}

	ctx := utils.Context(c)
	result := registry.Reg.DB.WithContext(ctx).Model(&models.Auth{}).Clauses(
		clause.OnConflict{
			UpdateAll: true,
			Columns:   []clause.Column{{Name: "id"}},
		}).Create(
		&models.Auth{
			AuthPure: body.AuthPure,
			ModelCU: apimodels.ModelCU{
				ID: apimodels.ID{ID: id},
			},
		},
	)

	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: result.Error.Error()})
	}

	//nolint:wrapcheck // checking before
	return c.NoContent(http.StatusNoContent)
}

// @Summary New auth
// @Tags auth
// @Description Send and record new auth
// @Security ApiKeyAuth
// @Router /auth [post]
// @Param payload body models.AuthPure{} false "send auth object"
// @Success 200 {object} apimodels.Data{data=apimodels.ID{}}
// @failure 400 {object} apimodels.Error{}
// @failure 409 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func postAuth(c echo.Context) error {
	var body models.AuthPure
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: err.Error()})
	}

	if body.Name == "" {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: apimodels.ErrRequiredName.Error()})
	}

	id, err := uuid.NewUUID()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: err.Error()})
	}

	ctx := utils.Context(c)
	result := registry.Reg.DB.WithContext(ctx).Model(&models.Auth{}).Create(
		&models.Auth{
			AuthPure: body,
			ModelCU: apimodels.ModelCU{
				ID: apimodels.ID{ID: id},
			},
		},
	)

	// check write error
	if result.Error != nil && errors.Is(result.Error, gorm.ErrDuplicatedKey) {
		return c.JSON(http.StatusConflict, apimodels.Error{Error: result.Error.Error()})
	}

	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: result.Error.Error()})
	}

	// return recorded data's id
	return c.JSON(
		http.StatusOK,
		apimodels.Data{
			Data: apimodels.ID{ID: id},
		},
	)
}

// @Summary Patch auth
// @Tags auth
// @Description Patch with a few data, id must exist in request
// @Security ApiKeyAuth
// @Router /auth [patch]
// @Param payload body AuthPureID{} false "send part of the user object"
// @Success 200 {object} apimodels.Data{data=apimodels.ID{}}
// @failure 400 {object} apimodels.Error{}
// @failure 404 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func patchAuth(c echo.Context) error {
	var body map[string]interface{}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: err.Error()})
	}

	if v, ok := body["id"].(string); !ok || v == "" {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: "id is required and cannot be empty"})
	}

	if body["groups"] != nil {
		var err error

		body["groups"], err = json.Marshal(body["groups"])
		if err != nil {
			return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: err.Error()})
		}
	}

	ctx := utils.Context(c)
	query := registry.Reg.DB.WithContext(ctx).Model(&models.Auth{}).Where("id = ?", body["id"])

	result := query.Updates(body)

	// check write error
	if result.Error != nil && errors.Is(result.Error, gorm.ErrDuplicatedKey) {
		return c.JSON(http.StatusConflict, apimodels.Error{Error: result.Error.Error()})
	}

	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: result.Error.Error()})
	}

	resultData := make(map[string]interface{})
	resultData["id"] = body["id"]

	return c.JSON(http.StatusOK,
		apimodels.Data{
			Data: resultData,
		},
	)
}

// @Summary Delete auth
// @Tags auth
// @Description Delete with id or name
// @Security ApiKeyAuth
// @Router /auth [delete]
// @Param id query string false "get by id"
// @Param name query string false "get by name"
// @Success 204 "No Content"
// @failure 400 {object} apimodels.Error{}
// @failure 404 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func deleteAuth(c echo.Context) error {
	id := c.QueryParam("id")

	if id == "" {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: apimodels.ErrRequiredID.Error()})
	}

	ctx := utils.Context(c)
	query := registry.Reg.DB.WithContext(ctx).Where("id = ?", id)

	// delete directly in DB
	result := query.Unscoped().Delete(&models.Auth{})

	if result.RowsAffected == 0 {
		return c.JSON(http.StatusNotFound, apimodels.Error{Error: "not found any releated data"})
	}

	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: result.Error.Error()})
	}

	//nolint:wrapcheck // checking before
	return c.NoContent(http.StatusNoContent)
}

func Auth(e *echo.Group, authMiddleware echo.MiddlewareFunc) {
	e.GET("/auths", listAuths, authMiddleware, middlewares.UserRole, middlewares.PatToken)
	e.GET("/auth", getAuth, authMiddleware, middlewares.UserRole, middlewares.PatToken)
	e.POST("/auth", postAuth, authMiddleware, middlewares.UserRole, middlewares.PatToken)
	e.PUT("/auth", putAuth, authMiddleware, middlewares.UserRole, middlewares.PatToken)
	e.PATCH("/auth", patchAuth, authMiddleware, middlewares.UserRole, middlewares.PatToken)
	e.DELETE("/auth", deleteAuth, authMiddleware, middlewares.UserRole, middlewares.PatToken)
}
