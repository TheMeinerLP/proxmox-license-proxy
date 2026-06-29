package cli

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"proxmox-license-proxy/internal/config"
	"proxmox-license-proxy/internal/discovery"
	"proxmox-license-proxy/internal/registry"
	httpserver "proxmox-license-proxy/internal/transport/httpapi"
)

var (
	serveListen  string
	serveTLSMode string
	serveTLSCert string
	serveTLSKey  string
	serveMDNS    bool
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the subscription server (emulates shop.proxmox.com)",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Command flags override the loaded config (empty = keep config value).
		if serveListen != "" {
			settings.Listen = serveListen
		}
		if serveTLSMode != "" {
			settings.TLS.Mode = config.TLSMode(serveTLSMode)
		}
		if serveTLSCert != "" {
			settings.TLS.Cert = serveTLSCert
		}
		if serveTLSKey != "" {
			settings.TLS.Key = serveTLSKey
		}

		log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: settings.LogLevel}))

		// One-time v1->v2 migration: pull a pre-2.0 registry (and auto cert) from
		// /var/lib/pmox into the configured location if it has not moved yet. The
		// packages do this in postinstall; this also covers manual/binary installs.
		if migrated, err := registry.MigrateLegacy(registry.LegacyRegistryPath, settings.RegistryFile); err != nil {
			log.Warn("legacy registry migration failed", "err", err)
		} else if len(migrated) > 0 {
			log.Info("migrated pre-2.0 registry to new location",
				"from", registry.LegacyRegistryPath, "to", settings.RegistryFile, "files", migrated)
		}

		store := registry.NewStore(settings.RegistryFile)

		srv, err := httpserver.New(settings, store, log)
		if err != nil {
			return err
		}

		// Announce on the local network so clients can auto-discover us.
		mdnsName := ""
		if serveMDNS {
			port := portFromListen(settings.Listen)
			txt := []string{
				"version=" + build.Version,
				"tls=" + string(settings.TLS.Mode),
			}
			if len(settings.TLS.Names) > 0 {
				txt = append(txt, "names="+strings.Join(settings.TLS.Names, ","))
			}
			if adv, aerr := discovery.Advertise("", port, txt); aerr != nil {
				log.Warn("mDNS advertise failed", "err", aerr)
			} else {
				log.Info("advertising via mDNS", "service", discovery.ServiceType, "port", port)
				if h, herr := os.Hostname(); herr == nil && h != "" {
					mdnsName = h + ".local"
				}
				defer adv.Close()
			}
		}

		printServeSummary(cmd.OutOrStdout(), srv, mdnsName)
		return srv.Run()
	},
}

// printServeSummary prints a human-friendly, copy-pasteable summary to stdout
// right before the server blocks: the URL hosts should use, the TLS mode and CA
// fingerprint (for trust-on-first-use verification), the mDNS name and the
// typical next steps. The structured slog lines still go to stderr underneath.
func printServeSummary(w io.Writer, srv *httpserver.Server, mdnsName string) {
	port := portFromListen(settings.Listen)
	scheme := "https"
	if settings.TLS.Mode == config.TLSModeHTTP {
		scheme = "http"
	}

	fmt.Fprintf(w, "\nproxmox-license-proxy is listening on %s (%s).\n\n", settings.Listen, scheme)

	fmt.Fprintln(w, "  Reachable at:")
	for _, ip := range localIPv4s() {
		fmt.Fprintf(w, "    %s://%s:%d\n", scheme, ip, port)
	}
	if mdnsName != "" {
		fmt.Fprintf(w, "    %s://%s:%d   (mDNS, auto-discovered by `client install`)\n", scheme, mdnsName, port)
	}

	fmt.Fprintf(w, "\n  TLS mode:    %s\n", settings.TLS.Mode)
	if fp := srv.CertFingerprint(); fp != "" {
		fmt.Fprintf(w, "  CA SHA-256:  %s\n", fp)
		fmt.Fprintln(w, "               (verify this on the Proxmox host when `client install` prints it)")
	}

	fmt.Fprintln(w, "\n  Next steps:")
	fmt.Fprintln(w, "    1. On each Proxmox host:  proxmox-license-proxy client install")
	fmt.Fprintln(w, "    2. Mint a lab key:        proxmox-license-proxy subscription generate")
	fmt.Fprintln(w, "    3. Approve the host:      proxmox-license-proxy server approve")
	fmt.Fprintln(w, "\n  Request logs follow below. Press Ctrl-C to stop.")
	fmt.Fprintln(w)
}

// localIPv4s returns this machine's non-loopback IPv4 addresses, so the serve
// summary can show admins exactly which URLs a Proxmox host can reach. Falls
// back to the hostname when no address can be enumerated.
func localIPv4s() []string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		if h, herr := os.Hostname(); herr == nil && h != "" {
			return []string{h}
		}
		return nil
	}
	var out []string
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() || ipnet.IP.IsLinkLocalUnicast() {
			continue
		}
		if ip4 := ipnet.IP.To4(); ip4 != nil {
			out = append(out, ip4.String())
		}
	}
	if len(out) == 0 {
		out = append(out, "127.0.0.1")
	}
	return out
}

// portFromListen extracts the TCP port from a listen address like ":443" or
// "0.0.0.0:8443", defaulting to 443.
func portFromListen(listen string) int {
	_, p, err := net.SplitHostPort(listen)
	if err != nil {
		return 443
	}
	n, err := strconv.Atoi(p)
	if err != nil {
		return 443
	}
	return n
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().StringVar(&serveListen, "listen", "", "address:port to listen on (overrides config)")
	serveCmd.Flags().StringVar(&serveTLSMode, "tls-mode", "", "tls mode: auto | files | http (overrides config)")
	serveCmd.Flags().StringVar(&serveTLSCert, "tls-cert", "", "path to TLS certificate (tls-mode=files)")
	serveCmd.Flags().StringVar(&serveTLSKey, "tls-key", "", "path to TLS key (tls-mode=files)")
	serveCmd.Flags().BoolVar(&serveMDNS, "mdns", true, "advertise this server on the local network via mDNS")

	_ = serveCmd.RegisterFlagCompletionFunc("tls-mode",
		cobra.FixedCompletions([]string{"auto", "files", "http"}, cobra.ShellCompDirectiveNoFileComp))
}
