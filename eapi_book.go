package zlib

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func makeEAPIBookRef(id, hash string) string {
	if id == "" || hash == "" {
		return ""
	}
	return id + ":" + hash
}

func parseEAPIBookRef(ref string) (string, string, error) {
	id, hash, ok := strings.Cut(strings.TrimSpace(ref), ":")
	if !ok || id == "" || hash == "" || strings.Contains(hash, ":") {
		return "", "", fmt.Errorf("zlibrary: EAPI book ID must use id:hash format")
	}
	return id, hash, nil
}

func (c *Client) fetchBookEAPI(ref string) (Book, error) {
	id, hash, err := parseEAPIBookRef(ref)
	if err != nil {
		return Book{}, err
	}
	path := "/eapi/book/" + url.PathEscape(id) + "/" + url.PathEscape(hash)
	body, err := c.eapiGet(path, nil)
	if err != nil {
		return Book{}, err
	}
	var result struct {
		Success int       `json:"success"`
		Error   string    `json:"error"`
		Book    *eapiBook `json:"book"`
	}
	if err := decodeEAPIJSON(body, &result); err != nil {
		return Book{}, fmt.Errorf("%w: invalid EAPI book response: %v", ErrEndpointResponse, err)
	}
	if result.Success != 1 {
		if result.Error == "" {
			result.Error = "book request was rejected"
		}
		return Book{}, fmt.Errorf("zlibrary: EAPI book failed: %s", result.Error)
	}
	if result.Book == nil {
		return Book{}, fmt.Errorf("%w: missing EAPI book", ErrEndpointResponse)
	}
	if eapiString(result.Book.ID) == "" {
		result.Book.ID = id
	}
	if result.Book.Hash == "" {
		result.Book.Hash = hash
	}
	return c.mapEAPIBook(*result.Book), nil
}

func (c *Client) resolveEAPIFileURL(ctx context.Context, rawURL string) (string, error) {
	target, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	// url.Parse returns a nil *URL on failure, and SetDomain assigns c.domain
	// before validating it, so this error must not be discarded.
	base, err := url.Parse(c.domain)
	if err != nil {
		return "", fmt.Errorf("zlibrary: invalid client domain %q: %w", c.domain, err)
	}
	if target.Scheme != base.Scheme || !strings.EqualFold(target.Host, base.Host) ||
		!strings.HasPrefix(target.Path, "/eapi/book/") || !strings.HasSuffix(target.Path, "/file") {
		return rawURL, nil
	}

	body, err := c.eapiRequest(ctx, http.MethodGet, target.EscapedPath(), nil)
	if err != nil {
		return "", err
	}
	var result struct {
		Success int    `json:"success"`
		Error   string `json:"error"`
		File    struct {
			DownloadLink string `json:"downloadLink"`
		} `json:"file"`
		DownloadLink string `json:"downloadLink"`
		URL          string `json:"url"`
		Link         string `json:"link"`
	}
	if err := decodeEAPIJSON(body, &result); err != nil {
		return "", fmt.Errorf("%w: invalid EAPI file response: %v", ErrEndpointResponse, err)
	}
	if result.Success != 1 {
		if result.Error == "" {
			result.Error = "file request was rejected"
		}
		return "", fmt.Errorf("zlibrary: EAPI file request failed: %s", result.Error)
	}
	resolved := result.File.DownloadLink
	if resolved == "" {
		resolved = result.DownloadLink
	}
	if resolved == "" {
		resolved = result.URL
	}
	if resolved == "" {
		resolved = result.Link
	}
	if resolved == "" {
		return "", fmt.Errorf("%w: EAPI returned no file link", ErrEndpointResponse)
	}
	return absolutizeURL(c.domain, resolved), nil
}

func (c *Client) resolveDownloadURL(ctx context.Context, rawURL string) (string, error) {
	target, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(target.Path, "/eapi/book/") || !strings.HasSuffix(target.Path, "/file") {
		return rawURL, nil
	}
	origin := target.Scheme + "://" + target.Host
	if origin != c.Domain() {
		if _, err := ParseAllowedDomain(origin); err != nil {
			return "", err
		}
	}
	parts := strings.Split(strings.Trim(target.Path, "/"), "/")
	if len(parts) != 5 || parts[2] == "" || parts[3] == "" {
		return "", fmt.Errorf("invalid EAPI download reference")
	}
	return runOperation(c, ctx, func(d *Client) (string, error) {
		if d.mode == ClientModeHTML {
			book, err := d.FetchBook(parts[2] + "/" + parts[3])
			return book.DownloadURL, err
		}
		return d.resolveEAPIFileURL(ctx, d.domain+target.EscapedPath())
	}, func(d *Client, _ error) (string, error) {
		book, err := d.FetchBook(parts[2] + "/" + parts[3])
		if err == nil && book.DownloadURL == "" {
			err = ErrDownloadFailed
		}
		return book.DownloadURL, err
	})
}
