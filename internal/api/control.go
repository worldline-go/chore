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

type ControlPureContentID struct {
	models.ControlPureContent
	apimodels.ID
}

type ControlPureID struct {
	models.ControlPure
	apimodels.ID
}

// @Summary List controls
// @Tags control
// @Description Get list of the controls
// @Security ApiKeyAuth
// @Router /controls [get]
// @Param limit query int false "set the limit, default is 20"
// @Param offset query int false "set the offset, default is 0"
// @Param search query string string "search item"
// @Success 200 {object} apimodels.DataMeta{data=[]ControlPureID{},meta=apimodels.Meta{}}
// @failure 400 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func listControls(c echo.Context) error {
	controlsPureID := []ControlPureID{}

	meta := &apimodels.Meta{Limit: apimodels.Limit}

	if err := c.Bind(meta); err != nil {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: err.Error()})
	}

	query := registry.Reg.DB.WithContext(c.Request().Context()).Model(&models.Control{})

	if meta.Search != "" {
		query = query.Where("name LIKE ?", meta.Search+"%")
	}

	result := query.Limit(meta.Limit).Offset(meta.Offset).Find(&controlsPureID)

	// check write error
	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: result.Error.Error()})
	}

	// get counts
	query = registry.Reg.DB.WithContext(c.Request().Context()).Model(&models.Control{})
	if meta.Search != "" {
		query = query.Where("name LIKE ?", meta.Search+"%")
	}

	query.Count(&meta.Count)

	return c.JSON(http.StatusOK,
		apimodels.DataMeta{
			Meta: meta,
			Data: apimodels.Data{Data: controlsPureID},
		},
	)
}

// @Summary Get control
// @Tags control
// @Description Get one control with id, content is base64 format
// @Security ApiKeyAuth
// @Router /control [get]
// @Param id query string false "get by id"
// @Param name query string false "get by name"
// @Param nodata query bool false "not get content"
// @Param dump query bool false "get for record values"
// @Param pretty query bool false "pretty output for dump"
// @Success 200 {object} apimodels.Data{data=ControlPureContentID{}}
// @failure 400 {object} apimodels.Error{}
// @failure 404 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func getControl(c echo.Context) error {
	nodata, err := parser.GetQueryBool(c, "nodata")
	if err != nil {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: err.Error()})
	}

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

	controlContent := new(ControlPureContentID)
	control := new(ControlPureID)

	query := registry.Reg.DB.WithContext(c.Request().Context()).Model(&models.Control{})

	if id != "" {
		query = query.Where("id = ?", id)
	}

	if name != "" {
		query = query.Where("name = ?", name)
	}

	var result *gorm.DB
	if nodata {
		result = query.First(&control)
	} else {
		result = query.First(&controlContent)
	}

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return c.JSON(http.StatusNotFound, apimodels.Error{Error: result.Error.Error()})
	}

	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: result.Error.Error()})
	}

	var ret interface{}
	if nodata {
		ret = control
	} else {
		// use only base64 for all control operations
		// if dump {
		// 	contentRaw, err := base64.StdEncoding.DecodeString(controlContent.Content)
		// 	if err != nil {
		// 		return c.Status(http.StatusInternalServerError).JSON(
		// 			apimodels.Error{
		// 				Error: err.Error(),
		// 			},
		// 		)
		// 	}
		// 	controlContent.Content = string(contentRaw)
		// }
		ret = controlContent
	}

	if dump {
		if pretty {
			return c.JSONPretty(http.StatusOK, ret, "  ")
		}

		return c.JSON(http.StatusOK, ret)
	}

	return c.JSON(http.StatusOK,
		apimodels.Data{
			Data: ret,
		},
	)
}

// @Summary New control
// @Tags control
// @Description Send and record new control, content must be base64 format
// @Security ApiKeyAuth
// @Router /control [post]
// @Param payload body models.ControlPureContent{} false "send control object"
// @Success 200 {object} apimodels.Data{data=apimodels.ID{}}
// @failure 400 {object} apimodels.Error{}
// @failure 409 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func postControl(c echo.Context) error {
	var body models.ControlPureContent
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: err.Error()})
	}

	if body.Name == "" {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: apimodels.ErrRequiredName.Error()})
	}

	// body content must be base64
	// body.Content = base64.StdEncoding.EncodeToString([]byte(body.Content))

	id, err := uuid.NewUUID()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: err.Error()})
	}

	ctx := utils.Context(c)
	result := registry.Reg.DB.WithContext(ctx).Model(&models.Control{}).Create(
		&models.Control{
			ControlPureContent: body,
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
	return c.JSON(http.StatusOK, apimodels.Data{Data: apimodels.ID{ID: id}})
}

// @Summary Clone control
// @Tags control
// @Description Clone existed control
// @Security ApiKeyAuth
// @Router /control/clone [post]
// @Param payload body models.ControlClone{} false "send control clone object"
// @Success 200 {object} apimodels.Data{data=apimodels.ID{}}
// @failure 400 {object} apimodels.Error{}
// @failure 409 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func cloneControl(c echo.Context) error {
	var body models.ControlClone
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: err.Error()})
	}

	if body.Name == "" || body.NewName == "" {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: apimodels.ErrRequiredName.Error()})
	}

	// body content must be base64
	// body.Content = base64.StdEncoding.EncodeToString([]byte(body.Content))

	// get control content
	controlContent := new(ControlPureContentID)

	ctx := utils.Context(c)
	result := registry.Reg.DB.WithContext(ctx).Model(&models.Control{}).Where("name = ?", body.Name).First(&controlContent)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return c.JSON(http.StatusNotFound, apimodels.Error{Error: result.Error.Error()})
	}

	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: result.Error.Error()})
	}

	id, err := uuid.NewUUID()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: err.Error()})
	}

	// set new name
	controlContent.ControlPureContent.Name = body.NewName

	result = registry.Reg.DB.WithContext(ctx).Model(&models.Control{}).Create(
		&models.Control{
			ControlPureContent: controlContent.ControlPureContent,
			ModelCU: apimodels.ModelCU{
				ID: apimodels.ID{ID: id},
			},
		},
	)

	if result.Error != nil && errors.Is(result.Error, gorm.ErrDuplicatedKey) {
		return c.JSON(http.StatusConflict, apimodels.Error{Error: result.Error.Error()})
	}

	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: result.Error.Error()})
	}

	// return recorded data's id
	return c.JSON(http.StatusOK,
		apimodels.Data{
			Data: apimodels.ID{ID: id},
		},
	)
}

// @Summary New or Update control
// @Tags control
// @Description Send and record control, content must be base64 format
// @Security ApiKeyAuth
// @Router /control [put]
// @Param payload body models.ControlPureContent{} false "send control object"
// @Success 204 "No Content"
// @failure 400 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func putControl(c echo.Context) error {
	var body models.ControlPureContent
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: err.Error()})
	}

	if body.Name == "" {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: apimodels.ErrRequiredName.Error()})
	}

	// body content must be base64
	// body.Content = base64.StdEncoding.EncodeToString([]byte(body.Content))

	id, err := uuid.NewUUID()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: err.Error()})
	}

	ctx := utils.Context(c)
	result := registry.Reg.DB.WithContext(ctx).Model(&models.Control{}).Clauses(
		clause.OnConflict{
			UpdateAll: true,
			Columns:   []clause.Column{{Name: "name"}},
		}).Create(
		&models.Control{
			ControlPureContent: body,
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

// @Summary Replace control
// @Tags control
// @Description Replace with new data, id or name must exist in request
// @Security ApiKeyAuth
// @Router /control [patch]
// @Param payload body ControlPureID{} false "send part of the control object"
// @Success 200 {object} apimodels.Data{data=apimodels.ID{}}
// @failure 400 {object} apimodels.Error{}
// @failure 409 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func patchControl(c echo.Context) error {
	var body map[string]interface{}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: err.Error()})
	}

	if v, ok := body["id"].(string); !ok || v == "" {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: "id is required and cannot be empty"})
	}

	// content, _ := body["content"].(string)
	// body["content"] = base64.StdEncoding.EncodeToString([]byte(content))

	var err error

	body["endpoints"], err = json.Marshal(body["endpoints"])
	if err != nil {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: err.Error()})
	}

	body["groups"], err = json.Marshal(body["groups"])
	if err != nil {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: err.Error()})
	}

	ctx := utils.Context(c)
	query := registry.Reg.DB.WithContext(ctx).Model(&models.Control{}).Where("id = ?", body["id"])

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

// @Summary Delete control
// @Tags control
// @Description Delete with id or name
// @Security ApiKeyAuth
// @Router /control [delete]
// @Param id query string false "get by id"
// @Param name query string false "get by name"
// @Success 204 "No Content"
// @failure 400 {object} apimodels.Error{}
// @failure 404 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func deleteControl(c echo.Context) error {
	id := c.QueryParam("id")
	name := c.QueryParam("name")

	if id == "" && name == "" {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: apimodels.ErrRequiredIDName.Error()})
	}

	ctx := utils.Context(c)
	query := registry.Reg.DB.WithContext(ctx)

	if id != "" {
		query = query.Where("id = ?", id)
	}

	if name != "" {
		query = query.Where("name = ?", name)
	}

	// delete directly in DB
	result := query.Unscoped().Delete(&models.Control{})

	if result.RowsAffected == 0 {
		return c.JSON(http.StatusNotFound, apimodels.Error{Error: apimodels.ErrNotFound.Error()})
	}

	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: result.Error.Error()})
	}

	//nolint:wrapcheck // checking before
	return c.NoContent(http.StatusNoContent)
}

func Control(e *echo.Group, authMiddleware echo.MiddlewareFunc) {
	e.POST("/control/clone", cloneControl, authMiddleware, middlewares.UserRole, middlewares.PatToken)
	e.GET("/controls", listControls, authMiddleware, middlewares.UserRole, middlewares.PatToken)
	e.GET("/control", getControl, authMiddleware, middlewares.UserRole, middlewares.PatToken)
	e.POST("/control", postControl, authMiddleware, middlewares.UserRole, middlewares.PatToken)
	e.PUT("/control", putControl, authMiddleware, middlewares.UserRole, middlewares.PatToken)
	e.PATCH("/control", patchControl, authMiddleware, middlewares.UserRole, middlewares.PatToken)
	e.DELETE("/control", deleteControl, authMiddleware, middlewares.UserRole, middlewares.PatToken)
}
