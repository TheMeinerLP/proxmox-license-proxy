// Package app holds the business logic shared by the HTTP server and the CLI:
// the verify decision and license defaulting. It depends on the domain
// (subscription) and the persistence adapter (registry), and is free of any
// transport or CLI concern.
package app

import (
	"fmt"
	"time"

	"proxmox-license-proxy/internal/registry"
	"proxmox-license-proxy/internal/subscription"
)

const dateLayout = "2006-01-02"

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
		return subscription.License{}, fmt.Errorf("license key is required")
	}
	if !in.Force && !subscription.ValidKey(in.Key) {
		return subscription.License{}, fmt.Errorf("key %q is not a valid Proxmox key (e.g. pbsc-1ab1234567); use --force to override", in.Key)
	}
	// Every license must be a lab key - this tool refuses to manage anything that
	// could be mistaken for a real production subscription. Not bypassable.
	if !subscription.IsLabKey(in.Key) {
		return subscription.License{}, fmt.Errorf(
			"key %q must be a lab key carrying the %q signature (e.g. pbsc-1ab1234567); generate one with `license generate`",
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

	if serverid == "" || hostStatus != subscription.Approved {
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
	if lic, ok, _ := s.store.GetLicense(key); ok {
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
