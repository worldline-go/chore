package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/worldline-go/chore/internal/parser"
	"github.com/worldline-go/chore/internal/server/middleware"
	"github.com/worldline-go/chore/models"
	"github.com/worldline-go/chore/models/apimodels"
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
// @Param search query string string "search item"
// @Success 200 {object} apimodels.DataMeta{data=[]AuthPureID{},meta=apimodels.Meta{}}
// @failure 400 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func listAuths(c *fiber.Ctx) error {
	auths := []AuthPureID{}

	meta := &apimodels.Meta{Limit: apimodels.Limit}

	if err := c.QueryParser(meta); err != nil {
		return c.Status(http.StatusBadRequest).JSON(
			apimodels.Error{
				Error: err.Error(),
			},
		)
	}

	reg := registry.Reg().Get(c.Locals("registry").(string))
	query := reg.DB.WithContext(c.UserContext()).Model(&models.Auth{}).Limit(meta.Limit).Offset(meta.Offset)

	if meta.Search != "" {
		query = query.Where("name LIKE ?", meta.Search+"%")
	}

	result := query.Find(&auths)

	// check write error
	if result.Error != nil {
		return c.Status(http.StatusInternalServerError).JSON(
			apimodels.Error{
				Error: result.Error.Error(),
			},
		)
	}

	// get counts
	query = reg.DB.WithContext(c.UserContext()).Model(&models.Auth{})
	if meta.Search != "" {
		query = query.Where("name LIKE ?", meta.Search+"%")
	}

	query.Count(&meta.Count)

	return c.Status(http.StatusOK).JSON(
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
func getAuth(c *fiber.Ctx) error {
	id := c.Query("id")
	name := c.Query("name")

	if id == "" && name == "" {
		return c.Status(http.StatusBadRequest).JSON(
			apimodels.Error{
				Error: apimodels.ErrRequiredIDName.Error(),
			},
		)
	}

	dump, err := parser.GetQueryBool(c, "dump")
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(
			apimodels.Error{
				Error: err.Error(),
			},
		)
	}

	pretty, err := parser.GetQueryBool(c, "pretty")
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(
			apimodels.Error{
				Error: err.Error(),
			},
		)
	}

	getData := new(AuthPureID)

	reg := registry.Reg().Get(c.Locals("registry").(string))

	query := reg.DB.WithContext(c.UserContext()).Model(&models.Auth{})
	if id != "" {
		query = query.Where("id = ?", id)
	}

	if name != "" {
		query = query.Where("name = ?", name)
	}

	result := query.First(&getData)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return c.Status(http.StatusNotFound).JSON(
			apimodels.Error{
				Error: result.Error.Error(),
			},
		)
	}

	if result.Error != nil {
		return c.Status(http.StatusInternalServerError).JSON(
			apimodels.Error{
				Error: result.Error.Error(),
			},
		)
	}

	if dump {
		return parser.JSON(c.Status(http.StatusOK), getData, pretty)
	}

	return c.Status(http.StatusOK).JSON(
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
// @Param payload body models.AuthPure{} false "send auth object"
// @Success 204 "No Content"
// @failure 400 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func putAuth(c *fiber.Ctx) error {
	var body models.AuthPure
	if err := c.BodyParser(&body); err != nil {
		return c.Status(http.StatusBadRequest).JSON(
			apimodels.Error{
				Error: err.Error(),
			},
		)
	}

	if body.Name == "" {
		return c.Status(http.StatusBadRequest).JSON(
			apimodels.Error{
				Error: apimodels.ErrRequiredName.Error(),
			},
		)
	}

	reg := registry.Reg().Get(c.Locals("registry").(string))

	id, err := uuid.NewUUID()
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(
			apimodels.Error{
				Error: err.Error(),
			},
		)
	}

	result := reg.DB.WithContext(c.UserContext()).Model(&models.Auth{}).Clauses(
		clause.OnConflict{
			UpdateAll: true,
			Columns:   []clause.Column{{Name: "name"}},
		}).Create(
		&models.Auth{
			AuthPure: body,
			ModelCU: apimodels.ModelCU{
				ID: apimodels.ID{ID: id},
			},
		},
	)

	if result.Error != nil {
		return c.Status(http.StatusInternalServerError).JSON(
			apimodels.Error{
				Error: result.Error.Error(),
			},
		)
	}

	//nolint:wrapcheck // checking before
	return c.SendStatus(http.StatusNoContent)
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
func postAuth(c *fiber.Ctx) error {
	var body models.AuthPure
	if err := c.BodyParser(&body); err != nil {
		return c.Status(http.StatusBadRequest).JSON(
			apimodels.Error{
				Error: err.Error(),
			},
		)
	}

	if body.Name == "" {
		return c.Status(http.StatusBadRequest).JSON(
			apimodels.Error{
				Error: apimodels.ErrRequiredName.Error(),
			},
		)
	}

	reg := registry.Reg().Get(c.Locals("registry").(string))

	id, err := uuid.NewUUID()
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(
			apimodels.Error{
				Error: err.Error(),
			},
		)
	}

	result := reg.DB.WithContext(c.UserContext()).Model(&models.Auth{}).Create(
		&models.Auth{
			AuthPure: body,
			ModelCU: apimodels.ModelCU{
				ID: apimodels.ID{ID: id},
			},
		},
	)

	// check write error
	var pErr *pgconn.PgError

	errors.As(result.Error, &pErr)

	if pErr != nil && pErr.Code == "23505" {
		return c.Status(http.StatusConflict).JSON(
			apimodels.Error{
				Error: result.Error.Error(),
			},
		)
	}

	if result.Error != nil {
		return c.Status(http.StatusInternalServerError).JSON(
			apimodels.Error{
				Error: result.Error.Error(),
			},
		)
	}

	// return recorded data's id
	return c.Status(http.StatusOK).JSON(
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
func patchAuth(c *fiber.Ctx) error {
	var body map[string]interface{}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(http.StatusBadRequest).JSON(
			apimodels.Error{
				Error: err.Error(),
			},
		)
	}

	if v, ok := body["id"].(string); !ok || v == "" {
		return c.Status(http.StatusBadRequest).JSON(
			apimodels.Error{
				Error: "id is required and cannot be empty",
			},
		)
	}

	if body["groups"] != nil {
		var err error

		body["groups"], err = json.Marshal(body["groups"])
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(
				apimodels.Error{
					Error: err.Error(),
				},
			)
		}
	}

	reg := registry.Reg().Get(c.Locals("registry").(string))

	query := reg.DB.WithContext(c.UserContext()).Model(&models.Auth{}).Where("id = ?", body["id"])

	result := query.Updates(body)

	// check write error
	var pErr *pgconn.PgError

	errors.As(result.Error, &pErr)

	if pErr != nil && pErr.Code == "23505" {
		return c.Status(http.StatusConflict).JSON(
			apimodels.Error{
				Error: result.Error.Error(),
			},
		)
	}

	if result.Error != nil {
		return c.Status(http.StatusInternalServerError).JSON(
			apimodels.Error{
				Error: result.Error.Error(),
			},
		)
	}

	resultData := make(map[string]interface{})
	resultData["id"] = body["id"]

	return c.Status(http.StatusOK).JSON(
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
func deleteAuth(c *fiber.Ctx) error {
	id := c.Query("id")

	if id == "" {
		return c.Status(http.StatusBadRequest).JSON(
			apimodels.Error{
				Error: apimodels.ErrRequiredID.Error(),
			},
		)
	}

	reg := registry.Reg().Get(c.Locals("registry").(string))

	query := reg.DB.WithContext(c.UserContext()).Where("id = ?", id)

	// delete directly in DB
	result := query.Unscoped().Delete(&models.Auth{})

	if result.RowsAffected == 0 {
		return c.Status(http.StatusNotFound).JSON(
			apimodels.Error{
				Error: "not found any releated data",
			},
		)
	}

	if result.Error != nil {
		return c.Status(http.StatusInternalServerError).JSON(
			apimodels.Error{
				Error: result.Error.Error(),
			},
		)
	}

	//nolint:wrapcheck // checking before
	return c.SendStatus(http.StatusNoContent)
}

func Auth(router fiber.Router) {
	router.Get("/auths", middleware.JWTCheck(nil, nil), listAuths)
	router.Get("/auth", middleware.JWTCheck(nil, nil), getAuth)
	router.Post("/auth", middleware.JWTCheck(nil, nil), postAuth)
	router.Put("/auth", middleware.JWTCheck(nil, nil), putAuth)
	router.Patch("/auth", middleware.JWTCheck(nil, nil), patchAuth)
	router.Delete("/auth", middleware.JWTCheck(nil, nil), deleteAuth)
}
