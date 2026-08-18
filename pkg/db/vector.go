package db

import (
	"acuity/pkg/ai"
	"context"
	"encoding/base64"
	"fmt"
	"math"
	"strconv"

	sqlitevec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"github.com/jmoiron/sqlx"
)

type ImageVector struct {
	ID     string    `json:"id"`
	Path   string    `json:"path"`
	Vector []float32 `json:"vector"`
}

type VectorClient struct {
	sDB      *SQLiteClient
	embedder *ai.CLIPEmbedder
}

func NewVectorClient(sDB *SQLiteClient, embedder *ai.CLIPEmbedder) *VectorClient {
	return &VectorClient{
		sDB:      sDB,
		embedder: embedder,
	}
}

func (v *VectorClient) SetEmbedder(embedder *ai.CLIPEmbedder) {
	v.embedder = embedder
}

func (v *VectorClient) InitSchema() error {
	query := `
		CREATE VIRTUAL TABLE IF NOT EXISTS vec_images USING vec0(
			image_id INTEGER PRIMARY KEY,
			embedding float[512] distance_metric=cosine
		);
	`
	_, err := v.sDB.DB.Exec(query)
	return err
}

func (v *VectorClient) InsertVector(imageID int, embedding []float32) error {
	blob, err := sqlitevec.SerializeFloat32(embedding)
	if err != nil {
		return err
	}
	_, _ = v.sDB.DB.Exec("DELETE FROM vec_images WHERE image_id = ?", imageID)
	_, err = v.sDB.DB.Exec("INSERT INTO vec_images(image_id, embedding) VALUES (?, ?)", imageID, blob)
	return err
}

func (v *VectorClient) SearchImages(ctx context.Context, query string, galleryID int, threshold float32) ([]SImage, error) {
	if v.embedder == nil {
		return nil, fmt.Errorf("AI search engine is still initializing")
	}

	embedding, err := v.embedder.EmbedText(query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed text query: %w", err)
	}

	blob, err := sqlitevec.SerializeFloat32(embedding)
	if err != nil {
		return nil, err
	}
	sqlQuery := `
		SELECT
			i.*,
			v.distance
		FROM vec_images v
		JOIN images i ON i.id = v.image_id
		WHERE v.embedding MATCH ?
			AND k = 200
			AND i.gallery_id = ?
			AND v.distance <= ?
		ORDER BY v.distance ASC
	`
	var images []SImage
	err = v.sDB.DB.SelectContext(ctx, &images, sqlQuery, blob, galleryID, threshold)
	if err != nil {
		return nil, err
	}
	return images, nil
}

func (v *VectorClient) SearchImage(ctx context.Context, base64Image string, galleryID int, threshold float32) ([]SImage, error) {
	if v.embedder == nil {
		return nil, fmt.Errorf("AI search engine is still initializing")
	}

	imageBytes, err := base64.StdEncoding.DecodeString(base64Image)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 image: %w", err)
	}

	embedding, err := v.embedder.EmbedImage(imageBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to embed image: %w", err)
	}

	blob, err := sqlitevec.SerializeFloat32(embedding)
	if err != nil {
		return nil, err
	}
	sqlQuery := `
		SELECT
			i.*,
			v.distance
		FROM vec_images v
		JOIN images i ON i.id = v.image_id
		WHERE v.embedding MATCH ?
			AND k = 200
			AND i.gallery_id = ?
			AND v.distance <= ?
		ORDER BY v.distance ASC
	`

	var images []SImage
	err = v.sDB.DB.SelectContext(ctx, &images, sqlQuery, blob, galleryID, threshold)
	if err != nil {
		return nil, err
	}

	return images, nil
}

func (v *VectorClient) FindDuplicates(ctx context.Context, imageID int, galleryID int, count int, threshold float32) ([]SImage, error) {
	var blob []byte
	err := v.sDB.DB.GetContext(ctx, &blob, "SELECT embedding FROM vec_images WHERE image_id = ?", imageID)
	if err != nil {
		return nil, fmt.Errorf("failed to get base image embedding: %w", err)
	}

	sqlQuery := `
		SELECT
			i.*,
			v.distance
		FROM vec_images v
		JOIN images i ON i.id = v.image_id
		WHERE v.embedding MATCH ?
			AND k = ?
			AND i.gallery_id = ?
			AND i.id != ?
			AND v.distance <= ?
		ORDER BY v.distance ASC
	`

	var images []SImage
	err = v.sDB.DB.SelectContext(ctx, &images, sqlQuery, blob, count, galleryID, imageID, threshold)
	if err != nil {
		return nil, err
	}

	return images, err
}

func (v *VectorClient) RemoveImages(ctx context.Context, imageIDs []int) error {
	if len(imageIDs) == 0 {
		return nil
	}

	query, args, err := sqlx.In("DELETE FROM vec_images WHERE image_id in(?)", imageIDs)
	if err != nil {
		return err
	}
	_, err = v.sDB.DB.ExecContext(ctx, query, args...)

	return err
}

func (v *VectorClient) RemoveGalleryImages(ctx context.Context, galleryID int) error {
	query := "DELETE FROM vec_images WHERE image_id IN (SELECT id FROM images WHERE gallery_id = ?)"
	_, err := v.sDB.DB.ExecContext(ctx, query, galleryID)
	return err
}

func (v *VectorClient) GetVectors(ctx context.Context, galleryIDs []int) ([]ImageVector, error) {
	type rawRow struct {
		ID        int    `db:"id"`
		Path      string `db:"path"`
		Embedding []byte `db:"embedding"`
	}

	query := `
		SELECT 
			v.image_id as id,
			i.filepath as path,
			v.embedding
		FROM vec_images v
		JOIN images i ON i.id = v.image_id
	`
	var rows []rawRow
	var err error
	if len(galleryIDs) > 0 {
		query += " WHERE i.gallery_id IN (?)"
		q, args, inErr := sqlx.In(query, galleryIDs)
		if inErr != nil {
			return nil, inErr
		}
		err = v.sDB.DB.SelectContext(ctx, &rows, q, args...)
	} else {
		err = v.sDB.DB.SelectContext(ctx, &rows, query)
	}
	if err != nil {
		return nil, err
	}

	result := make([]ImageVector, 0, len(rows))
	for _, row := range rows {
		if len(row.Embedding) >= 2048 {
			f32s := make([]float32, 512)
			for i := range 512 {
				bits := uint32(row.Embedding[i*4]) |
					uint32(row.Embedding[i*4+1])<<8 |
					uint32(row.Embedding[i*4+2])<<16 |
					uint32(row.Embedding[i*4+3])<<24
				f32s[i] = math.Float32frombits(bits)
			}
			result = append(result, ImageVector{
				ID:     strconv.Itoa(row.ID),
				Path:   row.Path,
				Vector: f32s,
			})
		}
	}
	return result, nil
}
