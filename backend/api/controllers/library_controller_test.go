package controllers

import (
	"reflect"
	"sort"
	"testing"

	"github.com/SheetAble/SheetAble/backend/api/models"
)

// func (left, right []models.ComposerSheetSafeNames) ([]models.ComposerSheetSafeNames, []models.ComposerSheetSafeNames) {
func TestFindOrphansInSortedComposerSheetNames(t *testing.T) {
	left := []models.ComposerSheetSafeNames{{ComposerSafeName: "Alpha", SheetSafeName: "Bravo"}, {ComposerSafeName: "Alpha", SheetSafeName: "Charlie"}, {ComposerSafeName: "Bravo", SheetSafeName: "Charlie"}, {ComposerSafeName: "Charlie", SheetSafeName: "Alpha"}, {ComposerSafeName: "Charlie", SheetSafeName: "Charlie"}}
	right := []models.ComposerSheetSafeNames{{ComposerSafeName: "Alpha", SheetSafeName: "Delta"}, {ComposerSafeName: "Alpha", SheetSafeName: "Charlie"}, {ComposerSafeName: "Bravo", SheetSafeName: "Delta"}, {ComposerSafeName: "Charlie", SheetSafeName: "Charlie"}}

	sort.Slice(left, func(i, j int) bool {
		return models.CompareComposerSheetSafeNames(left[i], left[j]) == -1
	})
	sort.Slice(right, func(i, j int) bool {
		return models.CompareComposerSheetSafeNames(right[i], right[j]) == -1
	})

	orphansLeft, orphansRight := findOrphansInSortedComposerSheetNames(left, right)
	expectedOrphansLeft := []models.ComposerSheetSafeNames{{ComposerSafeName: "Alpha", SheetSafeName: "Bravo"}, {ComposerSafeName: "Bravo", SheetSafeName: "Charlie"}, {ComposerSafeName: "Charlie", SheetSafeName: "Alpha"}}
	expectedOrphansRight := []models.ComposerSheetSafeNames{{ComposerSafeName: "Alpha", SheetSafeName: "Delta"}, {ComposerSafeName: "Bravo", SheetSafeName: "Delta"}}

	if !reflect.DeepEqual(orphansLeft, expectedOrphansLeft) {
		t.Errorf("Left orphans: %v not equal to expected ones: %v", orphansLeft, expectedOrphansLeft)
	}
	if !reflect.DeepEqual(orphansRight, expectedOrphansRight) {
		t.Errorf("Right orphans: %v not equal to expected ones: %v", orphansRight, expectedOrphansRight)
	}
}
