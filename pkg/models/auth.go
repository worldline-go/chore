package models

import (
	"github.com/worldline-go/chore/pkg/models/apimodels"

	"gorm.io/datatypes"
)

type AuthPure struct {
	Name    string            `json:"name" gorm:"unique;uniqueIndex;not null" example:"jira-deepcore"`
	Headers datatypes.JSONMap `json:"headers" swaggertype:"object,string" example:"Content-Type:application/json"`
	Data    string            `json:"data" example:"any data"`
	apimodels.Groups
}

type Auth struct {
	AuthPure
	apimodels.ModelCU
}
