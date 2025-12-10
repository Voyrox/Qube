package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"

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
	r.HandleFunc("/start", startContainerHandler).Methods("POST")
	r.HandleFunc("/delete", deleteContainerHandler).Methods("POST")
	r.HandleFunc("/info", containerInfoHandler).Methods("POST")
	r.HandleFunc("/images", listImagesHandler).Methods("GET")
	r.HandleFunc("/volumes", listVolumesHandler).Methods("GET")
	r.HandleFunc("/eval", evalWebSocketHandler)

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
	entries := tracking.GetAllTrackedEntries()

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
			if stats, err := cgroup.GetMemoryStats(entry.Name); err == nil {
				mb := stats.CurrentMB()
				info.MemoryMB = &mb
			} else if mem, err := cgroup.GetMemoryFromProc(entry.PID); err == nil && mem > 0 {
				mb := float64(mem) / (1024.0 * 1024.0)
				info.MemoryMB = &mb
			}

			if cpu, err := cgroup.GetCPUFromProc(entry.PID); err == nil {
				info.CPUPercent = &cpu
			}
		}

		containers = append(containers, info)
	}

	response := Response{Containers: containers}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func stopContainerHandler(w http.ResponseWriter, r *http.Request) {
	var params CommandParams
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pid, err := strconv.Atoi(params.ContainerID)
	if err != nil {
		http.Error(w, "Invalid container ID", http.StatusBadRequest)
		return
	}

	if err := container.StopContainer(pid); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := Response{
		Containers: []ContainerInfo{{
			Name: "Container stopped",
			PID:  pid,
		}},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func startContainerHandler(w http.ResponseWriter, r *http.Request) {
	var params CommandParams
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := container.StartContainer(params.ContainerID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func deleteContainerHandler(w http.ResponseWriter, r *http.Request) {
	var params CommandParams
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := container.DeleteContainer(params.ContainerID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func containerInfoHandler(w http.ResponseWriter, r *http.Request) {
	var params CommandParams
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	entries := tracking.GetAllTrackedEntries()
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
				if stats, err := cgroup.GetMemoryStats(entry.Name); err == nil {
					mb := stats.CurrentMB()
					info.MemoryMB = &mb
				}
			}

			response := Response{Containers: []ContainerInfo{info}}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
	}

	http.Error(w, "Container not found", http.StatusNotFound)
}

type ImageInfo struct {
	Name   string  `json:"name"`
	SizeMB float64 `json:"size_mb"`
	Path   string  `json:"path"`
}

func listImagesHandler(w http.ResponseWriter, r *http.Request) {
	imagesDir := "/var/lib/qube/images"
	var images []ImageInfo

	if entries, err := os.ReadDir(imagesDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				info, err := entry.Info()
				if err != nil {
					continue
				}

				sizeMB := float64(info.Size()) / 1048576.0
				images = append(images, ImageInfo{
					Name:   entry.Name(),
					SizeMB: sizeMB,
					Path:   imagesDir + "/" + entry.Name(),
				})
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(images)
}

type VolumeInfo struct {
	Name          string `json:"name"`
	HostPath      string `json:"host_path"`
	ContainerPath string `json:"container_path"`
	Container     string `json:"container"`
}

func listVolumesHandler(w http.ResponseWriter, r *http.Request) {
	entries := tracking.GetAllTrackedEntries()
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(volumes)
}

func evalWebSocketHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		color.Red("Failed to upgrade to WebSocket: %v", err)
		return
	}
	defer conn.Close()

	for {
		var params CommandParams
		if err := conn.ReadJSON(&params); err != nil {
			break
		}

		output := fmt.Sprintf("Executed command in container %s: %s", params.ContainerID, params.Command)

		if err := conn.WriteJSON(map[string]string{"output": output}); err != nil {
			break
		}
	}
}
