package subscription

type License struct {
	Key         string `json:"key" yaml:"key"`
	Product     string `json:"product" yaml:"product"`
	ProductName string `json:"productName" yaml:"productName"`
	RegDate     string `json:"regDate" yaml:"regDate"`
	NextDueDate string `json:"nextDueDate" yaml:"nextDueDate"`
	Status      Status `json:"status" yaml:"status"`
	// ServerID assigns the subscription to a host. A host running several Proxmox
	// products holds one subscription per product, each with the same ServerID.
	// Empty for an unassigned (admin-minted, not yet bound) key.
	ServerID string `json:"serverid,omitempty" yaml:"serverid,omitempty"`
	// Account is the ACME account (its key thumbprint) that owns the
	// subscription, set when a host self-issues it. Empty for admin-minted keys.
	Account string `json:"account,omitempty" yaml:"account,omitempty"`
}

// Active reports whether the subscription should be honoured by verify.php: it
// must not be revoked or otherwise denied. Pending/Approved/Registered keys are
// considered active for the emulation; Revoked/Blocked/Rejected/Failed are not.
func (l License) Active() bool {
	switch l.Status {
	case Revoked, Blocked, Rejected, Failed:
		return false
	default:
		return true
	}
}

type Registry struct {
	Licenses []License `json:"licenses" yaml:"licenses"`
	Servers  []Server  `json:"servers" yaml:"servers"`
	Accounts []Account `json:"accounts,omitempty" yaml:"accounts,omitempty"`
}
