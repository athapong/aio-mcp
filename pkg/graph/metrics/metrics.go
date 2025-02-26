package metrics

import (
	"runtime"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// System metrics
	SystemMemoryUsage = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "system_memory_bytes",
		Help: "Current system memory usage",
	})

	SystemGoroutines = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "system_goroutines",
		Help: "Number of goroutines",
	})

	// Pipeline metrics
	PipelineQueueLength = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "pipeline_queue_length",
		Help: "Number of documents waiting to be processed",
	})

	DocumentProcessingErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "document_processing_errors_total",
			Help: "Total number of document processing errors",
		},
		[]string{"processor", "error_type"},
	)

	// Graph metrics
	GraphNodeCount = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "graph_nodes_total",
			Help: "Total number of nodes in the graph",
		},
		[]string{"node_type"},
	)

	GraphEdgeCount = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "graph_edges_total",
			Help: "Total number of edges in the graph",
		},
		[]string{"edge_type"},
	)

	// Cache metrics
	CacheHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_hits_total",
			Help: "Number of cache hits",
		},
		[]string{"cache_type"},
	)

	CacheMisses = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_misses_total",
			Help: "Number of cache misses",
		},
		[]string{"cache_type"},
	)
)

// UpdateMetrics updates system-level metrics
func UpdateSystemMetrics() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	SystemMemoryUsage.Set(float64(m.Alloc))
	SystemGoroutines.Set(float64(runtime.NumGoroutine()))
}
