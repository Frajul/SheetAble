package controllers

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	. "github.com/SheetAble/SheetAble/backend/api/config"
	"github.com/SheetAble/SheetAble/backend/api/models"
	"github.com/SheetAble/SheetAble/backend/api/utils"
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
)

// This syncs sheet files in sheets/local-sheets folder with the database
// and does some clean up jobs
func (server *Server) SyncLibrary(c *gin.Context) {
	fmt.Printf("Syncing library...\n")

	sheetsPath := path.Join(Config().ConfigPath, "sheets")
	libraryPath := path.Join(Config().ConfigPath, "sheets/local-sheets")
	if err := utils.CreateDir(sheetsPath); err != nil {
		utils.DoError(c, http.StatusInternalServerError, fmt.Errorf("unable to create dir: %v", err.Error()))
		return
	}
	if err := utils.CreateDir(libraryPath); err != nil {
		utils.DoError(c, http.StatusInternalServerError, fmt.Errorf("unable to create dir: %v", err.Error()))
		return
	}

	log.Println("Syncing sheets...")
	server.syncSheets(c, libraryPath)

	// Actually only needed when something went wrong
	log.Println("Fixing missing thumbnails...")
	server.fixMissingThumbnails(c)

	// Actually only needed when something went wrong
	log.Println("Deleting unreferenced composers...")
	server.deleteUnreferencedComposers(c)

	c.String(http.StatusOK, "Sync successfull")
}

func (server *Server) fixMissingThumbnails(c *gin.Context) {
	thumbnailPath := path.Join(Config().ConfigPath, "sheets/thumbnails")
	utils.CreateDir(thumbnailPath)

	dbSheets, err := listAllSimpleSheetsInDB(server.DB)
	if err != nil {
		utils.DoError(c, http.StatusInternalServerError, fmt.Errorf("unable to retrieve all sheets from db: %v", err.Error()))
		return
	}
	for _, sheet := range *dbSheets {
		if !utils.ExistsThumbnailToPdf(sheet.File, sheet.Uuid) {
			log.Printf("Thumbnail to sheet %s does not exist, creating...", sheet.File)
			err := utils.CreateThumbnailFromPdf(sheet.File, sheet.Uuid)
			if err != nil {
				utils.DoError(c, http.StatusInternalServerError, fmt.Errorf("unable to create thumbnail: %v", err.Error()))
			}
		}
	}
}

func (server *Server) deleteUnreferencedComposers(c *gin.Context) {
	composers, err := models.ListComposers(server.DB)
	if err != nil {
		utils.DoError(c, http.StatusInternalServerError, fmt.Errorf("error retrieving all composers from db: %v", err.Error()))
		return
	}

	for _, composer := range composers {
		isUnreferenced, err := models.IsComposerUnreferenced(server.DB, composer.Uuid)
		if err != nil {
			utils.DoError(c, http.StatusInternalServerError, fmt.Errorf("error checking if composer unreferenced: %v", err.Error()))
		}
		if isUnreferenced {
			log.Printf("Composer %s unreferenced, deleting...", composer.Name)
			err = models.DeleteComposer(server.DB, composer.Uuid)
			if err != nil {
				utils.DoError(c, http.StatusInternalServerError, fmt.Errorf("error deleting composer: %v", err.Error()))
			}
		}
	}
}

func (server *Server) syncSheets(c *gin.Context, libraryPath string) {
	localSheets := listSheetsFromFiles(libraryPath)
	sort.Slice(localSheets, func(i, j int) bool {
		return localSheets[i].File < localSheets[j].File
	})

	dbSheets, err := listAllSimpleSheetsInDB(server.DB)
	if err != nil {
		utils.DoError(c, http.StatusInternalServerError, fmt.Errorf("unable to retrieve all sheets from db: %v", err.Error()))
		return
	}

	fmt.Printf("Local sheets: %v\n", len(localSheets))
	fmt.Printf("Db sheets: %v\n", len(*dbSheets))

	orphanLocalSheets, orphanDbSheets := findOrphans(localSheets, *dbSheets)
	// Add files to database which are not listed there
	if len(orphanLocalSheets) != 0 {
		fmt.Printf("Library sync found %v sheets in folder structure but not in database, adding...\n", len(orphanLocalSheets))
		for _, sheet := range orphanLocalSheets {
			sheetUuid, err := models.GenerateNonexistentSheetUuid(server.DB)
			if err != nil {
				utils.DoError(c, http.StatusInternalServerError, fmt.Errorf("unable to check existing uuids: %v", err.Error()))
				return
			}

			server.UploadSheet(c, sheet.SheetName, sheetUuid, sheet.ComposerName, sheet.File, false)
		}
	}

	// Remove sheets from database which have no local file
	if len(orphanDbSheets) != 0 {
		fmt.Printf("Library sync found %v sheets in database but not in local folder structure, removing...\n", len(orphanDbSheets))
		for _, sheet := range orphanDbSheets {
			err := models.DeleteSheet(server.DB, sheet.Uuid)
			if err != nil {
				utils.DoError(c, http.StatusInternalServerError, fmt.Errorf("unable to delete sheet: %v", err.Error()))
			}
		}
	}
}

func findOrphans(left, right []SimpleSheet) ([]SimpleSheet, []SimpleSheet) {
	// TODO: improve speed by using pointers?
	var orphansLeft, orphansRight []SimpleSheet
	indexLeft, indexRight := 0, 0
	for indexLeft < len(left) && indexRight < len(right) {
		if left[indexLeft].File == right[indexRight].File {
			indexLeft++
			indexRight++
		} else if left[indexLeft].File < right[indexRight].File { // left < right -> advance left
			orphansLeft = append(orphansLeft, left[indexLeft])
			indexLeft++
		} else { // right < left -> advance right
			orphansRight = append(orphansRight, right[indexRight])
			indexRight++
		}
	}

	if indexLeft < len(left) {
		orphansLeft = append(orphansLeft, left[indexLeft:]...)
	}
	if indexRight < len(right) {
		orphansRight = append(orphansRight, right[indexRight:]...)
	}
	return orphansLeft, orphansRight
}

func listSheetsFromFiles(libraryPath string) []SimpleSheet {
	composerEntries, err := os.ReadDir(libraryPath)
	if err != nil {
		log.Fatal(err)
	}

	sheets := []SimpleSheet{}
	for _, composerEntry := range composerEntries {
		if composerEntry.IsDir() {
			composer := composerEntry.Name()

			composerPath := path.Join(libraryPath, composer)
			err := filepath.WalkDir(composerPath, func(file string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}

				// Check if it's a regular file and has a .pdf extension
				if !d.IsDir() && strings.HasSuffix(d.Name(), ".pdf") {
					sheetName := strings.TrimSuffix(filepath.Base(file), ".pdf")

					sheets = append(sheets, SimpleSheet{File: file, SheetName: sheetName, ComposerName: composer})
				}

				return nil
			})

			if err != nil {
				fmt.Printf("Error walking directory: %v\n", err)
				return sheets
			}
		}
	}
	return sheets
}

type SimpleSheet struct {
	Uuid         string
	File         string
	SheetName    string
	ComposerName string
}

func listAllSimpleSheetsInDB(db *gorm.DB) (*[]SimpleSheet, error) {
	var sheets []models.Sheet
	var toReturn []SimpleSheet
	err := db.Where("was_uploaded = false").Order("file asc").Find(&sheets).Error
	if err != nil {
		return &toReturn, err
	}

	for _, sheet := range sheets {
		toReturn = append(toReturn, SimpleSheet{Uuid: sheet.Uuid, File: sheet.File})
	}

	return &toReturn, nil
}
