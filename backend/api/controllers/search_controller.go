package controllers

import (
	"fmt"
	"net/http"

	"github.com/SheetAble/SheetAble/backend/api/models"
	"github.com/SheetAble/SheetAble/backend/api/utils"
	"github.com/gin-gonic/gin"
)

func (server *Server) SearchSheets(c *gin.Context) {
	searchValue := c.Param("searchValue")

	sheets, err := models.SearchSheets(server.DB, searchValue)
	if err != nil {
		utils.DoError(c, http.StatusBadRequest, fmt.Errorf("Error searching sheets: %v", err))
		return
	}

	c.JSON(http.StatusOK, sheets)
}

func (server *Server) SearchComposers(c *gin.Context) {
	searchValue := c.Param("searchValue")

	composers, err := models.SearchComposers(server.DB, searchValue)
	if err != nil {
		utils.DoError(c, http.StatusBadRequest, fmt.Errorf("Error searching composers: %v", err))
		return
	}

	c.JSON(http.StatusOK, composers)
}
