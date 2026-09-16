package ai

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

const (
	onnxRuntimeVersion = "1.22.0"
	visionModelURL     = "https://huggingface.co/Xenova/clip-vit-base-patch32/resolve/main/onnx/vision_model.onnx"
	textModelURL       = "https://huggingface.co/Xenova/clip-vit-base-patch32/resolve/main/onnx/text_model.onnx"
)

// ortRelease describes where to fetch the ONNX Runtime shared library for
// the current platform and how to locate it inside the downloaded archive.
type ortRelease struct {
	url         string
	libFileName string // filename the runtime library is saved as in libDir

	// tar.gz archives (linux, darwin): libMatch identifies the real
	// library entry (as opposed to a symlink or debug-symbol bundle).
	libMatch func(archivePath string, size int64) bool

	// providersFile/providersOut: the (optional) separate providers_shared
	// library some platforms ship. Empty when the platform doesn't need one.
	providersFile string
	providersOut  string
}

func ortReleaseFor(goos string) (ortRelease, error) {
	base := "https://github.com/microsoft/onnxruntime/releases/download/v" + onnxRuntimeVersion

	switch goos {
	case "windows":
		return ortRelease{
			url:           base + "/onnxruntime-win-x64-" + onnxRuntimeVersion + ".zip",
			libFileName:   "onnxruntime.dll",
			providersFile: "lib/onnxruntime_providers_shared.dll",
			providersOut:  "onnxruntime_providers_shared.dll",
		}, nil
	case "darwin":
		return ortRelease{
			url:         base + "/onnxruntime-osx-universal2-" + onnxRuntimeVersion + ".tgz",
			libFileName: "libonnxruntime.dylib",
			libMatch: func(archivePath string, size int64) bool {
				name := filepath.Base(archivePath)
				return strings.HasPrefix(name, "libonnxruntime.") && strings.HasSuffix(name, ".dylib") &&
					name != "libonnxruntime.dylib" && !strings.Contains(archivePath, ".dSYM/")
			},
		}, nil
	case "linux":
		return ortRelease{
			url:         base + "/onnxruntime-linux-x64-" + onnxRuntimeVersion + ".tgz",
			libFileName: "libonnxruntime.so",
			libMatch: func(archivePath string, size int64) bool {
				return strings.HasPrefix(filepath.Base(archivePath), "libonnxruntime.so.") && size > 1_000_000
			},
			providersFile: "lib/libonnxruntime_providers_shared.so",
			providersOut:  "libonnxruntime_providers_shared.so",
		}, nil
	default:
		return ortRelease{}, fmt.Errorf("unsupported platform for ONNX Runtime: %s", goos)
	}
}

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
	release, err := ortReleaseFor(runtime.GOOS)
	if err != nil {
		return nil, err
	}

	cacheBase, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get cache dir: %w", err)
	}

	baseDir := filepath.Join(cacheBase, "acuity")
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
	libPath := filepath.Join(libDir, release.libFileName)

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
		slog.Info("Downloading ONNX Runtime shared library...", slog.String("url", release.url), slog.String("platform", runtime.GOOS))
		updateStatus(func(s *AIStatus) {
			s.Step = 1
			s.CurrentTask = "ONNX Runtime Engine"
			s.Percent = 0
			s.CurrentBytes = 0
			s.TotalBytes = 0
		})

		var extractErr error
		if strings.HasSuffix(release.url, ".zip") {
			extractErr = downloadAndExtractZip(release.url, libDir, map[string]string{
				release.providersFile: release.providersOut,
			}, release.libFileName)
		} else {
			extractErr = downloadAndExtractTarGz(release.url, libDir, release.libMatch, release.libFileName, release.providersFile, release.providersOut)
		}
		if extractErr != nil {
			updateStatus(func(s *AIStatus) { s.Error = extractErr.Error(); s.Downloading = false })
			return nil, fmt.Errorf("failed to download onnxruntime lib: %w", extractErr)
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

// downloadAndExtractTarGz streams a .tgz release, writing the entry matched
// by libMatch to destDir/libOut, and (if providersSuffix is non-empty) the
// entry whose path ends with providersSuffix to destDir/providersOut.
func downloadAndExtractTarGz(url string, destDir string, libMatch func(path string, size int64) bool, libOut string, providersSuffix string, providersOut string) error {
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
		if header.Typeflag != tar.TypeReg {
			continue
		}

		switch {
		case libMatch(header.Name, header.Size):
			if err := writeFile(filepath.Join(destDir, libOut), tr); err != nil {
				return err
			}
			found = true
		case providersSuffix != "" && strings.HasSuffix(header.Name, providersSuffix):
			if err := writeFile(filepath.Join(destDir, providersOut), tr); err != nil {
				return err
			}
		}
	}
	if !found {
		return fmt.Errorf("runtime library not found in archive %s", url)
	}
	return nil
}

// downloadAndExtractZip downloads a .zip release to a temp file (zip needs
// random access) and extracts libOut plus any entries named in extra
// (archive path suffix -> output filename).
func downloadAndExtractZip(url string, destDir string, extra map[string]string, libOut string) error {
	tmpFile, err := os.CreateTemp("", "onnxruntime-*.zip")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	resp, err := http.Get(url)
	if err != nil {
		tmpFile.Close()
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		tmpFile.Close()
		return fmt.Errorf("bad status downloading %s: %s", url, resp.Status)
	}

	pReader := &progressReader{
		reader: resp.Body,
		total:  resp.ContentLength,
		onProg: func(cur, tot int64, pct float64) {
			updateStatus(func(s *AIStatus) {
				s.CurrentBytes = cur
				s.TotalBytes = tot
				s.Percent = pct
			})
		},
	}

	if _, err := io.Copy(tmpFile, pReader); err != nil {
		tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}

	r, err := zip.OpenReader(tmpPath)
	if err != nil {
		return err
	}
	defer r.Close()

	found := false
	for _, f := range r.File {
		outName, wantMain := "", false
		if strings.HasSuffix(f.Name, "/lib/"+libOut) || filepath.Base(f.Name) == libOut {
			outName, wantMain = libOut, true
		} else if extraOut, ok := extra[f.Name]; ok {
			outName = extraOut
		} else {
			for suffix, mappedOut := range extra {
				if suffix != "" && strings.HasSuffix(f.Name, suffix) {
					outName = mappedOut
					break
				}
			}
		}
		if outName == "" {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}
		err = writeFile(filepath.Join(destDir, outName), rc)
		rc.Close()
		if err != nil {
			return err
		}
		if wantMain {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("runtime library %s not found in archive %s", libOut, url)
	}
	return nil
}

func writeFile(dest string, r io.Reader) error {
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, r)
	return err
}
