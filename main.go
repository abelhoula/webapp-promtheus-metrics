package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

type Response struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

var (
	appVersion string
	version    = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "version",
		Help: "Version information about this binary",
		ConstLabels: map[string]string{
			"version": appVersion,
		},
	})

	httpRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Count of all HTTP requests",
	}, []string{"code", "method"})

	httpRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Duration of all HTTP requests",
		Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5},
	}, []string{"code", "handler", "method"})
)

func main() {
	version.Set(1)
	bind := ""
	enableH2c := false
	flagset := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flagset.StringVar(&bind, "bind", ":8080", "The socket to bind to.")
	flagset.BoolVar(&enableH2c, "h2c", false, "Enable h2c (http/2 over tcp) protocol.")
	flagset.Parse(os.Args[1:])

	r := prometheus.NewRegistry()
	r.MustRegister(httpRequestsTotal)
	r.MustRegister(httpRequestDuration)
	r.MustRegister(version)

	foundHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello from sample promttp impl."))
	})
	notfoundHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := &Response{Code: 400, Msg: "notFound"}
		jsonResp, err := json.Marshal(resp)
		if err != nil {
			http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		w.Write(jsonResp)
	})
	internalErrorHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	chain400x := promhttp.InstrumentHandlerDuration(
		httpRequestDuration.MustCurryWith(prometheus.Labels{"handler": "notfound"}),
		promhttp.InstrumentHandlerCounter(httpRequestsTotal, notfoundHandler),
	)
	chain500x := promhttp.InstrumentHandlerDuration(
		httpRequestDuration.MustCurryWith(prometheus.Labels{"handler": "notfound"}),
		promhttp.InstrumentHandlerCounter(httpRequestsTotal, internalErrorHandler),
	)
	foundChain := promhttp.InstrumentHandlerDuration(
		httpRequestDuration.MustCurryWith(prometheus.Labels{"handler": "found"}),
		promhttp.InstrumentHandlerCounter(httpRequestsTotal, foundHandler),
	)
	httpok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Start a new session by incrementing the activeSessions gauge
		resp := &Response{Code: 200, Msg: "ok!"}
		jsonResp, err := json.Marshal(resp)
		if err != nil {
			http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(jsonResp)
	})

	mux := http.NewServeMux()
	mux.Handle("/", foundChain)
	mux.Handle("/400x", chain400x)
	mux.Handle("/500x", chain500x)
	mux.Handle("/healthz", httpok)
	mux.Handle("/err", promhttp.InstrumentHandlerCounter(httpRequestsTotal, notfoundHandler))
	mux.Handle("/internal-err", promhttp.InstrumentHandlerCounter(httpRequestsTotal, internalErrorHandler))
	mux.Handle("/metrics", promhttp.HandlerFor(r, promhttp.HandlerOpts{}))

	var srv *http.Server
	if enableH2c {
		srv = &http.Server{Addr: bind, Handler: h2c.NewHandler(mux, &http2.Server{})}
	} else {
		srv = &http.Server{Addr: bind, Handler: mux}
	}

	log.Fatal(srv.ListenAndServe())
}
