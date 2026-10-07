package zlib

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// EndpointInfo describes the selected endpoint, not user configuration.
type EndpointInfo struct {
	URL         string    `json:"url"`
	Source      string    `json:"source"`
	Manual      bool      `json:"manual"`
	LatencyMS   int64     `json:"latency_ms"`
	LastSuccess time.Time `json:"last_success,omitempty"`
}

// EndpointPolicy supplies runtime cache and notifications without coupling the
// Go client to the CLI's filesystem. WithDomain is manual unless overridden.
type EndpointPolicy struct {
	DisableHTMLFallback bool
	Manual              bool
	Source              string
	LastGood            string
	Seeds               []string
	OnSuccess           func(EndpointInfo)
	OnSwitch            func(string)
}

type endpointRouter struct {
	mu     sync.Mutex
	policy EndpointPolicy
	info   EndpointInfo
}

func WithEndpointPolicy(policy EndpointPolicy) ClientOption {
	return func(c *Client) {
		c.router.policy = policy
		c.router.info = EndpointInfo{URL: c.domain, Source: policy.Source, Manual: policy.Manual}
	}
}

func (c *Client) Endpoint() EndpointInfo {
	if c.router == nil {
		return EndpointInfo{URL: c.domain}
	}
	c.router.mu.Lock()
	defer c.router.mu.Unlock()
	return c.router.info
}

// operationClient is a request-local backend. Switching it never changes the
// backend used by concurrent calls or permanently demotes an EAPI session.
func (c *Client) operationClient() *Client {
	if c.router != nil {
		c.router.mu.Lock()
		defer c.router.mu.Unlock()
	}
	return c.cloneBackend(c.domain, c.mode)
}

func (c *Client) cloneBackend(domain string, mode ClientMode) *Client {
	d := NewClient(WithDomain(domain))
	d.router = nil
	d.mode = mode
	d.ctx = c.ctx
	d.httpClient.Transport = c.httpClient.Transport
	d.httpClient.Timeout = c.httpClient.Timeout
	d.downloadRetries = c.downloadRetries
	d.SetCookies(c.cookieSnapshot())
	d.loggedIn = c.loggedIn
	d.httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return ErrRedirectLoop
		}
		if req.URL.User != nil || req.URL.Scheme != via[0].URL.Scheme || !strings.EqualFold(req.URL.Host, via[0].URL.Host) {
			return fmt.Errorf("%w: cross-origin backend redirect", ErrEndpointResponse)
		}
		return nil
	}
	return d
}

// runOperation budgets one endpoint replacement and one backend fallback per
// business operation. Probe traffic is bounded and never carries credentials.
func runOperation[T any](c *Client, ctx context.Context, primary func(*Client) (T, error), fallback func(*Client, error) (T, error)) (T, error) {
	if ctx == context.Background() {
		ctx = c.ctx
	}
	d := c.operationClient()
	d.ctx = ctx
	start := time.Now()
	value, err := primary(d)
	if err != nil && ctx.Err() == nil && c.router != nil && isEndpointFailure(err) {
		c.router.mu.Lock()
		policy := c.router.policy
		c.router.mu.Unlock()
		if fallback == nil && d.mode == ClientModeEAPI {
			policy.DisableHTMLFallback = true // EAPI-only operations need an EAPI-capable endpoint.
		}
		if !policy.Manual {
			if endpoint, resolveErr := c.findEndpoint(ctx, d.domain, d.mode, policy); resolveErr == nil {
				d.SetDomain(endpoint.URL)
				// An endpoint is selected only after a real probe, even if this business
				// request subsequently reports a permission or validation error.
				c.router.mu.Lock()
				c.domain, c.loginDomain = d.domain, buildLoginURL(d.domain)
				c.router.info = endpoint
				c.router.mu.Unlock()
				if policy.OnSwitch != nil {
					policy.OnSwitch(d.domain)
				}
				start = time.Now()
				value, err = primary(d)
			}
		}
	}
	if err != nil && ctx.Err() == nil && d.mode == ClientModeEAPI && fallback != nil && c.router != nil && !c.router.policy.DisableHTMLFallback && isBackendFailure(err) {
		d.mode = ClientModeHTML
		start = time.Now()
		value, err = fallback(d, err)
	}
	if err == nil {
		if c.router != nil {
			c.router.mu.Lock()
			// A concurrent successful operation at an older endpoint must not undo
			// a failover which another operation has already adopted.
			if c.domain == d.domain {
				c.router.info.URL = d.domain
				c.router.info.LatencyMS = time.Since(start).Milliseconds()
				c.router.info.LastSuccess = time.Now()
				info, callback := c.router.info, c.router.policy.OnSuccess
				// Merge challenge/authentication updates under the cookie lock.
				c.cookiesMu.Lock()
				for key, value := range d.cookieSnapshot() {
					c.cookies[key] = value
				}
				c.loggedIn = d.loggedIn
				c.cookiesMu.Unlock()
				c.router.mu.Unlock()
				if callback != nil {
					callback(info)
				}
			} else {
				c.router.mu.Unlock()
			}
		}
	}
	return value, err
}

func isEndpointFailure(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	var business *EAPIBusinessError
	if errors.As(err, &business) {
		return false
	}
	if errors.Is(err, ErrEndpointResponse) || errors.Is(err, ErrBotProtection) || errors.Is(err, ErrRedirectLoop) || errors.Is(err, ErrChallengeFailed) {
		return true
	}
	var status *HTTPStatusError
	if errors.As(err, &status) {
		return isRetryableStatus(status.StatusCode) || isDiamWallStatus(status.StatusCode)
	}
	var network net.Error
	var transport *url.Error
	return errors.As(err, &network) || errors.As(err, &transport) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
}

func isBackendFailure(err error) bool {
	var business *EAPIBusinessError
	if errors.As(err, &business) {
		// A measured login-endpoint gate, not a general permission fallback.
		return business.LoginGate
	}
	return isEndpointFailure(err)
}

const maxEndpointCandidates = 16

func (c *Client) findEndpoint(ctx context.Context, failed string, mode ClientMode, policy EndpointPolicy) (EndpointInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*domainProbeTimeout)
	defer cancel()
	candidates := []string{policy.LastGood}
	seeds := policy.Seeds
	if seeds == nil {
		seeds = append([]string{DefaultEAPIDomain}, KnownDomains()...)
	}
	candidates = append(candidates, seeds...)
	seen := map[string]bool{failed: true}
	selectCandidate := func(raw []string, source string) EndpointInfo {
		domains := make([]string, 0, len(raw))
		for _, candidate := range raw {
			domain, err := ParseAllowedDomain(candidate)
			if err != nil || seen[domain] || len(seen) > maxEndpointCandidates {
				continue
			}
			seen[domain] = true
			domains = append(domains, domain)
		}
		checks := make([]DomainCheck, len(domains))
		htmlChecks := make([]DomainCheck, len(domains))
		elapsed := make([]int64, len(domains))
		var wg sync.WaitGroup
		for i, domain := range domains {
			wg.Add(1)
			go func(i int, domain string) {
				defer wg.Done()
				start := time.Now()
				opts := DomainProbeOptions{Transport: c.httpClient.Transport, RequireAllowedDomain: true}
				if mode == ClientModeEAPI {
					opts.Path = EAPIProbePath
					opts.RequireEAPI = true
				}
				checks[i] = ProbeDomainWithOptions(ctx, domain, opts)
				if mode == ClientModeEAPI && !policy.DisableHTMLFallback && checks[i].Status != DomainStatusHealthy && checks[i].Status != DomainStatusInvalidDomain && ctx.Err() == nil {
					html := ProbeDomainWithOptions(ctx, domain, DomainProbeOptions{Transport: c.httpClient.Transport, RequireAllowedDomain: true})
					if html.Status == DomainStatusHealthy || html.Status == DomainStatusChallenged {
						htmlChecks[i] = html
					}
				}
				elapsed[i] = time.Since(start).Milliseconds()
			}(i, domain)
		}
		wg.Wait()
		// Prefer an EAPI-capable candidate over an earlier HTML-only candidate.
		// Candidate order remains deterministic within each backend preference.
		batches := [][]DomainCheck{checks}
		if mode == ClientModeEAPI && !policy.DisableHTMLFallback {
			batches = append(batches, htmlChecks)
		}
		for batch, results := range batches {
			for i, check := range results {
				html := mode == ClientModeHTML || batch == 1
				if check.Status != DomainStatusHealthy && !(html && check.Status == DomainStatusChallenged) {
					continue
				}
				final, parseErr := url.Parse(check.FinalURL)
				if parseErr != nil {
					continue
				}
				domain, err := ParseAllowedDomain(final.Scheme + "://" + final.Host)
				if err != nil || domain == failed {
					continue
				}
				selectedSource := source
				if domain == policy.LastGood {
					selectedSource = "last_good"
				}
				return EndpointInfo{URL: domain, Source: selectedSource, LatencyMS: elapsed[i]}
			}
		}
		return EndpointInfo{}
	}
	if info := selectCandidate(candidates, "seed"); info.URL != "" {
		return info, nil
	}
	if ctx.Err() != nil {
		return EndpointInfo{}, ctx.Err()
	}
	// Discovery is read-only, once, and uses only allowlisted origins. It never
	// expands the credential allowlist or trusts an advertised host without a probe.
	discovered := c.discoverEndpoints(ctx, append([]string{failed}, seeds...))
	if info := selectCandidate(discovered, "discovery"); info.URL != "" {
		return info, nil
	}
	return EndpointInfo{}, ErrNoDomain
}

func (c *Client) discoverEndpoints(ctx context.Context, origins []string) []string {
	seen := map[string]bool{}
	for _, raw := range origins {
		origin, err := ParseAllowedDomain(raw)
		if err != nil || seen[origin] {
			continue
		}
		seen[origin] = true
		if len(seen) > 3 {
			break
		}
		d := c.operationClient()
		d.SetDomain(origin)
		d.mode = ClientModeEAPI
		d.Logout()
		probeCtx, cancel := context.WithTimeout(ctx, domainProbeTimeout)
		body, err := d.eapiRequest(probeCtx, http.MethodGet, EAPIProbePath, nil)
		cancel()
		if err != nil {
			continue
		}
		domains := parseDiscoveredDomains(body)
		if len(domains) > 0 {
			return domains
		}
	}
	return nil
}
