/*
	This file is for handeling the basic upload of sheets.
	It will upload given file in the uploaded sheets folder either under
	the unknown subfolder or under the author's name subfolder, depending on whether an author is given or not.
*/

package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/SheetAble/SheetAble/backend/api/auth"
	"github.com/SheetAble/SheetAble/backend/api/forms"
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"

	. "github.com/SheetAble/SheetAble/backend/api/config"
	"github.com/SheetAble/SheetAble/backend/api/models"
	"github.com/SheetAble/SheetAble/backend/api/utils"
)

// Structs for handling the response on the Open Opus API

type Response struct {
	Composers *[]Comp `json:"composers"`
}

// Composer from OpenOpusAPI
type Comp struct {
	Name         string `json:"name"`
	CompleteName string `json:"complete_name"`
	SafeName     string `json:"safe_name"`
	Birth        string `json:"birth"`
	Death        string `json:"death"`
	Epoch        string `json:"epoch"`
	Portrait     string `json:"portrait"`
}

func (server *Server) UploadFile(c *gin.Context) {
	var uploadForm forms.UploadRequest
	if err := c.ShouldBind(&uploadForm); err != nil {
		utils.DoError(c, http.StatusBadRequest, fmt.Errorf("bad upload request: %v", err))
		return
	}
	if err := uploadForm.ValidateForm(); err != nil {
		utils.DoError(c, http.StatusBadRequest, err)
		return
	}

	var composerName = strings.TrimSpace(uploadForm.ComposerName)

	// Create file
	theFile, err := uploadForm.File.Open()
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	defer theFile.Close()

	// TODO: Check if the file already exists
	uploadPath := path.Join(Config().ConfigPath, "sheets/uploaded-sheets")
	prePath := path.Join(Config().ConfigPath, "sheets")

	utils.CreateDir(prePath)
	utils.CreateDir(uploadPath)

	sheetUuid, err := models.GenerateNonexistentSheetUuid(server.DB)
	if err != nil {
		utils.DoError(c, http.StatusInternalServerError, fmt.Errorf("unable to check existing uuids: %v", err.Error()))
		return
	}
	fullpath := path.Join(uploadPath, sheetUuid+".pdf")
	err = utils.OsCreateFile(fullpath, theFile)
	if err != nil {
		utils.DoError(c, http.StatusInternalServerError, fmt.Errorf("unable to create sheet file: %v", err.Error()))
		return
	}

	server.UploadSheet(c, uploadForm.SheetName, sheetUuid, composerName, fullpath, true)
}

func (server *Server) UploadSheet(c *gin.Context, sheetName string, sheetUuid string, composerName string, filePath string, wasUploaded bool) {
	thumbnailPath := path.Join(Config().ConfigPath, "sheets/thumbnails")
	utils.CreateDir(thumbnailPath)

	composer, err := server.findComposerOrNew(composerName)
	if err != nil {
		utils.DoError(c, http.StatusInternalServerError, fmt.Errorf("error finding or creating composer: %v", err.Error()))
		return
	}
	sheet := models.NewSheet(sheetUuid, sheetName, composer.Uuid, filePath, wasUploaded)

	err = sheet.SaveToDb(server.DB)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	err = utils.CreateThumbnailFromPdf(filePath, sheetUuid)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusAccepted, "File uploaded successfully")
}

func (server *Server) findComposerOrNew(composerName string) (*models.Composer, error) {
	composer, err := models.FindComposerByNameCaseInsensitive(server.DB, composerName)
	if err == nil {
		return composer, nil
	} else if !gorm.IsRecordNotFoundError(err) {
		return nil, err
	}

	opusComposer, err := findComposerInOpenOpus(composerName)
	if err != nil {
		return nil, err
	}

	// Search again for existing composer, now with name from open opus
	composer, err = models.FindComposerByNameCaseInsensitive(server.DB, opusComposer.CompleteName)
	if err == nil {
		return composer, nil
	} else if !gorm.IsRecordNotFoundError(err) {
		return nil, err
	}

	// Composer does not exist, generate new composer
	composerUuid, err := models.GenerateNonexistentComposerUuid(server.DB)
	if err != nil {
		return nil, err
	}
	composer = models.NewComposer(composerUuid, opusComposer.CompleteName, opusComposer.Portrait, opusComposer.Epoch)
	err = composer.SaveToDb(server.DB)
	if err != nil {
		return nil, err
	}
	return composer, nil
}

func (server *Server) UpdateSheet(c *gin.Context) {

	// Check for authentication
	token := utils.ExtractToken(c)
	uid, err := auth.ExtractTokenID(token, Config().ApiSecret)
	if err != nil || uid == 0 {
		c.String(http.StatusUnauthorized, "Unauthorized")
		return
	}

	sheetUuid := c.Param("sheetUuid")
	if sheetUuid == "" {
		utils.DoError(c, http.StatusBadRequest, errors.New("no sheet uuid given"))
		return
	}

	// Delete Sheet
	err = models.DeleteSheet(server.DB, sheetUuid)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	server.UploadFile(c)

}

func findComposerInOpenOpus(composerName string) (*Comp, error) {
	// TODO: escape composerName to prevent injection attacks
	resp, err := http.Get("https://api.openopus.org/composer/list/search/" + composerName + ".json")
	if err != nil {
		return &Comp{}, err
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &Comp{}, err
	}
	response := &Response{
		Composers: &[]Comp{},
	}

	err = json.Unmarshal([]byte(string(body)), response)
	if err != nil {
		return &Comp{}, err
	}
	composers := *response.Composers

	// Check if the given name and the name from the API are alike
	if len(composers) == 0 || (!strings.EqualFold(composerName, composers[0].Name) && !strings.EqualFold(composerName, composers[0].CompleteName)) {
		return &Comp{
			CompleteName: composerName,
			SafeName:     composerName,
			Portrait:     "https://icon-library.com/images/unknown-person-icon/unknown-person-icon-4.jpg",
			Epoch:        "Unknown",
		}, nil
	}

	return &composers[0], nil
}

// func checkFileExists(pathName string, sheetName string) (string, error) {
// 	// Check if the file already exists
// 	fullpath := fmt.Sprintf("%s/%s.pdf", pathName, models.SanitizeName(sheetName))
// 	if _, err := os.Stat(fullpath); err == nil {
// 		return "", errors.New("file already exists")
// 	}
// 	return fullpath, nil
// }
