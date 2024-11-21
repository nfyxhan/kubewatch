package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	fieldValue = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "field_values",
			Help: "field values",
		},
		[]string{"group", "version", "kind", "namespace", "name", "field"},
	)
)

func GetMetricsFieldValues() *prometheus.GaugeVec {
	return fieldValue
}

func init() {
	metrics.Registry.MustRegister(fieldValue)
}
