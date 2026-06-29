package subscription

import (
	"crypto/md5" //nolint:gosec // G501: md5 is mandated by the Proxmox subscription protocol
	"encoding/hex"
	"fmt"
	"strings"
)

// SharedKeyData is a constant compiled into every Proxmox product (PVE/PBS/PMG).
// It is part of the open-source proxmox-subscription crate and is not a secret:
// the online check protects the response only with md5(SharedKeyData + check_token),
// where check_token is a challenge the client itself sends.
const SharedKeyData = "kjfdlskfhiuewhfk947368"

// ChallengeHash is the md5hash value the Proxmox client expects for an active
// response. checkToken is the value from the request's "check_token" field.
func ChallengeHash(checkToken string) string {
	//nolint:gosec // G401: the Proxmox protocol requires md5(SharedKeyData+check_token); not security-sensitive
	sum := md5.Sum([]byte(SharedKeyData + checkToken))
	return hex.EncodeToString(sum[:])
}

// Response builds the XML that the Proxmox client parses (flat <tag>value</tag>
// pairs; a well-formed document is not required).
type Response struct {
	Status      string // "active" | "invalid" | ...
	ServerID    string // echoed as <validdirectory> (must equal the request's dir)
	ProductName string
	RegDate     string
	NextDueDate string
	Message     string
	CheckToken  string // challenge from the request, used for md5hash
}

// RenderXML serializes the response.
func (r Response) RenderXML() string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	writeTag(&b, "status", r.Status)
	writeTag(&b, "validdirectory", r.ServerID)
	writeTag(&b, "productname", r.ProductName)
	writeTag(&b, "regdate", r.RegDate)
	writeTag(&b, "nextduedate", r.NextDueDate)
	if r.Message != "" {
		writeTag(&b, "message", r.Message)
	}
	writeTag(&b, "md5hash", ChallengeHash(r.CheckToken))
	return b.String()
}

func writeTag(b *strings.Builder, tag, value string) {
	value = strings.ReplaceAll(value, "&", "&amp;")
	value = strings.ReplaceAll(value, "<", "&lt;")
	fmt.Fprintf(b, "<%s>%s</%s>\n", tag, value, tag)
}
