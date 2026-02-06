package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Voyrox/Qube/internal/config"
	"github.com/Voyrox/Qube/internal/core/cgroup"
	"github.com/Voyrox/Qube/internal/core/container"
	"github.com/Voyrox/Qube/internal/core/tracking"
)

type testTrackedEntry struct {
	Name      string
	PID       int
	Dir       string
	Command   []string
	Timestamp uint64
	Image     string
	Ports     string
	Isolated  bool
	Volumes   [][2]string
	EnvVars   []string
}

func withTracked(entries []testTrackedEntry) func() {
	orig := getTrackedEntries
	getTrackedEntries = func() []tracking.ContainerEntry {
		var out []tracking.ContainerEntry
		for _, e := range entries {
			out = append(out, tracking.ContainerEntry{
				Name:      e.Name,
				PID:       e.PID,
				Dir:       e.Dir,
				Command:   e.Command,
				Timestamp: e.Timestamp,
				Image:     e.Image,
				Ports:     e.Ports,
				Isolated:  e.Isolated,
				Volumes:   e.Volumes,
				EnvVars:   e.EnvVars,
			})
		}
		return out
	}
	return func() { getTrackedEntries = orig }
}

func TestListContainersHandler(t *testing.T) {
	restore := withTracked([]testTrackedEntry{{Name: "c1", PID: 123, Dir: "/d", Command: []string{"echo"}, Image: "img", Ports: "8080", Isolated: true}})
	defer restore()

	memoryStats = func(name string) (*cgroup.MemoryStats, error) {
		return &cgroup.MemoryStats{CurrentBytes: 1024 * 1024}, nil
	}
	memoryFromProc = func(pid int) (uint64, error) { return 0, nil }
	cpuFromProc = func(pid int) (float64, error) { return 1.5, nil }
	defer func() {
		memoryStats = cgroup.GetMemoryStats
		memoryFromProc = cgroup.GetMemoryFromProc
		cpuFromProc = cgroup.GetCPUFromProc
	}()

	req := httptest.NewRequest(http.MethodGet, "/list", nil)
	w := httptest.NewRecorder()
	listContainersHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Containers) != 1 || resp.Containers[0].Name != "c1" || resp.Containers[0].PID != 123 {
		t.Fatalf("resp mismatch: %+v", resp)
	}
	if resp.Containers[0].CPUPercent == nil || *resp.Containers[0].CPUPercent != 1.5 {
		t.Fatalf("cpu missing: %+v", resp.Containers[0])
	}
	if resp.Containers[0].MemoryMB == nil || *resp.Containers[0].MemoryMB <= 0 {
		t.Fatalf("mem missing: %+v", resp.Containers[0])
	}
}

func TestStopContainerHandlerBadJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/stop", bytes.NewBufferString("not-json"))
	w := httptest.NewRecorder()
	stopContainerHandler(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestStopContainerHandlerInvalidID(t *testing.T) {
	body := bytes.NewBufferString(`{"container_id":"abc"}`)
	req := httptest.NewRequest(http.MethodPost, "/stop", body)
	w := httptest.NewRecorder()
	stopContainerHandler(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestStopContainerHandlerSuccess(t *testing.T) {
	stopCalled := 0
	stopContainer = func(pid int) error { stopCalled++; return nil }
	defer func() { stopContainer = container.StopContainer }()

	body := bytes.NewBufferString(`{"container_id":"5"}`)
	req := httptest.NewRequest(http.MethodPost, "/stop", body)
	w := httptest.NewRecorder()
	stopContainerHandler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if stopCalled != 1 {
		t.Fatalf("stop not called")
	}
}

func TestContainerInfoHandlerNotFound(t *testing.T) {
	restore := withTracked(nil)
	defer restore()

	body := bytes.NewBufferString(`{"container_id":"missing"}`)
	req := httptest.NewRequest(http.MethodPost, "/info", body)
	w := httptest.NewRecorder()
	containerInfoHandler(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestListImagesHandler(t *testing.T) {
	tmp := t.TempDir()
	prevBase := config.QubeContainersBase
	config.QubeContainersBase = tmp
	defer func() { config.QubeContainersBase = prevBase }()

	imgDir := filepath.Join(tmp, "images")
	_ = os.MkdirAll(imgDir, 0755)

	// create two files
	_ = os.WriteFile(filepath.Join(imgDir, "a.img"), []byte("1234"), 0644)
	_ = os.WriteFile(filepath.Join(imgDir, "b.img"), []byte("123456"), 0644)

	req := httptest.NewRequest(http.MethodGet, "/images", nil)
	w := httptest.NewRecorder()
	listImagesHandler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	var resp []ImageInfo
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp) != 2 {
		t.Fatalf("expected 2 images, got %d", len(resp))
	}
}

func TestListVolumesHandler(t *testing.T) {
	restore := withTracked([]testTrackedEntry{{Name: "c1", Volumes: [][2]string{{"/h", "/c"}}}})
	defer restore()

	req := httptest.NewRequest(http.MethodGet, "/volumes", nil)
	w := httptest.NewRecorder()
	listVolumesHandler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	var resp []VolumeInfo
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp) != 1 || resp[0].HostPath != "/h" || resp[0].ContainerPath != "/c" {
		t.Fatalf("vol mismatch: %+v", resp)
	}
}

func TestEvalWebSocketHandlerNotFound(t *testing.T) {
	restore := withTracked(nil)
	defer restore()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/eval/nope/x", nil)
	evalWebSocketHandler(w, r)
	if w.Code != http.StatusBadRequest && w.Body.Len() == 0 {
		t.Fatalf("expected failure on upgrade for missing container")
	}
}
