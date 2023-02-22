package graphql

import "github.com/prometheus/client_golang/prometheus"

var (
	gqlLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Subsystem: "",
		Name:      "gql_latency_seconds",
		Help:      "Duration histogram for GQL client",
		Buckets:   []float64{0.2, 0.5, 1.0, 2.5, 5.0, 10.0}},
		[]string{"operation", "code", "method"})

	gqlRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gql_requests_total",
			Help: "Number of HTTP requests, partitioned by operation, code, method.",
		},
		[]string{"operation", "code", "method"})

	gqlResponseErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gql_errors_total",
			Help: "Number of Graphql error, partitioned by operation, code, method.",
		},
		[]string{"operation", "code", "method"})
)
