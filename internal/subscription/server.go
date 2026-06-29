package subscription

// Server is a Proxmox host that has contacted the verify endpoint. Hosts are
// auto-registered as PENDING on first contact and become active only after an
// admin approves them. The server id is the host's hardware address (the "dir"
// the Proxmox client sends).
type Server struct {
	ServerID  string `json:"serverid" yaml:"serverid"`
	Key       string `json:"key" yaml:"key"`         // license key the host presented
	Product   string `json:"product" yaml:"product"` // pve | pbs | pmg (derived)
	Status    Status `json:"status" yaml:"status"`   // PENDING | APPROVED | BLOCKED
	FirstSeen string `json:"firstSeen" yaml:"firstSeen"`
	LastSeen  string `json:"lastSeen" yaml:"lastSeen"`
	Note      string `json:"note,omitempty" yaml:"note,omitempty"`
}
