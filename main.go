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

var certCache autocert.DirCache = "/opt/repository.ninja/certs"

// NewRedirectServer to handle redirects to HTTPS.
func NewRedirectServer(handler http.Handler) *http.Server {
	s := &http.Server{
		Addr:    ":http",
		Handler: handler,
	}
	return s
}

var client = &http.Client{}

func proxy(w http.ResponseWriter, r *http.Request) {
	r.Host = "127.0.0.1:8080"
	r.URL.Host = r.Host
	r.URL.Scheme = "http"

	resp, err := client.Do(r)
	if err != nil {
		w.WriteHeader(resp.StatusCode)
		return
	}
	defer resp.Body.Close()

	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Println("Failed to proxy response: %s", err)
	}
}

func main() {
	http.HandleFunc("/", proxy)

	m := autocert.Manager{
		Cache:      certCache,
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist("repository.ninja"),
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
