package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"path"

	. "github.com/SheetAble/SheetAble/backend/api/config"
	"github.com/SheetAble/SheetAble/backend/api/forms"
	"github.com/SheetAble/SheetAble/backend/api/models"
	"github.com/SheetAble/SheetAble/backend/api/utils"
	"github.com/gin-gonic/gin"
)

/*
This endpoint will return all sheets in Page like style.
Meaning POST request will have 3 attributes:
  - sort_by: (how is it sorted)
  - page: (what page)
  - limit: (limit number)
  - composer: (what composer)

Return:
  - sheets: [...]
  - page_max: [7] // How many pages there are
  - page_current: [1] // Which page is currently selected
*/
func (server *Server) GetSheetsPage(c *gin.Context) {
	var form forms.GetSheetsPageRequest
	if err := c.ShouldBind(&form); err != nil {
		utils.DoError(c, http.StatusBadRequest, err)
		return
	}

	pagination := models.Pagination{
		Sort:  form.SortBy,
		Limit: form.Limit,
		Page:  form.Page,
	}

	pageNew, err := models.ListSheets(server.DB, pagination, form.Composer)
	if err != nil {
		utils.DoError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, pageNew)
}

/*
Get PDF file and information about an individual sheet.
Example request:

	GET /sheet/Étude N. 1
*/
func (server *Server) GetSheet(c *gin.Context) {
	sheetUuid := c.Param("sheetUuid")
	if sheetUuid == "" {
		utils.DoError(c, http.StatusBadRequest, errors.New("no sheet uuid given"))
		return
	}

	sheet, err := models.FindSheetByUuid(server.DB, sheetUuid)
	if err != nil {
		utils.DoError(c, http.StatusInternalServerError, fmt.Errorf("unable to get sheet: %v", err.Error()))
		return
	}
	c.JSON(http.StatusOK, sheet)
}

/*
Serve the PDF file
Example request:

	GET /sheet/pdf/Frédéric Chopin/Étude N. 1

sheetname and composer name have to be the safeName of them
*/
func (server *Server) GetPDF(c *gin.Context) {
	sheetUuid := c.Param("sheetUuid")
	if sheetUuid == "" {
		utils.DoError(c, http.StatusBadRequest, errors.New("no sheet uuid given"))
		return
	}

	sheet, err := models.FindSheetByUuid(server.DB, sheetUuid)
	if err != nil {
		utils.DoError(c, http.StatusInternalServerError, fmt.Errorf("unable to get sheet: %v", err.Error()))
		return
	}
	filePath := path.Join(Config().ConfigPath, "sheets", sheet.File)
	c.File(filePath)
}

func (server *Server) GetThumbnail(c *gin.Context) {
	sheetUuid := c.Param("sheetUuid")
	if sheetUuid == "" {
		utils.DoError(c, http.StatusBadRequest, errors.New("no sheet uuid given"))
		return
	}
	filePath := path.Join(Config().ConfigPath, "sheets/thumbnails", sheetUuid)
	c.File(filePath)
}

func (server *Server) DeleteSheet(c *gin.Context) {
	sheetUuid := c.Param("sheetUuid")
	if sheetUuid == "" {
		utils.DoError(c, http.StatusBadRequest, errors.New("no sheet uuid given"))
		return
	}

	// Check if the sheet exist
	sheetExists, err := models.ExistsSheet(server.DB, sheetUuid)
	if err != nil {
		utils.DoError(c, http.StatusInternalServerError, fmt.Errorf("unable to find sheet: %v", err.Error()))
		return
	}
	if !sheetExists {
		c.String(http.StatusNotFound, "sheet not found")
		return
	}

	err = models.DeleteSheet(server.DB, sheetUuid)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, "Sheet was successfully deleted")
}

func (server *Server) DeleteTag(c *gin.Context) {
	/*
		This endpoint will delete a given Tag
		Example Request
		DELETE /api/tag/sheet/fuer-elise
	*/

	sheetUuid := c.Param("sheetUuid")
	if sheetUuid == "" {
		utils.DoError(c, http.StatusBadRequest, errors.New("no sheet uuid given"))
		return
	}

	sheet, err := models.FindSheetByUuid(server.DB, sheetUuid)
	if err != nil {
		utils.DoError(c, http.StatusInternalServerError, fmt.Errorf("unable to get sheet: %v", err.Error()))
		return
	}

	var updateTagForm forms.TagRequest
	if err := c.ShouldBind(&updateTagForm); err != nil {
		utils.DoError(c, http.StatusBadRequest, fmt.Errorf("bad upload request: %v", err))
		return
	}

	err = sheet.DeleteTag(server.DB, updateTagForm.TagValue)
	if err != nil {
		utils.DoError(c, http.StatusNotFound, fmt.Errorf("unable to find tag: %v", err))
		return
	}

	c.JSON(http.StatusOK, "Tag: ["+updateTagForm.TagValue+"] was successfully deleted")
}

func (server *Server) AppendTag(c *gin.Context) {
	/*
		This endpoint will append a new Tag
		Example Request
		POST /api/tag/sheet/fuer-elise
			Body (FormValue):
			- tagValue: New Tag
	*/

	sheetUuid := c.Param("sheetUuid")
	if sheetUuid == "" {
		utils.DoError(c, http.StatusBadRequest, errors.New("no sheet uuid given"))
		return
	}

	sheet, err := models.FindSheetByUuid(server.DB, sheetUuid)
	if err != nil {
		utils.DoError(c, http.StatusInternalServerError, fmt.Errorf("unable to get sheet: %v", err.Error()))
		return
	}

	var tagForm forms.TagRequest
	if err := c.ShouldBind(&tagForm); err != nil {
		utils.DoError(c, http.StatusBadRequest, fmt.Errorf("bad upload request: %v", err))
		return
	}
	if tagForm.TagValue == "" {
		utils.DoError(c, http.StatusBadRequest, fmt.Errorf("No tagValue given"))
		return
	}

	sheet.AppendTag(server.DB, tagForm.TagValue)

	c.JSON(http.StatusOK, "Tag: ["+tagForm.TagValue+"] was successfully appended")
}

func (server *Server) FindSheetsByTag(c *gin.Context) {

	var tagForm forms.TagRequest
	if err := c.ShouldBind(&tagForm); err != nil {
		utils.DoError(c, http.StatusBadRequest, fmt.Errorf("bad upload request: %v", err))
		return
	}
	if tagForm.TagValue == "" {
		utils.DoError(c, http.StatusBadRequest, fmt.Errorf("No tagValue given"))
		return
	}

	sheets, err := models.FindSheetByTag(server.DB, tagForm.TagValue)
	if err != nil {
		utils.DoError(c, http.StatusInternalServerError, fmt.Errorf("unable to get sheet: %v", err.Error()))
		return
	}

	c.JSON(http.StatusOK, sheets)

}

func (server *Server) UpdateSheetInformationText(c *gin.Context) {
	/*
		This endpoint will update a sheet information text
		Example Request
		POST /api/sheet/fuer-elise/info
			Body (FormValue):
			- informationText: This is Für Elise made by Beethoven
	*/

	sheetUuid := c.Param("sheetUuid")
	if sheetUuid == "" {
		utils.DoError(c, http.StatusBadRequest, errors.New("no sheet uuid given"))
		return
	}

	sheet, err := models.FindSheetByUuid(server.DB, sheetUuid)
	if err != nil {
		utils.DoError(c, http.StatusInternalServerError, fmt.Errorf("unable to get sheet: %v", err.Error()))
		return
	}

	var informationForm forms.InformationTextRequest
	if err := c.ShouldBind(&informationForm); err != nil {
		utils.DoError(c, http.StatusBadRequest, fmt.Errorf("bad upload request: %v", err))
		return
	}
	if informationForm.InformationText == "" {
		utils.DoError(c, http.StatusBadRequest, fmt.Errorf("No informationForm given"))
		return
	}

	err = sheet.UpdateSheetInformationText(server.DB, informationForm.InformationText)
	if err != nil {
		utils.DoError(c, http.StatusInternalServerError, fmt.Errorf("unable to update sheet: %v", err.Error()))
		return
	}

	c.JSON(http.StatusOK, sheet)
}
