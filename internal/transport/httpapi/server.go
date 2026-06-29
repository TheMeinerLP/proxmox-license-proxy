// Package httpapi is the server side of the tool: it emulates the Proxmox
// subscription endpoint (verify.php), auto-registers contacting hosts as
// pending, serves the CA certificate, exposes health/readiness probes for
// Kubernetes/Docker and a small REST API to manage licenses and hosts.
package httpapi

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	nethttp "net/http"
	"net/netip"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"proxmox-license-proxy/internal/acme"
	"proxmox-license-proxy/internal/app"
	"proxmox-license-proxy/internal/certs"
	"proxmox-license-proxy/internal/config"
	"proxmox-license-proxy/internal/fileio"
	"proxmox-license-proxy/internal/registry"
	"proxmox-license-proxy/internal/subscription"
)

// Server wires the settings, the registry store and the TLS material together.
type Server struct {
	settings *config.Settings
	store    *registry.Store
	app      *app.Service
	log      *slog.Logger

	certPEM []byte // also served at /ca.crt
	keyPEM  []byte

	// nonces backs the ACME-style anti-replay protection on the /api/v1 endpoints.
	nonces *acme.NonceStore

	// ready flips to false on shutdown so /readyz fails and load balancers
	// drain the pod before it stops.
	ready      atomic.Bool
	drainDelay time.Duration
}

// New builds a server and prepares its TLS material.
func New(settings *config.Settings, store *registry.Store, log *slog.Logger) (*Server, error) {
	s := &Server{
		settings:   settings,
		store:      store,
		app:        app.New(store),
		log:        log,
		drainDelay: time.Second,
		nonces:     acme.NewNonceStore(10 * time.Minute),
	}
	if err := s.setupTLS(); err != nil {
		return nil, err
	}
	s.ready.Store(true)
	return s, nil
}

func (s *Server) setupTLS() error {
	switch s.settings.TLS.Mode {
	case config.TLSModeAuto:
		// Persist the self-signed cert next to the registry so it survives
		// restarts and upgrades; otherwise every restart would mint a new cert
		// and break hosts that already trust the old one.
		certPath, keyPath := s.settings.AutoCertPaths()
		cert, key, ok := certs.LoadKeyPairIfValid(certPath, keyPath, s.settings.TLS.Names)
		if ok {
			s.log.Info("reusing persisted auto TLS certificate", "cert", certPath)
		} else {
			var err error
			cert, key, err = certs.GenerateSelfSigned(s.settings.TLS.Names, 10*365*24*time.Hour)
			if err != nil {
				return err
			}
			if werr := certs.WriteKeyPair(certPath, keyPath, cert, key); werr != nil {
				s.log.Warn("could not persist auto TLS cert; it will change on restart", "err", werr)
			} else {
				s.log.Info("generated and persisted auto TLS certificate",
					"cert", certPath, "fingerprint", certs.Fingerprint(cert))
			}
		}
		s.certPEM, s.keyPEM = cert, key
	case config.TLSModeFiles:
		cert, err := fileio.ReadFile(s.settings.TLS.Cert)
		if err != nil {
			return err
		}
		key, err := fileio.ReadFile(s.settings.TLS.Key)
		if err != nil {
			return err
		}
		s.certPEM, s.keyPEM = cert, key
	case config.TLSModeHTTP:
		// no TLS
	}
	return nil
}

// CertFingerprint returns the SHA-256 fingerprint of the served certificate so
// the serve command can print it for trust-on-first-use verification. Empty in
// http mode (no certificate).
func (s *Server) CertFingerprint() string {
	if len(s.certPEM) == 0 {
		return ""
	}
	return certs.Fingerprint(s.certPEM)
}

// maxRequestBody caps any request body; the verify form and license JSON are
// tiny, so this only bounds abusive clients.
const maxRequestBody = 1 << 16 // 64 KiB

// Handler builds the chi router.
func (s *Server) Handler() nethttp.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestSize(maxRequestBody))
	r.Use(s.logRequests)

	// The endpoint the Proxmox client hits (shop proxy).
	r.Post("/modules/servers/licensing/verify.php", s.handleVerify)

	// Certificate for clients to trust.
	r.Get("/ca.crt", s.handleCert)

	// Health/readiness for Kubernetes/Docker.
	r.Get("/healthz", s.handleLive)
	r.Get("/readyz", s.handleReady)
	r.Get("/status", s.handleStatus)

	// Subscription management REST API. /api/subscriptions is the canonical path
	// (Proxmox's own term); /api/licenses stays as a back-compat alias.
	subscriptionRoutes := func(r chi.Router) {
		r.Get("/", s.handleListLicenses)
		r.Post("/", s.handleAddLicense)
		r.Delete("/{key}", s.handleDeleteLicense)
	}
	r.Route("/api/subscriptions", subscriptionRoutes)
	r.Route("/api/licenses", subscriptionRoutes)

	// Versioned, ACME-style API: account keys + JWS for clients, a bearer token
	// for admin management. See apiv1.go.
	r.Route("/api/v1", s.routesV1)

	// Host management REST API.
	r.Route("/api/servers", func(r chi.Router) {
		r.Get("/", s.handleListServers)
		r.Delete("/{id}", s.handleDeleteServer)
		r.Post("/{id}/approve", s.handleApproveServer)
		r.Post("/{id}/block", s.handleBlockServer)
	})

	return r
}

// Run starts the server and blocks until a termination signal arrives, then
// shuts down gracefully.
func (s *Server) Run() error {
	srv := &nethttp.Server{
		Addr:              s.settings.Listen,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 16, // 64 KiB
	}

	plain := s.settings.TLS.Mode == config.TLSModeHTTP
	if !plain {
		pair, err := tls.X509KeyPair(s.certPEM, s.keyPEM)
		if err != nil {
			return err
		}
		srv.TLSConfig = &tls.Config{Certificates: []tls.Certificate{pair}}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		var err error
		if plain {
			s.log.Info("starting HTTP", "addr", s.settings.Listen)
			err = srv.ListenAndServe()
		} else {
			s.log.Info("starting HTTPS", "addr", s.settings.Listen, "tls", s.settings.TLS.Mode)
			err = srv.ListenAndServeTLS("", "")
		}
		if errors.Is(err, nethttp.ErrServerClosed) {
			err = nil
		}
		errCh <- err
	}()

	select {
	case err := <-errCh:
		return err // failed to start or crashed
	case <-ctx.Done():
		s.log.Info("shutdown signal received, draining")
		s.ready.Store(false)
		time.Sleep(s.drainDelay) // let load balancers notice /readyz failing

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return <-errCh
	}
}

func (s *Server) handleVerify(w nethttp.ResponseWriter, r *nethttp.Request) {
	if err := r.ParseForm(); err != nil {
		nethttp.Error(w, "bad form", nethttp.StatusBadRequest)
		return
	}
	key := r.PostFormValue("licensekey")
	dir := r.PostFormValue("dir")
	token := r.PostFormValue("check_token")

	w.Header().Set("Content-Type", "text/xml; charset=UTF-8")

	clientIP := clientAddr(r)
	autoApprove := s.settings.AutoApprove.Allows(clientIP)

	res := s.app.Verify(dir, key, token, autoApprove)
	switch {
	case res.RegisterErr != nil:
		s.log.Error("host registration failed", "err", res.RegisterErr, "serverid", dir)
	case res.Active:
		s.log.Info("subscription active", "key", key, "serverid", dir,
			"product", res.Response.ProductName, "auto_approved", autoApprove, "remote", clientIP.String())
	default:
		s.log.Warn("subscription denied", "key", key, "serverid", dir, "host_status", res.HostStatus)
	}
	_, _ = w.Write([]byte(res.Response.RenderXML()))
}

// clientAddr returns the source IP of the request, parsed from RemoteAddr. It
// returns an invalid Addr (which never matches a trusted network) when the
// address cannot be determined.
func clientAddr(r *nethttp.Request) netip.Addr {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	return addr.Unmap()
}

func (s *Server) handleCert(w nethttp.ResponseWriter, r *nethttp.Request) {
	if len(s.certPEM) == 0 {
		nethttp.Error(w, "no certificate (server runs in http mode)", nethttp.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/x-pem-file")
	w.Header().Set("Content-Disposition", `attachment; filename="pmox-ca.crt"`)
	_, _ = w.Write(s.certPEM)
}

// handleLive is the liveness probe: always 200 while the process is up.
func (s *Server) handleLive(w nethttp.ResponseWriter, r *nethttp.Request) {
	writeJSON(w, nethttp.StatusOK, map[string]string{"status": "ok"})
}

// handleReady is the readiness probe: 503 while draining or if the registry is
// unreadable.
func (s *Server) handleReady(w nethttp.ResponseWriter, r *nethttp.Request) {
	if !s.ready.Load() {
		writeJSON(w, nethttp.StatusServiceUnavailable, map[string]string{"status": "draining"})
		return
	}
	if _, err := s.store.Load(); err != nil {
		writeJSON(w, nethttp.StatusServiceUnavailable, map[string]string{"status": "registry unavailable"})
		return
	}
	writeJSON(w, nethttp.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) handleStatus(w nethttp.ResponseWriter, r *nethttp.Request) {
	reg, err := s.store.Load()
	if err != nil {
		nethttp.Error(w, err.Error(), nethttp.StatusInternalServerError)
		return
	}
	pending := 0
	for _, srv := range reg.Servers {
		if srv.Status == subscription.Pending {
			pending++
		}
	}
	writeJSON(w, nethttp.StatusOK, map[string]any{
		"status":   "ok",
		"ready":    s.ready.Load(),
		"licenses": len(reg.Licenses),
		"servers":  len(reg.Servers),
		"pending":  pending,
	})
}

func (s *Server) handleListLicenses(w nethttp.ResponseWriter, r *nethttp.Request) {
	licenses, err := s.store.ListLicenses()
	if err != nil {
		nethttp.Error(w, err.Error(), nethttp.StatusInternalServerError)
		return
	}
	writeJSON(w, nethttp.StatusOK, licenses)
}

func (s *Server) handleAddLicense(w nethttp.ResponseWriter, r *nethttp.Request) {
	var lic subscription.License
	if err := json.NewDecoder(r.Body).Decode(&lic); err != nil {
		nethttp.Error(w, "invalid JSON", nethttp.StatusBadRequest)
		return
	}
	created, err := s.app.AddLicense(app.AddLicenseInput{
		Key:         lic.Key,
		Product:     lic.Product,
		ProductName: lic.ProductName,
		StatusRaw:   string(lic.Status),
		RegDate:     lic.RegDate,
		NextDueDate: lic.NextDueDate,
	})
	if err != nil {
		nethttp.Error(w, err.Error(), nethttp.StatusBadRequest)
		return
	}
	writeJSON(w, nethttp.StatusCreated, created)
}

func (s *Server) handleDeleteLicense(w nethttp.ResponseWriter, r *nethttp.Request) {
	removed, err := s.store.RemoveLicense(chi.URLParam(r, "key"))
	if err != nil {
		nethttp.Error(w, err.Error(), nethttp.StatusInternalServerError)
		return
	}
	if !removed {
		nethttp.Error(w, "license not found", nethttp.StatusNotFound)
		return
	}
	w.WriteHeader(nethttp.StatusNoContent)
}

func (s *Server) handleListServers(w nethttp.ResponseWriter, r *nethttp.Request) {
	servers, err := s.store.ListServers()
	if err != nil {
		nethttp.Error(w, err.Error(), nethttp.StatusInternalServerError)
		return
	}
	writeJSON(w, nethttp.StatusOK, servers)
}

func (s *Server) handleApproveServer(w nethttp.ResponseWriter, r *nethttp.Request) {
	s.setServerStatus(w, r, subscription.Approved)
}

func (s *Server) handleBlockServer(w nethttp.ResponseWriter, r *nethttp.Request) {
	s.setServerStatus(w, r, subscription.Blocked)
}

func (s *Server) setServerStatus(w nethttp.ResponseWriter, r *nethttp.Request, status subscription.Status) {
	found, err := s.store.SetServerStatus(chi.URLParam(r, "id"), status)
	if err != nil {
		nethttp.Error(w, err.Error(), nethttp.StatusInternalServerError)
		return
	}
	if !found {
		nethttp.Error(w, "host not found", nethttp.StatusNotFound)
		return
	}
	writeJSON(w, nethttp.StatusOK, map[string]string{"serverid": chi.URLParam(r, "id"), "status": string(status)})
}

func (s *Server) handleDeleteServer(w nethttp.ResponseWriter, r *nethttp.Request) {
	removed, err := s.store.RemoveServer(chi.URLParam(r, "id"))
	if err != nil {
		nethttp.Error(w, err.Error(), nethttp.StatusInternalServerError)
		return
	}
	if !removed {
		nethttp.Error(w, "host not found", nethttp.StatusNotFound)
		return
	}
	w.WriteHeader(nethttp.StatusNoContent)
}

// logRequests logs every request with method, path, status and duration.
func (s *Server) logRequests(next nethttp.Handler) nethttp.Handler {
	return nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()
		next.ServeHTTP(ww, r)
		s.log.Info("request",
			"id", middleware.GetReqID(r.Context()),
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"bytes", ww.BytesWritten(),
			"duration", time.Since(start).String(),
			"remote", r.RemoteAddr,
		)
	})
}

func writeJSON(w nethttp.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
