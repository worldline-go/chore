package middlewares

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/worldline-go/chore/internal/server/claims"
	"github.com/worldline-go/chore/pkg/models"
	"github.com/worldline-go/chore/pkg/registry"
	"gorm.io/gorm"
)

func PatTokenExist(c echo.Context, claim *claims.Custom) error {
	if claim.TokenType != models.TypePersonalAccessToken {
		return nil
	}

	if claim.TokenID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "token id not found")
	}

	var count int64
	query := registry.Reg.DB.WithContext(c.Request().Context()).
		Model(&models.Token{}).Where("id = ?", claim.TokenID)

	result := query.Count(&count)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) || count == 0 {
		return echo.NewHTTPError(http.StatusUnauthorized, "pat token not exist")
	}
	if result.Error != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, result.Error.Error())
	}

	return nil
}
