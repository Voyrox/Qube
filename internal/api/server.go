package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/Voyrox/Qube/internal/config"
	"github.com/Voyrox/Qube/internal/core/cgroup"
	"github.com/Voyrox/Qube/internal/core/container"
	"github.com/Voyrox/Qube/internal/core/tracking"
	"github.com/fatih/color"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var (
	getTrackedEntries = tracking.GetAllTrackedEntries
	stopContainer     = container.StopContainer
	startContainer    = container.StartContainer
	deleteContainer   = container.DeleteContainer
	evalCommand       = container.EvalCommand
	memoryStats       = cgroup.GetMemoryStats
	memoryFromProc    = cgroup.GetMemoryFromProc
	cpuFromProc       = cgroup.GetCPUFromProc
)

type ContainerInfo struct {
	Name        string      `json:"name"`
	PID         int         `json:"pid"`
	Directory   string      `json:"directory"`
	Command     []string    `json:"command"`
	Image       string      `json:"image"`
	Timestamp   uint64      `json:"timestamp"`
	Ports       string      `json:"ports"`
	Isolated    bool        `json:"isolated"`
	Volumes     [][2]string `json:"volumes"`
	Environment []string    `json:"environment"`
	MemoryMB    *float64    `json:"memory_mb,omitempty"`
	CPUPercent  *float64    `json:"cpu_percent,omitempty"`
}

type Response struct {
	Containers []ContainerInfo `json:"containers"`
}

type CommandParams struct {
	ContainerID string `json:"container_id"`
	PID         int    `json:"pid"`
	Command     string `json:"command"`
}

func StartServer() {
	r := mux.NewRouter()

	r.Use(corsMiddleware)

	r.HandleFunc("/list", listContainersHandler).Methods("GET")
	r.HandleFunc("/stop", stopContainerHandler).Methods("POST")
	r.HandleFunc("/stop/{name}", stopContainerByNameHandler).Methods("POST")
	r.HandleFunc("/start", startContainerHandler).Methods("POST")
	r.HandleFunc("/start/{name}", startContainerByNameHandler).Methods("POST")
	r.HandleFunc("/delete", deleteContainerHandler).Methods("POST")
	r.HandleFunc("/info", containerInfoHandler).Methods("POST")
	r.HandleFunc("/images", listImagesHandler).Methods("GET")
	r.HandleFunc("/volumes", listVolumesHandler).Methods("GET")
	r.HandleFunc("/eval/{container}/{action}", evalWebSocketHandler)

	color.Green("API server is running at http://127.0.0.1:3030")

	if err := http.ListenAndServe("127.0.0.1:3030", r); err != nil {
		color.Red("Failed to start API server: %v", err)
		os.Exit(1)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func listContainersHandler(w http.ResponseWriter, r *http.Request) {
	entries := getTrackedEntries()

	var containers []ContainerInfo
	for _, entry := range entries {
		info := ContainerInfo{
			Name:        entry.Name,
			PID:         entry.PID,
			Directory:   entry.Dir,
			Command:     entry.Command,
			Image:       entry.Image,
			Timestamp:   entry.Timestamp,
			Ports:       entry.Ports,
			Isolated:    entry.Isolated,
			Volumes:     entry.Volumes,
			Environment: entry.EnvVars,
		}

		if entry.PID > 0 {
			if stats, err := memoryStats(entry.Name); err == nil {
				mb := stats.CurrentMB()
				info.MemoryMB = &mb
			} else if mem, err := memoryFromProc(entry.PID); err == nil && mem > 0 {
				mb := float64(mem) / (1024.0 * 1024.0)
				info.MemoryMB = &mb
			}

			cpu, err := cpuFromProc(entry.PID)
			if err == nil {
				info.CPUPercent = &cpu
			} else {
				zero := 0.0
				info.CPUPercent = &zero
			}
		}

		containers = append(containers, info)
	}

	writeJSON(w, http.StatusOK, Response{Containers: containers})
}

func stopContainerHandler(w http.ResponseWriter, r *http.Request) {
	var params CommandParams
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	pid, err := strconv.Atoi(params.ContainerID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid container ID")
		return
	}

	if err := stopContainer(pid); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to stop container: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

func stopContainerByNameHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	entries := getTrackedEntries()
	for _, entry := range entries {
		if entry.Name == name {
			if err := stopContainer(entry.PID); err != nil {
				writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to stop container: %v", err))
				return
			}

			writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
			return
		}
	}

	writeError(w, http.StatusNotFound, "Container not found")
}

func startContainerHandler(w http.ResponseWriter, r *http.Request) {
	var params CommandParams
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := startContainer(params.ContainerID); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to start container: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

func startContainerByNameHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	if err := startContainer(name); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to start container: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

func deleteContainerHandler(w http.ResponseWriter, r *http.Request) {
	var params CommandParams
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := deleteContainer(params.ContainerID); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete container: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

func containerInfoHandler(w http.ResponseWriter, r *http.Request) {
	var params CommandParams
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	entries := getTrackedEntries()
	for _, entry := range entries {
		if entry.Name == params.ContainerID || fmt.Sprintf("%d", entry.PID) == params.ContainerID {
			info := ContainerInfo{
				Name:        entry.Name,
				PID:         entry.PID,
				Directory:   entry.Dir,
				Command:     entry.Command,
				Image:       entry.Image,
				Timestamp:   entry.Timestamp,
				Ports:       entry.Ports,
				Isolated:    entry.Isolated,
				Volumes:     entry.Volumes,
				Environment: entry.EnvVars,
			}

			if entry.PID > 0 {
				if stats, err := memoryStats(entry.Name); err == nil {
					mb := stats.CurrentMB()
					info.MemoryMB = &mb
				}
			}

			writeJSON(w, http.StatusOK, Response{Containers: []ContainerInfo{info}})
			return
		}
	}

	writeError(w, http.StatusNotFound, "Container not found")
}

type ImageInfo struct {
	Name   string  `json:"name"`
	SizeMB float64 `json:"size_mb"`
	Path   string  `json:"path"`
}

func listImagesHandler(w http.ResponseWriter, r *http.Request) {
	imagesDir := imagesDir()
	var images []ImageInfo

	entries, err := os.ReadDir(imagesDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list images")
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		sizeMB := float64(info.Size()) / 1048576.0
		images = append(images, ImageInfo{
			Name:   entry.Name(),
			SizeMB: sizeMB,
			Path:   filepath.Join(imagesDir, entry.Name()),
		})
	}

	writeJSON(w, http.StatusOK, images)
}

type VolumeInfo struct {
	Name          string `json:"name"`
	HostPath      string `json:"host_path"`
	ContainerPath string `json:"container_path"`
	Container     string `json:"container"`
}

func listVolumesHandler(w http.ResponseWriter, r *http.Request) {
	entries := getTrackedEntries()
	var volumes []VolumeInfo

	for _, entry := range entries {
		for idx, volume := range entry.Volumes {
			volumes = append(volumes, VolumeInfo{
				Name:          fmt.Sprintf("vol-%d", idx),
				HostPath:      volume[0],
				ContainerPath: volume[1],
				Container:     entry.Name,
			})
		}
	}

	writeJSON(w, http.StatusOK, volumes)
}

func evalWebSocketHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	containerName := vars["container"]
	entries := getTrackedEntries()

	targetPID := findPIDByName(entries, containerName)
	if targetPID == 0 {
		writeError(w, http.StatusNotFound, "Container not found")
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		color.Red("Failed to upgrade to WebSocket: %v", err)
		return
	}
	defer conn.Close()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}

		cmd := string(message)
		if cmd == "" {
			continue
		}

		output, err := evalCommand(targetPID, cmd)
		if err != nil {
			conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Error: %v", err)))
		} else {
			conn.WriteMessage(websocket.TextMessage, []byte(output))
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func imagesDir() string {
	return filepath.Join(config.QubeContainersBase, "images")
}

func findPIDByName(entries []tracking.ContainerEntry, name string) int {
	for _, entry := range entries {
		if entry.Name == name {
			return entry.PID
		}
	}
	return 0
}
