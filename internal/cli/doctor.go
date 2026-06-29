package cli

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/spf13/cobra"

	"proxmox-license-proxy/internal/certs"
	"proxmox-license-proxy/internal/config"
	"proxmox-license-proxy/internal/fileio"
	"proxmox-license-proxy/internal/hosts"
	"proxmox-license-proxy/internal/registry"
	"proxmox-license-proxy/internal/release"
)

// trustStorePath is where `cert install` / `client install` place the trusted
// CA on Debian-family systems; doctor checks the same location.
const trustStorePath = "/usr/local/share/ca-certificates/pmox.crt"

// checkLevel ranks a diagnostic outcome.
type checkLevel int

const (
	levelOK checkLevel = iota
	levelInfo
	levelWarn
	levelFail
)

func (l checkLevel) tag() string {
	switch l {
	case levelOK:
		return "[ OK ]"
	case levelInfo:
		return "[INFO]"
	case levelWarn:
		return "[WARN]"
	default:
		return "[FAIL]"
	}
}

// check is one diagnostic line: a name, an outcome and optional detail/hint.
type check struct {
	level  checkLevel
	name   string
	detail string
	hint   string
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose a setup: binary, config, registry, TLS cert, hosts file and trust",
	Long: "Runs read-only checks that explain the common failure modes (a shadowing\n" +
		"binary on PATH, an unreadable registry, an expired/missing TLS cert, a\n" +
		"missing /etc/hosts entry or untrusted CA) and prints a hint for each.",
	RunE: func(cmd *cobra.Command, args []string) error {
		checks := []check{
			checkBinaryShadow(),
			checkConfigAndRegistry(),
			checkTLSCert(),
			checkHostsEntry(),
			checkTrustStore(),
			checkReachability(),
			checkLatestRelease(cmd.Context()),
		}
		if c, ok := checkLegacyRegistry(); ok {
			checks = append(checks, c)
		}

		worst := levelOK
		for _, c := range checks {
			fmt.Printf("%s %s\n", c.level.tag(), c.name)
			if c.detail != "" {
				fmt.Printf("       %s\n", c.detail)
			}
			if c.hint != "" {
				fmt.Printf("       hint: %s\n", c.hint)
			}
			if c.level > worst {
				worst = c.level
			}
		}

		fmt.Println()
		switch worst {
		case levelFail:
			fmt.Println("doctor found problems that likely break the proxy (see [FAIL] above).")
		case levelWarn:
			fmt.Println("doctor found warnings worth a look (see [WARN] above).")
		default:
			fmt.Println("doctor found nothing wrong.")
		}
		return nil
	},
}

// checkBinaryShadow catches the recurring footgun where a copy in /usr/local/bin
// shadows the packaged /usr/bin binary, so `version` and behaviour drift.
func checkBinaryShadow() check {
	const local, pkg = "/usr/local/bin/proxmox-license-proxy", "/usr/bin/proxmox-license-proxy"
	running, _ := os.Executable()
	_, localErr := os.Stat(local)
	_, pkgErr := os.Stat(pkg)
	if localErr == nil && pkgErr == nil {
		return check{
			level:  levelWarn,
			name:   "binary on PATH",
			detail: fmt.Sprintf("two binaries exist: %s and %s (running: %s)", local, pkg, running),
			hint:   "/usr/local/bin shadows /usr/bin; remove the stale one with `sudo rm " + local + "` if it is old",
		}
	}
	return check{level: levelOK, name: "binary on PATH", detail: "running " + running}
}

// checkConfigAndRegistry reports the loaded config and whether the registry can
// be read (the store that holds subscriptions and host approvals).
func checkConfigAndRegistry() check {
	cfg := cfgUsed
	if cfg == "" {
		cfg = "(defaults + environment)"
	}
	reg, err := store().Load()
	if err != nil {
		return check{
			level:  levelFail,
			name:   "registry",
			detail: fmt.Sprintf("config: %s; cannot read %s: %v", cfg, settings.RegistryFile, err),
			hint:   "check the path and permissions; the service writes it as group pmox (2770/0660)",
		}
	}
	return check{
		level: levelOK,
		name:  "registry",
		detail: fmt.Sprintf("config: %s; %s ok (%d subscriptions, %d hosts)",
			cfg, settings.RegistryFile, len(reg.Licenses), len(reg.Servers)),
	}
}

// checkTLSCert validates the certificate the server would serve, by TLS mode,
// and warns when it is missing or close to expiry.
func checkTLSCert() check {
	switch settings.TLS.Mode {
	case config.TLSModeHTTP:
		return check{level: levelInfo, name: "TLS certificate", detail: "tls.mode=http: serving plaintext (no certificate)"}
	case config.TLSModeFiles:
		return inspectCertFile("TLS certificate", settings.TLS.Cert)
	default: // auto
		certPath, _ := settings.AutoCertPaths()
		return inspectCertFile("TLS certificate (auto)", certPath)
	}
}

func inspectCertFile(name, path string) check {
	pem, err := fileio.ReadFile(path)
	if err != nil {
		return check{
			level:  levelWarn,
			name:   name,
			detail: fmt.Sprintf("cannot read %s: %v", path, err),
			hint:   "it is created on first `serve` (auto mode); start the server once",
		}
	}
	leaf, err := certs.Leaf(pem)
	if err != nil {
		return check{level: levelFail, name: name, detail: fmt.Sprintf("%s is not a valid certificate: %v", path, err)}
	}
	days := int(time.Until(leaf.NotAfter).Hours() / 24)
	detail := fmt.Sprintf("%s\n       SHA-256: %s\n       expires %s (%d days)",
		path, certs.Fingerprint(pem), leaf.NotAfter.Format("2006-01-02"), days)
	switch {
	case days < 0:
		return check{level: levelFail, name: name, detail: detail, hint: "expired; delete it so `serve` regenerates, then re-trust on hosts"}
	case days < 30:
		return check{level: levelWarn, name: name, detail: detail, hint: "expiring soon; plan to regenerate and re-trust on hosts"}
	default:
		return check{level: levelOK, name: name, detail: detail}
	}
}

// checkHostsEntry reports whether the managed shop.proxmox.com redirect is in
// the hosts file (relevant on a Proxmox host).
func checkHostsEntry() check {
	file := resolveHostsFile()
	present, block, err := hosts.Status(file)
	if err != nil {
		return check{level: levelWarn, name: "hosts file", detail: fmt.Sprintf("cannot read %s: %v", file, err)}
	}
	if !present {
		return check{
			level:  levelInfo,
			name:   "hosts file",
			detail: "no managed entry in " + file,
			hint:   "on a Proxmox host, run `client install` or `hosts enable` to redirect shop.proxmox.com",
		}
	}
	return check{level: levelOK, name: "hosts file", detail: "managed entry present in " + file + ":\n       " + block}
}

// checkTrustStore reports whether the proxy CA is installed in the system trust
// store (relevant on a Proxmox host).
func checkTrustStore() check {
	if _, err := os.Stat(trustStorePath); err != nil {
		return check{
			level:  levelInfo,
			name:   "trust store",
			detail: "no proxy CA at " + trustStorePath,
			hint:   "on a Proxmox host, trust the proxy cert with `client install` or `cert install`",
		}
	}
	detail := trustStorePath
	if pem, err := fileio.ReadFile(trustStorePath); err == nil {
		detail += "\n       SHA-256: " + certs.Fingerprint(pem)
	}
	return check{level: levelOK, name: "trust store", detail: detail}
}

// checkReachability does a best-effort TCP dial of the listen port on localhost,
// so a "is my server up?" check works on the proxy host without false alarms on
// a Proxmox host (where INFO is fine).
func checkReachability() check {
	port := portFromListen(settings.Listen)
	addr := net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return check{
			level:  levelInfo,
			name:   "local reachability",
			detail: fmt.Sprintf("nothing listening on %s", addr),
			hint:   "expected if this is a Proxmox host; on the proxy host, start `serve`",
		}
	}
	_ = conn.Close()
	return check{level: levelOK, name: "local reachability", detail: "server is listening on " + addr}
}

// checkLegacyRegistry surfaces an un-migrated pre-2.0 registry. It returns
// ok=false (and is omitted) unless there is genuinely something to migrate, to
// keep the normal output quiet.
func checkLegacyRegistry() (check, bool) {
	if settings.RegistryFile == registry.LegacyRegistryPath {
		return check{}, false
	}
	_, newErr := os.Stat(settings.RegistryFile)
	_, oldErr := os.Stat(registry.LegacyRegistryPath)
	if newErr == nil || oldErr != nil {
		return check{}, false // already migrated, or no legacy data
	}
	return check{
		level:  levelWarn,
		name:   "registry migration",
		detail: "a pre-2.0 registry exists at " + registry.LegacyRegistryPath + " but " + settings.RegistryFile + " is empty",
		hint:   "`serve` migrates it automatically on next start, or copy it across by hand",
	}, true
}

// checkLatestRelease compares the running build against the latest GitHub
// release. It is best-effort: an offline host or a non-release build degrades to
// INFO rather than failing the diagnostics.
func checkLatestRelease(ctx context.Context) check {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	info, err := release.NewChecker().Latest(ctx)
	if err != nil {
		return check{level: levelInfo, name: "latest release", detail: "update check skipped: " + err.Error()}
	}
	if release.IsNewer(build.Version, info.Tag) {
		return check{
			level:  levelWarn,
			name:   "latest release",
			detail: fmt.Sprintf("a newer release exists: %s (you have %s)", info.Tag, build.Version),
			hint:   "update via your package manager or " + info.URL,
		}
	}
	return check{level: levelOK, name: "latest release", detail: "running the latest release (" + info.Tag + ")"}
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
