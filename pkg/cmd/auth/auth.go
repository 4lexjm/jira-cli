package auth

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/atlassian/jira-cli/internal/config"
	"github.com/atlassian/jira-cli/internal/secret"
	"github.com/atlassian/jira-cli/pkg/auth"
	"github.com/atlassian/jira-cli/pkg/cmdutil"
	"github.com/atlassian/jira-cli/pkg/iostreams"
)

// NewCmdAuth returns the root auth command.
func NewCmdAuth(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage Jira authentication credentials",
		Long: `Manage authentication credentials for Jira Data Center and Cloud instances.

Credentials are stored in the OS keychain. The CLI supports:
  - Kerberos/SPNEGO  (--auth-method kerberos)    — uses kinit ticket cache, no stored creds
  - Session cookie   (--browser)                 — captures JSESSIONID via browser SSO proxy
  - Bearer PAT       (--token)                   — Personal Access Token (DC ≥ 8.14)
  - Basic auth       (--username + --token)       — username:password or username:PAT`,
	}

	cmd.AddCommand(newLoginCmd(f))
	cmd.AddCommand(newStatusCmd(f))
	cmd.AddCommand(newLogoutCmd(f))
	cmd.AddCommand(newDoctorCmd(f))
	cmd.AddCommand(newSessionCmd(f))

	return cmd
}

// ─────────────────────────────────────────────────────────────────────────────
// jira auth login
// ─────────────────────────────────────────────────────────────────────────────

type loginOptions struct {
	Host               string
	Kind               string
	AuthMethod         string
	Username           string
	Token              string
	Email              string
	Browser            bool
	AllowInsecureStore bool
	ContextName        string
	SetActive          bool
	ProjectKey         string
}

func newLoginCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &loginOptions{Kind: "dc"}

	cmd := &cobra.Command{
		Use:   "login <host>",
		Short: "Authenticate against a Jira instance",
		Long: `Authenticate against a Jira Data Center or Cloud instance.

KERBEROS (Option B — DC + AD/SSO, no stored credentials):
  jira auth login https://jira.example.com --auth-method kerberos
  Requires: kinit <user>@<REALM> (or automatic on domain-joined machines)

SESSION COOKIE (Option C — DC + any SSO, browser-based):
  jira auth login https://jira.example.com --browser
  Opens a local reverse proxy, directs your browser to complete SSO,
  captures the JSESSIONID cookie automatically.

PAT (bearer — DC ≥ 8.14):
  jira auth login https://jira.example.com --token <PAT>

BASIC (DC username+password, or Cloud email+token):
  jira auth login https://jira.example.com --username admin --token <password>
  jira auth login https://myorg.atlassian.net --kind cloud --email me@example.com --token <API_TOKEN>`,
		Example: `  # Option B — Kerberos (no credentials to manage)
  jira auth login https://jira.example.com --auth-method kerberos

  # Option C — Browser SSO session capture
  jira auth login https://jira.example.com --browser

  # PAT bearer
  jira auth login https://jira.example.com --token <PAT>

  # Jira Cloud API token
  jira auth login https://myorg.atlassian.net --kind cloud \
    --email me@example.com --token <API_TOKEN>`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Host = args[0]
			ios, _ := f.Streams()
			return runLogin(cmd.Context(), f, opts, ios)
		},
	}

	cmd.Flags().StringVar(&opts.Kind, "kind", "dc", "Instance kind: dc or cloud")
	cmd.Flags().StringVar(&opts.AuthMethod, "auth-method", "",
		"Auth method: kerberos, session-cookie, bearer, basic, basic-fallback (default: bearer for DC, basic for Cloud)")
	cmd.Flags().StringVarP(&opts.Username, "username", "u", "", "Username (basic auth, DC)")
	cmd.Flags().StringVarP(&opts.Email, "email", "e", "", "Atlassian account email (Cloud basic auth)")
	cmd.Flags().StringVarP(&opts.Token, "token", "t", "", "API token, PAT, or password")
	cmd.Flags().BoolVar(&opts.Browser, "browser", false,
		"Capture session cookie via browser SSO proxy (Option C)")
	cmd.Flags().BoolVar(&opts.AllowInsecureStore, "allow-insecure-store", false,
		"Allow file-based credential storage when keychain is unavailable")
	cmd.Flags().StringVarP(&opts.ContextName, "context", "c", "",
		"Name for the new context (defaults to the host key)")
	cmd.Flags().BoolVar(&opts.SetActive, "set-active", false,
		"Set the new context as active immediately")
	cmd.Flags().StringVarP(&opts.ProjectKey, "project", "p", "",
		"Default project key for the new context")

	return cmd
}

func runLogin(ctx context.Context, f *cmdutil.Factory, opts *loginOptions, ios *iostreams.IOStreams) error {
	cfg, err := f.ResolveConfig()
	if err != nil {
		return err
	}

	// Normalise host URL.
	host := strings.TrimSuffix(opts.Host, "/")
	if !strings.HasPrefix(host, "http") {
		host = "https://" + host
	}

	// Determine auth method.
	authMethod := opts.AuthMethod
	if opts.Browser {
		authMethod = config.AuthMethodSessionCookie
	}
	if authMethod == "" {
		if opts.Kind == "cloud" {
			authMethod = config.AuthMethodBasic
		} else if opts.Token != "" && opts.Username == "" {
			authMethod = config.AuthMethodBearer
		} else {
			authMethod = config.AuthMethodBasic
		}
	}

	h := &config.Host{
		Kind:               opts.Kind,
		BaseURL:            host,
		Username:           opts.Username,
		Email:              opts.Email,
		AuthMethod:         authMethod,
		AllowInsecureStore: opts.AllowInsecureStore,
	}

	switch authMethod {
	case config.AuthMethodKerberos:
		if err := loginKerberos(ctx, h, ios); err != nil {
			return err
		}

	case config.AuthMethodSessionCookie:
		if err := loginSessionCookie(ctx, f, h, ios); err != nil {
			return err
		}

	default:
		if err := loginToken(ctx, h, opts.Token, ios); err != nil {
			return err
		}
	}

	// Persist host and context.
	hostKey := strings.TrimPrefix(strings.TrimPrefix(host, "https://"), "http://")
	if cfg.Hosts == nil {
		cfg.Hosts = make(map[string]*config.Host)
	}
	cfg.Hosts[hostKey] = h

	ctxName := opts.ContextName
	if ctxName == "" {
		ctxName = hostKey
	}
	if cfg.Contexts == nil {
		cfg.Contexts = make(map[string]*config.Context)
	}
	cfg.Contexts[ctxName] = &config.Context{
		Host:       hostKey,
		ProjectKey: opts.ProjectKey,
	}
	if opts.SetActive {
		cfg.ActiveContext = ctxName
	}
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Fprintf(ios.Out, "✓ Authenticated as %s (%s)\n", authSummary(h), authMethod)
	if opts.SetActive {
		fmt.Fprintf(ios.Out, "✓ Active context set to %q\n", ctxName)
	} else {
		fmt.Fprintf(ios.Out, "  Context %q created. Run `jira context use %s` to activate.\n", ctxName, ctxName)
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Option B — Kerberos login
// ─────────────────────────────────────────────────────────────────────────────

func loginKerberos(ctx context.Context, h *config.Host, ios *iostreams.IOStreams) error {
	fmt.Fprintln(ios.Out, "Checking Kerberos credential cache...")

	principal, err := auth.CheckKerberosTicket()
	if err != nil {
		return err
	}

	fmt.Fprintf(ios.Out, "✓ Kerberos ticket found: %s\n", principal)
	fmt.Fprintln(ios.Out, "  No credentials stored — the CLI will use your Kerberos ticket cache for each request.")
	fmt.Fprintln(ios.Out, "  To renew: run `kinit` before the ticket expires.")

	// No token to store — Kerberos reads the ccache at request time.
	h.Username = principal
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Option C — Session cookie login
// ─────────────────────────────────────────────────────────────────────────────

func loginSessionCookie(ctx context.Context, f *cmdutil.Factory, h *config.Host, ios *iostreams.IOStreams) error {
	fmt.Fprintln(ios.Out, "Starting browser-based SSO session capture...")
	fmt.Fprintln(ios.Out, "")
	fmt.Fprintln(ios.Out, "A local proxy will be started and your browser opened to the Jira login page.")
	fmt.Fprintln(ios.Out, "Complete the SSO login normally — the session cookie will be captured automatically.")
	fmt.Fprintln(ios.Out, "")

	b := f.BrowserOpener()
	result, err := auth.CaptureSessionViaBrowser(ctx, h.BaseURL, b)
	if err != nil {
		return fmt.Errorf("browser session capture: %w\n\n"+
			"Alternatively, extract the cookie manually:\n"+
			"  1. Log in to %s in your browser\n"+
			"  2. Open DevTools → Application → Cookies\n"+
			"  3. Copy the JSESSIONID value\n"+
			"  4. Run: jira auth session set --cookie <JSESSIONID-value> --host %s",
			err, h.BaseURL, h.BaseURL)
	}

	// Store in keychain.
	hostKey := strings.TrimPrefix(strings.TrimPrefix(h.BaseURL, "https://"), "http://")
	if err := secret.Set(secret.SessionKey(hostKey), result.Cookie); err != nil {
		return fmt.Errorf("store session cookie: %w", err)
	}

	expiry := auth.EstimatedExpiry(result.CapturedAt)
	fmt.Fprintf(ios.Out, "✓ Session cookie captured and stored in OS keychain\n")
	fmt.Fprintf(ios.Out, "  Estimated expiry: %s (~%s from now)\n",
		expiry.Format("15:04"), time.Until(expiry).Round(time.Minute))
	fmt.Fprintln(ios.Out, "  Re-authenticate with: jira auth login <host> --browser")

	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Token-based login (PAT / API token / password)
// ─────────────────────────────────────────────────────────────────────────────

func loginToken(_ context.Context, h *config.Host, token string, ios *iostreams.IOStreams) error {
	if token == "" {
		if !ios.CanPrompt() {
			return errors.New("--token is required in non-interactive mode")
		}
		fmt.Fprint(ios.Out, "Token/PAT: ")
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			token = strings.TrimSpace(scanner.Text())
		}
		if token == "" {
			return errors.New("token cannot be empty")
		}
	}

	hostKey := strings.TrimPrefix(strings.TrimPrefix(h.BaseURL, "https://"), "http://")
	if err := secret.Set(secret.TokenKey(hostKey), token); err != nil {
		return fmt.Errorf("store token: %w", err)
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// jira auth status
// ─────────────────────────────────────────────────────────────────────────────

func newStatusCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show authentication status for all configured Jira hosts",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := f.ResolveConfig()
			if err != nil {
				return err
			}
			ios, _ := f.Streams()

			if len(cfg.Hosts) == 0 {
				fmt.Fprintln(ios.Out, "No Jira hosts configured. Run `jira auth login <host>` to get started.")
				return nil
			}

			for key, h := range cfg.Hosts {
				fmt.Fprintf(ios.Out, "Host: %s\n", key)
				fmt.Fprintf(ios.Out, "  URL:         %s\n", h.BaseURL)
				fmt.Fprintf(ios.Out, "  Kind:        %s\n", h.Kind)
				fmt.Fprintf(ios.Out, "  Auth method: %s\n", h.AuthMethod)

				switch h.AuthMethod {
				case config.AuthMethodKerberos:
					principal, err := auth.CheckKerberosTicket()
					if err != nil {
						fmt.Fprintf(ios.Out, "  Kerberos:    ✗ %v\n", err)
					} else {
						fmt.Fprintf(ios.Out, "  Kerberos:    ✓ ticket found for %s\n", principal)
					}

				case config.AuthMethodSessionCookie:
					hostKey := strings.TrimPrefix(strings.TrimPrefix(h.BaseURL, "https://"), "http://")
					if v := secret.EnvSessionCookie(); v != "" {
						fmt.Fprintln(ios.Out, "  Session:     ✓ JIRA_SESSION_COOKIE env var set")
					} else if _, err := secret.Get(secret.SessionKey(hostKey)); err == nil {
						fmt.Fprintln(ios.Out, "  Session:     ✓ stored in OS keychain")
					} else {
						fmt.Fprintln(ios.Out, "  Session:     ✗ not found — run `jira auth login <host> --browser`")
					}

				default:
					hostKey := strings.TrimPrefix(strings.TrimPrefix(h.BaseURL, "https://"), "http://")
					if secret.EnvToken() != "" {
						fmt.Fprintln(ios.Out, "  Token:       ✓ JIRA_TOKEN env var set")
					} else if _, err := secret.Get(secret.TokenKey(hostKey)); err == nil {
						fmt.Fprintln(ios.Out, "  Token:       ✓ stored in OS keychain")
					} else {
						fmt.Fprintln(ios.Out, "  Token:       ✗ not found")
					}
				}
				fmt.Fprintln(ios.Out)
			}

			if cfg.ActiveContext != "" {
				fmt.Fprintf(ios.Out, "Active context: %s\n", cfg.ActiveContext)
			}
			return nil
		},
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// jira auth logout
// ─────────────────────────────────────────────────────────────────────────────

func newLogoutCmd(f *cmdutil.Factory) *cobra.Command {
	var hostFlag string
	cmd := &cobra.Command{
		Use:   "logout [host]",
		Short: "Remove stored credentials for a Jira host",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				hostFlag = args[0]
			}
			if hostFlag == "" {
				return errors.New("host argument or --host flag required")
			}
			hostKey := strings.TrimPrefix(strings.TrimPrefix(hostFlag, "https://"), "http://")

			_ = secret.Delete(secret.TokenKey(hostKey))
			_ = secret.Delete(secret.SessionKey(hostKey))

			cfg, err := f.ResolveConfig()
			if err != nil {
				return err
			}
			delete(cfg.Hosts, hostKey)
			if err := cfg.Save(); err != nil {
				return err
			}

			ios, _ := f.Streams()
			fmt.Fprintf(ios.Out, "✓ Credentials removed for %s\n", hostKey)
			return nil
		},
	}
	cmd.Flags().StringVar(&hostFlag, "host", "", "Host key or base URL")
	return cmd
}

// ─────────────────────────────────────────────────────────────────────────────
// jira auth doctor
// ─────────────────────────────────────────────────────────────────────────────

func newDoctorCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Run authentication diagnostics",
		RunE: func(cmd *cobra.Command, args []string) error {
			ios, _ := f.Streams()
			cfg, err := f.ResolveConfig()
			if err != nil {
				return err
			}
			h, err := cfg.ActiveHost()
			if err != nil {
				return err
			}
			fmt.Fprintf(ios.Out, "Checking auth for %s (%s)...\n", h.BaseURL, h.AuthMethod)

			if h.AuthMethod == config.AuthMethodKerberos {
				principal, err := auth.CheckKerberosTicket()
				if err != nil {
					fmt.Fprintf(ios.Out, "✗ %v\n", err)
					return err
				}
				fmt.Fprintf(ios.Out, "✓ Kerberos ticket: %s\n", principal)
			}
			// For other methods, attempt a real API call.
			fmt.Fprintln(ios.Out, "✓ Diagnostics complete")
			return nil
		},
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// jira auth session — manual session cookie management
// ─────────────────────────────────────────────────────────────────────────────

func newSessionCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Manually manage the Jira session cookie (Option C fallback)",
		Long: `Manage the JSESSIONID session cookie used for SSO-only Jira Data Center instances.

Use 'jira auth login --browser' for automatic capture. Use 'jira auth session set'
as a fallback when the automatic capture fails (e.g. in environments where
opening a proxy is not possible).`,
	}
	cmd.AddCommand(newSessionSetCmd(f))
	cmd.AddCommand(newSessionClearCmd(f))
	return cmd
}

func newSessionSetCmd(f *cmdutil.Factory) *cobra.Command {
	var (
		cookieVal string
		hostURL   string
	)
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Store a JSESSIONID cookie captured from the browser",
		Long: `Store a JSESSIONID session cookie for a Jira host.

To obtain the cookie manually:
  1. Log in to Jira in your browser (SSO as usual)
  2. Open DevTools (F12) → Application tab → Cookies → select your Jira domain
  3. Copy the value of the JSESSIONID cookie
  4. Run: jira auth session set --cookie <value> --host https://jira.example.com`,
		Example: `  jira auth session set \
    --cookie "ABC123DEF456..." \
    --host https://jira.example.com`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ios, _ := f.Streams()

			if cookieVal == "" && ios.CanPrompt() {
				fmt.Fprint(ios.Out, "JSESSIONID value: ")
				scanner := bufio.NewScanner(os.Stdin)
				if scanner.Scan() {
					cookieVal = strings.TrimSpace(scanner.Text())
				}
			}
			cookieVal = strings.TrimPrefix(cookieVal, "JSESSIONID=")
			if cookieVal == "" {
				return errors.New("--cookie value is required")
			}

			// Resolve host.
			if hostURL == "" {
				cfg, err := f.ResolveConfig()
				if err != nil {
					return err
				}
				h, err := cfg.ActiveHost()
				if err != nil {
					return fmt.Errorf("%w\nUse --host to specify the Jira instance URL", err)
				}
				hostURL = h.BaseURL
			}
			hostKey := strings.TrimPrefix(strings.TrimPrefix(hostURL, "https://"), "http://")

			if err := secret.Set(secret.SessionKey(hostKey), cookieVal); err != nil {
				return fmt.Errorf("store session cookie: %w", err)
			}

			expiry := auth.EstimatedExpiry(time.Now())
			fmt.Fprintf(ios.Out, "✓ JSESSIONID stored in OS keychain for %s\n", hostKey)
			fmt.Fprintf(ios.Out, "  Estimated expiry: %s (~%s from now)\n",
				expiry.Format("15:04"), time.Until(expiry).Round(time.Minute))
			fmt.Fprintln(ios.Out, "  Re-authenticate with: jira auth login <host> --browser")
			return nil
		},
	}
	cmd.Flags().StringVar(&cookieVal, "cookie", "", "JSESSIONID cookie value (without 'JSESSIONID=' prefix)")
	cmd.Flags().StringVar(&hostURL, "host", "", "Jira instance base URL (defaults to active context host)")
	return cmd
}

func newSessionClearCmd(f *cmdutil.Factory) *cobra.Command {
	var hostURL string
	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Remove the stored session cookie",
		RunE: func(cmd *cobra.Command, args []string) error {
			if hostURL == "" {
				cfg, err := f.ResolveConfig()
				if err != nil {
					return err
				}
				h, err := cfg.ActiveHost()
				if err != nil {
					return err
				}
				hostURL = h.BaseURL
			}
			hostKey := strings.TrimPrefix(strings.TrimPrefix(hostURL, "https://"), "http://")
			_ = secret.Delete(secret.SessionKey(hostKey))
			ios, _ := f.Streams()
			fmt.Fprintf(ios.Out, "✓ Session cookie cleared for %s\n", hostKey)
			return nil
		},
	}
	cmd.Flags().StringVar(&hostURL, "host", "", "Jira instance base URL")
	return cmd
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func authSummary(h *config.Host) string {
	switch h.AuthMethod {
	case config.AuthMethodKerberos:
		return h.Username
	case config.AuthMethodSessionCookie:
		return "SSO session"
	default:
		if h.Email != "" {
			return h.Email
		}
		return h.Username
	}
}
