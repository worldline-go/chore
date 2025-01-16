package store

import (
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/worldline-go/chore/internal/store/db"
)

func OpenConnection(typeDB string, cfg map[string]interface{}) (*gorm.DB, error) {
	switch strings.ToLower(typeDB) { //nolint:gocritic // newer types in future
	case "postgres":
		gormDB, err := db.PostgresDB(cfg)
		if err != nil {
			return nil, fmt.Errorf("cannot open conneection to db; %w", err)
		}

		return gormDB, nil
	}

	return nil, fmt.Errorf("%s not implemented", typeDB)
}
