package main

import (
	// "log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"

	"github.com/go-chi/chi"
	flags "github.com/jessevdk/go-flags"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	httplog "github.com/stn1slv/chi-httplog"
)

var opts struct {
	Port     int    `short:"p" long:"port" env:"PORT" default:"7070" description:"port"`
	Endpoint string `short:"e" long:"endpoint" env:"ENDPOINT" default:"/" description:"endpoint"`
	Verb     string `short:"v" long:"verb" env:"VERB" default:"GET" description:"HTTP verb"`
	//Logging settings
	JSONlog bool `long:"jsonLog" env:"JSON_LOG" description:"JSON format for logs"`
	//Stub settings
	StubMode bool   `short:"s" long:"stubMode" env:"STUB_MODE" description:"stub mode"`
	StubCode int    `long:"stubCode" env:"STUB_CODE" default:"200" description:"stub response code"`
	StubBody string `long:"stubBody" env:"STUB_BODY" default:"" description:"stub response body"`
	//Proxy settings
	TargetService string `long:"targetService" env:"TARGET" default:"http://localhost:8080" description:"target endpoint for proxy"`
}

func main() {
	// zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	log.Info().Msg("HTTP Proxy logger is starting")
	if _, err := flags.Parse(&opts); err != nil {
		log.Fatal().Err(err).Msg("Error parse flags")
		os.Exit(1)
	}
	log.Info().Msg("Listening port: " + strconv.Itoa(opts.Port))

	// Logger
	logger := httplog.NewLogger("http-logger-proxy", httplog.Options{
		JSON: opts.JSONlog,
		// Concise: true,
		Body: true,
		// Tags: map[string]string{
		// 	"version": "v1.0-81aa4244d9fc8076a",
		// 	"env":     "dev",
		// },
	})

	// Service
	r := chi.NewRouter()
	r.Use(httplog.RequestLogger(logger))

	fn := func(w http.ResponseWriter, r *http.Request) {}
	if opts.StubMode {
		log.Info().Msg("Stub mode")
		fn = StubHandler
	} else {
		log.Info().Msg("Proxy mode")
		fn = ProxyHandler
	}
	switch opts.Verb {
	case "GET":
		r.Get(opts.Endpoint, fn)
	case "POST":
		r.Post(opts.Endpoint, fn)
	case "DELETE":
		r.Delete(opts.Endpoint, fn)
	case "PUT":
		r.Put(opts.Endpoint, fn)
	}

	http.ListenAndServe(":"+strconv.Itoa(opts.Port), r)
}

//StubHandler is handler for stub operation
func StubHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain")
	w.WriteHeader(opts.StubCode)
	w.Write([]byte(opts.StubBody))
}

//ProxyHandler is handler for proxy operation
func ProxyHandler(w http.ResponseWriter, r *http.Request) {
	target, err := url.Parse(opts.TargetService)
	log.Printf("forwarding to -> %s%s\n", target.Scheme, target.Host)

	if err != nil {
		log.Fatal().Err(err)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ServeHTTP(w, r)
}
