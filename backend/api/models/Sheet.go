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

	"github.com/jinzhu/gorm"
)

type Sheet struct {
	Uuid            string         `gorm:"primary_key"`
	Name            string         `json:"sheet_name"`
	ComposerUuid    string         `json:"composer_uuid"`
	ReleaseDate     time.Time      `json:"release_date"`
	File            string         `json:"file"`
	FileHash        string         `json:"file_hash"`
	WasUploaded     bool           `json:"was_uploaded"`
	UploaderID      uint32         `gorm:"not null" json:"uploader_id"`
	CreatedAt       time.Time      `gorm:"default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt       time.Time      `gorm:"default:CURRENT_TIMESTAMP" json:"updated_at"`
	Tags            pq.StringArray `gorm:"type:text[]" json:"tags"`
	InformationText string         `json:"information_text"`
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
	db.Save(&sheet)
	return db.Error
}

func (sheet *Sheet) UpdateAtDb(db *gorm.DB) error {
	db.Save(&sheet)
	return db.Error
}

func DeleteSheet(db *gorm.DB, uuid string) error {
	var sheet Sheet
	if sheet.WasUploaded {
		return errors.New("Sheet was not uploaded, please delete it by removing the file")
	}
	db.Where("uuid = ?", uuid).Delete(&sheet)

	if db.Error != nil {
		if gorm.IsRecordNotFoundError(db.Error) {
			return errors.New("Sheet not found")
		}
		return db.Error
	}

	// Delete sheet file and thumbnail
	paths := []string{
		path.Join(Config().ConfigPath, "sheets", sheet.File),
		path.Join(Config().ConfigPath, "sheets/thumbnails", sheet.Uuid+".png"),
	}

	for _, path := range paths {
		e := os.Remove(path)
		if e != nil {
			return e
		}
	}

	// Delete unknown composer if not referenced anymore
	if sheet.ComposerUuid == "" {
		isUnreferenced, err := IsComposerUnreferenced(db, sheet.ComposerUuid)
		if err != nil {
			return err
		}
		if isUnreferenced {
			DeleteComposer(db, sheet.ComposerUuid)
		}
	}

	return nil
}

func SearchSheets(db *gorm.DB, searchValue string) ([]*Sheet, error) {
	var sheets []*Sheet
	searchValue = "%" + searchValue + "%"
	db.Where("name LIKE ?", searchValue).Find(&sheets)
	return sheets, db.Error
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
	db.First(&sheet, "uuid = ?", uuid)
	if db.Error != nil {
		return &Sheet{}, db.Error
	}
	return &sheet, nil
}

func GetAllSheets(db *gorm.DB) (*[]Sheet, error) {
	/*
		This method will return max 20 sheets, to find more or specific one you need to specify it.
		Currently it sorts it by the newest updates
	*/
	var err error
	sheets := []Sheet{}

	err = db.Order("updated_at desc").Limit(20).Find(&sheets).Error

	if err != nil {
		return &[]Sheet{}, err
	}
	return &sheets, err
}

func ListSheets(db *gorm.DB, pagination Pagination, composerUuid string) (*Pagination, error) {
	var sheets []*Sheet
	if composerUuid != "" {
		db.Scopes(composerEqual(composerUuid), paginate(sheets, &pagination, db)).Find(&sheets)
	} else {
		db.Scopes(paginate(sheets, &pagination, db)).Find(&sheets)
	}
	if db.Error != nil {
		return &Pagination{}, nil
	}
	pagination.Rows = sheets

	return &pagination, nil
}

func composerEqual(composerUuid string) func(db *gorm.DB) *gorm.DB {
	// Scope that composer is equal to composer (if you only want sheets from a certain composer)
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("composer_uuid = ?", composerUuid)
	}
}

func (s *Sheet) AppendTag(db *gorm.DB, tag string) error {
	newArray := append(s.Tags, tag)

	db.Model(&s).Update(Sheet{Tags: newArray})
	return db.Error
}

func (s *Sheet) DeleteTag(db *gorm.DB, tag string) error {
	// Deleting a tag by it's value
	index := utils.FindIndexByValue(s.Tags, tag)

	if index == -1 {
		return errors.New("Given tag was not in tag list")
	}

	newArray := pq.StringArray(utils.RemoveElementOfSlice(s.Tags, index))
	db.Model(&s).Update(Sheet{Tags: newArray})

	return db.Error
}

func (s *Sheet) UpdateSheetInformationText(db *gorm.DB, value string) error {
	s.InformationText = value
	db.Save(s)
	return db.Error
}

func FindSheetByTag(db *gorm.DB, tag string) ([]*Sheet, error) {
	var allSheets []*Sheet
	var affectedSheets []*Sheet

	// TODO: improve by using db native search
	db.Find(&allSheets)
	if db.Error != nil {
		return affectedSheets, db.Error
	}

	for _, sheet := range allSheets {
		if utils.CheckSliceContains(sheet.Tags, tag) {
			affectedSheets = append(affectedSheets, sheet)
		}
	}

	return affectedSheets, nil
}
