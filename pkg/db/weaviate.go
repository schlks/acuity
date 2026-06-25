package db

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"runtime"

	"golang.org/x/sync/errgroup"

	"github.com/weaviate/weaviate-go-client/v5/weaviate"
	"github.com/weaviate/weaviate-go-client/v5/weaviate/graphql"
	"github.com/weaviate/weaviate-go-client/v5/weaviate/filters"
	"github.com/weaviate/weaviate/entities/models"

	"github.com/google/uuid"
	"github.com/go-openapi/strfmt"
	"github.com/h2non/bimg"
)

type WeaviateClient struct {
	Client *weaviate.Client
}

type Image struct {
	ID			string
	Path		string
	Base64		string
	Distance	float64
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

func (w *WeaviateClient) InitSchema() error {
	className := "Image"
	ctx := context.Background()

	exists, err := w.Client.Schema().ClassExistenceChecker().WithClassName(className).Do(ctx)
	if err != nil {
		return fmt.Errorf("fehler bei der existenzprüfung: %w", err)
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
					Name: "gallery_id",
					DataType: []string{"int"},
				},
			},
		}

		err = w.Client.Schema().ClassCreator().WithClass(classObj).Do(ctx)
		if err != nil {
			return fmt.Errorf("fehler beim erstellen des schemas: %w", err)
		}
		fmt.Println("Weaviate-Schema: 'Image' erfolgreich erstellt.")
	}

	return nil
}

func (w *WeaviateClient) ImportImages(ctx context.Context, filePaths []string, galleryID int) {
	threads := runtime.NumCPU()
	maxWorkers := max(threads-2, threads/2)

	resultChan := make(chan Image, 100)
	done := make(chan struct{})

	go func() {
		var batch []*models.Object
		batchSize := 100

		for result := range resultChan {
			id := uuid.NewMD5(uuid.NameSpaceURL, []byte(result.Path)).String()
			obj := &models.Object{
				ID:    strfmt.UUID(id),
				Class: "Image",
				Properties: map[string]any{
					"filepath":   result.Path,
					"image":      result.Base64,
					"gallery_id": galleryID,
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
				log.Printf("Fehler beim Dekodieren: %v", err)
				return err
			}

			bimgImg := bimg.NewImage(data)
			jpegBuffer, err := bimgImg.Convert(bimg.JPEG)
			if err != nil {
				log.Printf("Fehler beim konvertieren zu JPEG: %v", err)
				return err
			}

			base64Image := base64.StdEncoding.EncodeToString(jpegBuffer)

			resultChan <- Image{
				Path:   path,
				Base64: base64Image,
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		log.Printf("Beim Importieren der Bilder sind Fehler aufgetreten: %v", err)
	}

	close(resultChan)
	<- done
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
		return fmt.Errorf("batch delete fehlgeschlagen für gallery %d: %w", galleryID, err)
	}

	if response.Results != nil && response.Results.Matches > 0 && len(response.Results.Objects) > 0 {
		for _, obj := range response.Results.Objects {
			if obj.Errors != nil {
				return fmt.Errorf("fehler beim löschen eines objektes: %v", obj.Errors)
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
		return fmt.Errorf("fehler beim löschen eines eines Bildes (ID: %s): %v",image.ID , err)
	}
	return nil
}

func (w *WeaviateClient) getData(result *models.GraphQLResponse) []Image {
	var images []Image
	if getMap, ok := result.Data["Get"].(map[string]any); ok {
		if imageArray, ok := getMap["Image"].([]any); ok {
			for _, item := range imageArray {
				imgProps := item.(map[string]any)
				filepath := imgProps["filepath"].(string)

				var distance float64
				var id string
				if additional, ok := imgProps["_additional"].(map[string]any); ok {
					if d, ok := additional["distance"].(float64); ok {
						distance = d
					}
					if i, ok := additional["distance"].(string); ok {
						id = i
					}
				}

				images = append(images, Image{
					ID: id,
					Path: filepath,
					Distance: distance,
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
		log.Printf("Fehler beim Batch-Schreiben: %v", err)
	} else {
		log.Printf("Erfolgreich %d Bilder in Weaviate geschrieben", len(batch))
	}
}

func (w *WeaviateClient) SearchImage(ctx context.Context, search string, limit int, galleryID int) ([]Image, error) {
	// TODO: bis zu einer bestimmten certainty
	nearText := w.Client.GraphQL().
		NearTextArgBuilder().
		WithConcepts([]string{search})//.
		//WithDistance(0.9)

	query:= w.Client.GraphQL().Get().
		WithClassName("Image").
		WithFields(
			graphql.Field{Name: "filepath"},
			graphql.Field{Name: "image"},
			graphql.Field{Name: "gallery_id"},
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
			WithLimit(limit)
	} else {
		query = query.
			WithNearText(nearText).
			WithLimit(limit)
	}

	result, err := query.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("fehler beim suchen nach Bildern: %w", err)
	}

	images := w.getData(result)
	return images, nil
}

func (w *WeaviateClient) SearchImage64(ctx context.Context, image string, limit int, galleryID int) ([]Image, error) {
	// TODO: bis zu einer bestimmten certainty
	nearImage := w.Client.GraphQL().
		NearImageArgBuilder().
		WithImage(image)//.
		//WithDistance(0.9)

	query:= w.Client.GraphQL().Get().
		WithClassName("Image").
		WithFields(
			graphql.Field{Name: "filepath"},
			graphql.Field{Name: "image"},
			graphql.Field{Name: "gallery_id"},
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
			WithLimit(limit)
	} else {
		query = query.
			WithNearImage(nearImage).
			WithLimit(limit)
	}

	result, err := query.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("fehler beim suchen nach Bildern: %w", err)
	}

	images := w.getData(result)
	return images, nil
}

func (w *WeaviateClient) FindDublicates(ctx context.Context, imageID int, galleryID int) ([]Image, error) {
	query:= w.Client.GraphQL().Get().
		WithClassName("Image").
		WithFields(
			graphql.Field{Name: "filepath"},
			graphql.Field{Name: "image"},
			graphql.Field{Name: "gallery_id"},
			graphql.Field{
				Name: "_additional",
				Fields: []graphql.Field{
					{Name: "id"},
				},
			},
		)

	var filter *filters.WhereBuilder
	if galleryID >= 0 {
		filter1 := filters.Where().
			WithPath([]string{"id"}).
			WithOperator(filters.Equal).
			WithValueInt(int64(imageID))

		filter2 := filters.Where().
			WithPath([]string{"gallery_id"}).
			WithOperator(filters.Equal).
			WithValueInt(int64(galleryID))

		filter = filters.Where().
			WithOperator(filters.And).
			WithOperands([]*filters.WhereBuilder{filter1, filter2})
	} else {
		filter = filters.Where().
			WithPath([]string{"id"}).
			WithOperator(filters.Equal).
			WithValueInt(int64(imageID))
	}

	query = query.
		WithWhere(filter).
		WithLimit(100_000)

	result, err := query.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("fehler beim erhalten des Bildes")
	}

	data := w.getData(result)
	if data[0].ID == "" {
		return nil, fmt.Errorf("fehler beim erhalten der ID: %w", err)
	}
	imageObj := w.Client.GraphQL().NearObjectArgBuilder().WithID(data[0].ID)

	result, err = w.Client.GraphQL().Get().
		WithClassName("Image").
		WithFields(
			graphql.Field{Name: "filepath"},
			graphql.Field{Name: "image"},
		).
		WithNearObject(imageObj).
		WithLimit(100_000). // Weaviate Standard-Limit ist 10 – explizit hochsetzen!
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("fehler beim finden der duplikate")
	}

	images := w.getData(result)
	var filtered []Image

	for _, img := range images {
		if img.ID != data[0].ID {
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
	
	props := map[string]any{
		"gallery_id": newID,
	}
	for _, img := range images {
		g.Go(func() error {
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

func (w *WeaviateClient) RemoveImages(ctx context.Context, images []Image) error {
	threads := runtime.NumCPU()
	maxWorkers := max(threads-2, threads/2)
	maxWorkers = max(maxWorkers, 1)

	g := new(errgroup.Group)
	g.SetLimit(maxWorkers)

	for _, image := range images {
		g.Go(func() error{
			return w.RemoveImage(ctx, image)
		})
	}
	
	return g.Wait()
}

func (w *WeaviateClient) GetAll(ctx context.Context, galleryID int) ([]Image, error) {
	result, err := w.Client.GraphQL().Get().
		WithClassName("Image").
		WithFields(
			graphql.Field{Name: "filepath"},
			graphql.Field{Name: "image"},
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
		WithLimit(100_000).
		Do(ctx)

	if err != nil {
		return nil, fmt.Errorf("fehler beim erhalt aller Bildern: %w", err)
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
		WithLimit(100_000). // Weaviate Standard-Limit ist 10 – explizit hochsetzen!
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("fehler bei der Weaviate abfrage: %w", err)
	}

	data := result.Data["Get"].(map[string]any)
	images := data["Image"].([]any)

	for _, imgObj := range images {
		img := imgObj.(map[string]any)
		path := img["filepath"].(string)
		knownPaths[path] = struct{}{}
	}

	return knownPaths, nil
}

func (w *WeaviateClient) ResetDatabase(ctx context.Context) error {
	className := "Image"

	exists, err := w.Client.Schema().ClassExistenceChecker().WithClassName(className).Do(ctx)
	if err != nil {
		return fmt.Errorf("fehler bei der überprüfung der klasse: %w", err)
	}

	if !exists {
		log.Println("Die Datenbank ist bereits leer (Klasse existiert nicht).")
		return nil
	}

	err = w.Client.Schema().ClassDeleter().WithClassName(className).Do(ctx)
	if err != nil {
		return fmt.Errorf("fehler beim löschen der datenbank: %w", err)
	}

	log.Println("Die Weaviate-Datenbank wurde erfolgreich und restlos geleert.")
	return nil
}
