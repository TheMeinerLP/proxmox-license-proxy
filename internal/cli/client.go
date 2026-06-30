package cli

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"proxmox-license-proxy/internal/certs"
	"proxmox-license-proxy/internal/client"
	"proxmox-license-proxy/internal/discovery"
	"proxmox-license-proxy/internal/hosts"
)

const defaultInstallDest = "/usr/local/bin/proxmox-license-proxy"

var (
	clientDest      string
	clientServerURL string
	clientHostsIP   string
	clientFrom      string
	clientNoCert    bool
	clientNoHosts   bool
	clientNoBinary  bool
	clientYes       bool
)

var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "Client-side setup for a Proxmox host",
	Long: "Client-side setup for a Proxmox host. Run without a subcommand on a\n" +
		"terminal to open a guided menu (install / discover / uninstall).",
	RunE: menuOrHelp,
}

var clientInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install/update the binary and prepare this host (cert, /etc/hosts)",
	Long: `Installs or updates this binary into a system path and prepares the Proxmox
host to use the subscription proxy: trusts the server certificate and redirects
shop.proxmox.com via /etc/hosts.

Shell completion is shipped by the OS package (deb/rpm/apk), not installed here.

Interactive by default (asks where the server is). Pass --yes for an
unattended run driven entirely by flags.`,
	RunE: runClientInstall,
}

func runClientInstall(cmd *cobra.Command, args []string) error {
	opts := installChoices{
		dest:          orDefault(clientDest, defaultInstallDest),
		serverURL:     clientServerURL,
		hostsIP:       clientHostsIP,
		trustCert:     !clientNoCert,
		editHosts:     !clientNoHosts,
		installBinary: !clientNoBinary,
	}

	if !clientYes {
		if err := opts.ask(); err != nil {
			return err
		}
	}

	return opts.run()
}

// installChoices bundles every decision; gathering and execution are split so
// the flow stays readable and the steps testable.
type installChoices struct {
	dest          string
	serverURL     string
	hostsIP       string
	trustCert     bool
	editHosts     bool
	installBinary bool
}

func (o *installChoices) ask() error {
	// Offer mDNS-discovered servers when no --server was given. Best-effort: any
	// failure just falls through to manual entry below.
	if o.serverURL == "" {
		_ = o.discoverServer()
	}
	if o.hostsIP == "" {
		o.hostsIP = hostFromURL(o.serverURL)
	}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Install location").Value(&o.dest),
			huh.NewInput().Title("Proxy server URL (where the proxy runs)").
				Placeholder("https://10.0.0.5").Value(&o.serverURL),
			huh.NewConfirm().Title("Trust the server's certificate?").Value(&o.trustCert),
			huh.NewConfirm().Title("Redirect shop.proxmox.com via /etc/hosts?").Value(&o.editHosts),
			huh.NewInput().Title("Proxy IP for /etc/hosts").Value(&o.hostsIP),
		),
	)
	return form.Run()
}

// discoverServer browses the local network and lets the user pick which server
// address to use. It offers every discovered IP, the advertised .local hostname,
// and always a "localhost" option for a single-host setup (server + client on the
// same machine), where same-host mDNS may not loop back. The choice fills in
// serverURL and hostsIP; "Enter manually" leaves them empty for the form below.
func (o *installChoices) discoverServer() error {
	fmt.Println("searching the local network for proxmox-license-proxy servers (mDNS)...")
	servers, _ := discovery.Browse(context.Background(), 3*time.Second)

	type choice struct{ url, ip string }
	var choices []choice
	options := make([]huh.Option[int], 0)
	add := func(label, u, ip string) {
		options = append(options, huh.NewOption(label, len(choices)))
		choices = append(choices, choice{url: u, ip: ip})
	}
	for _, s := range servers {
		for _, ip := range s.IPs {
			host := ip.String()
			u := fmt.Sprintf("%s://%s", s.Scheme(), net.JoinHostPort(host, strconv.Itoa(s.Port)))
			add(fmt.Sprintf("%s - %s", s.Instance, u), u, host)
		}
		// The advertised .local name works on the same host and across subnets,
		// even when the routable IP list is empty.
		if h := strings.TrimSuffix(s.Host, "."); h != "" {
			u := fmt.Sprintf("%s://%s", s.Scheme(), net.JoinHostPort(h, strconv.Itoa(s.Port)))
			add(fmt.Sprintf("%s - %s", s.Instance, u), u, h)
		}
	}
	// Single-host fallback: the proxy is reachable on loopback whether or not
	// mDNS round-tripped to this machine.
	add("localhost (this machine) - https://localhost", "https://localhost", "127.0.0.1")
	options = append(options, huh.NewOption("Enter manually", -1))

	sel := -1
	if err := huh.NewSelect[int]().
		Title("Pick which server address to use").
		Options(options...).
		Value(&sel).
		Run(); err != nil {
		return err
	}
	if sel >= 0 {
		o.serverURL = choices[sel].url
		o.hostsIP = choices[sel].ip
	}
	return nil
}

func (o *installChoices) run() error {
	var summary []string

	// 1) install / update the binary (skipped with --no-binary, e.g. on the host
	//    that already runs the proxy from the package, to avoid a /usr/local/bin
	//    copy shadowing the packaged /usr/bin binary).
	if o.installBinary {
		src := ""
		if clientFrom != "" {
			path, err := client.DownloadTo(clientFrom)
			if err != nil {
				return err
			}
			defer func() { _ = os.Remove(path) }()
			src = path
		} else {
			path, err := client.CurrentBinary()
			if err != nil {
				return err
			}
			src = path
		}
		res, err := client.InstallBinary(src, o.dest)
		if err != nil {
			return err
		}
		summary = append(summary, fmt.Sprintf("binary %s at %s", res.Action, res.Path))
	}

	// 2) trust the server certificate
	if o.trustCert {
		if o.serverURL == "" {
			return fmt.Errorf("a server URL is required to trust the certificate (use --server or --no-cert)")
		}
		pem, err := certs.Download(strings.TrimRight(o.serverURL, "/") + "/ca.crt")
		if err != nil {
			return fmt.Errorf("download certificate: %w", err)
		}
		// Trust-on-first-use: show the fingerprint so the user can confirm the CA
		// out of band (compare with `cert generate` output on the server).
		if fp := certs.Fingerprint(pem); fp != "" {
			fmt.Printf("server CA SHA-256: %s\n", fp)
		}
		dst := "/usr/local/share/ca-certificates/pmox.crt"
		if err := certs.InstallTrust(pem, dst); err != nil {
			return err
		}
		summary = append(summary, "certificate trusted at "+dst)
	}

	// 3) /etc/hosts redirect
	if o.editHosts {
		ip := o.hostsIP
		if ip == "" {
			ip = hostFromURL(o.serverURL)
		}
		if ip == "" {
			return fmt.Errorf("a proxy IP is required for /etc/hosts (use --ip or --no-hosts)")
		}
		if _, err := hosts.Enable("/etc/hosts", ip, []string{"shop.proxmox.com"}, false); err != nil {
			return err
		}
		summary = append(summary, "shop.proxmox.com -> "+ip+" in /etc/hosts")
	}

	fmt.Println(colorOK("client install complete:"))
	for _, s := range summary {
		fmt.Println("  -", s)
	}
	return nil
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func hostFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

var clientDiscoverTimeout time.Duration

var clientDiscoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Find proxmox-license-proxy servers on the local network via mDNS",
	RunE: func(cmd *cobra.Command, args []string) error {
		servers, err := discovery.Browse(context.Background(), clientDiscoverTimeout)
		if err != nil {
			return err
		}
		if len(servers) == 0 {
			fmt.Println("no proxmox-license-proxy servers found on the local network")
			return nil
		}
		rows := make([][]string, 0, len(servers))
		for _, s := range servers {
			addrs := make([]string, 0, len(s.IPs))
			for _, ip := range s.IPs {
				addrs = append(addrs, ip.String())
			}
			rows = append(rows, []string{s.Instance, s.Host, strconv.Itoa(s.Port), strings.Join(addrs, ", ")})
		}
		printTable([]string{"INSTANCE", "HOST", "PORT", "ADDRESSES"}, rows, nil)
		return nil
	},
}

var (
	uninstallDest     string
	uninstallNoBinary bool
	uninstallNoCert   bool
	uninstallNoHosts  bool
	uninstallYes      bool
)

var clientUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Roll back everything client install did (binary, cert, hosts)",
	RunE:  runClientUninstall,
}

func runClientUninstall(cmd *cobra.Command, args []string) error {
	dest := orDefault(uninstallDest, defaultInstallDest)
	removeBinary := !uninstallNoBinary
	removeCert := !uninstallNoCert
	disableHosts := !uninstallNoHosts

	if !uninstallYes {
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().Title("Remove the installed binary at "+dest+"?").Value(&removeBinary),
				huh.NewConfirm().Title("Remove the trusted certificate?").Value(&removeCert),
				huh.NewConfirm().Title("Remove the /etc/hosts redirect?").Value(&disableHosts),
			),
		)
		if err := form.Run(); err != nil {
			return err
		}
	}

	var summary []string

	if removeCert {
		existed, err := certs.RemoveTrust("/usr/local/share/ca-certificates/pmox.crt")
		if err != nil {
			return err
		}
		if existed {
			summary = append(summary, "certificate removed from trust store")
		} else {
			summary = append(summary, "certificate not present")
		}
	}

	if disableHosts {
		if _, err := hosts.Disable("/etc/hosts", false); err != nil {
			return err
		}
		summary = append(summary, "removed shop.proxmox.com redirect from /etc/hosts")
	}

	// Remove the binary last so the steps above can still run from it.
	if removeBinary {
		existed, err := client.UninstallBinary(dest)
		if err != nil {
			return err
		}
		if existed {
			summary = append(summary, "binary removed from "+dest)
		} else {
			summary = append(summary, "binary not present at "+dest)
		}
	}

	fmt.Println(colorOK("client uninstall complete:"))
	for _, s := range summary {
		fmt.Println("  -", s)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(clientCmd)
	clientCmd.AddCommand(clientInstallCmd, clientUninstallCmd, clientDiscoverCmd)

	clientDiscoverCmd.Flags().DurationVar(&clientDiscoverTimeout, "timeout", 3*time.Second, "how long to listen for mDNS responses")

	u := clientUninstallCmd.Flags()
	u.StringVar(&uninstallDest, "dest", "", "installed binary path (default "+defaultInstallDest+")")
	u.BoolVar(&uninstallNoBinary, "no-binary", false, "keep the installed binary")
	u.BoolVar(&uninstallNoCert, "no-cert", false, "keep the trusted certificate")
	u.BoolVar(&uninstallNoHosts, "no-hosts", false, "keep the /etc/hosts redirect")
	u.BoolVar(&uninstallYes, "yes", false, "non-interactive: use flags/defaults, no prompts")

	f := clientInstallCmd.Flags()
	f.StringVar(&clientDest, "dest", "", "install path (default "+defaultInstallDest+")")
	f.StringVar(&clientServerURL, "server", "", "proxy server URL, e.g. https://10.0.0.5")
	f.StringVar(&clientHostsIP, "ip", "", "proxy IP for /etc/hosts (default: host from --server)")
	f.StringVar(&clientFrom, "from", "", "download the binary from this URL instead of installing the current one")
	f.BoolVar(&clientNoCert, "no-cert", false, "skip trusting the server certificate")
	f.BoolVar(&clientNoHosts, "no-hosts", false, "skip editing /etc/hosts")
	f.BoolVar(&clientNoBinary, "no-binary", false, "skip installing the binary (use on the host that already runs the proxy from the package)")
	f.BoolVar(&clientYes, "yes", false, "non-interactive: use flags/defaults, no prompts")
}
