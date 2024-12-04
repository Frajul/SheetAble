package models

import (
	"errors"
	"strings"
	"time"

	"github.com/jinzhu/gorm"
)

type Composer struct {
	Uuid        string    `gorm:"primary_key"`
	Name        string    `json:"name"`
	PortraitUrl string    `json:"portrait_url"`
	Epoch       string    `json:"epoch"`
	CreatedAt   time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt   time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updated_at"`
}

func NewComposer(uuid string, name string, portraitUrl string, epoch string) *Composer {
	return &Composer{
		Uuid:        strings.TrimSpace(uuid),
		Name:        strings.TrimSpace(name),
		PortraitUrl: strings.TrimSpace(portraitUrl),
		Epoch:       strings.TrimSpace(epoch),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

func (composer *Composer) SaveToDb(db *gorm.DB) error {
	exists, err := ExistsComposer(db, composer.Uuid)
	if err != nil {
		return err
	}
	if exists {
		return errors.New("Composer with this uuid already exists")
	}
	db.Save(&composer)
	return db.Error
}

func (composer *Composer) UpdateAtDb(db *gorm.DB) error {
	db.Save(&composer)
	return db.Error
}

func DeleteComposer(db *gorm.DB, uuid string) error {
	db.Where("uuid = ?", uuid).Delete(&Composer{})

	if db.Error != nil {
		if gorm.IsRecordNotFoundError(db.Error) {
			return errors.New("Composer not found")
		}
		return db.Error
	}

	// Swap sheets composer to Unknown
	err := assureUnknownComposerExists(db)
	if err != nil {
		return err
	}
	db.Exec("UPDATE 'sheets' SET 'composer' = '' WHERE (composer = ?);", uuid)
	if db.Error != nil {
		return db.Error
	}
	return nil
}

func assureUnknownComposerExists(db *gorm.DB) error {
	unknownComposerExists, err := ExistsComposer(db, "")
	if err != nil {
		return err
	}
	if !unknownComposerExists {
		composer := NewComposer("", "Unknown", "Unknown", "https://icon-library.com/images/unknown-person-icon/unknown-person-icon-4.jpg")
		err = composer.SaveToDb(db)
		return err
	}
	return nil
}

func IsComposerUnreferenced(db *gorm.DB, uuid string) (bool, error) {
	var sheets []Sheet

	result := db.Model(&Sheet{}).Where("composer_uuid = ?", uuid).Find(&sheets)
	if db.Error != nil {
		return false, db.Error
	}
	return result.RowsAffected <= 1, nil
}

func SearchComposers(db *gorm.DB, searchValue string) ([]*Composer, error) {
	var composers []*Composer
	searchValue = "%" + searchValue + "%"
	db.Where("name LIKE ?", searchValue).Find(&composers)
	return composers, db.Error
}

func ListComposers(db *gorm.DB, pagination Pagination) (*Pagination, error) {
	var composers []*Composer
	db.Scopes(paginate(composers, &pagination, db)).Find(&composers)
	if db.Error != nil {
		return &Pagination{}, nil
	}
	pagination.Rows = composers

	return &pagination, nil
}

func ExistsComposer(db *gorm.DB, uuid string) (bool, error) {
	_, err := FindComposerByUuid(db, uuid)
	if err == nil {
		return true, nil
	}
	if gorm.IsRecordNotFoundError(err) {
		return false, nil
	}
	return false, err
}

func FindComposerByUuid(db *gorm.DB, uuid string) (*Composer, error) {
	var composer Composer
	db.First(&composer, "uuid = ?", uuid)
	if db.Error != nil {
		return &Composer{}, db.Error
	}
	return &composer, nil
}
