package models

import (
	"errors"
	"os"
	"path"
	"strings"
	"time"

	. "github.com/SheetAble/SheetAble/backend/api/config"
	"github.com/SheetAble/SheetAble/backend/api/utils"
	"github.com/lib/pq"
	"github.com/rs/xid"

	"github.com/jinzhu/gorm"
)

type Sheet struct {
	Uuid            string         `gorm:"primary_key" json:"uuid"`
	Name            string         `json:"sheet_name"`
	ComposerUuid    string         `json:"composer_uuid"`
	ReleaseDate     time.Time      `json:"release_date"`
	File            string         `json:"file"`
	FileHash        string         `json:"file_hash"`
	WasUploaded     bool           `json:"was_uploaded"`
	UploaderID      uint32         `gorm:"not null" json:"uploader_id"`
	CreatedAt       time.Time      `gorm:"default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt       time.Time      `gorm:"default:CURRENT_TIMESTAMP" json:"updated_at"`
	LastOpened      time.Time      `gorm:"default:CURRENT_TIMESTAMP" json:"last_opened"`
	Tags            pq.StringArray `gorm:"type:text[]" json:"tags"`
	InformationText string         `json:"information_text"`
}

type SimpleSheet struct {
	Uuid         string `json:"uuid"`
	Name         string `json:"sheet_name"`
	ComposerUuid string `json:"composer_uuid"`
	ComposerName string `json:"composer_name"`
}

func NewSheet(uuid string, name string, composerUuid string, file string, wasUploaded bool) *Sheet {
	return &Sheet{
		Uuid:            strings.TrimSpace(uuid),
		Name:            strings.TrimSpace(name),
		ComposerUuid:    strings.TrimSpace(composerUuid),
		File:            strings.TrimSpace(file),
		WasUploaded:     wasUploaded,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		LastOpened:      time.Now(),
		Tags:            pq.StringArray{},
		ReleaseDate:     createDate("1999-12-31"),
		InformationText: "",
	}
}

func createDate(date string) time.Time {
	// Create a usable date
	const layoutISO = "2006-01-02"
	t, _ := time.Parse(layoutISO, date)
	return t
}

func (sheet *Sheet) SaveToDb(db *gorm.DB) error {
	exists, err := ExistsSheet(db, sheet.Uuid)
	if err != nil {
		return err
	}
	if exists {
		return errors.New("Sheet with this uuid already exists")
	}
	result := db.Save(&sheet)
	return result.Error
}

// Updates only columns with non nil
func (sheet *Sheet) UpdateAtDb(db *gorm.DB) error {
	result := db.Model(&sheet).Updates(&sheet)
	return result.Error
}

func DeleteSheet(db *gorm.DB, uuid string) error {
	var sheet Sheet
	result := db.Where("uuid = ?", uuid).Delete(&sheet)

	if result.Error != nil {
		if gorm.IsRecordNotFoundError(result.Error) {
			return errors.New("Sheet not found")
		}
		return result.Error
	}

	path := path.Join(Config().ConfigPath, "sheets/thumbnails", uuid+".png")
	e := os.Remove(path)
	if e != nil {
		return e
	}
	// Do not remove local-file
	if sheet.WasUploaded {
		path := sheet.File
		e := os.Remove(path)
		if e != nil {
			return e
		}
	}

	// Delete composer if not referenced anymore
	isUnreferenced, err := IsComposerUnreferenced(db, sheet.ComposerUuid)
	if err != nil {
		return err
	}
	if isUnreferenced {
		err = DeleteComposer(db, sheet.ComposerUuid)
		if err != nil {
			return err
		}
	}

	return nil
}

func SearchSheets(db *gorm.DB, searchValue string) ([]*Sheet, error) {
	var sheets []*Sheet
	searchValue = "%" + searchValue + "%"
	result := db.Where("name LIKE ?", searchValue).Find(&sheets)
	return sheets, result.Error
}

func ExistsSheet(db *gorm.DB, uuid string) (bool, error) {
	_, err := FindSheetByUuid(db, uuid)
	if err == nil {
		return true, nil
	}
	if gorm.IsRecordNotFoundError(err) {
		return false, nil
	}
	return false, err
}

func FindSheetByUuid(db *gorm.DB, uuid string) (*Sheet, error) {
	var sheet Sheet
	result := db.First(&sheet, "uuid = ?", uuid)
	if result.Error != nil {
		return &Sheet{}, result.Error
	}
	return &sheet, nil
}

func ListSheets(db *gorm.DB, pagination Pagination, composerUuid string) (*Pagination, error) {
	var sheets []*Sheet
	if composerUuid != "" {
		result := db.Scopes(composerEqual(composerUuid), paginate(sheets, &pagination, db)).Find(&sheets)
		if result.Error != nil {
			return &Pagination{}, result.Error
		}
	} else {
		result := db.Scopes(paginate(sheets, &pagination, db)).Find(&sheets)
		if result.Error != nil {
			return &Pagination{}, result.Error
		}
	}
	pagination.Rows = sheets

	return &pagination, nil
}

func ListSimpleSheets(db *gorm.DB) ([]*SimpleSheet, error) {
	var sheets []*SimpleSheet
	result := db.Model(&Sheet{}).Select("sheets.uuid as uuid, sheets.name as name, sheets.composer_uuid as composer_uuid, composers.name as composer_name").Joins("left join composers on composers.uuid = sheets.composer_uuid").Order("sheets.last_opened desc").Scan(&sheets)
	return sheets, result.Error
}

func composerEqual(composerUuid string) func(db *gorm.DB) *gorm.DB {
	// Scope that composer is equal to composer (if you only want sheets from a certain composer)
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("composer_uuid = ?", composerUuid)
	}
}

func (s *Sheet) AppendTag(db *gorm.DB, tag string) error {
	newArray := append(s.Tags, tag)

	err := db.Model(&s).Update(Sheet{Tags: newArray}).Error
	return err
}

func SetSheetLastOpenedToNow(db *gorm.DB, sheetUuid string) error {
	err := db.Model(Sheet{Uuid: sheetUuid}).Update(Sheet{LastOpened: time.Now()}).Error
	return err
}

func (s *Sheet) DeleteTag(db *gorm.DB, tag string) error {
	// Deleting a tag by it's value
	index := utils.FindIndexByValue(s.Tags, tag)

	if index == -1 {
		return errors.New("Given tag was not in tag list")
	}

	newArray := pq.StringArray(utils.RemoveElementOfSlice(s.Tags, index))
	err := db.Model(&s).Update(Sheet{Tags: newArray}).Error

	return err
}

func (s *Sheet) UpdateSheetInformationText(db *gorm.DB, value string) error {
	s.InformationText = value
	err := db.Model(s).Updates(s).Error
	return err
}

func FindSheetByTag(db *gorm.DB, tag string) ([]*Sheet, error) {
	var allSheets []*Sheet
	var affectedSheets []*Sheet

	// TODO: improve by using db native search
	result := db.Find(&allSheets)
	if result.Error != nil {
		return affectedSheets, result.Error
	}

	for _, sheet := range allSheets {
		if utils.CheckSliceContains(sheet.Tags, tag) {
			affectedSheets = append(affectedSheets, sheet)
		}
	}

	return affectedSheets, nil
}

func GenerateNonexistentSheetUuid(db *gorm.DB) (string, error) {
	for i := 0; i < 10; i++ {
		uuid := xid.New().String()
		exists, err := ExistsSheet(db, uuid)
		if err != nil {
			return "", err
		}

		if !exists {
			return uuid, nil
		}
	}
	return "", errors.New("Somehow unable to generate new uuid for sheet.")
}
