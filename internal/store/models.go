package store

import "github.com/worldline-go/chore/pkg/models"

// Models using for migrate database tables.
var Models = []interface{}{
	&models.Auth{},
	&models.Template{},
	&models.Folder{},
	&models.Group{},
	&models.User{},
	&models.Token{},
	&models.Control{},
	&models.Settings{},
	// &models.Test{},
}
