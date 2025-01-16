package models

import (
	"gorm.io/datatypes"

	"github.com/worldline-go/chore/pkg/models/apimodels"
)

// ControlEndpoint is representation of Endpoints json object.
type ControlEndpoint struct {
	Methods []string `json:"methods"`
	Public  bool     `json:"public"`
}

type ControlPure struct {
	Name string `json:"name" gorm:"unique;uniqueIndex;not null"`
	Endpoints
	apimodels.Groups
}

type Endpoints struct {
	Endpoints datatypes.JSON `json:"endpoints" swaggertype:"object,string"`
}

type ControlPureContent struct {
	Content string `json:"content" swaggertype:"string" format:"base64" example:"aGVsbG8ge3submFtZX19Cg=="`
	ControlPure
}

type Control struct {
	ControlPureContent
	apimodels.ModelCU
}

type ControlClone struct {
	Name    string `json:"name" swaggertype:"string"`
	NewName string `json:"new_name" swaggertype:"string"`
}
