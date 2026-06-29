package httpapi

import (
	"crypto/ed25519"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	nethttp "net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"proxmox-license-proxy/internal/acme"
	"proxmox-license-proxy/internal/app"
	"proxmox-license-proxy/internal/subscription"
)

// routesV1 mounts the versioned, ACME-style API. Client endpoints are protected
// by per-host account keys (JWS); /admin/* is protected by the configured bearer
// token. The shape is deliberately Let's-Encrypt-like: fetch a nonce, sign a
// request with your account key, the server issues or revokes subscriptions.
func (s *Server) routesV1(r chi.Router) {
	r.Get("/directory", s.v1Directory)
	r.Get("/new-nonce", s.v1NewNonce)
	r.Head("/new-nonce", s.v1NewNonce)
	r.Post("/new-account", s.v1NewAccount)
	r.Post("/new-order", s.v1NewOrder)
	r.Post("/subscriptions", s.v1ListSubscriptions) // POST-as-GET (JWS-authenticated)
	r.Post("/revoke", s.v1Revoke)

	r.Route("/admin", func(r chi.Router) {
		r.Get("/hosts", s.v1AdminListHosts)
		r.Post("/hosts/{id}/approve", s.v1AdminApproveHost)
		r.Post("/hosts/{id}/block", s.v1AdminBlockHost)
		r.Get("/subscriptions", s.v1AdminListSubscriptions)
		r.Post("/subscriptions/{key}/revoke", s.v1AdminRevoke)
		r.Delete("/subscriptions/{key}", s.v1AdminDeleteSubscription)
	})
}

// baseURL reconstructs the externally-visible base (scheme://host) from the
// request, so the directory and the JWS url check agree with what the client saw.
func (s *Server) baseURL(r *nethttp.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if p := r.Header.Get("X-Forwarded-Proto"); p != "" {
		scheme = p
	}
	return scheme + "://" + r.Host
}

func (s *Server) v1Directory(w nethttp.ResponseWriter, r *nethttp.Request) {
	base := s.baseURL(r) + "/api/v1"
	writeJSON(w, nethttp.StatusOK, map[string]string{
		"newNonce":      base + "/new-nonce",
		"newAccount":    base + "/new-account",
		"newOrder":      base + "/new-order",
		"subscriptions": base + "/subscriptions",
		"revoke":        base + "/revoke",
	})
}

// freshNonce sets a Replay-Nonce header so a client can immediately make its next
// signed request, mirroring ACME's behaviour on every response.
func (s *Server) freshNonce(w nethttp.ResponseWriter) {
	if n, err := s.nonces.Issue(); err == nil {
		w.Header().Set("Replay-Nonce", n)
	}
}

func (s *Server) v1NewNonce(w nethttp.ResponseWriter, r *nethttp.Request) {
	s.freshNonce(w)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(nethttp.StatusNoContent)
}

// problem writes an RFC 7807-ish JSON error and a fresh nonce, so a client whose
// nonce was rejected can retry without a separate round-trip.
func (s *Server) problem(w nethttp.ResponseWriter, status int, detail string) {
	s.freshNonce(w)
	writeJSON(w, status, map[string]string{"type": "about:blank", "detail": detail})
}

// verifyJWS reads and cryptographically verifies a JWS request body, checks the
// signed url matches this endpoint and consumes the replay nonce. It returns the
// verified envelope (with the account thumbprint) or writes an error and false.
func (s *Server) verifyJWS(w nethttp.ResponseWriter, r *nethttp.Request) (*acme.Verified, bool) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.problem(w, nethttp.StatusBadRequest, "cannot read body")
		return nil, false
	}
	v, err := acme.Verify(body, func(kid string) (ed25519.PublicKey, bool) {
		acc, ok, aerr := s.store.GetAccount(kid)
		if aerr != nil || !ok {
			return nil, false
		}
		jwk := acme.JWK{Kty: "OKP", Crv: "Ed25519", X: acc.PublicKey}
		pub, perr := jwk.Ed25519()
		if perr != nil {
			return nil, false
		}
		return pub, true
	})
	if err != nil {
		status := nethttp.StatusBadRequest
		if errors.Is(err, acme.ErrUnknownAccount) {
			status = nethttp.StatusUnauthorized
		}
		s.problem(w, status, "JWS verification failed: "+err.Error())
		return nil, false
	}
	if v.Header.URL != s.baseURL(r)+r.URL.Path {
		s.problem(w, nethttp.StatusBadRequest, "JWS url does not match request")
		return nil, false
	}
	if !s.nonces.Use(v.Header.Nonce) {
		s.problem(w, nethttp.StatusBadRequest, "bad or expired nonce; fetch a new one")
		return nil, false
	}
	return v, true
}

type newAccountReq struct {
	Contact  string `json:"contact"`
	ServerID string `json:"serverid"`
}

func (s *Server) v1NewAccount(w nethttp.ResponseWriter, r *nethttp.Request) {
	v, ok := s.verifyJWS(w, r)
	if !ok {
		return
	}
	if v.JWK == nil {
		s.problem(w, nethttp.StatusBadRequest, "new-account must embed the account key (jwk)")
		return
	}
	var req newAccountReq
	_ = json.Unmarshal(v.Payload, &req)

	acc, err := s.app.RegisterAccount(v.Thumbprint, v.JWK.X, req.ServerID, req.Contact)
	if err != nil {
		s.problem(w, nethttp.StatusBadRequest, err.Error())
		return
	}
	s.freshNonce(w)
	writeJSON(w, nethttp.StatusCreated, map[string]string{
		"thumbprint": acc.Thumbprint,
		"serverid":   acc.ServerID,
		"status":     "valid",
	})
}

type newOrderReq struct {
	ServerID string   `json:"serverid"`
	Products []string `json:"products"`
	Level    string   `json:"level"`
	Sockets  string   `json:"sockets"`
}

type subscriptionDTO struct {
	Product     string `json:"product"`
	Key         string `json:"key"`
	Status      string `json:"status"`
	ProductName string `json:"productName"`
	NextDueDate string `json:"nextDueDate"`
}

func (s *Server) v1NewOrder(w nethttp.ResponseWriter, r *nethttp.Request) {
	v, ok := s.verifyJWS(w, r)
	if !ok {
		return
	}
	var req newOrderReq
	if err := json.Unmarshal(v.Payload, &req); err != nil {
		s.problem(w, nethttp.StatusBadRequest, "invalid order payload")
		return
	}
	if req.ServerID == "" || len(req.Products) == 0 {
		s.problem(w, nethttp.StatusBadRequest, "serverid and at least one product are required")
		return
	}

	autoApprove := s.settings.AutoApprove.Allows(clientAddr(r))
	var issued []subscriptionDTO
	var pending []string
	problems := map[string]string{}
	for _, product := range req.Products {
		lic, err := s.app.IssueSubscription(app.IssueInput{
			Thumbprint:  v.Thumbprint,
			ServerID:    req.ServerID,
			Product:     product,
			Level:       req.Level,
			Sockets:     req.Sockets,
			AutoApprove: autoApprove,
		})
		switch {
		case err == nil:
			issued = append(issued, subscriptionDTO{
				Product: lic.Product, Key: lic.Key, Status: string(lic.Status),
				ProductName: lic.ProductName, NextDueDate: lic.NextDueDate,
			})
		case errors.Is(err, app.ErrHostNotApproved):
			pending = append(pending, product)
		default:
			problems[product] = err.Error()
		}
	}

	status := "valid"
	if len(issued) == 0 || len(pending) > 0 || len(problems) > 0 {
		status = "pending"
	}
	s.freshNonce(w)
	writeJSON(w, nethttp.StatusOK, map[string]any{
		"serverid":      req.ServerID,
		"status":        status,
		"subscriptions": issued,
		"pending":       pending,
		"problems":      problems,
	})
}

func (s *Server) v1ListSubscriptions(w nethttp.ResponseWriter, r *nethttp.Request) {
	v, ok := s.verifyJWS(w, r)
	if !ok {
		return
	}
	all, err := s.store.ListLicenses()
	if err != nil {
		s.problem(w, nethttp.StatusInternalServerError, err.Error())
		return
	}
	out := make([]subscriptionDTO, 0)
	for _, l := range all {
		if l.Account == v.Thumbprint {
			out = append(out, subscriptionDTO{
				Product: l.Product, Key: l.Key, Status: string(l.Status),
				ProductName: l.ProductName, NextDueDate: l.NextDueDate,
			})
		}
	}
	s.freshNonce(w)
	writeJSON(w, nethttp.StatusOK, map[string]any{"subscriptions": out})
}

type revokeReq struct {
	Key string `json:"key"`
}

func (s *Server) v1Revoke(w nethttp.ResponseWriter, r *nethttp.Request) {
	v, ok := s.verifyJWS(w, r)
	if !ok {
		return
	}
	var req revokeReq
	if err := json.Unmarshal(v.Payload, &req); err != nil || req.Key == "" {
		s.problem(w, nethttp.StatusBadRequest, "key is required")
		return
	}
	lic, found, err := s.store.GetLicense(req.Key)
	if err != nil {
		s.problem(w, nethttp.StatusInternalServerError, err.Error())
		return
	}
	// Only the owning account may revoke its own subscription over the client API.
	if !found || lic.Account != v.Thumbprint {
		s.problem(w, nethttp.StatusForbidden, "subscription not found for this account")
		return
	}
	if _, err := s.app.RevokeSubscription(req.Key); err != nil {
		s.problem(w, nethttp.StatusInternalServerError, err.Error())
		return
	}
	s.freshNonce(w)
	writeJSON(w, nethttp.StatusOK, map[string]string{"key": req.Key, "status": string(subscription.Revoked)})
}

// ---- admin API (bearer token) -------------------------------------------------

// requireAdmin enforces the configured admin bearer token. With no token set the
// admin endpoints are disabled (404-equivalent 403) so they cannot be reached
// unauthenticated.
func (s *Server) requireAdmin(w nethttp.ResponseWriter, r *nethttp.Request) bool {
	token := s.settings.API.AdminToken
	if token == "" {
		nethttp.Error(w, "admin API is disabled (set api.admin_token)", nethttp.StatusForbidden)
		return false
	}
	const prefix = "Bearer "
	got := r.Header.Get("Authorization")
	if !strings.HasPrefix(got, prefix) ||
		subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(got, prefix)), []byte(token)) != 1 {
		nethttp.Error(w, "invalid admin token", nethttp.StatusUnauthorized)
		return false
	}
	return true
}

func (s *Server) v1AdminListHosts(w nethttp.ResponseWriter, r *nethttp.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	servers, err := s.store.ListServers()
	if err != nil {
		nethttp.Error(w, err.Error(), nethttp.StatusInternalServerError)
		return
	}
	writeJSON(w, nethttp.StatusOK, servers)
}

func (s *Server) v1AdminApproveHost(w nethttp.ResponseWriter, r *nethttp.Request) {
	s.adminSetHostStatus(w, r, subscription.Approved)
}

func (s *Server) v1AdminBlockHost(w nethttp.ResponseWriter, r *nethttp.Request) {
	s.adminSetHostStatus(w, r, subscription.Blocked)
}

func (s *Server) adminSetHostStatus(w nethttp.ResponseWriter, r *nethttp.Request, status subscription.Status) {
	if !s.requireAdmin(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	found, err := s.store.SetServerStatus(id, status)
	if err != nil {
		nethttp.Error(w, err.Error(), nethttp.StatusInternalServerError)
		return
	}
	if !found {
		nethttp.Error(w, "host not found", nethttp.StatusNotFound)
		return
	}
	writeJSON(w, nethttp.StatusOK, map[string]string{"serverid": id, "status": string(status)})
}

func (s *Server) v1AdminListSubscriptions(w nethttp.ResponseWriter, r *nethttp.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	licenses, err := s.store.ListLicenses()
	if err != nil {
		nethttp.Error(w, err.Error(), nethttp.StatusInternalServerError)
		return
	}
	writeJSON(w, nethttp.StatusOK, licenses)
}

func (s *Server) v1AdminRevoke(w nethttp.ResponseWriter, r *nethttp.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	key := chi.URLParam(r, "key")
	found, err := s.app.RevokeSubscription(key)
	if err != nil {
		nethttp.Error(w, err.Error(), nethttp.StatusInternalServerError)
		return
	}
	if !found {
		nethttp.Error(w, "subscription not found", nethttp.StatusNotFound)
		return
	}
	writeJSON(w, nethttp.StatusOK, map[string]string{"key": key, "status": string(subscription.Revoked)})
}

func (s *Server) v1AdminDeleteSubscription(w nethttp.ResponseWriter, r *nethttp.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	key := chi.URLParam(r, "key")
	removed, err := s.store.RemoveLicense(key)
	if err != nil {
		nethttp.Error(w, err.Error(), nethttp.StatusInternalServerError)
		return
	}
	if !removed {
		nethttp.Error(w, "subscription not found", nethttp.StatusNotFound)
		return
	}
	w.WriteHeader(nethttp.StatusNoContent)
}
