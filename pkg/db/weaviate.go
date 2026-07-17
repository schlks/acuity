package db

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/google/uuid"
	"github.com/weaviate/weaviate-go-client/v5/weaviate"
	"github.com/weaviate/weaviate-go-client/v5/weaviate/filters"
	"github.com/weaviate/weaviate-go-client/v5/weaviate/graphql"
	"github.com/weaviate/weaviate/entities/models"
)

type WeaviateClient struct {
	Client *weaviate.Client
	Worker int
}

type ImageVector struct {
	ID     string
	Path   string
	Vector []float64
}

type WImage struct {
	ID        string
	Path      string
	Base64    string
	GalleryID int
	Distance  float64
}

const className = "Image"

func NewWeaviateClient(host string) (*WeaviateClient, error) {
	cfg := weaviate.Config{
		Host:   host,
		Scheme: "http",
	}

	client, err := weaviate.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	worker := max(1, runtime.NumCPU())

	w := &WeaviateClient{
		Client: client,
		Worker: worker,
	}

	return w, nil
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
			return fmt.Errorf("timout waiting for weaviate %v", ctx.Err())
		case <-time.After(10 * time.Second):
			slog.Info("Waiting for Weaviate Server...")
		}
	}
}

func (w *WeaviateClient) InitSchema() error {
	ctx := context.Background()

	exists, err := w.Client.Schema().ClassExistenceChecker().WithClassName(className).Do(ctx)
	if err != nil {
		return fmt.Errorf("error checking existance: %v", err)
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
					Name:         "filepath",
					DataType:     []string{"text"},
					Tokenization: "field",
				},
				{
					Name:     "image",
					DataType: []string{"blob"},
				},
				{
					Name:     "gallery_id",
					DataType: []string{"int"},
				},
			},
		}

		if err := w.Client.Schema().ClassCreator().WithClass(classObj).Do(ctx); err != nil {
			return fmt.Errorf("error creating schema: %v", err)
		}
		fmt.Printf("Weaviate schema: 'Image' successfully created")
	}

	return nil
}

func (w *WeaviateClient) RemoveGalleryImages(ctx context.Context, galleryID int) error {
	filter := filters.Where().
		WithPath([]string{"gallery_id"}).
		WithOperator(filters.Equal).
		WithValueInt(int64(galleryID))

	response, err := w.Client.Batch().ObjectsBatchDeleter().
		WithClassName(className).
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

func (w *WeaviateClient) getData(result *models.GraphQLResponse) []WImage {
	if len(result.Errors) > 0 {
		var errMsgs []string
		for _, err := range result.Errors {
			errMsgs = append(errMsgs, err.Message)
		}
		slog.Error("GraphQL query returned errors", slog.String("graphql_errors", strings.Join(errMsgs, " | ")))
	}

	var images []WImage
	if getMap, ok := result.Data["Get"].(map[string]any); ok {
		if imageArray, ok := getMap["Image"].([]any); ok {
			for _, item := range imageArray {
				imgProps, ok := item.(map[string]any)
				if !ok {
					continue
				}

				var filePath, image, id string
				var galleryID int
				var distance float64

				if val, ok := imgProps["filepath"].(string); ok {
					filePath = val
				}
				if val, ok := imgProps["image"].(string); ok {
					image = val
				}
				if val, ok := imgProps["gallery_id"].(int); ok {
					galleryID = val
				}
				if additional, ok := imgProps["_additional"].(map[string]any); ok {
					if val, ok := additional["id"].(string); ok {
						id = val
					}
					if val, ok := additional["distance"].(float64); ok {
						distance = val
					}
				}

				images = append(images, WImage{
					ID:        id,
					Path:      filePath,
					GalleryID: galleryID,
					Base64:    image,
					Distance:  distance,
				})
			}
		}
	}

	return images
}

func (w *WeaviateClient) getQueryWithWhere(ctx context.Context, distance bool, galleryID int, page int, imagesPerPage int) (*graphql.GetBuilder, *filters.WhereBuilder) {
	additional := []graphql.Field{
		{Name: "id"},
	}
	if distance {
		additional = append(additional, graphql.Field{Name: "distance"})
	}
	query := w.Client.GraphQL().Get().
		WithClassName(className).
		WithFields(
			graphql.Field{Name: "filepath"},
			graphql.Field{Name: "gallery_id"},
			graphql.Field{Name: "image"},
			graphql.Field{
				Name:   "_additional",
				Fields: additional,
			}).
		WithLimit(imagesPerPage).
		WithOffset(page * imagesPerPage)
	var whereFilter *filters.WhereBuilder

	if galleryID >= 0 {
		whereFilter = filters.Where().
			WithPath([]string{"gallery_id"}).
			WithOperator(filters.Equal).
			WithValueInt(int64(galleryID))
	}

	return query, whereFilter
}

func (w *WeaviateClient) WriteBatchDB(ctx context.Context, batch []*models.Object) int {
	res, err := w.Client.Batch().ObjectsBatcher().WithObjects(batch...).Do(ctx)
	if err != nil {
		slog.Error("Error durcing batch wirte to Weaviate", slog.Any("error", err))
		return 0
	}

	success := 0
	failed := 0
	for _, r := range res {
		if r.Result != nil && r.Result.Errors != nil && len(r.Result.Errors.Error) > 0 {
			failed++
			var errMsgs []string
			for _, e := range r.Result.Errors.Error {
				if e != nil {
					errMsgs = append(errMsgs, e.Message)
				}
			}
			slog.Error("Failed to insert Object", slog.String("errors", strings.Join(errMsgs, " | ")))
		} else {
			success++
		}
	}
	slog.Info("Batch write to Weaviate complete", slog.Int("success", success), slog.Int("failed", failed))
	return failed
}

func (w *WeaviateClient) SearchImage(ctx context.Context, search string, galleryID int, page int, imagesPerPage int, threshold float32) ([]WImage, error) {
	nearText := w.Client.GraphQL().
		NearTextArgBuilder().
		WithConcepts([]string{search}).
		WithDistance(threshold)

	query, whereFilter := w.getQueryWithWhere(ctx, true, galleryID, page, imagesPerPage)

	if whereFilter != nil {
		query = query.
			WithWhere(whereFilter).
			WithNearText(nearText)
	} else {
		query = query.
			WithNearText(nearText)
	}

	result, err := query.Do(ctx)
	if err != nil {
		return nil, err
	}

	images := w.getData(result)
	return images, nil
}

func (w *WeaviateClient) SearchImage64(ctx context.Context, image string, galleryID int, page int, imagesPerPage int, threshold float32) ([]WImage, error) {
	nearImage := w.Client.GraphQL().
		NearImageArgBuilder().
		WithImage(image).
		WithDistance(threshold)

	query, whereFilter := w.getQueryWithWhere(ctx, true, galleryID, page, imagesPerPage)

	if whereFilter != nil {
		query = query.
			WithWhere(whereFilter).
			WithNearImage(nearImage)
	} else {
		query = query.
			WithNearImage(nearImage)
	}

	result, err := query.Do(ctx)
	if err != nil {
		return nil, err
	}

	images := w.getData(result)
	return images, nil
}

func (w *WeaviateClient) FindDublicates(ctx context.Context, imageID string, galleryID int, page int, imagesPerPage int, threshold float32) ([]WImage, error) {
	imageObj := w.Client.GraphQL().NearObjectArgBuilder().
		WithID(imageID).
		WithDistance(threshold)

	query, whereFilter := w.getQueryWithWhere(ctx, true, galleryID, page, imagesPerPage)

	if whereFilter != nil {
		query = query.
			WithWhere(whereFilter).
			WithNearObject(imageObj)
	} else {
		query = query.
			WithNearObject(imageObj)
	}

	result, err := query.Do(ctx)
	if err != nil {
		return nil, err
	}
	images := w.getData(result)
	var filtered []WImage

	for _, img := range images {
		if img.ID != imageID {
			filtered = append(filtered, img)
		}
	}

	return filtered, nil
}

func (w *WeaviateClient) ChangeGallery(ctx context.Context, newID int, images []WImage) error {
	g := new(errgroup.Group)
	g.SetLimit(w.Worker)
	for _, img := range images {
		img := img
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
				WithClassName(className).
				Do(ctx)
		})
	}

	return g.Wait()
}

func (w *WeaviateClient) CopyToGallery(ctx context.Context, newGalleryID int, images []WImage) error {
	g := new(errgroup.Group)
	g.SetLimit(w.Worker)

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
				"filepath":   newPath,
				"image":      info.Base64,
				"gallery_id": newGalleryID,
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

func (w *WeaviateClient) RemoveImage(ctx context.Context, image WImage) error {
	err := w.Client.Data().Deleter().
		WithClassName(className).
		WithID(image.ID).
		Do(ctx)
	if err != nil {
		return fmt.Errorf("error deleting image (ID %s): %v", image.ID, err)
	}
	return nil
}

func (w *WeaviateClient) RemoveImages(ctx context.Context, images []WImage) error {
	g := new(errgroup.Group)
	g.SetLimit(w.Worker)

	for _, img := range images {
		g.Go(func() error {
			return w.RemoveImage(ctx, img)
		})
	}

	return g.Wait()
}

func (w *WeaviateClient) GetAll(ctx context.Context, galleryID int, sortBy string, sortOrder string, page int, imagesPerPage int, flagFilter string, folderFilter string) ([]WImage, error) {
	query, whereFilter := w.getQueryWithWhere(ctx, false, galleryID, page, imagesPerPage)

	if flagFilter != "" && flagFilter != "any" {
		flagVal, err := strconv.Atoi(flagFilter)
		if err != nil {
			return nil, err
		}
		flagCond := filters.Where().
			WithPath([]string{"flag"}).
			WithOperator(filters.Equal).
			WithValueNumber(float64(flagVal))

		if whereFilter != nil {
			whereFilter = filters.Where().
				WithOperator(filters.And).
				WithOperands([]*filters.WhereBuilder{whereFilter, flagCond})
		} else {
			whereFilter = flagCond
		}
	}

	if folderFilter != "" {
		folderCond := filters.Where().
			WithPath([]string{"filepath"}).
			WithOperator(filters.Like).
			WithValueString(folderFilter + "/*")

		if whereFilter != nil {
			whereFilter = filters.Where().
				WithOperator(filters.And).
				WithOperands([]*filters.WhereBuilder{whereFilter, folderCond})
		} else {
			whereFilter = folderCond
		}
	}

	if whereFilter != nil {
		query = query.
			WithWhere(whereFilter).
			WithLimit(imagesPerPage).
			WithOffset(page * imagesPerPage)
	} else {
		query = query.
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

func (w *WeaviateClient) GetInfo(ctx context.Context, imageID string) (WImage, error) {
	query, _ := w.getQueryWithWhere(ctx, false, -1, 0, 1)
	result, err := query.Do(ctx)
	if err != nil {
		return WImage{}, err
	}

	data := w.getData(result)
	if len(data) == 0 {
		return WImage{}, fmt.Errorf("image not found")
	}
	return data[0], nil
}

func (w *WeaviateClient) GetVectors(ctx context.Context, galleryIDs []int) ([]ImageVector, error) {
	var images []ImageVector
	query := w.Client.GraphQL().Get().
		WithClassName("Image").
		WithFields(
			graphql.Field{Name: "filepath"},
			graphql.Field{
				Name: "_additional",
				Fields: []graphql.Field{
					{Name: "id"},
					{Name: "vector"},
				},
			},
		)

	if len(galleryIDs) > 0 {
		var operands []*filters.WhereBuilder
		for _, id := range galleryIDs {
			operands = append(operands, filters.Where().
				WithPath([]string{"gallery_id"}).
				WithOperator(filters.Equal).
				WithValueInt(int64(id)))
		}

		var whereFilter *filters.WhereBuilder
		if len(operands) == 1 {
			whereFilter = operands[0]
		} else {
			whereFilter = filters.Where().
				WithOperator(filters.Or).
				WithOperands(operands)
		}
		query = query.WithWhere(whereFilter)
	}

	result, err := query.Do(ctx)
	if err != nil {
		return nil, err
	}

	if getMap, ok := result.Data["Get"].(map[string]any); ok {
		if imageArray, ok := getMap["Image"].([]any); ok {
			for _, obj := range imageArray {
				item := obj.(map[string]any)
				path := item["filepath"].(string)

				additional := item["_additional"].(map[string]any)
				id := additional["id"].(string)
				vectorAny := additional["vector"].([]any)

				vector := make([]float64, len(vectorAny))
				for i, v := range vectorAny {
					vector[i] = v.(float64)
				}
				images = append(images, ImageVector{
					ID:     id,
					Path:   path,
					Vector: vector,
				})
			}
		}
	}

	return images, nil
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

	err = w.InitSchema()
	if err != nil {
		return fmt.Errorf("failed to re-initialize schema after reset: %w", err)
	}

	return nil
}
