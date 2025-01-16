package models

import (
	"github.com/worldline-go/chore/pkg/models/apimodels"
	"gorm.io/datatypes"
)

type Email struct {
	Host     string `json:"host"`
	Email    string `json:"email"`
	Port     string `json:"port"`
	NoAuth   bool   `json:"no_auth"`
	Password string `json:"password"`
}

type SettingsPure struct {
	Name      string            `json:"name" gorm:"uniqueIndex:idx_name_namespace" example:"email-1"`
	Namespace string            `json:"namespace" gorm:"uniqueIndex:idx_name_namespace;not null" example:"email"`
	Data      datatypes.JSONMap `json:"data" swaggertype:"object,string"`
}

type Settings struct {
	apimodels.ModelCUPure
	SettingsPure
}
