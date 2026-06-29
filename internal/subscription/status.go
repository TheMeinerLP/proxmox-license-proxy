package subscription

import (
	"fmt"
	"strings"
)

type Status string

const (
	Approved   Status = "APPROVED"
	Pending    Status = "PENDING"
	Blocked    Status = "BLOCKED"
	Failed     Status = "FAILED"
	Rejected   Status = "REJECTED"
	Registered Status = "REGISTERED"
	// Revoked marks a subscription the server has invalidated (Let's Encrypt
	// style): verify.php then refuses it, so the host goes inactive on its next
	// check. It applies to subscriptions, not hosts.
	Revoked Status = "REVOKED"
)

func (s Status) IsValid() bool {
	switch s {
	case Approved, Pending, Blocked, Failed, Rejected, Registered, Revoked:
		return true
	default:
		return false
	}
}

func ParseStatus(raw string) (Status, error) {
	s := Status(strings.ToUpper(strings.TrimSpace(raw)))
	if !s.IsValid() {
		return "", fmt.Errorf("invalid subscription status: %q", raw)
	}
	return s, nil
}
