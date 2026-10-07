package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/heartleo/zlib"
	"github.com/heartleo/zlib/internal/config"
	"github.com/spf13/cobra"
)

// selectEndpoint performs only local precedence resolution. Network recovery
// belongs to the client and happens only after an eligible request failure.
func selectEndpoint(flag, environment, session, lastGood string) (zlib.EndpointInfo, error) {
	for _, candidate := range []struct {
		value, source string
		manual        bool
	}{
		{flag, "flag", true}, {environment, "environment", true}, {session, "session", false}, {lastGood, "last_good", false}, {zlib.DefaultEAPIDomain, "seed", false},
	} {
		if strings.TrimSpace(candidate.value) == "" {
			continue
		}
		origin, err := zlib.ParseAllowedDomain(candidate.value)
		if err != nil {
			if candidate.manual {
				return zlib.EndpointInfo{}, err
			}
			continue // stale cache/session origins are not trusted automatic candidates
		}
		return zlib.EndpointInfo{URL: origin, Source: candidate.source, Manual: candidate.manual}, nil
	}
	return zlib.EndpointInfo{}, zlib.ErrNoDomain
}

func backendMode(requested, saved string) (zlib.ClientMode, error) {
	switch requested {
	case "", "auto":
		if saved == "html" || saved == "legacy" {
			return zlib.ClientModeHTML, nil
		}
		return zlib.ClientModeEAPI, nil
	case "eapi":
		return zlib.ClientModeEAPI, nil
	case "html":
		return zlib.ClientModeHTML, nil
	default:
		return "", fmt.Errorf("backend must be auto, eapi, or html")
	}
}

func configureClient(cmd *cobra.Command, session config.Session) (*zlib.Client, error) {
	domain := endpointFlag(cmd, "domain")
	backend := endpointFlag(cmd, "backend")
	cached := config.LoadState()
	endpoint, err := selectEndpoint(domain, os.Getenv(zlib.EnvDomain), session.Domain, cached.Endpoint.URL)
	if err != nil {
		return nil, err
	}
	mode, err := backendMode(backend, session.Mode)
	if err != nil {
		return nil, err
	}
	c := zlib.NewClient(zlib.WithDomain(endpoint.URL), zlib.WithContext(cmd.Context()), zlib.WithEndpointPolicy(zlib.EndpointPolicy{
		Manual: endpoint.Manual, Source: endpoint.Source, LastGood: cached.Endpoint.URL,
		DisableHTMLFallback: backend == "eapi",
		OnSuccess: func(info zlib.EndpointInfo) {
			if err := config.SaveEndpoint(info); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Could not save endpoint cache: %v\n", err)
			}
		},
		OnSwitch: func(domain string) {
			fmt.Fprintf(cmd.ErrOrStderr(), "Endpoint unavailable; switched to %s\n", strings.TrimPrefix(domain, "https://"))
		},
	}))
	c.SetMode(mode)
	if len(session.Cookies) > 0 {
		c.SetCookies(session.Cookies)
	}
	return c, nil
}

var endpointCmd = &cobra.Command{
	Use:   "endpoint",
	Short: "Show endpoint selection and cached health",
	RunE: func(cmd *cobra.Command, args []string) error {
		session, _ := config.LoadSession()
		c, err := configureClient(cmd, session)
		if err != nil {
			return err
		}
		info := c.Endpoint()
		cached := config.LoadState()
		if cached.Endpoint.URL == info.URL {
			info.LatencyMS = cached.Endpoint.LatencyMS
			info.LastSuccess = cached.Endpoint.LastSuccess
		}
		if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(info)
		}
		mode := "auto"
		if info.Manual {
			mode = "manual"
		}
		latency, last := "-", "-"
		if !info.LastSuccess.IsZero() {
			latency = fmt.Sprintf("%dms", info.LatencyMS)
			last = time.Since(info.LastSuccess).Round(time.Second).String() + " ago"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Mode         %s\nEndpoint     %s\nSource       %s\nBackend      %s\nLatency      %s\nLast success %s\n", mode, info.URL, info.Source, c.Mode(), latency, last)
		return nil
	},
}

func init() { endpointCmd.Flags().Bool("json", false, "Output endpoint selection as JSON") }

func endpointFlag(cmd *cobra.Command, name string) string {
	for _, get := range []func(string) (string, error){cmd.Flags().GetString, cmd.PersistentFlags().GetString, cmd.InheritedFlags().GetString} {
		if value, err := get(name); err == nil {
			return value
		}
	}
	return ""
}
