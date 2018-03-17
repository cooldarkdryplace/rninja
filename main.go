package main

import (
	"context"
	"crypto/tls"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/crypto/acme/autocert"
)

var transport = &http.Transport{}

const (
	defaultCertCache  autocert.DirCache = "/opt/repository.ninja/certs"
	defaultTargetHost                   = "127.0.0.1:8080"
)

var (
	certCache  = autocert.DirCache(os.Getenv("CERT_CACHE_DIR"))
	targetHost = os.Getenv("TARGET")
	domain     = os.Getenv("DOMAIN")
)

func init() {
	if domain == "" {
		log.Fatal("DOMAIN environment variable not set.")
	}

	if certCache == "" {
		log.Printf("WARNING: Using default cache folder: %s", defaultCertCache)
		certCache = defaultCertCache
	}
	if targetHost == "" {
		log.Printf("WARNING: Using default target host: %s", defaultTargetHost)
		targetHost = defaultTargetHost
	}

}

// NewRedirectServer to handle redirects to HTTPS.
func NewRedirectServer(handler http.Handler) *http.Server {
	s := &http.Server{
		Addr:    ":http",
		Handler: handler,
	}
	return s
}

func proxy(w http.ResponseWriter, r *http.Request) {
	r.Host = targetHost
	r.URL.Host = r.Host
	r.URL.Scheme = "http"
	r.RequestURI = ""

	resp, err := transport.RoundTrip(r)
	if err != nil {
		log.Printf("Proxy error: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	for k, vs := range resp.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}

	w.WriteHeader(resp.StatusCode)

	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Println("Failed to proxy response: %s", err)
	}
}

func main() {
	http.HandleFunc("/", proxy)

	m := autocert.Manager{
		Cache:      certCache,
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(domain),
	}

	s := &http.Server{
		Addr:      ":https",
		Handler:   http.DefaultServeMux,
		TLSConfig: &tls.Config{GetCertificate: m.GetCertificate},
	}

	var (
		errChan    = make(chan error)
		signalChan = make(chan os.Signal, 1)
	)

	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	rs := NewRedirectServer(m.HTTPHandler(nil))
	go func() {
		errChan <- rs.ListenAndServe()
	}()

	go func() {
		errChan <- s.ListenAndServeTLS("", "")
	}()

	select {
	case err := <-errChan:
		log.Println(err)
	case <-signalChan:
		log.Println("Interrupt recieved. Graceful shutdown.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go rs.Shutdown(ctx)
	if err := s.Shutdown(ctx); err != nil {
		log.Fatalf("Graceful shutdown failed: %s", err)
	}
}
