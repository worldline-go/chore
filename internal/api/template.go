package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

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

type TemplatePureID struct {
	models.TemplatePure
	apimodels.ID
}

type MetaFolder struct {
	Folder string `json:"folder" query:"folder" example:"folderx"`
	apimodels.Meta
}

type ItemName struct {
	Item string `json:"item" example:"template1"`
	Name string `json:"name" example:"deepcore/template1"`
}

// @Summary List templates
// @Tags template
// @Description Get list of the templates, specify key query to get inner paths
// @Security ApiKeyAuth
// @Router /templates [get]
// @Param folder query string false "set the limit, default is empty"
// @Param limit query int false "set the limit, default is 20"
// @Param offset query int false "set the offset, default is 0"
// @Success 200 {object} apimodels.DataMeta{data=[]ItemName{}}
// @failure 400 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func listTemplates(c echo.Context) error {
	items := []ItemName{}

	meta := &MetaFolder{Meta: apimodels.Meta{Limit: apimodels.Limit}}

	if err := c.Bind(meta); err != nil {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: err.Error()})
	}

	// Table(reg.DB.Config.NamingStrategy.JoinTableName("folders"))

	ctx := c.Request().Context()
	query := registry.Reg.DB.WithContext(ctx).Model(&models.Folder{}).Select("item", "name")

	if meta.Limit >= 0 {
		query = query.Limit(meta.Limit)
	}

	result := query.Offset(meta.Offset).Where(
		"folder = ?", meta.Folder,
	).Order("dtype DESC").Find(&items)

	// check write error
	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: result.Error.Error()})
	}

	// get counts
	registry.Reg.DB.WithContext(ctx).Model(&models.Folder{}).Count(&meta.Count)

	return c.JSON(http.StatusOK,
		apimodels.DataMeta{
			Meta: meta,
			Data: apimodels.Data{Data: items},
		},
	)
}

// @Summary Get template
// @Tags template
// @Description Get one template with id
// @Security ApiKeyAuth
// @Router /template [get]
// @Param id query string false "get by id"
// @Param name query string false "get by name"
// @Param dump query bool false "get raw content"
// @Success 200 {object} apimodels.Data{data=TemplatePureID{}}
// @failure 400 {object} apimodels.Error{}
// @failure 404 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func getTemplate(c echo.Context) error {
	id := c.QueryParam("id")
	name := c.QueryParam("name")

	if id == "" && name == "" {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: apimodels.ErrRequiredIDName.Error()})
	}

	dump, err := parser.GetQueryBool(c, "dump")
	if err != nil {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: err.Error()})
	}

	getData := TemplatePureID{}

	ctx := c.Request().Context()
	query := registry.Reg.DB.WithContext(ctx).Model(&models.Template{})

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
		v, err := base64.StdEncoding.DecodeString(getData.Content)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: result.Error.Error()})
		}

		return c.Blob(http.StatusOK, "text/plain", v)
	}

	return c.JSON(http.StatusOK,
		apimodels.Data{
			Data: getData,
		},
	)
}

// @Summary New template
// @Tags template
// @Description Send and record new template
// @Security ApiKeyAuth
// @Router /template [post]
// @Param name query string true "name of file 'deepcore/template1'"
// @Param groups query string false "group names 'group1,group2'"
// @Param payload body string false "send template object"
// @Accept plain
// @Success 200 {object} apimodels.Data{data=apimodels.ID{}}
// @failure 400 {object} apimodels.Error{}
// @failure 409 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func postTemplate(c echo.Context) error {
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: err.Error()})
	}

	template := new(models.Template)
	template.Content = base64.StdEncoding.EncodeToString(body)

	name := c.QueryParam("name")
	if name == "" {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: "name is required"})
	}

	// trim slash
	template.Name = strings.Trim(name, "/")

	if groups := c.QueryParam("groups"); groups != "" {
		var err error

		template.Groups.Groups, err = json.Marshal(strings.Split(groups, ","))
		if err != nil {
			return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: err.Error()})
		}
	}

	template.ID.ID, err = uuid.NewUUID()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: err.Error()})
	}

	ctx := utils.Context(c)
	result := registry.Reg.DB.WithContext(ctx).Create(template)

	// check write error
	if result.Error != nil && errors.Is(result.Error, gorm.ErrDuplicatedKey) {
		return c.JSON(http.StatusConflict, apimodels.Error{Error: result.Error.Error()})
	}

	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: result.Error.Error()})
	}

	// create folder
	folderMap := utils.FolderFile(template.Name)

	// on conflict do nothing
	registry.Reg.DB.WithContext(ctx).Model(models.Folder{}).Clauses(
		clause.OnConflict{DoNothing: true},
	).Create(folderMap)

	// return recorded data's id
	return c.JSON(http.StatusOK,
		apimodels.Data{
			Data: apimodels.ID{ID: template.ID.ID},
		},
	)
}

// @Summary New or Update template
// @Tags template
// @Description Send and record template
// @Security ApiKeyAuth
// @Router /template [put]
// @Param name query string true "name of file 'deepcore/template1'"
// @Param groups query string false "group names 'group1,group2'"
// @Param payload body string false "send template object"
// @Accept plain
// @Success 204 "No Content"
// @failure 400 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func putTemplate(c echo.Context) error {
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: err.Error()})
	}

	template := new(models.Template)
	template.Content = base64.StdEncoding.EncodeToString(body)

	name := c.QueryParam("name")
	if name == "" {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: "name is required"})
	}

	// trim slash
	template.Name = strings.Trim(name, "/")

	if groups := c.QueryParam("groups"); groups != "" {
		var err error

		template.Groups.Groups, err = json.Marshal(strings.Split(groups, ","))
		if err != nil {
			return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: err.Error()})
		}
	}

	template.ID.ID, err = uuid.NewUUID()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: err.Error()})
	}

	ctx := utils.Context(c)
	result := registry.Reg.DB.WithContext(ctx).Clauses(
		clause.OnConflict{
			UpdateAll: true,
			Columns:   []clause.Column{{Name: "name"}},
		}).Create(template)

	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: result.Error.Error()})
	}

	// create folder
	folderMap := utils.FolderFile(template.Name)

	// on conflict do nothing
	registry.Reg.DB.WithContext(ctx).Model(models.Folder{}).Clauses(
		clause.OnConflict{DoNothing: true},
	).Create(folderMap)

	//nolint:wrapcheck // checking before
	return c.NoContent(http.StatusNoContent)
}

// TODO: currently just changeable inside of the data.

// @Summary Replace template
// @Tags template
// @Description Replace with new data, id or name must exist in request
// @Security ApiKeyAuth
// @Router /template [patch]
// @Param name query string false "get by name"
// @Param payload body string false "send template object"
// @Accept plain
// @Success 200 {object} apimodels.Data{data=apimodels.ID{}}
// @failure 400 {object} apimodels.Error{}
// @failure 404 {object} apimodels.Error{}
// @failure 409 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func patchTemplate(c echo.Context) error {
	name := c.QueryParam("name")

	if name == "" {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: "name is required and cannot be empty"})
	}

	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: err.Error()})
	}

	// fix parameter
	name = strings.Trim(name, "/")

	data := models.Template{
		TemplatePure: models.TemplatePure{
			Name:    name,
			Content: base64.StdEncoding.EncodeToString(body),
		},
	}

	ctx := utils.Context(c)
	// save new value
	result := registry.Reg.DB.WithContext(ctx).Where("name = ?", name).Updates(&data)

	// check write error
	if result.Error != nil && errors.Is(result.Error, gorm.ErrDuplicatedKey) {
		return c.JSON(http.StatusConflict, apimodels.Error{Error: result.Error.Error()})
	}

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return c.JSON(http.StatusNotFound, apimodels.Error{Error: result.Error.Error()})
	}

	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: result.Error.Error()})
	}

	// // update from folder table
	// if prevValues.Name != body["name"].(string) {
	// 	reg.DB.WithContext(c.UserContext()).Where("name = ?", prevValues.Name).Delete(&models.Folder{})

	// 	// create folder
	// 	folderMap := utils.FolderFile(body["name"].(string))

	// 	// on conflict do nothing
	// 	reg.DB.WithContext(c.UserContext()).Model(models.Folder{}).Clauses(
	// 		clause.OnConflict{DoNothing: true},
	// 	).Create(folderMap)
	// }

	return c.JSON(http.StatusOK,
		apimodels.Data{
			Data: map[string]interface{}{"id": data.ID},
		},
	)
}

// @Summary Delete template
// @Tags template
// @Description Delete with id, name
// @Security ApiKeyAuth
// @Router /template [delete]
// @Param id query string false "get by id"
// @Param name query string false "get by name"
// @Success 204 "No Content"
// @failure 400 {object} apimodels.Error{}
// @failure 404 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func deleteTemplate(c echo.Context) error {
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
		if name[len(name)-1] == '/' {
			query = query.Where("name LIKE ?", name+"%")
		} else {
			query = query.Where("name = ?", name)
		}
	}

	// delete directly in DB
	result := query.Unscoped().Delete(&models.Template{})

	if result.RowsAffected == 0 {
		return c.JSON(http.StatusNotFound, apimodels.Error{Error: "not found any releated data"})
	}

	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: result.Error.Error()})
	}

	// delete from folder table
	query = registry.Reg.DB.WithContext(ctx)
	if name[len(name)-1] == '/' {
		query = query.Where("name LIKE ?", name+"%")
	} else {
		query = query.Where("name = ?", name)
	}

	query.Delete(&models.Folder{})

	//nolint:wrapcheck // checking before
	return c.NoContent(http.StatusNoContent)
}

func Template(e *echo.Group, authMiddleware echo.MiddlewareFunc) {
	e.GET("/templates", listTemplates, authMiddleware, middlewares.UserRole, middlewares.PatToken)
	e.GET("/template", getTemplate, authMiddleware, middlewares.UserRole, middlewares.PatToken)
	e.POST("/template", postTemplate, authMiddleware, middlewares.UserRole, middlewares.PatToken)
	e.PUT("/template", putTemplate, authMiddleware, middlewares.UserRole, middlewares.PatToken)
	e.PATCH("/template", patchTemplate, authMiddleware, middlewares.UserRole, middlewares.PatToken)
	e.DELETE("/template", deleteTemplate, authMiddleware, middlewares.UserRole, middlewares.PatToken)
}
