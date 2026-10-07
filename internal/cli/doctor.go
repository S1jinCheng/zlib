package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	zlib "github.com/heartleo/zlib"
	"github.com/spf13/cobra"
)

const eapiProbePath = zlib.EAPIProbePath

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check Z-Library domain connectivity",
	Long:  "Automatically check both the website and EAPI endpoints of known Z-Library domains without logging in or changing your active domain.",
	RunE: func(cmd *cobra.Command, args []string) error {
		proxyValue, _ := cmd.Flags().GetString("proxy")
		eapi, _ := cmd.Flags().GetBool("eapi")
		proxyURL, err := validateDoctorProxy(proxyValue)
		if err != nil {
			return err
		}
		results := probeDoctorDomains(cmd.Context(), zlib.KnownDomains(), proxyURL, eapi)
		if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(results)
		}

		rows := make([][]string, 0, len(results))
		for _, result := range results {
			httpStatus := "-"
			if result.HTTPStatus != 0 {
				httpStatus = strconv.Itoa(result.HTTPStatus)
			}
			detail := result.Detail
			if detail == "" && result.FinalURL != "" && result.FinalURL != result.Domain {
				detail = "final: " + result.FinalURL
			}
			rows = append(rows, []string{result.Domain, string(result.Mode), string(result.Status), httpStatus, detail})
		}

		t := table.New().
			Border(lipgloss.RoundedBorder()).
			BorderStyle(lipgloss.NewStyle().Foreground(currentTheme.Accent)).
			Headers("Domain", "Mode", "Status", "HTTP", "Detail").
			Rows(rows...).
			StyleFunc(func(row, col int) lipgloss.Style {
				pad := lipgloss.NewStyle().PaddingLeft(1).PaddingRight(1)
				if row == table.HeaderRow {
					return pad.Bold(true).Foreground(currentTheme.Accent)
				}
				switch col {
				case 0, 1, 3:
					return pad.Foreground(currentTheme.Muted)
				case 2:
					return pad.Foreground(doctorStatusColor(zlib.DomainStatus(rows[row][2])))
				default:
					return pad
				}
			})

		fmt.Fprintln(cmd.OutOrStdout(), t)
		return nil
	},
}

// doctorCheck preserves the probe fields and identifies the endpoint checked.
type doctorCheck struct {
	zlib.DomainCheck
	Mode zlib.ClientMode `json:"mode"`
}

// probeDoctorDomains checks both endpoints concurrently, retaining domain order
// and placing HTML before EAPI so output is stable regardless of response time.
func probeDoctorDomains(ctx context.Context, domains []string, proxyURL string, eapiOnly bool) []doctorCheck {
	modes := []zlib.ClientMode{zlib.ClientModeHTML, zlib.ClientModeEAPI}
	if eapiOnly {
		modes = modes[1:]
	}
	results := make([]doctorCheck, len(domains)*len(modes))
	var wg sync.WaitGroup
	for i, domain := range domains {
		for j, mode := range modes {
			wg.Add(1)
			go func(index int, domain string, mode zlib.ClientMode) {
				defer wg.Done()
				opts := zlib.DomainProbeOptions{ProxyURL: proxyURL}
				if mode == zlib.ClientModeEAPI {
					opts.Path = eapiProbePath
					opts.RequireEAPI = true
				}
				results[index] = doctorCheck{
					DomainCheck: zlib.ProbeDomainWithOptions(ctx, domain, opts),
					Mode:        mode,
				}
			}(i*len(modes)+j, domain, mode)
		}
	}
	wg.Wait()
	return results
}

func validateDoctorProxy(rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", nil
	}
	proxyURL, err := url.ParseRequestURI(rawURL)
	if err != nil || proxyURL.Host == "" {
		return "", fmt.Errorf("invalid proxy URL")
	}
	switch strings.ToLower(proxyURL.Scheme) {
	case "http", "https", "socks5", "socks5h":
	default:
		return "", fmt.Errorf("unsupported proxy scheme %q", proxyURL.Scheme)
	}
	if proxyURL.Path != "" && proxyURL.Path != "/" || proxyURL.RawQuery != "" || proxyURL.Fragment != "" {
		return "", fmt.Errorf("proxy URL must not contain a path, query, or fragment")
	}
	return rawURL, nil
}

func doctorStatusColor(status zlib.DomainStatus) lipgloss.Color {
	switch status {
	case zlib.DomainStatusHealthy, zlib.DomainStatusChallenged:
		// Challenged domains are usable; the client clears the challenge.
		return currentTheme.Success
	case zlib.DomainStatusDiamWallBlocked, zlib.DomainStatusRedirectLoop:
		return currentTheme.Error
	default:
		return currentTheme.Muted
	}
}

func init() {
	doctorCmd.Flags().Bool("json", false, "Output domain checks as JSON")
	doctorCmd.Flags().String("proxy", "", "Proxy URL for checks (http, https, socks5, or socks5h)")
	doctorCmd.Flags().Bool("eapi", false, "Check only the EAPI health endpoint (both endpoints are checked by default)")
	_ = doctorCmd.Flags().MarkDeprecated("eapi", "doctor now checks both HTML and EAPI automatically")
}
