package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
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
	clientDest         string
	clientServerURL    string
	clientHostsIP      string
	clientFrom         string
	clientNoCert       bool
	clientNoHosts      bool
	clientNoCompletion bool
	clientYes          bool
)

var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "Client-side setup for a Proxmox host",
}

var clientInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install/update the binary and prepare this host (cert, /etc/hosts, completion)",
	Long: `Installs or updates this binary into a system path and prepares the Proxmox
host to use the license proxy: trusts the server certificate, redirects
shop.proxmox.com via /etc/hosts and installs shell completion.

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
		addCompletion: !clientNoCompletion,
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
	addCompletion bool
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
			huh.NewInput().Title("License server URL (where the proxy runs)").
				Placeholder("https://10.0.0.5").Value(&o.serverURL),
			huh.NewConfirm().Title("Trust the server's certificate?").Value(&o.trustCert),
			huh.NewConfirm().Title("Redirect shop.proxmox.com via /etc/hosts?").Value(&o.editHosts),
			huh.NewInput().Title("Proxy IP for /etc/hosts").Value(&o.hostsIP),
			huh.NewConfirm().Title("Install shell completion?").Value(&o.addCompletion),
		),
	)
	return form.Run()
}

// discoverServer browses the local network and, if servers are found, lets the
// user pick exactly which server IP to use. The choice fills in serverURL and
// hostsIP; picking "Enter manually" leaves them empty for the form below.
func (o *installChoices) discoverServer() error {
	fmt.Println("searching the local network for license-proxy servers (mDNS)...")
	servers, err := discovery.Browse(context.Background(), 3*time.Second)
	if err != nil || len(servers) == 0 {
		return err
	}

	type choice struct{ url, ip string }
	var choices []choice
	options := make([]huh.Option[int], 0)
	for _, s := range servers {
		for _, ip := range s.IPs {
			host := ip.String()
			u := fmt.Sprintf("%s://%s", s.Scheme(), net.JoinHostPort(host, strconv.Itoa(s.Port)))
			options = append(options, huh.NewOption(fmt.Sprintf("%s — %s", s.Instance, u), len(choices)))
			choices = append(choices, choice{url: u, ip: host})
		}
	}
	options = append(options, huh.NewOption("Enter manually", -1))

	sel := -1
	if err := huh.NewSelect[int]().
		Title("Discovered servers — pick which server IP to use").
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

	// 1) install / update the binary
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

	// 2) shell completion (best-effort, non-fatal)
	if o.addCompletion {
		if path, err := installShellCompletion(detectShell()); err != nil {
			summary = append(summary, "completion skipped ("+err.Error()+")")
		} else {
			summary = append(summary, "completion installed at "+path)
		}
	}

	// 3) trust the server certificate
	if o.trustCert {
		if o.serverURL == "" {
			return fmt.Errorf("a server URL is required to trust the certificate (use --server or --no-cert)")
		}
		pem, err := certs.Download(strings.TrimRight(o.serverURL, "/") + "/ca.crt")
		if err != nil {
			return fmt.Errorf("download certificate: %w", err)
		}
		dst := "/usr/local/share/ca-certificates/pmox.crt"
		if err := certs.InstallTrust(pem, dst); err != nil {
			return err
		}
		summary = append(summary, "certificate trusted at "+dst)
	}

	// 4) /etc/hosts redirect
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

	fmt.Println("client install complete:")
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

func detectShell() string {
	switch sh := filepath.Base(os.Getenv("SHELL")); sh {
	case "bash", "zsh", "fish":
		return sh
	default:
		return "bash"
	}
}

// completionPath returns the standard completion file location and a generator
// for the given shell. Shared by install and uninstall so the path can never
// drift between the two.
func completionPath(shell string) (string, func(*os.File) error) {
	switch shell {
	case "zsh":
		return "/usr/share/zsh/site-functions/_proxmox-license-proxy",
			func(f *os.File) error { return rootCmd.GenZshCompletion(f) }
	case "fish":
		return os.ExpandEnv("$HOME/.config/fish/completions/proxmox-license-proxy.fish"),
			func(f *os.File) error { return rootCmd.GenFishCompletion(f, true) }
	default: // bash
		return "/etc/bash_completion.d/proxmox-license-proxy",
			func(f *os.File) error { return rootCmd.GenBashCompletionV2(f, true) }
	}
}

// installShellCompletion writes the completion script for the given shell.
func installShellCompletion(shell string) (string, error) {
	path, gen := completionPath(shell)
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	if err := gen(f); err != nil {
		return "", err
	}
	return path, nil
}

// removeShellCompletion deletes the completion file. The bool reports whether it
// existed.
func removeShellCompletion(shell string) (string, bool, error) {
	path, _ := completionPath(shell)
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return path, false, nil
		}
		return path, false, err
	}
	return path, true, nil
}

var clientDiscoverTimeout time.Duration

var clientDiscoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Find license-proxy servers on the local network via mDNS",
	RunE: func(cmd *cobra.Command, args []string) error {
		servers, err := discovery.Browse(context.Background(), clientDiscoverTimeout)
		if err != nil {
			return err
		}
		if len(servers) == 0 {
			fmt.Println("no license-proxy servers found on the local network")
			return nil
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "INSTANCE\tHOST\tPORT\tADDRESSES")
		for _, s := range servers {
			addrs := make([]string, 0, len(s.IPs))
			for _, ip := range s.IPs {
				addrs = append(addrs, ip.String())
			}
			fmt.Fprintf(tw, "%s\t%s\t%d\t%s\n", s.Instance, s.Host, s.Port, strings.Join(addrs, ", "))
		}
		return tw.Flush()
	},
}

var (
	uninstallDest         string
	uninstallNoBinary     bool
	uninstallNoCert       bool
	uninstallNoHosts      bool
	uninstallNoCompletion bool
	uninstallYes          bool
)

var clientUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Roll back everything client install did (binary, cert, hosts, completion)",
	RunE:  runClientUninstall,
}

func runClientUninstall(cmd *cobra.Command, args []string) error {
	dest := orDefault(uninstallDest, defaultInstallDest)
	removeBinary := !uninstallNoBinary
	removeCert := !uninstallNoCert
	disableHosts := !uninstallNoHosts
	removeCompletion := !uninstallNoCompletion

	if !uninstallYes {
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().Title("Remove the installed binary at "+dest+"?").Value(&removeBinary),
				huh.NewConfirm().Title("Remove the trusted certificate?").Value(&removeCert),
				huh.NewConfirm().Title("Remove the /etc/hosts redirect?").Value(&disableHosts),
				huh.NewConfirm().Title("Remove shell completion?").Value(&removeCompletion),
			),
		)
		if err := form.Run(); err != nil {
			return err
		}
	}

	var summary []string

	if removeCompletion {
		path, existed, err := removeShellCompletion(detectShell())
		switch {
		case err != nil:
			summary = append(summary, "completion skipped ("+err.Error()+")")
		case existed:
			summary = append(summary, "completion removed from "+path)
		default:
			summary = append(summary, "completion not present")
		}
	}

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

	fmt.Println("client uninstall complete:")
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
	u.BoolVar(&uninstallNoCompletion, "no-completion", false, "keep shell completion")
	u.BoolVar(&uninstallYes, "yes", false, "non-interactive: use flags/defaults, no prompts")

	f := clientInstallCmd.Flags()
	f.StringVar(&clientDest, "dest", "", "install path (default "+defaultInstallDest+")")
	f.StringVar(&clientServerURL, "server", "", "license server URL, e.g. https://10.0.0.5")
	f.StringVar(&clientHostsIP, "ip", "", "proxy IP for /etc/hosts (default: host from --server)")
	f.StringVar(&clientFrom, "from", "", "download the binary from this URL instead of installing the current one")
	f.BoolVar(&clientNoCert, "no-cert", false, "skip trusting the server certificate")
	f.BoolVar(&clientNoHosts, "no-hosts", false, "skip editing /etc/hosts")
	f.BoolVar(&clientNoCompletion, "no-completion", false, "skip installing shell completion")
	f.BoolVar(&clientYes, "yes", false, "non-interactive: use flags/defaults, no prompts")
}
