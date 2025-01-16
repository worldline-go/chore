package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/worldline-go/chore/internal/config"
	"github.com/worldline-go/chore/pkg/models"
	"github.com/worldline-go/chore/pkg/sec"
)

func AutoMigrate(ctx context.Context, dbConn *gorm.DB) error {
	// check migrate exists
	var err error

	dbConnMigrate := dbConn
	if config.Application.Migrate != (config.Store{}) {
		// open db connection for migration
		dbConnMigrate, err = OpenConnection(choiceExist(config.Application.Migrate.Type, config.Application.Store.Type), map[string]interface{}{
			"host":     choiceExist(config.Application.Migrate.Host, config.Application.Store.Host),
			"port":     choiceExist(config.Application.Migrate.Port, config.Application.Store.Port),
			"password": choiceExist(config.Application.Migrate.Password, config.Application.Store.Password),
			"user":     choiceExist(config.Application.Migrate.User, config.Application.Store.User),
			"dbName":   choiceExist(config.Application.Migrate.DBName, config.Application.Store.DBName),
			"schema":   choiceExist(config.Application.Migrate.Schema, config.Application.Store.Schema),
			"timeZone": choiceExist(config.Application.Migrate.TimeZone, config.Application.Store.TimeZone),
			"dsn":      choiceExist(config.Application.Migrate.DBDataSource, config.Application.Store.DBDataSource),
		})
		if err != nil {
			return fmt.Errorf("cannot open db: %w", err)
		}

		db, err := dbConnMigrate.DB()
		if err != nil {
			return fmt.Errorf("cannot get db: %w", err)
		}

		defer db.Close()
	}

	if err := dbConnMigrate.AutoMigrate(Models...); err != nil {
		return fmt.Errorf("auto migrate failed: %w", err)
	}

	if err := initializeAdminGroup(ctx, dbConnMigrate); err != nil {
		return err
	}

	if err := initializeAdminUser(ctx, dbConnMigrate); err != nil {
		return err
	}

	return nil
}

// if v1 not empty return v1 else v2
func choiceExist(v1, v2 string) string {
	if v1 == "" {
		return v2
	}

	return v1
}

func initializeAdminGroup(ctx context.Context, dbConn *gorm.DB) error {
	// initialize admin group
	groupAdmin := models.Group{}
	result := dbConn.WithContext(ctx).Where("name = ?", "admin").Find(&groupAdmin)

	if result.RowsAffected != 0 {
		return nil
	}

	groupAdmin.Name = "admin"
	groupAdmin.ID.ID = uuid.New()

	result = dbConn.WithContext(ctx).Create(&groupAdmin)

	return result.Error
}

func initializeAdminUser(ctx context.Context, dbConn *gorm.DB) error {
	// first admin initialize if not exist
	userOne := models.User{}
	result := dbConn.WithContext(ctx).Limit(1).Find(&userOne)

	if result.RowsAffected != 0 {
		return nil
	}

	addAdmin := models.User{}

	addAdmin.Groups.Groups = datatypes.JSON(`["admin"]`)

	addAdmin.ID.ID = uuid.New()

	addAdmin.Name = config.Application.User.Name

	hashPass, err := sec.HashPassword([]byte(config.Application.User.Password))
	if err != nil {
		return err
	}

	addAdmin.Password = string(hashPass)

	result = dbConn.WithContext(ctx).Create(&addAdmin)

	return result.Error
}
