package db

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/weaviate/weaviate-go-client/v5/weaviate"
	"github.com/weaviate/weaviate-go-client/v5/weaviate/filters"
	"github.com/weaviate/weaviate-go-client/v5/weaviate/graphql"
	"github.com/weaviate/weaviate/entities/models"

	"github.com/go-openapi/strfmt"
	"github.com/google/uuid"
	"github.com/h2non/bimg"
	"github.com/rwcarlsen/goexif/exif"
)

type WeaviateClient struct {
	Client *weaviate.Client
}

type Image struct {
	ID          string
	Path        string
	Base64      string
	GalleryID   string
	Distance    float64
	Name        string
	Rating      int
	Extension   string
	Date        string // Weaviate akzeptiert und liefert RFC3339 formatierte Strings für Datum
	Taken       string
	Size        float64
	Resolution  int
	AspectRatio float64
}

func NewWeaviateClient(host string) (*WeaviateClient, error) {
	cfg := weaviate.Config{
		Host:   host,
		Scheme: "http",
	}

	client, err := weaviate.NewClient(cfg)
	if err != nil {
		return nil, err
	}

	return &WeaviateClient{Client: client}, nil
}

func (w *WeaviateClient) WaitForReady(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for {
		ready, err := w.Client.Misc().ReadyChecker().Do(ctx)
		if ready && err == nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("timout waiting for weaviate: %w", ctx.Err())
		case <-time.After(10 * time.Second):
			slog.Info("Waiting for Weaviate server...")
		}
	}
}

func (w *WeaviateClient) InitSchema() error {
	className := "Image"
	ctx := context.Background()

	exists, err := w.Client.Schema().ClassExistenceChecker().WithClassName(className).Do(ctx)
	if err != nil {
		return fmt.Errorf("error checking existence: %w", err)
	}

	if !exists {
		classObj := &models.Class{
			Class:      className,
			Vectorizer: "multi2vec-clip",
			ModuleConfig: map[string]any{
				"multi2vec-clip": map[string]any{
					"imageFields": []string{"image"},
				},
			},
			Properties: []*models.Property{
				{
					Name:     "filepath",
					DataType: []string{"text"},
				},
				{
					Name:     "image",
					DataType: []string{"blob"},
				},
				{
					Name:     "gallery_id",
					DataType: []string{"int"},
				},
				{
					Name:         "name",
					DataType:     []string{"text"},
					Tokenization: "field",
				},
				{
					Name:     "rating",
					DataType: []string{"int"},
				},
				{
					Name:     "extension",
					DataType: []string{"text"},
				},
				{
					Name:     "date",
					DataType: []string{"date"},
				},
				{
					Name:     "taken",
					DataType: []string{"date"},
				},
				{
					Name:     "size",
					DataType: []string{"int"},
				},
				{
					Name:     "resolution",
					DataType: []string{"int"},
				},
				{
					Name:     "aspect_ratio",
					DataType: []string{"number"},
				},
			},
		}

		err = w.Client.Schema().ClassCreator().WithClass(classObj).Do(ctx)
		if err != nil {
			return fmt.Errorf("error creating schema: %w", err)
		}
		fmt.Println("Weaviate schema: 'Image' successfully created.")
	}

	return nil
}

func (w *WeaviateClient) ImportImages(ctx context.Context, filePaths []string, galleryID int) {
	threads := runtime.NumCPU()
	maxWorkers := max(threads-2, threads/2)

	resultChan := make(chan Image, 50)
	done := make(chan struct{})

	go func() {
		var batch []*models.Object
		batchSize := 50

		for result := range resultChan {
			id := uuid.NewMD5(uuid.NameSpaceURL, []byte(result.Path+strconv.Itoa(galleryID))).String()
			obj := &models.Object{
				ID:    strfmt.UUID(id),
				Class: "Image",
				Properties: map[string]any{
					"filepath":     result.Path,
					"image":        result.Base64,
					"gallery_id":   galleryID,
					"name":         filepath.Base(result.Path),
					"rating":       0,
					"extension":    filepath.Ext(result.Path),
					"date":         result.Date,
					"taken":        result.Taken,
					"size":         result.Size,
					"resolution":   result.Resolution,
					"aspect_ratio": result.AspectRatio,
				},
			}
			batch = append(batch, obj)
			if len(batch) >= batchSize {
				w.WriteBatchDB(ctx, batch)
				batch = batch[:0]
			}
		}
		if len(batch) > 0 {
			w.WriteBatchDB(ctx, batch)
		}
		close(done)
	}()

	g := new(errgroup.Group)
	g.SetLimit(maxWorkers)

	for _, path := range filePaths {
		g.Go(func() error {
			data, err := bimg.Read(path)
			if err != nil {
				slog.Error("Error decoding image", slog.String("path", path), slog.Any("error", err))
				return err
			}

			bimgImg := bimg.NewImage(data)
			bimgSize, err := bimgImg.Size()

			var resolution int
			var aspectRatio float64

			if err == nil {
				resolution = bimgSize.Width * bimgSize.Height
				if bimgSize.Height > 0 {
					aspectRatio = float64(bimgSize.Width) / float64(bimgSize.Height)
				}
			}

			jpegBuffer, err := bimgImg.Convert(bimg.JPEG)
			if err != nil {
				slog.Error("Error converting image to JPEG", slog.String("path", path), slog.Any("error", err))
				return err
			}

			stat, err := os.Stat(path)
			var modTime time.Time
			var imgSize float64
			if err == nil {
				modTime = stat.ModTime()
				imgSize = float64(stat.Size())
			} else {
				modTime = time.Now()
				imgSize = float64(stat.Size())
			}

			fileDate := modTime.Format(time.RFC3339)
			takenDate := fileDate
			file, err := os.Open(path)
			if err == nil {
				defer func(file *os.File) {
					err := file.Close()
					if err != nil {
						return
					}
				}(file)
				x, err := exif.Decode(file)
				if err == nil {
					tm, err := x.DateTime()
					if err == nil {
						takenDate = tm.Format(time.RFC3339)
					}
				}
			}

			base64Image := base64.StdEncoding.EncodeToString(jpegBuffer)

			resultChan <- Image{
				Path:        path,
				Base64:      base64Image,
				Date:        fileDate,
				Taken:       takenDate,
				Size:        imgSize,
				Resolution:  resolution,
				AspectRatio: aspectRatio,
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		slog.Error("Errors occurred while importing images", slog.Any("error", err))
	}

	close(resultChan)
	<-done
}

func (w *WeaviateClient) RemoveGalleryImages(ctx context.Context, galleryID int) error {
	filter := filters.Where().
		WithPath([]string{"gallery_id"}).
		WithOperator(filters.Equal).
		WithValueInt(int64(galleryID))

	response, err := w.Client.Batch().ObjectsBatchDeleter().
		WithClassName("Image").
		WithWhere(filter).
		WithOutput("minimal").
		Do(ctx)
	if err != nil {
		return fmt.Errorf("batch delete failed for gallery %d: %w", galleryID, err)
	}

	if response.Results != nil && response.Results.Matches > 0 && len(response.Results.Objects) > 0 {
		for _, obj := range response.Results.Objects {
			if obj.Errors != nil {
				return fmt.Errorf("error deleting an object: %v", obj.Errors)
			}
		}
	}
	return nil
}

func (w *WeaviateClient) RemoveImage(ctx context.Context, image Image) error {
	err := w.Client.Data().Deleter().
		WithClassName("Image").
		WithID(image.ID).
		Do(ctx)
	if err != nil {
		return fmt.Errorf("error deleting an image (ID: %s): %v", image.ID, err)
	}
	return nil
}

func (w *WeaviateClient) getData(result *models.GraphQLResponse) []Image {
	var images []Image
	if getMap, ok := result.Data["Get"].(map[string]any); ok {
		if imageArray, ok := getMap["Image"].([]any); ok {
			for _, item := range imageArray {
				imgProps, ok := item.(map[string]any)
				if !ok {
					continue
				}

				var filePath, gallery, image string

				var name string

				if val, ok := imgProps["filepath"].(string); ok {
					filePath = val
				}
				if val, ok := imgProps["gallery_id"].(float64); ok {
					gallery = fmt.Sprintf("%.0f", val)
				}
				if val, ok := imgProps["image"].(string); ok {
					image = val
				}
				if val, ok := imgProps["name"].(string); ok {
					name = val
				}

				var distance float64
				var id string

				if additional, ok := imgProps["_additional"].(map[string]any); ok {
					if d, ok := additional["distance"].(float64); ok {
						distance = d
					}
					if i, ok := additional["id"].(string); ok {
						id = i
					}
				}

				var rating, resolution int
				var extension, date, taken string
				var size float64
				var aspectRatio float64

				if val, ok := imgProps["rating"].(float64); ok {
					rating = int(val)
				}
				if val, ok := imgProps["extension"].(string); ok {
					extension = val
				}
				if val, ok := imgProps["date"].(string); ok {
					date = val
				}
				if val, ok := imgProps["taken"].(string); ok {
					taken = val
				}
				if val, ok := imgProps["size"].(float64); ok {
					size = float64(val)
				}
				if val, ok := imgProps["resolution"].(float64); ok {
					resolution = int(val)
				}
				if val, ok := imgProps["aspectRatio"].(float64); ok {
					aspectRatio = val
				}
				if val, ok := imgProps["aspect_ratio"].(float64); ok {
					aspectRatio = val
				}

				images = append(images, Image{
					ID:          id,
					Path:        filePath,
					Base64:      image,
					Name:        name,
					GalleryID:   gallery,
					Distance:    distance,
					Rating:      rating,
					Extension:   extension,
					Date:        date,
					Taken:       taken,
					Size:        size,
					Resolution:  resolution,
					AspectRatio: aspectRatio,
				})
			}
		}
	}

	return images
}

func (w *WeaviateClient) WriteBatchDB(ctx context.Context, batch []*models.Object) {
	_, err := w.Client.Batch().
		ObjectsBatcher().
		WithObjects(batch...).
		Do(ctx)
	if err != nil {
		slog.Error("Error during batch write to Weaviate", slog.Any("error", err))
	} else {
		slog.Info("Successfully wrote images to Weaviate", slog.Int("count", len(batch)))
	}
}

func (w *WeaviateClient) SearchImage(ctx context.Context, search string, galleryID int, sortBy string, sortOrder string, page int, imagesPerPage int) ([]Image, error) {
	// TODO: bis zu einer bestimmten certainty
	nearText := w.Client.GraphQL().
		NearTextArgBuilder().
		WithConcepts([]string{search}) //.
	//WithDistance(0.9)

	query := w.Client.GraphQL().Get().
		WithClassName("Image").
		WithFields(
			graphql.Field{Name: "filepath"},
			graphql.Field{Name: "gallery_id"},
			graphql.Field{Name: "rating"},
			graphql.Field{Name: "extension"},
			graphql.Field{Name: "date"},
			graphql.Field{Name: "taken"},
			graphql.Field{Name: "size"},
			graphql.Field{Name: "resolution"},
			graphql.Field{Name: "aspect_ratio"},
			graphql.Field{
				Name: "_additional",
				Fields: []graphql.Field{
					{Name: "id"},
					{Name: "distance"},
				},
			},
		)

	if galleryID >= 0 {
		filter := filters.Where().
			WithPath([]string{"gallery_id"}).
			WithOperator(filters.Equal).
			WithValueInt(int64(galleryID))

		query = query.
			WithWhere(filter).
			WithNearText(nearText).
			WithLimit(imagesPerPage).
			WithOffset(page * imagesPerPage)
	} else {
		query = query.
			WithNearText(nearText).
			WithLimit(imagesPerPage).
			WithOffset(page * imagesPerPage)
	}

	var order graphql.SortOrder

	switch strings.ToLower(sortOrder) {
	case "desc":
		order = graphql.Desc
	case "asc":
		order = graphql.Asc
	}
	sort := graphql.Sort{
		Path:  []string{sortBy},
		Order: order,
	}
	query = query.WithSort(sort)

	result, err := query.Do(ctx)
	if err != nil {
		return nil, err
	}

	images := w.getData(result)
	return images, nil
}

func (w *WeaviateClient) SearchImage64(ctx context.Context, image string, galleryID int, sortBy string, sortOrder string, page int, imagesPerPage int) ([]Image, error) {
	// TODO: bis zu einer bestimmten certainty
	nearImage := w.Client.GraphQL().
		NearImageArgBuilder().
		WithImage(image) //.
	//WithDistance(0.9)

	query := w.Client.GraphQL().Get().
		WithClassName("Image").
		WithFields(
			graphql.Field{Name: "filepath"},
			graphql.Field{Name: "gallery_id"},
			graphql.Field{Name: "rating"},
			graphql.Field{Name: "extension"},
			graphql.Field{Name: "date"},
			graphql.Field{Name: "taken"},
			graphql.Field{Name: "size"},
			graphql.Field{Name: "resolution"},
			graphql.Field{Name: "aspect_ratio"},
			graphql.Field{
				Name: "_additional",
				Fields: []graphql.Field{
					{Name: "id"},
					{Name: "distance"},
				},
			},
		)

	if galleryID >= 0 {
		filter := filters.Where().
			WithPath([]string{"gallery_id"}).
			WithOperator(filters.Equal).
			WithValueInt(int64(galleryID))

		query = query.
			WithWhere(filter).
			WithNearImage(nearImage).
			WithLimit(imagesPerPage).
			WithOffset(page * imagesPerPage)
	} else {
		query = query.
			WithNearImage(nearImage).
			WithLimit(imagesPerPage).
			WithOffset(page * imagesPerPage)
	}

	var order graphql.SortOrder

	switch strings.ToLower(sortOrder) {
	case "desc":
		order = graphql.Desc
	case "asc":
		order = graphql.Asc
	}
	sort := graphql.Sort{
		Path:  []string{sortBy},
		Order: order,
	}
	query = query.WithSort(sort)

	result, err := query.Do(ctx)
	if err != nil {
		return nil, err
	}

	images := w.getData(result)
	return images, nil
}

func (w *WeaviateClient) FindDublicates(ctx context.Context, imageID string, galleryID int, page int, imagesPerPage int) ([]Image, error) {
	imageObj := w.Client.GraphQL().NearObjectArgBuilder().WithID(imageID)

	query := w.Client.GraphQL().Get().
		WithClassName("Image").
		WithFields(
			graphql.Field{Name: "filepath"},
			graphql.Field{Name: "gallery_id"},
			graphql.Field{Name: "rating"},
			graphql.Field{Name: "extension"},
			graphql.Field{Name: "date"},
			graphql.Field{Name: "taken"},
			graphql.Field{Name: "size"},
			graphql.Field{Name: "resolution"},
			graphql.Field{Name: "aspect_ratio"},
			graphql.Field{
				Name: "_additional",
				Fields: []graphql.Field{
					{Name: "id"},
					{Name: "distance"},
				},
			},
		).
		WithNearObject(imageObj)

	if galleryID >= 0 {
		filter := filters.Where().
			WithPath([]string{"gallery_id"}).
			WithOperator(filters.Equal).
			WithValueInt(int64(galleryID))
		query = query.WithWhere(filter)
	}

	query = query.WithLimit(imagesPerPage)
	if page >= 0 {
		query = query.WithOffset(page * imagesPerPage)
	}

	result, err := query.Do(ctx)

	if err != nil {
		return nil, err
	}

	images := w.getData(result)
	var filtered []Image

	for _, img := range images {
		if img.ID != imageID {
			filtered = append(filtered, img)
		}
	}

	return filtered, nil
}

func (w *WeaviateClient) ChangeGallery(ctx context.Context, newID int, images []Image) error {
	threads := runtime.NumCPU()
	maxWorkers := max(threads-2, threads/2)
	maxWorkers = max(maxWorkers, 1)

	g := new(errgroup.Group)
	g.SetLimit(maxWorkers)

	for _, img := range images {
		img := img // Create local copy for closure
		g.Go(func() error {
			props := map[string]any{
				"gallery_id": newID,
			}
			if img.Path != "" {
				props["filepath"] = img.Path
			}
			return w.Client.Data().Updater().
				WithMerge().
				WithID(img.ID).
				WithClassName("Image").
				WithProperties(props).
				Do(ctx)
		})
	}

	return g.Wait()
}

func (w *WeaviateClient) CopyToGallery(ctx context.Context, newGalleryID int, images []Image) error {
	threads := runtime.NumCPU()
	maxWorkers := max(threads-2, threads/2)
	maxWorkers = max(maxWorkers, 1)

	g := new(errgroup.Group)
	g.SetLimit(maxWorkers)

	for _, img := range images {
		img := img // Copy for closure
		g.Go(func() error {
			newPath := img.Path
			info, err := w.GetInfo(ctx, img.ID)
			if err != nil {
				return err
			}
			if newPath == "" {
				newPath = info.Path
			}

			newImageID := uuid.NewMD5(uuid.NameSpaceURL, []byte(newPath+strconv.Itoa(newGalleryID))).String()
			props := map[string]any{
				"filepath":     newPath,
				"image":        info.Base64,
				"gallery_id":   newGalleryID,
				"name":         info.Name,
				"rating":       info.Rating,
				"extension":    info.Extension,
				"date":         info.Date,
				"taken":        info.Taken,
				"size":         info.Size,
				"resolution":   info.Resolution,
				"aspect_ratio": info.AspectRatio,
			}
			_, err = w.Client.Data().Creator().
				WithClassName("Image").
				WithID(newImageID).
				WithProperties(props).
				Do(ctx)
			return err
		})
	}

	return g.Wait()
}

func (w *WeaviateClient) RemoveImages(ctx context.Context, images []Image) error {
	threads := runtime.NumCPU()
	maxWorkers := max(threads-2, threads/2)
	maxWorkers = max(maxWorkers, 1)

	g := new(errgroup.Group)
	g.SetLimit(maxWorkers)

	for _, image := range images {
		g.Go(func() error {
			return w.RemoveImage(ctx, image)
		})
	}

	return g.Wait()
}

func (w *WeaviateClient) GetAll(ctx context.Context, galleryID int, sortBy string, sortOrder string, page int, imagesPerPage int) ([]Image, error) {
	query := w.Client.GraphQL().Get().
		WithClassName("Image").
		WithFields(
			graphql.Field{Name: "filepath"},
			graphql.Field{Name: "gallery_id"},
			graphql.Field{Name: "rating"},
			graphql.Field{Name: "extension"},
			graphql.Field{Name: "date"},
			graphql.Field{Name: "taken"},
			graphql.Field{Name: "size"},
			graphql.Field{Name: "resolution"},
			graphql.Field{Name: "aspect_ratio"},
			graphql.Field{
				Name: "_additional",
				Fields: []graphql.Field{
					{Name: "id"},
				},
			},
		).
		WithWhere(
			filters.Where().
				WithPath([]string{"gallery_id"}).
				WithOperator(filters.Equal).
				WithValueInt(int64(galleryID))).
		WithLimit(imagesPerPage)

	if page >= 0 {
		query = query.WithOffset(page * imagesPerPage)
	}

	var order graphql.SortOrder

	switch strings.ToLower(sortOrder) {
	case "desc":
		order = graphql.Desc
	case "asc":
		order = graphql.Asc
	}

	sort := graphql.Sort{
		Path:  []string{sortBy},
		Order: order,
	}
	query = query.WithSort(sort)

	result, err := query.Do(ctx)
	if err != nil {
		return nil, err
	}

	images := w.getData(result)

	return images, nil
}

func (w *WeaviateClient) GetKnownPaths(ctx context.Context, galleryID int) (map[string]struct{}, error) {
	knownPaths := make(map[string]struct{})

	result, err := w.Client.GraphQL().Get().
		WithClassName("Image").
		WithFields(graphql.Field{Name: "filepath"}).
		WithWhere(
					filters.Where().
						WithPath([]string{"gallery_id"}).
						WithOperator(filters.Equal).
						WithValueInt(int64(galleryID))).
		WithLimit(10_000). // Weaviate Standard-Limit ist 10 – explizit hochsetzen!
		Do(ctx)
	if err != nil {
		return nil, err
	}

	data, ok := result.Data["Get"].(map[string]any)
	if !ok || data["Image"] == nil {
		return knownPaths, nil
	}
	images, ok := data["Image"].([]any)
	if !ok {
		return knownPaths, nil
	}
	for _, imgObj := range images {
		img := imgObj.(map[string]any)
		path := img["filepath"].(string)
		knownPaths[path] = struct{}{}
	}

	return knownPaths, nil
}

func (w *WeaviateClient) GetInfo(ctx context.Context, imageID string) (Image, error) {
	result, err := w.Client.GraphQL().Get().
		WithClassName("Image").
		WithFields(
			graphql.Field{Name: "filepath"},
			graphql.Field{Name: "image"},
			graphql.Field{Name: "gallery_id"},
			graphql.Field{Name: "name"},
			graphql.Field{Name: "rating"},
			graphql.Field{Name: "extension"},
			graphql.Field{Name: "date"},
			graphql.Field{Name: "taken"},
			graphql.Field{Name: "size"},
			graphql.Field{Name: "resolution"},
			graphql.Field{Name: "aspect_ratio"},
			graphql.Field{
				Name: "_additional",
				Fields: []graphql.Field{
					{Name: "id"},
				},
			},
		).
		WithWhere(
			filters.Where().
				WithPath([]string{"id"}).
				WithOperator(filters.Equal).
				WithValueText(imageID)).
		WithLimit(1).
		Do(ctx)
	if err != nil {
		return Image{}, err
	}

	return w.getData(result)[0], nil
}

func (w *WeaviateClient) GetGalleryCount(ctx context.Context, galleryID int) (int, error) {
	result, err := w.Client.GraphQL().Aggregate().
		WithClassName("Image").
		WithFields(
			graphql.Field{
				Name: "meta", Fields: []graphql.Field{
					{Name: "count"},
				},
			},
		).
		WithWhere(
			filters.Where().
				WithPath([]string{"gallery_id"}).
				WithOperator(filters.Equal).
				WithValueInt(int64(galleryID))).
		Do(ctx)
	if err != nil {
		return 0, err
	}

	aggMap, ok := result.Data["Aggregate"].(map[string]any)
	if !ok {
		return 0, fmt.Errorf("missing Aggregate in response")
	}

	imageArr, ok := aggMap["Image"].([]any)
	if !ok || len(imageArr) == 0 {
		return 0, nil
	}

	imgObj, ok := imageArr[0].(map[string]any)
	if !ok {
		return 0, fmt.Errorf("invalid Image object format")
	}

	metaObj, ok := imgObj["meta"].(map[string]any)
	if !ok {
		return 0, fmt.Errorf("missing meta in response")
	}

	countFloat, ok := metaObj["count"].(float64)
	if !ok {
		return 0, fmt.Errorf("missing count in meta")
	}

	return int(countFloat), nil
}

func (w *WeaviateClient) ResetDatabase(ctx context.Context) error {
	className := "Image"

	exists, err := w.Client.Schema().ClassExistenceChecker().WithClassName(className).Do(ctx)
	if err != nil {
		return err
	}

	if !exists {
		slog.Info("Database is already empty (class does not exist)")
		return nil
	}

	err = w.Client.Schema().ClassDeleter().WithClassName(className).Do(ctx)
	if err != nil {
		return err
	}

	slog.Info("Weaviate database successfully and completely cleared")
	return nil
}

func (w *WeaviateClient) SetRating(ctx context.Context, imageID string, rating int) error {
	props := map[string]any{
		"rating": rating,
	}
	return w.Client.Data().Updater().
		WithMerge().
		WithID(imageID).
		WithClassName("Image").
		WithProperties(props).
		Do(ctx)
}
