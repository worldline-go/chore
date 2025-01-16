package models

import (
	"github.com/worldline-go/chore/pkg/models/apimodels"
	"gopkg.in/guregu/null.v4"
)

type UserRequest struct {
	Name     null.String `json:"name"`
	Email    null.String `json:"email"`
	Groups   []string    `json:"groups"`
	Password null.String `json:"password"`
	ID       string      `json:"id"`
}

type UserData struct {
	Name  string `json:"name" gorm:"unique;uniqueIndex;not null" example:"userX"`
	Email string `json:"email" example:"userx@example.com"`
	apimodels.Groups
}

type UserPrivate struct {
	Password string `json:"password" gorm:"not null" example:"pass1234"`
}

type UserPure struct {
	UserPrivate
	UserData
}

type User struct {
	UserPure
	apimodels.ModelCU
}
