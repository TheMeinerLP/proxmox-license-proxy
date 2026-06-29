// Package app holds the business logic shared by the HTTP server and the CLI:
// the verify decision and license defaulting. It depends on the domain
// (subscription) and the persistence adapter (registry), and is free of any
// transport or CLI concern.
package app

import (
	"errors"
	"fmt"
	"time"

	"proxmox-license-proxy/internal/registry"
	"proxmox-license-proxy/internal/subscription"
)

const dateLayout = "2006-01-02"

// Sentinel errors for the self-service issuance flow, so the transport can map
// them to the right status (e.g. 403 for a host awaiting approval).
var (
	// ErrAccountNotApproved means the ACME account is registered but not yet
	// approved, so no subscription is issued. The client polls (Let's-Encrypt
	// style) until an admin approves the account, or the auto-approve-by-IP
	// policy approves it on registration.
	ErrAccountNotApproved = errors.New("account is not approved yet")
	// ErrSubscriptionRevoked means a subscription for this host/product exists
	// but was invalidated by an operator; the client must not silently re-issue.
	ErrSubscriptionRevoked = errors.New("subscription was revoked by an operator")
)

// Service is the application service over the registry store.
type Service struct {
	store *registry.Store
}

// New returns a service backed by the given store.
func New(store *registry.Store) *Service { return &Service{store: store} }

// Store exposes the underlying store for plain read/CRUD operations that carry
// no business rules (listing, removing, status changes).
func (s *Service) Store() *registry.Store { return s.store }

// AddLicenseInput is the request to add a license. Empty optional fields are
// filled with sensible defaults; StatusRaw and the dates are validated.
type AddLicenseInput struct {
	Key         string
	Product     string // optional, default derived from key
	ProductName string // optional, default derived from key
	StatusRaw   string // optional, default APPROVED
	RegDate     string // optional YYYY-MM-DD, default today
	NextDueDate string // optional YYYY-MM-DD, default today+1y
	Force       bool   // skip key-format validation
}

// AddLicense validates and defaults the input, persists the license and returns
// the stored value. It is the single place where a license gets its defaults,
// so the CLI and the REST API behave identically.
func (s *Service) AddLicense(in AddLicenseInput) (subscription.License, error) {
	if in.Key == "" {
		return subscription.License{}, fmt.Errorf("subscription key is required")
	}
	if !in.Force && !subscription.ValidKey(in.Key) {
		return subscription.License{}, fmt.Errorf("key %q is not a valid Proxmox key (e.g. pbsc-1ab1234567); use --force to override", in.Key)
	}
	// Every license must be a lab key - this tool refuses to manage anything that
	// could be mistaken for a real production subscription. Not bypassable.
	if !subscription.IsLabKey(in.Key) {
		return subscription.License{}, fmt.Errorf(
			"key %q must be a lab key carrying the %q signature (e.g. pbsc-1ab1234567); generate one with `subscription generate`",
			in.Key, subscription.LabSignature)
	}

	product, name, _ := subscription.Describe(in.Key)
	if in.Product != "" {
		product = in.Product
	}
	if in.ProductName != "" {
		name = in.ProductName
	}

	status := subscription.Approved
	if in.StatusRaw != "" {
		parsed, err := subscription.ParseStatus(in.StatusRaw)
		if err != nil {
			return subscription.License{}, err
		}
		status = parsed
	}

	regDate, err := defaultDate(in.RegDate, time.Now())
	if err != nil {
		return subscription.License{}, fmt.Errorf("reg date %w", err)
	}
	dueDate, err := defaultDate(in.NextDueDate, time.Now().AddDate(1, 0, 0))
	if err != nil {
		return subscription.License{}, fmt.Errorf("due date %w", err)
	}

	lic := subscription.License{
		Key:         in.Key,
		Product:     product,
		ProductName: name,
		RegDate:     regDate,
		NextDueDate: dueDate,
		Status:      status,
	}
	if err := s.store.AddLicense(lic); err != nil {
		return subscription.License{}, err
	}
	return lic, nil
}

// GenerateLicense creates a lab-marked license (see subscription.GenerateKey):
// the key carries the visible "1ab" signature and the product name is tagged
// with subscription.LabMarker, so its lab origin shows everywhere. When store is
// true it is persisted with status APPROVED.
func (s *Service) GenerateLicense(product, level, sockets string, store bool) (subscription.License, error) {
	key, err := subscription.GenerateKey(product, level, sockets)
	if err != nil {
		return subscription.License{}, err
	}
	_, name, _ := subscription.Describe(key)
	lic := subscription.License{
		Key:         key,
		Product:     product,
		ProductName: name + " " + subscription.LabMarker,
		RegDate:     time.Now().Format(dateLayout),
		NextDueDate: time.Now().AddDate(1, 0, 0).Format(dateLayout),
		Status:      subscription.Approved,
	}
	if store {
		if err := s.store.AddLicense(lic); err != nil {
			return subscription.License{}, err
		}
	}
	return lic, nil
}

// RegisterAccount stores (or refreshes) an ACME account: its JWK thumbprint is
// the id, publicKey is the base64url Ed25519 key used to verify its signatures.
// A new account starts APPROVED when autoApprove is set (it registered from a
// trusted network), otherwise PENDING until an admin approves it. An existing
// account keeps its status and creation time on re-registration.
func (s *Service) RegisterAccount(thumbprint, publicKey, serverid, contact string, autoApprove bool) (subscription.Account, error) {
	if thumbprint == "" || publicKey == "" {
		return subscription.Account{}, fmt.Errorf("account thumbprint and public key are required")
	}
	status := subscription.Pending
	if autoApprove {
		status = subscription.Approved
	}
	acc := subscription.Account{
		Thumbprint: thumbprint,
		PublicKey:  publicKey,
		ServerID:   serverid,
		Contact:    contact,
		Status:     status,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	if existing, ok, _ := s.store.GetAccount(thumbprint); ok {
		acc.CreatedAt = existing.CreatedAt
		acc.Status = existing.Status // an admin decision is not undone by re-registration
	}
	if err := s.store.UpsertAccount(acc); err != nil {
		return subscription.Account{}, err
	}
	return acc, nil
}

// IssueInput is a self-service subscription request from a host's ACME account.
type IssueInput struct {
	Thumbprint string // owning account (must be APPROVED)
	ServerID   string // the Proxmox host (verify "dir"), as reported by the client
	Product    string // pve | pbs | pmg
	Level      string // optional, default community
	Sockets    string // PVE only
}

// IssueSubscription is the Let's-Encrypt-style issuance: an APPROVED account may
// mint a subscription for a product. It is idempotent - an existing active
// subscription for the account/product is returned as-is, and a revoked one is
// refused rather than silently re-minted. The gate is the ACME account, not the
// host: the host's real serverid is only known once it contacts verify.php,
// which then honours the issued key by value.
func (s *Service) IssueSubscription(in IssueInput) (subscription.License, error) {
	if !subscription.ProductCode(in.Product) {
		return subscription.License{}, fmt.Errorf("unknown product %q (want pve, pbs or pmg)", in.Product)
	}

	acc, ok, err := s.store.GetAccount(in.Thumbprint)
	if err != nil {
		return subscription.License{}, err
	}
	if !ok || acc.Status != subscription.Approved {
		return subscription.License{}, ErrAccountNotApproved
	}

	// Idempotent: reuse an existing assignment for this account+product.
	if lic, ok, err := s.licenseForAccountProduct(in.Thumbprint, in.Product); err != nil {
		return subscription.License{}, err
	} else if ok {
		if !lic.Active() {
			return subscription.License{}, ErrSubscriptionRevoked
		}
		return lic, nil
	}

	key, err := subscription.GenerateKey(in.Product, in.Level, in.Sockets)
	if err != nil {
		return subscription.License{}, err
	}
	_, name, _ := subscription.Describe(key)
	lic := subscription.License{
		Key:         key,
		Product:     in.Product,
		ProductName: name + " " + subscription.LabMarker,
		RegDate:     time.Now().Format(dateLayout),
		NextDueDate: time.Now().AddDate(1, 0, 0).Format(dateLayout),
		Status:      subscription.Approved,
		ServerID:    in.ServerID,
		Account:     in.Thumbprint,
	}
	if err := s.store.AddLicense(lic); err != nil {
		return subscription.License{}, err
	}
	return lic, nil
}

// licenseForAccountProduct finds the subscription an account already holds for a
// product, so issuance stays idempotent.
func (s *Service) licenseForAccountProduct(thumbprint, product string) (subscription.License, bool, error) {
	all, err := s.store.ListLicenses()
	if err != nil {
		return subscription.License{}, false, err
	}
	for _, l := range all {
		if l.Account == thumbprint && l.Product == product {
			return l, true, nil
		}
	}
	return subscription.License{}, false, nil
}

// RevokeSubscription invalidates a subscription so verify.php stops honouring it.
// The bool reports whether the key existed.
func (s *Service) RevokeSubscription(key string) (bool, error) {
	return s.store.SetLicenseStatus(key, subscription.Revoked)
}

// defaultDate returns raw if it is a valid YYYY-MM-DD date, or the formatted
// fallback when raw is empty.
func defaultDate(raw string, fallback time.Time) (string, error) {
	if raw == "" {
		return fallback.Format(dateLayout), nil
	}
	if _, err := time.Parse(dateLayout, raw); err != nil {
		return "", fmt.Errorf("must be YYYY-MM-DD: %w", err)
	}
	return raw, nil
}

// VerifyResult is the outcome of a verify request. Response is always set (the
// caller renders it as XML); the remaining fields support logging.
type VerifyResult struct {
	Response    subscription.Response
	Active      bool
	HostStatus  subscription.Status
	RegisterErr error // non-nil if the host could not be (auto-)registered
}

// Verify auto-registers the contacting host and decides whether it gets an
// active subscription. A host must be Approved first; known license metadata
// then refines the product name and dates. A registration failure degrades to
// an "invalid" response (reported via RegisterErr) rather than an error, so the
// Proxmox client always receives well-formed XML.
//
// autoApprove (set by the transport when the host contacted from a trusted
// network) approves a new or still-pending host on the spot. The product is
// derived from the key, so auto-registration is correct for PVE, PBS and PMG.
func (s *Service) Verify(serverid, key, token string, autoApprove bool) VerifyResult {
	var hostStatus subscription.Status
	var regErr error
	if serverid != "" {
		product, _, _ := subscription.Describe(key)
		srv, err := s.store.UpsertServer(serverid, key, product, autoApprove)
		if err != nil {
			regErr = err
		} else {
			hostStatus = srv.Status
		}
	}

	lic, licKnown, _ := s.store.GetLicense(key)

	// A known-but-revoked (or otherwise denied) subscription is always inactive,
	// so an operator can invalidate a key Let's-Encrypt style and have the host
	// drop to unsubscribed on its next check - even on an approved host.
	if licKnown && !lic.Active() {
		return VerifyResult{
			Response: subscription.Response{
				Status:     "invalid",
				ServerID:   serverid,
				Message:    "subscription " + string(lic.Status),
				CheckToken: token,
			},
			HostStatus: hostStatus,
		}
	}

	// Authorized when the host is approved, OR the key is an active subscription
	// this server issued via the ACME API (the account was the gate at issuance,
	// so the key is honoured by value regardless of host-approval state).
	acmeIssued := licKnown && lic.Account != ""
	if serverid == "" || (hostStatus != subscription.Approved && !acmeIssued) {
		return VerifyResult{
			Response: subscription.Response{
				Status:     "invalid",
				ServerID:   serverid,
				Message:    "no valid subscription",
				CheckToken: token,
			},
			HostStatus:  hostStatus,
			RegisterErr: regErr,
		}
	}

	regDate := time.Now().Format(dateLayout)
	dueDate := time.Now().AddDate(1, 0, 0).Format(dateLayout)
	_, productName, _ := subscription.Describe(key)
	if licKnown {
		productName = lic.ProductName
		if lic.RegDate != "" {
			regDate = lic.RegDate
		}
		if lic.NextDueDate != "" {
			dueDate = lic.NextDueDate
		}
	}

	return VerifyResult{
		Response: subscription.Response{
			Status:      "active",
			ServerID:    serverid,
			ProductName: productName,
			RegDate:     regDate,
			NextDueDate: dueDate,
			CheckToken:  token,
		},
		Active:     true,
		HostStatus: hostStatus,
	}
}
