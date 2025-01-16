package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/worldline-go/chore/internal/server/middlewares"
	"github.com/worldline-go/chore/internal/utils"
	"github.com/worldline-go/chore/pkg/models"
	"github.com/worldline-go/chore/pkg/models/apimodels"
	"github.com/worldline-go/chore/pkg/registry"
	"github.com/worldline-go/chore/pkg/sec"
)

type UserDataID struct {
	models.UserData
	apimodels.ID
}

type UserPureID struct {
	models.UserPure
	apimodels.ID
}

type UserMetaAdmin struct {
	Admin bool `json:"admin,omitempty" query:"admin"`
	apimodels.Meta
}

// @Summary List users
// @Tags user
// @Description Get list of the users
// @Security ApiKeyAuth
// @Router /users [get]
// @Param limit query int false "set the limit, default is 20"
// @Param offset query int false "set the offset, default is 0"
// @Param search query string false "search item"
// @Success 200 {object} apimodels.DataMeta{data=[]UserDataID{},meta=apimodels.Meta{}}
// @failure 400 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func listUsers(c echo.Context) error {
	users := []UserDataID{}

	meta := &UserMetaAdmin{
		Meta:  apimodels.Meta{Limit: apimodels.Limit},
		Admin: false,
	}

	if err := c.Bind(meta); err != nil {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: err.Error()})
	}

	query := registry.Reg.DB.WithContext(c.Request().Context()).Model(&models.User{}).Limit(meta.Limit).Offset(meta.Offset)

	if meta.Search != "" {
		query = query.Where("name LIKE ?", meta.Search+"%")
	}

	result := query.Find(&users)

	// check write error
	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: result.Error.Error()})
	}

	// get counts
	query = registry.Reg.DB.WithContext(c.Request().Context()).Model(&models.User{})
	if meta.Search != "" {
		query = query.Where("name LIKE ?", meta.Search+"%")
	}

	query.Count(&meta.Count)

	return c.JSON(http.StatusOK,
		apimodels.DataMeta{
			Meta: meta.Meta,
			Data: apimodels.Data{Data: users},
		},
	)
}

// @Summary Get user
// @Tags user
// @Description Get one user with id or name
// @Security ApiKeyAuth
// @Router /user [get]
// @Param id query string false "get by id"
// @Success 200 {object} apimodels.Data{data=UserDataID{}}
// @failure 400 {object} apimodels.Error{}
// @failure 404 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func getUser(c echo.Context) error {
	id := c.QueryParam("id")

	if id == "" {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: apimodels.ErrRequiredIDName.Error()})
	}

	user := new(UserDataID)

	query := registry.Reg.DB.WithContext(c.Request().Context()).Model(&models.User{})
	if id != "" {
		query = query.Where("id = ?", id)
	}

	result := query.First(&user)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return c.JSON(http.StatusNotFound, apimodels.Error{Error: result.Error.Error()})
	}

	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: result.Error.Error()})
	}

	return c.JSON(http.StatusOK,
		apimodels.Data{
			Data: user,
		},
	)
}

// @Summary Delete user
// @Tags user
// @Description Delete with id or name
// @Security ApiKeyAuth
// @Router /user [delete]
// @Param id query string false "get by id"
// @Success 204 "No Content"
// @failure 400 {object} apimodels.Error{}
// @failure 404 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func deleteUser(c echo.Context) error {
	id := c.QueryParam("id")

	if id == "" {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: apimodels.ErrRequiredIDName.Error()})
	}

	ctx := utils.Context(c)
	query := registry.Reg.DB.WithContext(ctx)
	if id != "" {
		query = query.Where("id = ?", id)
	}

	// delete directly in DB
	result := query.Unscoped().Delete(&models.User{})

	if result.RowsAffected == 0 {
		return c.JSON(http.StatusNotFound, apimodels.Error{Error: "not found any releated data"})
	}

	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: result.Error.Error()})
	}

	//nolint:wrapcheck // checking before
	return c.NoContent(http.StatusNoContent)
}

// @Summary New user
// @Tags user
// @Description Send and record new user
// @Security ApiKeyAuth
// @Router /user [post]
// @Param payload body models.UserPure{} false "send user object"
// @Success 200 {object} apimodels.Data{data=apimodels.ID{}}
// @failure 400 {object} apimodels.Error{}
// @failure 409 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func postUser(c echo.Context) error {
	// body := map[string]interface{}{}
	body := models.UserRequest{}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: err.Error()})
	}

	if !body.Name.Valid {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: "name is required"})
	}

	if !body.Password.Valid {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: "password is required"})
	}

	// hash password
	hashedPassword, err := sec.HashPassword([]byte(body.Password.String))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: err.Error()})
	}

	var rawBodyGroups []byte

	if body.Groups != nil {
		rawBodyGroups, err = json.Marshal(body.Groups)
		if err != nil {
			return c.JSON(http.StatusBadGateway, apimodels.Error{Error: err.Error()})
		}
	}

	id, err := uuid.NewUUID()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: err.Error()})
	}

	ctx := utils.Context(c)

	result := registry.Reg.DB.WithContext(ctx).Create(
		&models.User{
			UserPure: models.UserPure{
				UserPrivate: models.UserPrivate{
					Password: string(hashedPassword),
				},
				UserData: models.UserData{
					Name: body.Name.String,
					Groups: apimodels.Groups{
						Groups: datatypes.JSON(rawBodyGroups),
					},
					Email: body.Email.String,
				},
			},
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

// @Summary Replace user
// @Tags user
// @Description Replace with new data, id or name must exist in request
// @Security ApiKeyAuth
// @Router /user [patch]
// @Param payload body UserPureID{} false "send part of the user object"
// @Success 200 {object} apimodels.Data{data=apimodels.ID{}}
// @failure 400 {object} apimodels.Error{}
// @failure 409 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func patchUser(c echo.Context) error {
	bodySend := map[string]interface{}{}
	body := models.UserRequest{}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: err.Error()})
	}

	if body.ID == "" {
		return c.JSON(http.StatusBadRequest, apimodels.Error{Error: "id is required and cannot be empty"})
	}

	// hash password
	if body.Password.Valid {
		hashedPassword, err := sec.HashPassword([]byte(body.Password.String))
		if err != nil {
			return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: err.Error()})
		}

		bodySend["password"] = hashedPassword
	}

	if body.Groups != nil {
		bodySend["groups"], _ = json.Marshal(body.Groups)
	}

	ctx := utils.Context(c)
	query := registry.Reg.DB.WithContext(ctx).Model(&models.User{}).Where("id = ?", body.ID)

	result := query.Updates(bodySend)

	// check write error
	if result.Error != nil && errors.Is(result.Error, gorm.ErrDuplicatedKey) {
		return c.JSON(http.StatusConflict, apimodels.Error{Error: result.Error.Error()})
	}

	if result.Error != nil {
		return c.JSON(http.StatusInternalServerError, apimodels.Error{Error: result.Error.Error()})
	}

	return c.JSON(http.StatusOK,
		apimodels.Data{
			Data: map[string]interface{}{
				"id": body.ID,
			},
		},
	)
}

func User(e *echo.Group, authMiddleware echo.MiddlewareFunc) {
	e.GET("/users", listUsers, authMiddleware, middlewares.AdminRole, middlewares.PatToken)
	e.GET("/user", getUser, authMiddleware, middlewares.JWTCheck(middlewares.IDFromQuery), middlewares.AdminRole, middlewares.PatToken)
	e.POST("/user", postUser, authMiddleware, middlewares.AdminRole, middlewares.PatToken)
	e.PATCH("/user", patchUser, authMiddleware, middlewares.JWTCheck(middlewares.IDFromBody), middlewares.AdminRole, middlewares.PatToken)
	e.DELETE("/user", deleteUser, authMiddleware, middlewares.JWTCheck(middlewares.IDFromQuery), middlewares.AdminRole, middlewares.PatToken)
}
