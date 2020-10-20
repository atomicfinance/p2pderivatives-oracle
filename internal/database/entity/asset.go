package entity

import (
	"fmt"
	"github.com/jinzhu/gorm"
)

// Asset represents an asset currency
type Asset struct {
	Base
	AssetID     string `gorm:"primarykey" gorm:"uniqueIndex"`
	Description string
	DLCData     []DLCData `gorm:"foreignkey:AssetID"`
}

// FindAsset will try to find in the db the asset corresponding to the id
func FindAsset(db *gorm.DB, assetID string) (*Asset, error) {
	fmt.Println("assetId find asset", assetID)
	filterCondition := &Asset{
		AssetID: assetID,
	}
	fmt.Println("filterCondition", filterCondition)
	// existingAsset := &Asset{AssetID: assetID}
	err := db.Where("asset_id = ?", assetID).First(filterCondition).Error
	return filterCondition, err
}
