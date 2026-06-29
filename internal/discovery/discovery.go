// Package discovery lets the server announce itself on the local network via
// mDNS / DNS-SD and lets the client find it without being told an address.
package discovery

import (
	"context"
	"net"
	"net/netip"
	"os"
	"sort"
	"time"

	"github.com/grandcat/zeroconf"
)

const (
	// ServiceType is the DNS-SD service the proxy advertises and the client browses for.
	ServiceType = "_pmox._tcp"
	domain      = "local."
)

// Server is a running mDNS advertisement; close it to stop announcing.
type Server struct{ inner *zeroconf.Server }

// Close stops the advertisement (safe on a nil Server).
func (s *Server) Close() {
	if s != nil && s.inner != nil {
		s.inner.Shutdown()
	}
}

// Advertise announces the license proxy on the local network so clients can
// discover it. instance defaults to the machine hostname.
func Advertise(instance string, port int, txt []string) (*Server, error) {
	if instance == "" {
		if h, err := os.Hostname(); err == nil && h != "" {
			instance = h
		} else {
			instance = "proxmox-license-proxy"
		}
	}
	srv, err := zeroconf.Register(instance, ServiceType, domain, port, txt, nil)
	if err != nil {
		return nil, err
	}
	return &Server{inner: srv}, nil
}

// Found is a discovered server. A server can expose several IPs (one per network
// interface); the client chooses which one to use.
type Found struct {
	Instance string
	Host     string
	Port     int
	IPs      []netip.Addr
	Text     []string
}

// Scheme returns "http" or "https" based on the advertised tls mode (https unless
// the server announced tls=http).
func (f Found) Scheme() string {
	for _, t := range f.Text {
		if t == "tls=http" {
			return "http"
		}
	}
	return "https"
}

// Browse looks for advertised servers for up to timeout, de-duplicated by instance.
func Browse(ctx context.Context, timeout time.Duration) ([]Found, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, err
	}

	entries := make(chan *zeroconf.ServiceEntry)
	byInstance := map[string]*Found{}
	done := make(chan struct{})
	go func() {
		for e := range entries {
			f := byInstance[e.Instance]
			if f == nil {
				f = &Found{Instance: e.Instance, Host: e.HostName, Port: e.Port, Text: e.Text}
				byInstance[e.Instance] = f
			}
			f.addIPs(e.AddrIPv4)
			f.addIPs(e.AddrIPv6)
		}
		close(done)
	}()

	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := resolver.Browse(cctx, ServiceType, domain, entries); err != nil {
		return nil, err
	}
	<-cctx.Done()
	<-done

	out := make([]Found, 0, len(byInstance))
	for _, f := range byInstance {
		out = append(out, *f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Instance < out[j].Instance })
	return out, nil
}

func (f *Found) addIPs(ips []net.IP) {
	for _, ip := range ips {
		a, ok := netip.AddrFromSlice(ip)
		if !ok {
			continue
		}
		a = a.Unmap()
		// Drop addresses a client can't usefully connect to across the network.
		if a.IsLoopback() || a.IsLinkLocalUnicast() || a.IsLinkLocalMulticast() ||
			a.IsMulticast() || a.IsUnspecified() {
			continue
		}
		if !containsAddr(f.IPs, a) {
			f.IPs = append(f.IPs, a)
		}
	}
}

func containsAddr(s []netip.Addr, a netip.Addr) bool {
	for _, x := range s {
		if x == a {
			return true
		}
	}
	return false
}
