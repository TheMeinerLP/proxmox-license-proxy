package cli

import (
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
	Short: "Run the license server (emulates shop.proxmox.com)",
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
		store := registry.NewStore(settings.RegistryFile)

		srv, err := httpserver.New(settings, store, log)
		if err != nil {
			return err
		}

		// Announce on the local network so clients can auto-discover us.
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
				defer adv.Close()
			}
		}

		return srv.Run()
	},
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
