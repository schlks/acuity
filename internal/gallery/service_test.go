package gallery

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestDeleteGallery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSDB := NewMocksService(ctrl)
	mockWDB := NewMockwService(ctrl)

	galleryID := 42

	mockSDB.EXPECT().RemoveGallery(galleryID).Return(nil)
	mockWDB.EXPECT().RemoveGalleryImages(gomock.Any(), galleryID).Return(nil)

	service := &GalleryService{
		sDB: mockSDB,
		wDB: mockWDB,
	}

	err := service.DeleteGallery(context.Background(), galleryID)

	assert.NoError(t, err, "DeleteGallery no error expected")
}
