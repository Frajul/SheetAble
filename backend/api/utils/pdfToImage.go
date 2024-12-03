package utils

import (
	"path"

	"log"

	"github.com/h2non/bimg"

	. "github.com/SheetAble/SheetAble/backend/api/config"
)

// POST request onto pdf creation
func CreateThumbnailFromPdf(pdf_file_path string, name string) (err error) {
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
	err = bimg.Write(path.Join(Config().ConfigPath, "sheets/thumbnails", name+".jpg"), image_resized)
	log.Printf("Written thumbnail")

	if err != nil {
		return err
	}

	return
}
