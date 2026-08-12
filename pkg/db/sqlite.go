// Package db provides database clients and operations for both
// the relational metadata storage (SQLite) and the vector search
// engine (Weaviate).
package db

import (
	"database/sql"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/jmoiron/sqlx"
	"modernc.org/sqlite"
)

type SQLiteClient struct {
	DB *sqlx.DB
}

type SImage struct {
	ID           int      `db:"id" json:"id"`
	GalleryID    int      `db:"gallery_id" json:"gallery_id"`
	FilePath     string   `db:"filepath" json:"filepath"`
	Blurhash     string   `db:"blurhash" json:"blurhash"`
	Rating       int      `db:"rating" json:"rating"`
	Flag         int      `db:"flag" json:"flag"`
	Extension    string   `db:"extension" json:"extension"`
	Date         string   `db:"date" json:"date"`
	Taken        string   `db:"taken" json:"taken"`
	Size         float64  `db:"size" json:"size"`
	Resolution   int      `db:"resolution" json:"resolution"`
	AspectRatio  float64  `db:"aspect_ratio" json:"aspect_ratio"`
	CameraMake   string   `db:"camera_make" json:"camera_make"`
	LensMake     string   `db:"lens_make" json:"lens_make"`
	FocalLength  string   `db:"focal_length" json:"focal_length"`
	Aperture     string   `db:"aperture" json:"aperture"`
	ShutterSpeed string   `db:"shutter_speed" json:"shutter_speed"`
	Iso          string   `db:"iso" json:"iso"`
	Flash        bool     `db:"flash" json:"flash"`
	Distance     *float64 `db:"-" json:"distance,omitempty"`
}

type Gallery struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

func init() {
	sqlite.MustRegisterCollationUtf8("NATSORT", naturalCompare)
}

func naturalCompare(a, b string) int {
	aRunes, bRunes := []rune(strings.ToLower(a)), []rune(strings.ToLower(b))
	i, j := 0, 0

	for i < len(aRunes) && j < len(bRunes) {
		aIsDigit := unicode.IsDigit(aRunes[i])
		bIsDigit := unicode.IsDigit(bRunes[j])

		if aIsDigit && bIsDigit {
			aStart := i
			for i < len(aRunes) && unicode.IsDigit(aRunes[i]) {
				i++
			}

			bStart := j
			for j < len(bRunes) && unicode.IsDigit(bRunes[j]) {
				j++
			}

			aVal, _ := strconv.ParseUint(string(aRunes[aStart:i]), 10, 64)
			bVal, _ := strconv.ParseUint(string(bRunes[bStart:j]), 10, 64)

			if aVal != bVal {
				if aVal < bVal {
					return -1
				}
				return 1
			}
		} else {
			if aRunes[i] != bRunes[j] {
				if aRunes[i] < bRunes[j] {
					return -1
				}
				return 1
			}
			i++
			j++
		}
	}

	if len(aRunes) == len(bRunes) {
		return 0
	}
	if len(aRunes) < len(bRunes) {
		return -1
	}
	return 1
}

func NewSqliteDB(path string) (*SQLiteClient, error) {
	if dir := filepath.Dir(path); dir != "" {
		_ = os.MkdirAll(dir, 0755)
	}
	db, err := sqlx.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA synchronous=NORMAL;",
		"PRAGMA busy_timeout=5000;",
	}
	for _, pragma := range pragmas {
		if _, err = db.Exec(pragma); err != nil {
			_ = db.Close()
			return nil, err
		}
	}

	return &SQLiteClient{DB: db}, nil
}

func (s *SQLiteClient) InitTable() error {
	query := `
	CREATE TABLE IF NOT EXISTS galleries (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		path TEXT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS images (
	    id INTEGER PRIMARY KEY AUTOINCREMENT,
	    gallery_id INTEGER,
	    filepath TEXT UNIQUE,
	    blurhash TEXT NOT NULL,
	    rating INTEGER DEFAULT 0,
	    flag INTEGER DEFAULT 0,
		extension TEXT,
		date DATETIME,
		taken DATETIME,
		size INTEGER,
		resolution INTEGER,
		aspect_ratio REAL,
		camera_make TEXT,
		lens_make TEXT,
		focal_length TEXT,
		aperture TEXT,
		shutter_speed TEXT,
		iso TEXT,
		flash BOOLEAN,
		FOREIGN KEY(gallery_id) REFERENCES galleries(id)
	);
	CREATE INDEX IF NOT EXISTS idx_gallery_id ON images(gallery_id);
	CREATE INDEX IF NOT EXISTS idx_delta ON images(date);
	CREATE INDEX IF NOT EXISTS idx_rating on images(rating);`
	_, err := s.DB.Exec(query)
	if err != nil {
		return err
	}
	return nil
}

func (s *SQLiteClient) InsertGallery(path string, name string) error {
	query := "INSERT INTO galleries (name, path) VALUES (?, ?);"

	_, err := s.DB.Exec(query, name, path)
	if err != nil {
		return err
	}

	return nil
}

func (s *SQLiteClient) InsertImage(images []SImage) error {
	tx, err := s.DB.Beginx()
	if err != nil {
		return err
	}
	defer func(tx *sqlx.Tx) {
		err := tx.Rollback()
		if err != nil {

		}
	}(tx)

	query, err := tx.PrepareNamed(`
		INSERT INTO images (gallery_id, filepath, blurhash, rating, flag, extension, date, taken, size, resolution, aspect_ratio, camera_make, lens_make, focal_length, aperture, shutter_speed, 
iso, flash)
		VALUES (:gallery_id, :filepath, :blurhash, :rating, :flag, :extension, :date, :taken, :size, :resolution, :aspect_ratio, :camera_make, :lens_make, :focal_length, :aperture, :shutter_speed, 
:iso, :flash)
		ON CONFLICT(filepath) DO UPDATE SET
                gallery_id = :gallery_id,
                blurhash = :blurhash,
                size = :size,
				date = :date;
	`)
	if err != nil {
		err := tx.Rollback()
		if err != nil {
			return err
		}
		return err
	}
	defer func(query *sqlx.NamedStmt) {
		err := query.Close()
		if err != nil {

		}
	}(query)

	for _, img := range images {
		_, err := query.Exec(img)
		if err != nil {
			slog.Error("failed to insert image into SQLite", slog.String("imagePath", img.FilePath), slog.Any("error", err))
			return err
		}
	}
	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (s *SQLiteClient) GetAllImages(galleryID int, sortBy string, sortOrder string, page int, imagesPerPage int, flagFilter string, folderFilter string) ([]SImage, error) {
	var images []SImage

	// Basis-Query
	query := "SELECT * FROM images WHERE gallery_id = ?"
	args := []any{galleryID}

	// Filtering
	if flagFilter != "" && flagFilter != "any" && flagFilter != "all" {
		query += " AND flag = ?"
		args = append(args, flagFilter)
	}

	if folderFilter != "" && folderFilter != "any" && folderFilter != "all" {
		query += " AND filepath LIKE ?"
		args = append(args, "%"+folderFilter+"%")
	}

	// Default sorting
	if sortBy == "" {
		sortBy = "id"
		sortOrder = "DESC"
	}

	order := "ASC"
	if strings.ToUpper(sortOrder) == "DESC" {
		order = "DESC"
	}

	// Map frontend sort fields to db columns
	column := "id"
	switch sortBy {
	case "name":
		column = "filepath COLLATE NATSORT"
	case "name_lex":
		column = "filepath"
	case "date":
		column = "date"
	case "size":
		column = "size"
	case "rating":
		column = "rating"
	case "resolution":
		column = "resolution"
	case "aspect_ratio":
		column = "aspect_ratio"
	case "flag":
		column = "flag"
	}

	if column != "id" {
		query += " ORDER BY " + column + " " + order + ", id DESC"
	} else {
		query += " ORDER BY id " + order
	}

	// Pagination
	if imagesPerPage > 0 {
		query += " LIMIT ?"
		args = append(args, imagesPerPage)
		if page > 0 {
			query += " OFFSET ?"
			args = append(args, page*imagesPerPage)
		}
	}

	err := s.DB.Select(&images, query, args...)
	if err != nil {
		return nil, err
	}
	return images, nil
}

func (s *SQLiteClient) RemoveGallery(id int) error {
	tx, err := s.DB.Beginx()
	if err != nil {
		return err
	}
	defer func(tx *sqlx.Tx) {
		err := tx.Rollback()
		if err != nil {
		}
	}(tx)

	if _, err := tx.Exec("DELETE FROM images WHERE gallery_id = ?", id); err != nil {
		return err
	}

	if _, err := tx.Exec("DELETE FROM galleries WHERE id = ?", id); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *SQLiteClient) RemoveImage(id int) error {
	query := "DELETE FROM images WHERE id = ?;"

	_, err := s.DB.Exec(query, id)
	if err != nil {
		return err
	}
	return nil
}

func (s *SQLiteClient) UpdateGallery(id int, name string) error {
	query := "UPDATE galleries SET name = ? WHERE id = ?;"

	_, err := s.DB.Exec(query, name, id)
	if err != nil {
		return err
	}

	return nil
}

func (s *SQLiteClient) GetGalleryByName(name string) (Gallery, error) {
	query := "SELECT id, name, path FROM galleries WHERE name = ?;"

	var gallery Gallery
	err := s.DB.QueryRow(query, name).Scan(&gallery.ID, &gallery.Name, &gallery.Path)
	if err != nil {
		return Gallery{}, err
	}

	return gallery, nil
}

func (s *SQLiteClient) GetGalleryByID(id int) (Gallery, error) {
	query := "SELECT id, name, path FROM galleries WHERE id = ?;"

	var gallery Gallery
	err := s.DB.QueryRow(query, id).Scan(&gallery.ID, &gallery.Name, &gallery.Path)
	if err != nil {
		return Gallery{}, err
	}

	return gallery, nil
}

func (s *SQLiteClient) GetImageByID(id int) (SImage, error) {
	query := "SELECT * FROM images WHERE id = ?;"

	var image SImage
	err := s.DB.Get(&image, query, id)
	if err != nil {
		return SImage{}, err
	}

	return image, nil
}

func (s *SQLiteClient) GetImageByPath(path string) (SImage, error) {
	query := "SELECT * FROM images WHERE filepath = ?;"

	var image SImage
	err := s.DB.Get(&image, query, path)
	if err != nil {
		return SImage{}, err
	}

	return image, nil
}

func (s *SQLiteClient) GetGalleryCount(galleryID int) (int, error) {
	var count int
	var err error
	if galleryID == 0 {
		err = s.DB.QueryRow("SELECT COUNT(*) FROM images").Scan(&count)
	} else {
		err = s.DB.QueryRow("SELECT COUNT(*) FROM images WHERE gallery_id = ?", galleryID).Scan(&count)
	}
	return count, err
}

func (s *SQLiteClient) GetKnownPaths(galleryID int) ([]string, error) {
	query := "SELECT filepath FROM images WHERE gallery_id = ?;"
	rows, err := s.DB.Query(query, galleryID)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
		}
	}(rows)

	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, nil
}

func (s *SQLiteClient) UpdateImageFlag(id int, flag int) error {
	query := "UPDATE images SET flag = ? WHERE id = ?;"
	_, err := s.DB.Exec(query, flag, id)
	return err
}

func (s *SQLiteClient) UpdateImageRating(id int, rating int) error {
	query := "UPDATE images SET rating = ? WHERE id = ?;"
	_, err := s.DB.Exec(query, rating, id)
	return err
}

func (s *SQLiteClient) GetAllGalleries() ([]Gallery, error) {
	query := "SELECT id, name, path FROM galleries;"
	rows, err := s.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			return
		}
	}(rows)

	var galleries []Gallery
	for rows.Next() {
		var g Gallery
		if err := rows.Scan(&g.ID, &g.Name, &g.Path); err != nil {
			return nil, err
		}
		galleries = append(galleries, g)
	}
	return galleries, nil
}

func (s *SQLiteClient) ChangeGallery(newGalleryID int, sImages []SImage) error {
	tx, err := s.DB.Beginx()
	if err != nil {
		return err
	}
	defer func(tx *sqlx.Tx) {
		err := tx.Rollback()
		if err != nil {
		}
	}(tx)

	query, err := tx.Prepare("UPDATE images SET gallery_id = ? WHERE id = ?")
	if err != nil {
		err := tx.Rollback()
		if err != nil {
			return err
		}
		return err
	}
	defer func(query *sql.Stmt) {
		err := query.Close()
		if err != nil {
		}
	}(query)

	for _, img := range sImages {
		_, err := query.Exec(newGalleryID, img.ID)
		if err != nil {
			message := "failed to insert into SQLite"
			slog.Error(message, slog.String("file", img.FilePath), slog.Any("error", err))
			return err
		}
	}
	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (s *SQLiteClient) Unflag(galleryID int) error {
	query := "UPDATE images SET flag = ? WHERE gallery_id = ?;"
	_, err := s.DB.Exec(query, 0, galleryID)
	return err
}

func (s *SQLiteClient) ResetDatabase() error {
	query := `
		DROP TABLE IF EXISTS galleries;
		DROP TABLE IF EXISTS images;
	`
	if _, err := s.DB.Exec(query); err != nil {
		return err
	}
	return s.InitTable()
}
