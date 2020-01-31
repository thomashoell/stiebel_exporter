package metrics

import (
	"fmt"
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
	sender    = 0x680
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
			requestLoop(status)
			time.Sleep(10 * time.Second)
		}
	}()

	log.Info("starting to gather metrics")
	_ = bus.ConnectAndPublish()
}

func metricsHandler(frm can.Frame) {
	data := stiebel.trimSuffix(frm.Data[:], 0x00)
	length := fmt.Sprintf("[%x]", frm.Length)

	chars := fmt.Sprintf("'%s'", stiebel.printableString(data[:]))
	rcvr := stiebel.ReceiverId(frm.Data[:2])

	if rcvr != sender {
		return
	}

	formatted := fmt.Sprintf("%-4x %-3s % -24X %-10s %6x ", frm.ID, length, data, chars, rcvr)

	reg, payload := stiebel.Payload(data)
	formatted += fmt.Sprintf("%04X ", reg)

	if data[0]&stiebel.Response != 0 {
		if r := stiebel.Reading(reg); r != nil {
			val := stiebel.DecodeValue(payload, r.Type)

			if val == nil {
				return
			}

			valStr := stiebel.ValueString(val)

			switch r.Name {
			case "KUNDEN_KENNUNG":
				model.customerId = valStr
			case "HERSTELLER_KENNUNG":
				model.manufacturerId = valStr
			case "SOFTWARE_NUMMER":
				model.softwareNumber = valStr
			case "SOFTWARE_VERSION":
				model.softwareVersion = valStr
			}

			formatted += fmt.Sprintf("%-24s %11s", stiebel.left(r.Name, 20), valStr)
		}
	}

	log.Info(formatted)
}

