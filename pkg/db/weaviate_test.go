package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weaviate/weaviate/entities/models"
)

func TestGetData(t *testing.T) {
	response := &models.GraphQLResponse{
		Data: map[string]models.JSONObject{
			"Get": map[string]any{
				"Image": []any{
					map[string]any{
						"filepath":   "/images/test1.jpg",
						"gallery_id": float64(10), // JSON-Zahlen werden in GraphQL als float64 geparst
						"rating":     float64(4),
						"extension":  ".jpg",
						"size":       float64(2048),
						"_additional": map[string]any{
							"id":       "uuid-1234",
							"distance": float64(0.123),
						},
					},
					map[string]any{
						"filepath":   "/images/test2.png",
						"gallery_id": float64(20),
					},
				},
			},
		},
	}

	w := &WeaviateClient{}
	images := w.getData(response)

	assert.Len(t, images, 2)

	assert.Equal(t, "uuid-1234", images[0].ID)
	assert.Equal(t, "/images/test1.jpg", images[0].Path)
	assert.Equal(t, "10", images[0].GalleryID)
	assert.Equal(t, 4, images[0].Rating)
	assert.Equal(t, int64(2048), images[0].Size)
	assert.Equal(t, float64(0.123), images[0].Distance)

	assert.Equal(t, "/images/test2.png", images[1].Path)
	assert.Equal(t, "20", images[1].GalleryID)
}
