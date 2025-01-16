package utils

import (
	"strings"

	"github.com/google/uuid"

	"github.com/worldline-go/chore/pkg/models"
	"github.com/worldline-go/chore/pkg/models/apimodels"
)

func FolderFile(fileName string) []models.Folder {
	fileName = strings.Trim(fileName, "/")
	if fileName == "" {
		return nil
	}

	keyArray := strings.SplitAfter(fileName, "/")

	items := make([]models.Folder, 0, len(keyArray))

	for i := len(keyArray) - 1; i >= 0; i-- {
		key := ""
		if i > 0 {
			key = strings.Join(keyArray[:i], "")
		}

		items = append(items, models.Folder{
			FolderPure: models.FolderPure{
				Folder: key,
				Item:   keyArray[i],
				Name:   key + keyArray[i],
				Dtype:  keyArray[i][len(keyArray[i])-1] == '/',
			},
			ID: apimodels.ID{ID: uuid.New()},
		})
	}

	return items
}
