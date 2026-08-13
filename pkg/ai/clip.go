package ai

import (
	"bytes"
	"fmt"
	"image/jpeg"
	"math"
	"sync"

	"github.com/h2non/bimg"
	ort "github.com/yalue/onnxruntime_go"
)

type CLIPEmbedder struct {
	mu sync.Mutex

	imageSession      *ort.AdvancedSession
	imageInputTensor  *ort.Tensor[float32]
	imageOutputTensor *ort.Tensor[float32]
	imageInputBuffer  []float32
	imageOutputBuffer []float32

	textSession      *ort.AdvancedSession
	textInputTensor  *ort.Tensor[int64]
	textOutputTensor *ort.Tensor[float32]
	textInputBuffer  []int64
	textOutputBuffer []float32

	tokenizer *CLIPTokenizer
}

func NewCLIPEmbedder(visualModelPath string, textualModelPath string, onnxLibPath string) (*CLIPEmbedder, error) {
	if !ort.IsInitialized() {
		if onnxLibPath != "" {
			ort.SetSharedLibraryPath(onnxLibPath)
		}
		if err := ort.InitializeEnvironment(); err != nil {
			return nil, fmt.Errorf("failed to init onnx env: %w", err)
		}
	}

	tokenizer, err := NewCLIPTokenizer()
	if err != nil {
		return nil, fmt.Errorf("failed to init tokenizer: %w", err)
	}

	imageInputShape := ort.NewShape(1, 3, 224, 224)
	imageInputBuffer := make([]float32, 1*3*224*224)
	imageInputTensor, err := ort.NewTensor(imageInputShape, imageInputBuffer)
	if err != nil {
		return nil, fmt.Errorf("failed to create input tensor: %w", err)
	}

	imageOutputShape := ort.NewShape(1, 512)
	imageOutputBuffer := make([]float32, 512)
	imageOutputTensor, err := ort.NewTensor(imageOutputShape, imageOutputBuffer)
	if err != nil {
		_ = imageInputTensor.Destroy()
		return nil, fmt.Errorf("failed to create output tensor: %w", err)
	}

	imageSession, err := ort.NewAdvancedSession(
		visualModelPath,
		[]string{"pixel_values"},
		[]string{"image_embeds"},
		[]ort.ArbitraryTensor{imageInputTensor},
		[]ort.ArbitraryTensor{imageOutputTensor},
		nil,
	)
	if err != nil {
		_ = imageInputTensor.Destroy()
		_ = imageOutputTensor.Destroy()
		return nil, fmt.Errorf("failed to load visual onnx model: %w", err)
	}

	textInputShape := ort.NewShape(1, 77)
	textInputBuffer := make([]int64, 77)
	textInputTensor, err := ort.NewTensor(textInputShape, textInputBuffer)
	if err != nil {
		_ = imageSession.Destroy()
		_ = imageInputTensor.Destroy()
		_ = imageOutputTensor.Destroy()
		return nil, err
	}

	textOutputShape := ort.NewShape(1, 512)
	textOutputBuffer := make([]float32, 512)
	textOutputTensor, err := ort.NewTensor(textOutputShape, textOutputBuffer)
	if err != nil {
		_ = imageSession.Destroy()
		_ = textInputTensor.Destroy()
		_ = imageInputTensor.Destroy()
		_ = imageOutputTensor.Destroy()
		return nil, err
	}

	textSession, err := ort.NewAdvancedSession(
		textualModelPath,
		[]string{"input_ids"},
		[]string{"text_embeds"},
		[]ort.ArbitraryTensor{textInputTensor},
		[]ort.ArbitraryTensor{textOutputTensor},
		nil,
	)
	if err != nil {
		_ = imageSession.Destroy()
		_ = textInputTensor.Destroy()
		_ = textOutputTensor.Destroy()
		_ = imageInputTensor.Destroy()
		_ = imageOutputTensor.Destroy()
		return nil, fmt.Errorf("failed to load textual model: %w", err)
	}

	return &CLIPEmbedder{
		imageSession:      imageSession,
		imageInputTensor:  imageInputTensor,
		imageOutputTensor: imageOutputTensor,
		imageInputBuffer:  imageInputBuffer,
		imageOutputBuffer: imageOutputBuffer,
		textSession:       textSession,
		textInputTensor:   textInputTensor,
		textOutputTensor:  textOutputTensor,
		textInputBuffer:   textInputBuffer,
		textOutputBuffer:  textOutputBuffer,
		tokenizer:         tokenizer,
	}, nil
}

func (c *CLIPEmbedder) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.imageSession != nil {
		_ = c.imageSession.Destroy()
	}
	if c.imageInputTensor != nil {
		_ = c.imageInputTensor.Destroy()
	}
	if c.imageOutputTensor != nil {
		_ = c.imageOutputTensor.Destroy()
	}
	if c.textSession != nil {
		_ = c.textSession.Destroy()
	}
	if c.textInputTensor != nil {
		_ = c.textInputTensor.Destroy()
	}
	if c.textOutputTensor != nil {
		_ = c.textOutputTensor.Destroy()
	}
}

func (c *CLIPEmbedder) EmbedImage(imageBytes []byte) ([]float32, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	bimgImg := bimg.NewImage(imageBytes)
	resizedBytes, err := bimgImg.Process(bimg.Options{
		Width:         224,
		Height:        224,
		Crop:          true,
		Type:          bimg.JPEG,
		StripMetadata: true,
	})
	if err != nil {
		return nil, fmt.Errorf("bimg resize failed: %w", err)
	}

	img, err := jpeg.Decode(bytes.NewReader(resizedBytes))
	if err != nil {
		return nil, fmt.Errorf("jpeg decode failed: %w", err)
	}

	channelSize := 224 * 224
	for y := 0; y < 224; y++ {
		for x := 0; x < 224; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			rf := float32(r>>8) / 255.0
			gf := float32(g>>8) / 255.0
			bf := float32(b>>8) / 255.0

			idx := y*224 + x
			c.imageInputBuffer[0*channelSize+idx] = (rf - 0.48145466) / 0.26862954
			c.imageInputBuffer[1*channelSize+idx] = (gf - 0.45782750) / 0.26130258
			c.imageInputBuffer[2*channelSize+idx] = (bf - 0.40821073) / 0.27577711
		}
	}

	if err := c.imageSession.Run(); err != nil {
		return nil, fmt.Errorf("onnx image inference failed: %w", err)
	}

	result := make([]float32, 512)
	copy(result, c.imageOutputBuffer)
	normalizeL2(result)

	return result, nil
}

func (c *CLIPEmbedder) EmbedText(prompt string) ([]float32, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	tokens, err := c.tokenizer.Encode(prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to encode prompt: %w", err)
	}

	for i := 0; i < 77; i++ {
		if i < len(tokens) {
			c.textInputBuffer[i] = int64(tokens[i])
		} else {
			c.textInputBuffer[i] = 0
		}
	}

	if err := c.textSession.Run(); err != nil {
		return nil, fmt.Errorf("onnx text inference failed: %w", err)
	}

	result := make([]float32, 512)
	copy(result, c.textOutputBuffer)
	normalizeL2(result)

	return result, nil
}

func normalizeL2(vec []float32) {
	var sum float64
	for _, val := range vec {
		sum += float64(val * val)
	}
	if sum <= 0 {
		return
	}
	norm := float32(math.Sqrt(sum))
	for i := range vec {
		vec[i] /= norm
	}
}
