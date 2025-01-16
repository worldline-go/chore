package apitest

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"

	"github.com/worldline-go/chore/internal/utils"
	"github.com/worldline-go/chore/pkg/models"
	"github.com/worldline-go/chore/pkg/models/apimodels"
	"github.com/worldline-go/chore/pkg/registry"
)

// @Summary New Test
// @Tags test
// @Description Send and record new user
// @Router /test [post]
// @Param payload body models.TestPure{} false "send test object"
// @Success 200 {object} apimodels.Data{data=apimodels.ID{}}
// @failure 400 {object} apimodels.Error{}
// @failure 409 {object} apimodels.Error{}
// @failure 500 {object} apimodels.Error{}
func postTest(c echo.Context) error {
	body := new(models.TestPure)
	if err := c.Bind(body); err != nil {
		return c.JSON(
			http.StatusBadRequest,
			apimodels.Error{
				Error: err.Error(),
			},
		)
	}

	if body.Name == "" {
		return c.JSON(
			http.StatusBadRequest,
			apimodels.Error{
				Error: "name is required",
			},
		)
	}

	tx := registry.Reg.DB.Begin()

	test := models.Test{
		TestPure: models.TestPure{
			Name: body.Name,
		},
	}

	ctx := utils.Context(c)
	result := tx.WithContext(ctx).Create(&test)

	if result.Error != nil {
		tx.Rollback()
	}

	log.Debug().Msgf("TEST-ID: %v", test.ID)

	// check write error
	if result.Error != nil && errors.Is(result.Error, gorm.ErrDuplicatedKey) {
		return c.JSON(
			http.StatusConflict,
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

	tx.Commit()

	// return recorded data's id
	return c.JSON(
		http.StatusOK,
		apimodels.Data{
			Data: apimodels.ID{ID: uuid.Nil},
		},
	)
}

func Test(e *echo.Group) {
	e.POST("/test", postTest)
}
