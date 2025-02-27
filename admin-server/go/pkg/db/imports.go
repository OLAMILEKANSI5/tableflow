package db

import (
	"errors"
	"gorm.io/gorm"
	"tableflow/go/pkg/model"
	"tableflow/go/pkg/tf"
)

func GetImport(id string) (*model.Import, error) {
	if len(id) == 0 {
		return nil, errors.New("no import ID provided")
	}
	var imp model.Import
	err := tf.DB.First(&imp, model.ParseID(id)).Error
	if err != nil {
		return nil, err
	}
	if !imp.ID.Valid {
		return nil, gorm.ErrRecordNotFound
	}
	return &imp, nil
}

func GetImportForAdminAPI(id string) (*model.Import, error) {
	if len(id) == 0 {
		return nil, errors.New("no import ID provided")
	}
	var imp model.Import
	err := tf.DB.Omit("StorageBucket").
		First(&imp, model.ParseID(id)).Error
	if err != nil {
		return nil, err
	}
	if !imp.ID.Valid {
		return nil, gorm.ErrRecordNotFound
	}
	return &imp, nil
}

func GetImportsForAdminAPI(workspaceID string) ([]*model.Import, error) {
	if len(workspaceID) == 0 {
		return nil, errors.New("no workspace ID provided")
	}
	var imports []*model.Import
	err := tf.DB.Preload("Importer").
		Omit("StorageBucket").
		Where("workspace_id = ?", model.ParseID(workspaceID)).
		Order("created_at desc").
		Find(&imports).Error
	if err != nil {
		return nil, err
	}
	return imports, nil
}
