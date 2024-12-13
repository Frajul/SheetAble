package models

import (
	"errors"
	"strings"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/rs/xid"
)

type Composer struct {
	Uuid        string    `gorm:"primary_key" json:"uuid"`
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
	err = db.Save(&composer).Error
	return err
}

// Updates only columns with non nil
func (composer *Composer) UpdateAtDb(db *gorm.DB) error {
	err := db.Model(&composer).Updates(&composer).Error
	return err
}

func DeleteComposer(db *gorm.DB, uuid string) error {
	err := db.Where("uuid = ?", uuid).Delete(&Composer{}).Error

	if err != nil {
		if gorm.IsRecordNotFoundError(err) {
			return errors.New("Composer not found")
		}
		return err
	}

	// Swap sheets composer to Unknown
	unknownUuid, err := assureUnknownComposerExists(db)
	if err != nil {
		return err
	}
	err = db.Exec("UPDATE sheets SET composer_uuid = ? WHERE composer_uuid = ?", unknownUuid, uuid).Error
	if err != nil {
		return err
	}
	return nil
}

// Returns uuid of the unknown composer
func assureUnknownComposerExists(db *gorm.DB) (string, error) {
	composer, err := FindComposerByNameCaseInsensitive(db, "Unknown")
	if err == nil {
		return composer.Uuid, nil // Unknown exists
	}
	if !gorm.IsRecordNotFoundError(err) {
		return "", err // error
	}

	composerUuid, err := GenerateNonexistentComposerUuid(db)
	if err == nil {
		return composerUuid, nil
	}

	composer = NewComposer(composerUuid, "Unknown", "Unknown", "https://icon-library.com/images/unknown-person-icon/unknown-person-icon-4.jpg")
	err = composer.SaveToDb(db)
	return composerUuid, err
}

func IsComposerUnreferenced(db *gorm.DB, uuid string) (bool, error) {
	var sheets []Sheet

	result := db.Model(&Sheet{}).Where("composer_uuid = ?", uuid).Find(&sheets)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected <= 1, nil
}

func SearchComposers(db *gorm.DB, searchValue string) ([]*Composer, error) {
	var composers []*Composer
	searchValue = "%" + searchValue + "%"
	err := db.Where("name LIKE ?", searchValue).Find(&composers).Error
	return composers, err
}

func ListComposersPaginated(db *gorm.DB, pagination Pagination) (*Pagination, error) {
	var composers []*Composer
	err := db.Scopes(paginate(composers, &pagination, db)).Find(&composers).Error
	if err != nil {
		return &Pagination{}, nil
	}
	pagination.Rows = composers

	return &pagination, nil
}

func ListComposers(db *gorm.DB) ([]*Composer, error) {
	var composers []*Composer
	err := db.Find(&composers).Error
	return composers, err
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
	err := db.First(&composer, "uuid = ?", uuid).Error
	if err != nil {
		return &Composer{}, err
	}
	return &composer, nil
}

func FindComposerByNameCaseInsensitive(db *gorm.DB, name string) (*Composer, error) {
	var composer Composer
	err := db.First(&composer, "LOWER(name) LIKE LOWER(?)", name).Error
	if err != nil {
		return &Composer{}, err
	}
	return &composer, nil
}

func GenerateNonexistentComposerUuid(db *gorm.DB) (string, error) {
	for i := 0; i < 10; i++ {
		uuid := xid.New().String()
		exists, err := ExistsComposer(db, uuid)
		if err != nil {
			return "", err
		}

		if !exists {
			return uuid, nil
		}
	}
	return "", errors.New("Somehow unable to generate new uuid for composer.")
}
