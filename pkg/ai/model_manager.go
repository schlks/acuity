package ai

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	onnxLibURL     = "https://github.com/microsoft/onnxruntime/releases/download/v1.22.0/onnxruntime-linux-x64-1.22.0.tgz"
	visionModelURL = "https://huggingface.co/Xenova/clip-vit-base-patch32/resolve/main/onnx/vision_model.onnx"
	textModelURL   = "https://huggingface.co/Xenova/clip-vit-base-patch32/resolve/main/onnx/text_model.onnx"
)

type AIStatus struct {
	Ready        bool    `json:"ready"`
	Downloading  bool    `json:"downloading"`
	CurrentTask  string  `json:"current_task"`
	CurrentBytes int64   `json:"current_bytes"`
	TotalBytes   int64   `json:"total_bytes"`
	Percent      float64 `json:"percent"`
	Step         int     `json:"step"`
	TotalSteps   int     `json:"total_steps"`
	Error        string  `json:"error,omitempty"`
}

var (
	statusMu sync.RWMutex
	status   = AIStatus{
		Ready:       false,
		Downloading: false,
		TotalSteps:  3,
	}
)

func GetStatus() AIStatus {
	statusMu.RLock()
	defer statusMu.RUnlock()
	return status
}

func updateStatus(fn func(*AIStatus)) {
	statusMu.Lock()
	defer statusMu.Unlock()
	fn(&status)
}

type ModelPaths struct {
	VisualModelPath  string
	TextualModelPath string
	OnnxLibPath      string
}

type progressReader struct {
	reader  io.Reader
	total   int64
	current int64
	onProg  func(current, total int64, percent float64)
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.current += int64(n)
	if pr.total > 0 && pr.onProg != nil {
		pct := float64(pr.current) / float64(pr.total) * 100.0
		pr.onProg(pr.current, pr.total, pct)
	}
	return n, err
}

func EnsureModels() (*ModelPaths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home dir: %w", err)
	}

	baseDir := filepath.Join(home, ".local", "share", "acuity")
	modelsDir := filepath.Join(baseDir, "models")
	libDir := filepath.Join(baseDir, "lib")

	if err := os.MkdirAll(modelsDir, 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(libDir, 0755); err != nil {
		return nil, err
	}

	visPath := filepath.Join(modelsDir, "vision_model.onnx")
	textPath := filepath.Join(modelsDir, "text_model.onnx")
	libPath := filepath.Join(libDir, "libonnxruntime.so")

	needLib := false
	needVis := false
	needText := false

	if info, err := os.Stat(libPath); os.IsNotExist(err) || (err == nil && info.Size() < 1000000) {
		needLib = true
		_ = os.Remove(libPath)
	}
	if info, err := os.Stat(visPath); os.IsNotExist(err) || (err == nil && info.Size() < 1000000) {
		needVis = true
		_ = os.Remove(visPath)
	}
	if info, err := os.Stat(textPath); os.IsNotExist(err) || (err == nil && info.Size() < 1000000) {
		needText = true
		_ = os.Remove(textPath)
	}

	if needLib || needVis || needText {
		updateStatus(func(s *AIStatus) {
			s.Downloading = true
			s.Ready = false
		})
	}

	// 1. ONNX Runtime Library
	if needLib {
		slog.Info("Downloading ONNX Runtime shared library...", slog.String("url", onnxLibURL))
		updateStatus(func(s *AIStatus) {
			s.Step = 1
			s.CurrentTask = "ONNX Runtime Engine"
			s.Percent = 0
			s.CurrentBytes = 0
			s.TotalBytes = 0
		})
		if err := downloadAndExtract(onnxLibURL, libDir, "libonnxruntime.so"); err != nil {
			updateStatus(func(s *AIStatus) { s.Error = err.Error(); s.Downloading = false })
			return nil, fmt.Errorf("failed to download onnxruntime lib: %w", err)
		}
	}

	// 2. Vision Model
	if needVis {
		slog.Info("Downloading CLIP Vision Model...", slog.String("dest", visPath))
		updateStatus(func(s *AIStatus) {
			s.Step = 2
			s.CurrentTask = "CLIP Vision Model (~150 MB)"
			s.Percent = 0
			s.CurrentBytes = 0
			s.TotalBytes = 0
		})
		if err := downloadFile(visionModelURL, visPath); err != nil {
			updateStatus(func(s *AIStatus) { s.Error = err.Error(); s.Downloading = false })
			return nil, fmt.Errorf("failed to download vision model: %w", err)
		}
	}

	// 3. Text Model
	if needText {
		slog.Info("Downloading CLIP Text Model...", slog.String("dest", textPath))
		updateStatus(func(s *AIStatus) {
			s.Step = 3
			s.CurrentTask = "CLIP Text Model (~150 MB)"
			s.Percent = 0
			s.CurrentBytes = 0
			s.TotalBytes = 0
		})
		if err := downloadFile(textModelURL, textPath); err != nil {
			updateStatus(func(s *AIStatus) { s.Error = err.Error(); s.Downloading = false })
			return nil, fmt.Errorf("failed to download text model: %w", err)
		}
	}

	updateStatus(func(s *AIStatus) {
		s.Ready = true
		s.Downloading = false
		s.CurrentTask = "Ready"
		s.Percent = 100
		s.Step = 3
	})

	return &ModelPaths{
		VisualModelPath:  visPath,
		TextualModelPath: textPath,
		OnnxLibPath:      libPath,
	}, nil
}

func downloadFile(url string, dest string) error {
	tmpDest := dest + ".tmp"
	defer os.Remove(tmpDest)

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status downloading %s: %s", url, resp.Status)
	}

	totalBytes := resp.ContentLength
	pReader := &progressReader{
		reader: resp.Body,
		total:  totalBytes,
		onProg: func(cur, tot int64, pct float64) {
			updateStatus(func(s *AIStatus) {
				s.CurrentBytes = cur
				s.TotalBytes = tot
				s.Percent = pct
			})
		},
	}

	out, err := os.Create(tmpDest)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, pReader); err != nil {
		return err
	}

	return os.Rename(tmpDest, dest)
}

func downloadAndExtract(url string, destDir string, targetPrefix string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	totalBytes := resp.ContentLength
	pReader := &progressReader{
		reader: resp.Body,
		total:  totalBytes,
		onProg: func(cur, tot int64, pct float64) {
			updateStatus(func(s *AIStatus) {
				s.CurrentBytes = cur
				s.TotalBytes = tot
				s.Percent = pct
			})
		},
	}

	gz, err := gzip.NewReader(pReader)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	found := false
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		baseName := filepath.Base(header.Name)
		if header.Typeflag == tar.TypeReg && strings.HasPrefix(baseName, "libonnxruntime.so.") && header.Size > 1000000 {
			destFile := filepath.Join(destDir, "libonnxruntime.so")
			out, err := os.OpenFile(destFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
			found = true
		} else if header.Typeflag == tar.TypeReg && baseName == "libonnxruntime_providers_shared.so" {
			destFile := filepath.Join(destDir, "libonnxruntime_providers_shared.so")
			out, err := os.OpenFile(destFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}
	if !found {
		return fmt.Errorf("file %s not found in archive", targetPrefix)
	}
	return nil
}
