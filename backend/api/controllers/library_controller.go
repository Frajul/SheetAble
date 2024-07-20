package controllers

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"

	. "github.com/SheetAble/SheetAble/backend/api/config"
	"github.com/SheetAble/SheetAble/backend/api/models"
	"github.com/SheetAble/SheetAble/backend/api/utils"
	"github.com/gin-gonic/gin"
)

func (server *Server) SyncLibrary(c *gin.Context) {
	fmt.Printf("Syncing library...\n")

	libraryPath := path.Join(Config().ConfigPath, "sheets/uploaded-sheets")

	server.SyncSheets(libraryPath)

	c.String(http.StatusOK, "Sync successful")
}

func (server *Server) SyncSheets(libraryPath string) {
	localSheets := listSheetsFromFiles(libraryPath)
	dbSheets := models.ListAllSafeSheetNamesAndComposers(server.DB)

	sort.Slice(localSheets, func(i, j int) bool {
		return models.CompareComposerSheetSafeNames(localSheets[i], localSheets[j]) == -1
	})
	sort.Slice(dbSheets, func(i, j int) bool {
		return models.CompareComposerSheetSafeNames(dbSheets[i], dbSheets[j]) == -1
	})

	fmt.Printf("Local sheets: %v\n", localSheets)
	fmt.Printf("Db sheets: %v\n", dbSheets)

	orphanLocalSheets, orphanDbSheets := findOrphansInSortedComposerSheetNames(localSheets, dbSheets)
	sanitizedLocalOrphans := []models.ComposerSheetSafeNames{}
	if len(orphanLocalSheets) != 0 {
		fmt.Printf("Library sync found %v sheets in folder structure but not in database, adding...\n", len(orphanLocalSheets))
		for _, sheet := range orphanLocalSheets {
			// TODO: this assumes file name is already "safe name", therefore make it so!
			if sheet.IsSanitized() {
				fmt.Printf("Saving sheet %v\n", sheet)
				SafeComposer(server, sheet, sheet)
				SafeSheet(server, sheet, sheet)
			} else {
				// Sanitizing file by renaming it
				fmt.Printf("Sanitizing file of %v\n", sheet)
				sanitizedSheet := sheet
				sanitizedSheet.Sanitize()

				sanitizedLocalOrphans = append(sanitizedLocalOrphans, sanitizedSheet)

				// Make sure composer dir exists
				utils.CreateDir(sanitizedSheet.ComposerDirPath())

				// Move sheet file
				e := os.Rename(sheet.SheetPath(), sanitizedSheet.SheetPath())
				if e != nil {
					log.Fatal(e)
				}

				fmt.Printf("Saving sanitized sheet and composer %v\n", sheet)
				SafeComposer(server, sheet, sheet)
				SafeSheet(server, sheet, sanitizedSheet)
			}

		}
	}

	// Again check for db Orphans, now against sanitized local orphans
	if len(sanitizedLocalOrphans) != 0 {
		fmt.Printf("Sanitized %v local orphans: checking if still not in db...\n", sanitizedLocalOrphans)
		sort.Slice(sanitizedLocalOrphans, func(i, j int) bool {
			return models.CompareComposerSheetSafeNames(sanitizedLocalOrphans[i], sanitizedLocalOrphans[j]) == -1
		})

		_, orphanDbSheets = findOrphansInSortedComposerSheetNames(sanitizedLocalOrphans, orphanDbSheets)
	}

	if len(orphanDbSheets) != 0 {
		fmt.Printf("Library sync found %v sheets in database but not in local folder structure, removing...\n", len(orphanDbSheets))
		for _, sheetAndComposer := range orphanDbSheets {
			sheet := &models.Sheet{}
			_, err := sheet.DeleteSheet(server.DB, sheetAndComposer.SheetSafeName)
			if err != nil {
				log.Fatal(err)
			}
		}
	}

}

func findOrphansInSortedComposerSheetNames(left, right []models.ComposerSheetSafeNames) ([]models.ComposerSheetSafeNames, []models.ComposerSheetSafeNames) {
	// TODO: improve speed by using pointers?
	var orphansLeft, orphansRight []models.ComposerSheetSafeNames
	indexLeft, indexRight := 0, 0
	for indexLeft < len(left) && indexRight < len(right) {
		compare := models.CompareComposerSheetSafeNames(left[indexLeft], right[indexRight])
		switch compare {
		case -1: // left < right -> advance left
			orphansLeft = append(orphansLeft, left[indexLeft])
			indexLeft++
		case 1: // right < left -> advance right
			orphansRight = append(orphansRight, right[indexRight])
			indexRight++
		default: // both are equal
			indexLeft++
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

func listSheetsFromFiles(libraryPath string) []models.ComposerSheetSafeNames {
	composerEntries, err := os.ReadDir(libraryPath)
	if err != nil {
		log.Fatal(err)
	}

	sheets := []models.ComposerSheetSafeNames{}
	for _, composerEntry := range composerEntries {
		if composerEntry.IsDir() {
			composer := composerEntry.Name()

			sheetEntries, err := os.ReadDir(path.Join(libraryPath, composer))
			if err != nil {
				log.Fatal(err)
			}

			for _, sheetEntry := range sheetEntries {
				sheet := sheetEntry.Name()
				if strings.HasSuffix(sheet, ".pdf") {
					sheetName := strings.TrimSuffix(sheet, ".pdf")

					sheets = append(sheets, models.ComposerSheetSafeNames{ComposerSafeName: composer, SheetSafeName: sheetName})
				}
			}
		}
	}
	return sheets
}
