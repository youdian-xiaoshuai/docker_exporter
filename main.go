package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	containerStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "docker_container_status",
			Help: "Status of Docker containers",
		},
		[]string{"container_id", "container_name"},
	)
	containerCPU = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "docker_container_cpu_usage",
			Help: "CPU usage of Docker containers",
		},
		[]string{"container_id", "container_name"},
	)
	containerMemory = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "docker_container_memory_usage",
			Help: "Memory usage of Docker containers",
		},
		[]string{"container_id", "container_name"},
	)
)

func init() {
	prometheus.MustRegister(containerStatus)
	prometheus.MustRegister(containerCPU)
	prometheus.MustRegister(containerMemory)
}

func collectMetrics() {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("Error creating Docker client: %v", err)
	}

	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		log.Fatalf("Error listing containers: %v", err)
	}

	for _, container := range containers {
		containerID := container.ID
		containerName := strings.TrimPrefix(container.Names[0], "/")
		containerState := container.State

		stats, err := cli.ContainerStatsOneShot(context.Background(), containerID)
		if err != nil {
			log.Printf("Error getting container stats: %v", err)
			continue
		}
		defer stats.Body.Close()

		var v types.StatsJSON
		err = json.NewDecoder(stats.Body).Decode(&v)
		if err != nil {
			log.Printf("Error decoding container stats: %v", err)
			continue
		}

		cpuUsage := v.CPUStats.CPUUsage.TotalUsage
		memoryUsage := v.MemoryStats.Usage

		containerStatus.WithLabelValues(containerID, containerName).Set(func() float64 {
			if containerState == "running" {
				return 1
			}
			return 0
		}())
		containerCPU.WithLabelValues(containerID, containerName).Set(float64(cpuUsage))
		containerMemory.WithLabelValues(containerID, containerName).Set(float64(memoryUsage))
	}
}

func main() {
	go func() {
		for {
			collectMetrics()
			time.Sleep(10 * time.Second)
		}
	}()

	http.Handle("/metrics", promhttp.Handler())
	log.Println("Starting server on :8000")
	log.Fatal(http.ListenAndServe(":8000", nil))
}
