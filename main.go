// wlp_exporter reads values from a Stiebel Eltron heat pump over the CAN bus and exports them as
// Prometheus metrics.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/brutella/can"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/thomashoell/wlp_exporter/metrics"
)

// set by the build (-ldflags "-X main.gitTag=... -X main.gitCommit=...")
var (
	gitTag    = "dev"
	gitCommit = "unknown"
)

type config struct {
	device      string
	listen      string
	interval    time.Duration
	logLevel    slog.Level
	showVersion bool
}

func parseFlags(args []string, output io.Writer) (config, error) {
	var cfg config
	fs := flag.NewFlagSet("wlp_exporter", flag.ContinueOnError)
	fs.SetOutput(output)
	fs.StringVar(&cfg.device, "device", "can0", "CAN interface connected to the heat pump")
	fs.StringVar(&cfg.listen, "listen", ":9000", "address of the metrics endpoint")
	fs.DurationVar(&cfg.interval, "interval", 10*time.Second, "interval between two polls of the heat pump")
	fs.TextVar(&cfg.logLevel, "log-level", slog.LevelInfo, "log level (debug, info, warn, error)")
	fs.BoolVar(&cfg.showVersion, "version", false, "print the version and exit")

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	if fs.NArg() > 0 {
		return cfg, fmt.Errorf("unexpected arguments: %v", fs.Args())
	}
	if cfg.interval <= 0 {
		return cfg, fmt.Errorf("interval must be positive, got %s", cfg.interval)
	}
	return cfg, nil
}

func main() {
	cfg, err := parseFlags(os.Args[1:], os.Stderr)
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	if err != nil {
		os.Exit(2)
	}
	if cfg.showVersion {
		fmt.Printf("wlp_exporter %s (commit %s, %s)\n", gitTag, gitCommit, runtime.Version())
		return
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.logLevel}))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, cfg, logger); err != nil {
		logger.Error("exporter failed", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg config, logger *slog.Logger) error {
	logger.Info("starting wlp_exporter", "version", gitTag, "commit", gitCommit, "device", cfg.device)

	bus, err := can.NewBusForInterfaceWithName(cfg.device)
	if err != nil {
		return fmt.Errorf("cannot open CAN interface %s: %w", cfg.device, err)
	}

	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", cfg.listen)
	if err != nil {
		return err
	}

	return serve(ctx, cfg, logger, bus, ln)
}

// serve exports the metrics on ln and polls the heat pump over bus until ctx is canceled or an error occurs.
func serve(ctx context.Context, cfg config, logger *slog.Logger, bus *can.Bus, ln net.Listener) error {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		buildInfo(),
	)

	exporter, err := metrics.New(reg, bus, logger)
	if err != nil {
		return err
	}
	bus.SubscribeFunc(exporter.HandleFrame)

	srv := &http.Server{
		Handler:           newHandler(reg),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	errs := make(chan error, 2)

	go func() {
		logger.Info("listening", "address", ln.Addr().String())
		if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
			errs <- fmt.Errorf("metrics endpoint: %w", err)
		}
	}()

	go func() {
		// returns when the bus is disconnected or reading from the interface fails
		err := bus.ConnectAndPublish()
		if ctx.Err() == nil {
			errs <- fmt.Errorf("CAN bus closed: %w", err)
		}
	}()

	polled := make(chan struct{})
	go func() {
		defer close(polled)
		logger.Info("starting to poll the heat pump", "interval", cfg.interval)
		exporter.Run(ctx, cfg.interval)
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutting down")
		err = nil
	case err = <-errs:
	}

	cancel()
	<-polled

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	return errors.Join(err, srv.Shutdown(shutdownCtx), bus.Disconnect())
}

func newHandler(reg *prometheus.Registry) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{
		ErrorHandling: promhttp.HTTPErrorOnError,
	}))
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, `<html><head><title>wlp_exporter</title></head>`+
			`<body><h1>wlp_exporter</h1><p><a href="/metrics">Metrics</a></p></body></html>`)
	})
	return mux
}

func buildInfo() prometheus.Collector {
	g := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "stiebel",
		Subsystem: "exporter",
		Name:      "build_info",
		Help:      "Version of wlp_exporter, always 1.",
	}, []string{"version", "revision", "goversion"})
	g.WithLabelValues(gitTag, gitCommit, runtime.Version()).Set(1)
	return g
}
