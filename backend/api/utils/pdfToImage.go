package utils

import (
	"errors"
	"log"
	"os"
	"path"

	. "github.com/SheetAble/SheetAble/backend/api/config"
	"github.com/h2non/bimg"
)

func CreateThumbnailFromPdf(pdf_file_path string, uuid string) (err error) {
	log.Printf("Reading pdf file path: %s", pdf_file_path)
	buffer, err := bimg.Read(pdf_file_path)
	if err != nil {
		return err
	}

	image, err := bimg.NewImage(buffer).Convert(bimg.JPEG)
	if err != nil {
		return err
	}

	const scale_factor = 2.5
	image_resized, err := bimg.NewImage(image).Resize(152*scale_factor, 214*scale_factor)
	if err != nil {
		return err
	}

	log.Printf("Successfully converted and rescaled")
	err = bimg.Write(path.Join(Config().ConfigPath, "sheets/thumbnails", uuid+".jpg"), image_resized)
	log.Printf("Written thumbnail")

	if err != nil {
		return err
	}

	return
}

func ExistsThumbnailToPdf(pdf_file_path string, uuid string) bool {
	thumbnailPath := path.Join(Config().ConfigPath, "sheets/thumbnails", uuid+".jpg")
	if _, err := os.Stat(thumbnailPath); errors.Is(err, os.ErrNotExist) {
		return false
	}
	return true
}
