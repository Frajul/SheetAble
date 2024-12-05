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
	"github.com/rs/xid"

	. "github.com/SheetAble/SheetAble/backend/api/config"
	"github.com/SheetAble/SheetAble/backend/api/models"
	"github.com/SheetAble/SheetAble/backend/api/utils"
)

// Structs for handling the response on the Open Opus API

type Response struct {
	Composers *[]Comp `json: "composers"`
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
	// Check for authentication
	// TODO: check it always for all api!!
	token := utils.ExtractToken(c)
	uid, err := auth.ExtractTokenID(token, Config().ApiSecret)
	if err != nil || uid == 0 {
		c.String(http.StatusUnauthorized, "Unauthorized")
		return
	}

	var uploadForm forms.UploadRequest
	if err = c.ShouldBind(&uploadForm); err != nil {
		utils.DoError(c, http.StatusBadRequest, fmt.Errorf("bad upload request: %v", err))
		return
	}
	if err = uploadForm.ValidateForm(); err != nil {
		utils.DoError(c, http.StatusBadRequest, err)
		return
	}

	prePath := path.Join(Config().ConfigPath, "sheets")
	uploadPath := path.Join(Config().ConfigPath, "sheets/uploaded-sheets")
	thumbnailPath := path.Join(Config().ConfigPath, "sheets/thumbnails")

	existsComposer, err := models.ExistsComposer(server.DB, uploadForm.Composer) // TODO: Add Form Field ComposerUuid
	if err != nil {
		utils.DoError(c, http.StatusInternalServerError, fmt.Errorf("unable to check composer existence: %v", err.Error()))
		return
	}

	var composerUuid string

	if !existsComposer {
		opusComposer, err := findComposerInOpenOpus(uploadForm.Composer) // TODO: Rename Form Field to ComposerName
		if err != nil {
			utils.DoError(c, http.StatusInternalServerError, fmt.Errorf("unable to check openopus: %v", err.Error()))
			return
		}

		composerUuid, err := generateNonexistentComposerUuid(server)
		if err != nil {
			utils.DoError(c, http.StatusInternalServerError, fmt.Errorf("unable to check existing uuids: %v", err.Error()))
			return
		}
		composer := models.NewComposer(composerUuid, opusComposer.CompleteName, opusComposer.Portrait, opusComposer.Epoch)
		composer.SaveToDb(server.DB)
	} else {
		composerUuid = "" // TODO: get from form or databsae
	}

	utils.CreateDir(prePath)
	utils.CreateDir(uploadPath)
	utils.CreateDir(thumbnailPath)

	// TODO: Check if the file already exists
	sheetUuid, err := generateNonexistentSheetUuid(server)
	if err != nil {
		utils.DoError(c, http.StatusInternalServerError, fmt.Errorf("unable to check existing uuids: %v", err.Error()))
		return
	}
	// releaseDate := uploadForm.ReleaseDate

	// fullpath, err := checkFileExists(uploadPath, sheetUuid)
	// if fullpath == "" || err != nil {
	// 	return
	// }

	// Create file
	theFile, err := uploadForm.File.Open()
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	defer theFile.Close()

	fullpath := path.Join(uploadPath, sheetUuid+".pdf")
	err = utils.OsCreateFile(fullpath, theFile)
	if err != nil {
		utils.DoError(c, http.StatusInternalServerError, fmt.Errorf("unable to create sheet file: %v", err.Error()))
		return
	}

	sheet := models.NewSheet(sheetUuid, uploadForm.SheetName, composerUuid, fullpath, true)
	err = sheet.SaveToDb(server.DB)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	err = utils.CreateThumbnailFromPdf(fullpath, sheetUuid)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusAccepted, "File uploaded successfully")
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

func generateNonexistentComposerUuid(server *Server) (string, error) {
	for i := 0; i < 10; i++ {
		uuid := xid.New().String()
		exists, err := models.ExistsComposer(server.DB, uuid)
		if err != nil {
			return "", err
		}

		if !exists {
			return uuid, nil
		}
	}
	return "", errors.New("Somehow unable to generate new uuid for composer.")
}

func generateNonexistentSheetUuid(server *Server) (string, error) {
	for i := 0; i < 10; i++ {
		uuid := xid.New().String()
		exists, err := models.ExistsSheet(server.DB, uuid)
		if err != nil {
			return "", err
		}

		if !exists {
			return uuid, nil
		}
	}
	return "", errors.New("Somehow unable to generate new uuid for sheet.")
}

// func checkFileExists(pathName string, sheetName string) (string, error) {
// 	// Check if the file already exists
// 	fullpath := fmt.Sprintf("%s/%s.pdf", pathName, models.SanitizeName(sheetName))
// 	if _, err := os.Stat(fullpath); err == nil {
// 		return "", errors.New("file already exists")
// 	}
// 	return fullpath, nil
// }
