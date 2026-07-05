// Package db provides database clients and operations for both
// the relational metadata storage (SQLite) and the vector search
// engine (Weaviate).
package db

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

type SQLiteClient struct {
	DB *sql.DB
}

type Gallery struct {
	ID   int
	Name string
	Path string
}

func NewSqliteDB(path string) (*SQLiteClient, error) {
	db, err := sql.Open("sqlite", path)
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
	);`
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

func (s *SQLiteClient) RemoveGallery(id int) error {
	query := "DELETE FROM galleries WHERE id = ?;"

	_, err := s.DB.Exec(query, id)
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
