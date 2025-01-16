package api

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"github.com/worldline-go/chore/internal/server/middlewares"
	"github.com/worldline-go/chore/internal/utils"
	"github.com/worldline-go/chore/pkg/models"
	"github.com/worldline-go/chore/pkg/models/apimodels"
	"github.com/worldline-go/chore/pkg/registry"
)

// @Summary Get Settings
// @Tags settings
// @Description Get whole settings
// @Security ApiKeyAuth
// @Router /settings [get]
// @Param namespace query string true "get by namespace (email, oauth2)"
// @Param name query string false "name like email-1"
// @Success 200 {object} apimodels.Data{}
// @failure 400 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func getSettings(c echo.Context) error {
	namespace := c.QueryParam("namespace")
	if namespace == "" {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: "required namespace"})
	}

	name := c.QueryParam("name")

	var setting models.SettingsPure
	var settings []models.SettingsPure

	var result *gorm.DB

	var returnData interface{}
	query := registry.Reg.DB.WithContext(c.Request().Context()).Model(&models.Settings{}).Where("namespace = ?", namespace)
	if name != "" {
		query.Where("name = ?", name)
		result = query.First(&setting)
		returnData = setting
	} else {
		result = query.Find(&settings)
		returnData = settings
	}

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return c.JSON(http.StatusOK, apimodels.Data{Data: setting})
	}

	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: result.Error.Error()})
	}

	return c.JSON(http.StatusOK,
		apimodels.Data{
			Data: returnData,
		},
	)
}

// @Summary Replace settings
// @Tags settings
// @Description Replace with new data
// @Security ApiKeyAuth
// @Router /settings [patch]
// @Param payload body models.Settings{} false "send part of the settings object"
// @Param namespace query string true "get by namespace (email, oauth2)"
// @Param name query string false "name like email-1"
// @Success 204 "No Content"
// @failure 400 {object} apimodels.Error{}
// @failure 409 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func patchSettings(c echo.Context) error {
	namespace := c.QueryParam("namespace")
	if namespace == "" {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: "required namespace"})
	}

	name := c.QueryParam("name")

	var body map[string]interface{}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: err.Error()})
	}

	bodyModel := models.Settings{
		SettingsPure: models.SettingsPure{
			Name:      name,
			Namespace: namespace,
			Data:      body,
		},
	}

	ctx := utils.Context(c)
	query := registry.Reg.DB.WithContext(ctx).Model(&models.Settings{})

	queryUpdate := query.Where("namespace = ?", namespace).Where("name = ?", name)
	result := queryUpdate.Updates(&bodyModel)

	if result.RowsAffected == 0 {
		query.Create(&bodyModel)
	}

	if result.Error != nil {
		// check write error
		if errors.Is(result.Error, gorm.ErrDuplicatedKey) {
			return c.JSON(http.StatusConflict, apimodels.Error{Error: result.Error.Error()})
		}

		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: result.Error.Error()})
	}

	//nolint:wrapcheck // checking before
	return c.NoContent(http.StatusNoContent)
}

// @Summary Delete settings
// @Tags settings
// @Description Replace with new data
// @Security ApiKeyAuth
// @Router /settings [delete]
// @Param namespace query string true "get by namespace (email, oauth2)"
// @Param name query string false "name like email-1"
// @Success 204 "No Content"
// @failure 400 {object} apimodels.Error{}
// @failure 404 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func deleteSettings(c echo.Context) error {
	namespace := c.QueryParam("namespace")
	if namespace == "" {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: "required namespace"})
	}

	name := c.QueryParam("name")

	ctx := utils.Context(c)
	query := registry.Reg.DB.WithContext(ctx).Model(&models.Settings{})

	queryDelete := query.Where("namespace = ?", namespace).Where("name = ?", name)

	// delete directly in DB
	result := queryDelete.Delete(&models.Settings{})
	if result.RowsAffected == 0 {
		return c.JSON(http.StatusNotFound, apimodels.Error{Error: "not found any releated data"})
	}

	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: result.Error.Error()})
	}

	//nolint:wrapcheck // checking before
	return c.NoContent(http.StatusNoContent)
}

func Settings(c *echo.Group, authMiddleware echo.MiddlewareFunc) {
	c.GET("/settings", getSettings, authMiddleware, middlewares.AdminRole, middlewares.PatToken)
	c.PATCH("/settings", patchSettings, authMiddleware, middlewares.AdminRole, middlewares.PatToken)
	c.DELETE("/settings", deleteSettings, authMiddleware, middlewares.AdminRole, middlewares.PatToken)
}
