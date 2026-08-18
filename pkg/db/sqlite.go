// Package db provides database clients and operations for both
// the relational metadata storage (SQLite) and the vector search
// engine (sqlite-vec).
package db

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	sqlitevec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/jmoiron/sqlx"
	"github.com/mattn/go-sqlite3"
)

type SQLiteClient struct {
	DB *sqlx.DB
}

type SImage struct {
	ID           int      `db:"id" json:"id"`
	GalleryID    int      `db:"gallery_id" json:"gallery_id"`
	FilePath     string   `db:"filepath" json:"filepath"`
	FileHash     string   `db:"file_hash" json:"file_hash"`
	Blurhash     string   `db:"blurhash" json:"blurhash"`
	Width        int      `db:"width" json:"width"`
	Height       int      `db:"height" json:"height"`
	Rating       int      `db:"rating" json:"rating"`
	Flag         int      `db:"flag" json:"flag"`
	Tags         string   `db:"tags" json:"tags"`
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
	Distance     *float64 `db:"distance" json:"distance,omitempty"`
}

type Gallery struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

type CacheItem struct {
	ID       int      `json:"id"`
	Distance *float64 `json:"distance,omitempty"`
}

func init() {
	sqlitevec.Auto()
	sql.Register("sqlite3_custom", &sqlite3.SQLiteDriver{
		ConnectHook: func(conn *sqlite3.SQLiteConn) error {
			return conn.RegisterCollation("NATSORT", naturalCompare)
		},
	})
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
	db, err := sqlx.Open("sqlite3_custom", path)
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
	    file_hash TEXT,
	    blurhash TEXT NOT NULL,
	    width INTEGER,
	    height INTEGER,
	    rating INTEGER DEFAULT 0,
	    flag INTEGER DEFAULT 0,
	    tags TEXT DEFAULT '',
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
	CREATE INDEX IF NOT EXISTS idx_rating ON images(rating);
	CREATE INDEX IF NOT EXISTS idx_flag ON images(flag);
	CREATE INDEX IF NOT EXISTS idx_gallery_flag ON images(gallery_id, flag);
	CREATE INDEX IF NOT EXISTS idx_file_hash ON images(file_hash);
	CREATE TABLE IF NOT EXISTS duplicate_cache (
		gallery_ids TEXT NOT NULL,
		threshold REAL NOT NULL,
		group_data TEXT NOT NULL,
		PRIMARY KEY (gallery_ids, threshold)
	);`
	_, err := s.DB.Exec(query)
	if err != nil {
		return err
	}
	return nil
}

func (s *SQLiteClient) InsertGallery(name string, path string) error {
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
		INSERT INTO images (gallery_id, filepath, file_hash, blurhash, width, height, rating, flag, tags, extension, date, taken, size, resolution, aspect_ratio, camera_make, lens_make, focal_length, aperture, shutter_speed, iso, flash)
		VALUES (:gallery_id, :filepath, :file_hash, :blurhash, :width, :height, :rating, :flag, :tags, :extension, :date, :taken, :size, :resolution, :aspect_ratio, :camera_make, :lens_make, :focal_length, :aperture, :shutter_speed, :iso, :flash)
		ON CONFLICT(filepath) DO UPDATE SET
                gallery_id = :gallery_id,
                file_hash = :file_hash,
                blurhash = :blurhash,
                width = :width,
                height = :height,
                tags = :tags,
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
	if imagesPerPage < 1 {
		imagesPerPage = 100
	}
	query := "SELECT * FROM images WHERE gallery_id = ?"
	args := []any{galleryID}

	if flagFilter != "" {
		if flagVal, err := strconv.Atoi(flagFilter); err == nil {
			query += " AND flag = ?"
			args = append(args, flagVal)
		}
	}

	if folderFilter != "" {
		query += " AND filepath LIKE ?"
		args = append(args, folderFilter+"%")
	}

	switch sortBy {
	case "date":
		query += " ORDER BY date "
	case "taken":
		query += " ORDER BY taken "
	case "rating":
		query += " ORDER BY rating "
	case "flag":
		query += " ORDER BY flag "
	case "size":
		query += " ORDER BY size "
	case "resolution":
		query += " ORDER BY resolution "
	case "aspect_ratio":
		query += " ORDER BY aspect_ratio "
	case "camera_make":
		query += " ORDER BY camera_make "
	case "lens_make":
		query += " ORDER BY lens_make "
	case "focal_length":
		query += " ORDER BY focal_length "
	case "aperture":
		query += " ORDER BY aperture "
	case "shutter_speed":
		query += " ORDER BY shutter_speed "
	case "iso":
		query += " ORDER BY iso "
	case "name":
		query += " ORDER BY filepath COLLATE NATSORT "
	default:
		query += " ORDER BY filepath COLLATE NATSORT "
	}

	if sortOrder == "asc" {
		query += "ASC"
	} else {
		query += "DESC"
	}

	if page > 0 {
		query += " LIMIT ? OFFSET ?;"
		offset := (page - 1) * imagesPerPage
		args = append(args, imagesPerPage, offset)
	}

	var images []SImage
	err := s.DB.Select(&images, query, args...)
	if err != nil {
		return nil, err
	}

	return images, nil
}

func (s *SQLiteClient) RemoveImage(id int) error {
	query := "DELETE FROM images WHERE id = ?;"
	_, err := s.DB.Exec(query, id)
	if err != nil {
		return err
	}
	return nil
}

func (s *SQLiteClient) RemoveImages(images []SImage) error {
	tx, err := s.DB.Beginx()
	if err != nil {
		return err
	}
	defer func(tx *sqlx.Tx) {
		err := tx.Rollback()
		if err != nil {

		}
	}(tx)

	query, err := tx.Prepare("DELETE FROM images WHERE id = ?;")
	if err != nil {
		return err
	}
	defer func(query *sql.Stmt) {
		err := query.Close()
		if err != nil {

		}
	}(query)

	for _, img := range images {
		_, err := query.Exec(img.ID)
		if err != nil {
			return err
		}
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil
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

func (s *SQLiteClient) UpdateImage(id int, rating int, flag int) error {
	query := "UPDATE images SET rating = ?, flag = ? WHERE id = ?;"
	_, err := s.DB.Exec(query, rating, flag, id)
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

func (s *SQLiteClient) GetCachedDuplicates(galleryIDs string, threshold float64) ([][]CacheItem, error) {
	var groupData string
	err := s.DB.Get(&groupData, "SELECT group_data FROM duplicate_cache WHERE gallery_ids = ? AND threshold = ?", galleryIDs, threshold)
	if err != nil {
		return nil, err
	}
	var groups [][]CacheItem
	if err := json.Unmarshal([]byte(groupData), &groups); err != nil {
		return nil, err
	}
	return groups, nil
}

func (s *SQLiteClient) SaveCachedDuplicates(galleryIDs string, threshold float64, groups [][]CacheItem) error {
	data, err := json.Marshal(groups)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec("INSERT OR REPLACE INTO duplicate_cache (gallery_ids, threshold, group_data) VALUES (?, ?, ?)", galleryIDs, threshold, string(data))
	return err
}

func (s *SQLiteClient) ClearAllDuplicateCache() error {
	_, err := s.DB.Exec("DELETE FROM duplicate_cache;")
	return err
}

func (s *SQLiteClient) GetImagesByIDs(ids []int) (map[int]SImage, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	query, args, err := sqlx.In("SELECT * FROM images WHERE id IN (?)", ids)
	if err != nil {
		return nil, err
	}
	var images []SImage
	err = s.DB.Select(&images, query, args...)
	if err != nil {
		return nil, err
	}
	m := make(map[int]SImage)
	for _, img := range images {
		m[img.ID] = img
	}
	return m, nil
}

func (s *SQLiteClient) RemoveImagesFromDuplicateCache(deletedIDs []int) error {
	if len(deletedIDs) == 0 {
		return nil
	}
	deletedMap := make(map[int]bool)
	for _, id := range deletedIDs {
		deletedMap[id] = true
	}

	type cacheRow struct {
		GalleryIDs string  `db:"gallery_ids"`
		Threshold  float64 `db:"threshold"`
		GroupData  string  `db:"group_data"`
	}
	var rows []cacheRow
	if err := s.DB.Select(&rows, "SELECT gallery_ids, threshold, group_data FROM duplicate_cache"); err != nil {
		return err
	}

	tx, err := s.DB.Beginx()
	if err != nil {
		return err
	}
	defer func(tx *sqlx.Tx) {
		err := tx.Rollback()
		if err != nil {

		}
	}(tx)

	for _, row := range rows {
		var groups [][]CacheItem
		if err := json.Unmarshal([]byte(row.GroupData), &groups); err != nil {
			continue
		}

		var updatedGroups [][]CacheItem
		for _, g := range groups {
			var updatedGroup []CacheItem
			for _, item := range g {
				if !deletedMap[item.ID] {
					updatedGroup = append(updatedGroup, item)
				}
			}
			if len(updatedGroup) > 1 {
				updatedGroups = append(updatedGroups, updatedGroup)
			}
		}

		if len(updatedGroups) > 0 {
			newData, _ := json.Marshal(updatedGroups)
			_, err = tx.Exec("UPDATE duplicate_cache SET group_data = ? WHERE gallery_ids = ? AND threshold = ?", string(newData), row.GalleryIDs, row.Threshold)
		} else {
			_, err = tx.Exec("DELETE FROM duplicate_cache WHERE gallery_ids = ? AND threshold = ?", row.GalleryIDs, row.Threshold)
		}
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
