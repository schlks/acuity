// Package db provides database clients and operations for both
// the relational metadata storage (SQLite) and the vector search
// engine (Weaviate).
package db

import (
	"database/sql"
	"strings"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

type SQLiteClient struct {
	DB *sqlx.DB
}

type SImage struct {
	ID           int     `db:"id"`
	GalleryID    int     `db:"gallery_id"`
	FilePath     string  `db:"filepath"`
	Blurhash     string  `db:"blurhash"`
	Rating       int     `db:"rating"`
	Flag         int     `db:"flag"`
	Extension    string  `db:"extension"`
	Date         string  `db:"date"`
	Taken        string  `db:"taken"`
	Size         float64 `db:"size"`
	Resolution   int     `db:"resolution"`
	AspectRatio  float64 `db:"aspect_ratio"`
	CameraMake   string  `db:"camera_make"`
	LensMake     string  `db:"lens_make"`
	FocalLength  string  `db:"focal_length"`
	Aperture     string  `db:"aperture"`
	ShutterSpeed string  `db:"shutter_speed"`
	Iso          string  `db:"iso"`
	Flash        bool    `db:"flash"`
}

type Gallery struct {
	ID   int
	Name string
	Path string
}

func NewSqliteDB(path string) (*SQLiteClient, error) {
	db, err := sqlx.Connect("sqlite", path)
	if err != nil {
		return nil, err
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
	    filepath TEXT,
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

func (s *SQLiteClient) InsertImage(image []SImage) error {
	tx, err := s.DB.Beginx()
	if err != nil {
		return err
	}

	query, err := tx.PrepareNamed(`
		INSERT INTO images (gallery_id, filepath, blurhash, rating, flag, extension, date, taken, size, resolution, aspect_ratio, camera_make, lens_make, focal_length, aperture, shutter_speed, 
iso, flash)
		VALUES (:gallery_id, :filepath, :blurhash, :rating, :flag, :extension, :date, :taken, :size, :resolution, :aspect_ratio, :camera_make, :lens_make, :focal_length, :aperture, :shutter_speed, 
:iso, :flash)
	`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer query.Close()

	for _, img := range image {
		_, err := query.Exec(img)
		if err != nil {
			tx.Rollback()
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
	args := []interface{}{galleryID}

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

	if sortBy != "" {
		order := "ASC"
		if strings.ToUpper(sortOrder) == "DESC" {
			order = "DESC"
		}

		// Map frontend sort fields to db columns
		column := "id"
		switch sortBy {
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

		query += " ORDER BY " + column + " " + order
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
	query := "DELETE FROM galleries WHERE id = ?;"

	_, err := s.DB.Exec(query, id)
	if err != nil {
		return err
	}
	return nil
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
	query := "UPDATE  galleries SET name = ? WHERE id = ?;"

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
