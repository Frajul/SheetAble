package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	. "github.com/SheetAble/SheetAble/backend/api/config"
	"github.com/SheetAble/SheetAble/backend/api/forms"
	"github.com/SheetAble/SheetAble/backend/api/models"
	"github.com/SheetAble/SheetAble/backend/api/utils"
	"github.com/gin-gonic/gin"
)

/*
This endpoint will return all composers in Page like style.
Meaning POST request will have 3 attributes:
  - sort_by: (how is it sorted)
  - page: (what page)
  - limit: (limit number)

Return:
  - composers: [...]
  - page_max: [7] // How many pages there are
  - page_current: [1] // Which page is currently selected
*/
func (server *Server) GetComposersPage(c *gin.Context) {
	var form forms.GetComposersPageRequest
	if err := c.ShouldBind(&form); err != nil {
		utils.DoError(c, http.StatusBadRequest, err)
		return
	}

	pagination := models.Pagination{
		Sort:  form.SortBy,
		Limit: form.Limit,
		Page:  form.Page,
	}

	pageNew, err := models.ListComposers(server.DB, pagination)
	if err != nil {
		utils.DoError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, pageNew)
}

/*
Update a composer via PUT request
body - formdata
example:
  - name: Chopin
  - portrait_url: url
  - epoch: romance
*/
func (server *Server) UpdateComposer(c *gin.Context) {
	composerUuid := c.Param("composerUuid")
	if composerUuid == "" {
		utils.DoError(c, http.StatusBadRequest, errors.New("no composer uuid given"))
		return
	}

	var form forms.UpdateComposersRequest
	if err := c.ShouldBind(&form); err != nil {
		utils.DoError(c, http.StatusBadRequest, fmt.Errorf("unable to parse form: %v", err))
		return
	}

	// Uploads a portrait to the server if given
	uploadSuccess := uploadPortait(form, composerUuid)

	portraitUrl := form.PortraitUrl
	if uploadSuccess {
		portraitUrl = "/composer/portrait/" + composerUuid
	}

	var composer models.Composer
	composer.Uuid = composerUuid
	if name := strings.TrimSpace(form.Name); name != "" {
		composer.Name = name
	}
	if portraitUrl != "" {
		composer.PortraitUrl = portraitUrl
	}
	if epoch := strings.TrimSpace(form.Epoch); epoch != "" {
		composer.Epoch = epoch
	}
	composer.UpdatedAt = time.Now()

	err := composer.UpdateAtDb(server.DB)
	if err != nil {
		utils.DoError(c, http.StatusNotFound, fmt.Errorf("Failed to update composer: %v", err))
		return
	}
	c.JSON(http.StatusOK, composer)
}

func (server *Server) DeleteComposer(c *gin.Context) {
	composerUuid := c.Param("composerUuid")
	if composerUuid == "" {
		utils.DoError(c, http.StatusBadRequest, errors.New("no composer uuid given"))
		return
	}

	err := models.DeleteComposer(server.DB, composerUuid)
	if err != nil {
		utils.DoError(c, http.StatusNotFound, fmt.Errorf("failed to delete composer: %v", err))
		return
	}

	c.JSON(http.StatusOK, "Composer deleted successfully")
}

/*
Serve the Composer Portraits
Example request:

	GET /composer/portrait/Chopin
*/
func (server *Server) ServePortraits(c *gin.Context) {
	composerUuid := c.Param("composerUuid")
	if composerUuid == "" {
		utils.DoError(c, http.StatusBadRequest, errors.New("no composer uuid given"))
		return
	}
	filePath := path.Join(Config().ConfigPath, "composer", composerUuid+".png")
	c.File(filePath)
}

/*
Upload a portrait
! Currently only PNG files supported
*/
func uploadPortait(form forms.UpdateComposersRequest, composerUuid string) bool {
	if form.File == nil {
		return false
	}
	portrait, err := form.File.Open()
	if err != nil {
		return false
	}

	defer portrait.Close()

	// Create the composer Directory if it doesn't exist yet
	dir := path.Join(Config().ConfigPath, "composer")
	utils.CreateDir(dir)

	fullpath := path.Join(dir, composerUuid+".png")

	err = utils.OsCreateFile(fullpath, portrait)
	if err != nil {
		return false
	}
	return true
}
