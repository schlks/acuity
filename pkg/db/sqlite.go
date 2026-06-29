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
	ID		int
	Name	string
	Path	string
}

func NewSqliteDB(path string) (*SQLiteClient, error) {
	db, err := sql.Open("sqlite", "acuity.db")
	if err != nil {
		return nil, err
	}
	defer db.Close()

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

func (s *SQLiteClient) InitRatingTable() error {
	query := `
	CREATE TABLE IF NOT EXISTS ratings (
		id INTEGER PRIMARY KEY,
		rating INTEGER,
		CONSTRAINT chk_rating CHECK (rating < 10 AND rating >= 0)
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

func (s *SQLiteClient) InsertRating(id int, rating int) error {
	query := "INSERT INTO ratings (id, rating) VALUES (?, ?);"

	_, err := s.DB.Exec(query, id, rating)
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

func (s *SQLiteClient) RemoveRating(id int) error {
	query := "DELETE FROM ratings WHERE id = ?;"

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

func (s *SQLiteClient) GetRating(id int) (int, error) {
	query := "SELECT id, rating FROM ratings WHERE id = ?;"

	var rating int
	err := s.DB.QueryRow(query, id).Scan(&rating)
	if err != nil {
		return -1, err
	}

	return rating, nil
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
