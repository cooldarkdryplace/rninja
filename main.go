package main

import (
	"context"
	"crypto/tls"
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
func NewRedirectServer() *http.Server {
	s := &http.Server{
		Addr: ":http",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			url := r.URL
			// In all other cases handle redirect ot HTTPS.
			url.Host = r.Host
			url.Scheme = "https"
			http.Redirect(w, r, url.String(), http.StatusMovedPermanently)
		}),
	}
	return s
}

func main() {
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

	rs := NewRedirectServer()
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
	log.Fatal(s.Shutdown(ctx))
}
