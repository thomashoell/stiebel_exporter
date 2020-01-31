package metrics

import (
	"github.com/brutella/can"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"net/http"
	"time"
	"wlp_exporter/stiebel"
)

type modelInfo struct {
	customerId      string
	manufacturerId  string
	softwareNumber  string
	softwareVersion string
}

const (
	namespace = "stiebel"
)

var (
	model    modelInfo
	registry = prometheus.NewRegistry()
	handler  = promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		ErrorHandling: promhttp.HTTPErrorOnError,
	})
	status *prometheus.GaugeVec
)

func Handler() http.Handler {
	return handler
}

func GatherMetrics(bus *can.Bus) {
	status = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "up",
		Help:      "meta metric",
	},
		[]string{
			"customer_id",
			"manufacturer_id",
			"software_number",
			"software_version",
		})

	registry.MustRegister(status)

	bus.SubscribeFunc(metricsHandler)

	go func() {
		for true {
			//requestLoop(status)
			time.Sleep(10 * time.Second)
		}
	}()

	log.Info("starting to gather metrics")
	_ = bus.ConnectAndPublish()
}

func metricsHandler(frm can.Frame) {
	stiebel.LogFrame(frm)
}

