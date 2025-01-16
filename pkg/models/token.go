package models

import (
	"time"

	"github.com/google/uuid"

	"github.com/worldline-go/chore/pkg/models/apimodels"
)

var (
	TypePersonalAccessToken = "pat"
	TypeAccessToken         = "login"
)

type TokenData struct {
	Name string     `json:"name" example:"mytoken" gorm:"not null;default:null"`
	Date *time.Time `json:"date" example:"2021-02-18T21:54:42.123Z"`
	apimodels.Groups
}

type TokenDataBy struct {
	TokenData
	CreatedBy uuid.UUID `json:"createdby" gorm:"not null;type:uuid" example:"cf8a07d4-077e-402e-a46b-ac0ed50989ec"`
}

type TokenPure struct {
	TokenPrivate
	TokenDataBy
}

type TokenPrivate struct {
	Token string `json:"token" gorm:"primarykey" example:"tokenJWT"`
}

type Token struct {
	apimodels.ModelC
	TokenPure
}
