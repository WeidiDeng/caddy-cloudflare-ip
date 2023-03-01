package caddy_cloudflare_ip

import (
	"bufio"
	"context"
	"net/http"
	"net/netip"
	"sync"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

const (
	ipv4 = "https://www.cloudflare.com/ips-v4"
	ipv6 = "https://www.cloudflare.com/ips-v6"
)

func init() {
	caddy.RegisterModule(CloudflareIPRange{})
}

// CloudflareIPRange provides a range of IP address prefixes (CIDRs) retrieved from cloudflare.
type CloudflareIPRange struct {
	// refresh Interval
	Interval caddy.Duration `json:"interval,omitempty"`
	// request Timeout
	Timeout caddy.Duration `json:"timeout,omitempty"`

	// Holds the parsed CIDR ranges from Ranges.
	ranges []netip.Prefix

	ctx  caddy.Context
	lock *sync.RWMutex
}

// CaddyModule returns the Caddy module information.
func (CloudflareIPRange) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.ip_sources.cloudflare",
		New: func() caddy.Module { return new(CloudflareIPRange) },
	}
}

// getContext returns a cancelable context, with a timeout if configured.
func (s *CloudflareIPRange) getContext() (context.Context, context.CancelFunc) {
	if s.Timeout > 0 {
		return context.WithTimeout(s.ctx, time.Duration(s.Timeout))
	}
	return context.WithCancel(s.ctx)
}

func (s *CloudflareIPRange) fetch(api string) ([]netip.Prefix, error) {
	ctx, cancel := s.getContext()
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, api, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	var prefixes []netip.Prefix
	for scanner.Scan() {
		prefix, err := caddyhttp.CIDRExpressionToPrefix(scanner.Text())
		if err != nil {
			return nil, err
		}
		prefixes = append(prefixes, prefix)
	}
	return prefixes, nil
}

func (s *CloudflareIPRange) Provision(ctx caddy.Context) error {
	s.ctx = ctx
	s.lock = new(sync.RWMutex)

	// fetch ipv4 list
	prefixes, err := s.fetch(ipv4)
	if err != nil {
		return err
	}
	s.ranges = append(s.ranges, prefixes...)

	// fetch ipv6 list
	prefixes, err = s.fetch(ipv6)
	if err != nil {
		return err
	}
	s.ranges = append(s.ranges, prefixes...)

	// update in background
	go s.refreshLoop()
	return nil
}

func (s *CloudflareIPRange) refreshLoop() {
	if s.Interval == 0 {
		s.Interval = caddy.Duration(time.Hour)
	}

	ticker := time.NewTicker(time.Duration(s.Interval))
	for {
		select {
		case <-ticker.C:
			var fullPrefixes []netip.Prefix
			prefixes, err := s.fetch(ipv4)
			if err != nil {
				break
			}
			fullPrefixes = append(fullPrefixes, prefixes...)

			prefixes, err = s.fetch(ipv6)
			if err != nil {
				break
			}
			fullPrefixes = append(fullPrefixes, prefixes...)

			s.lock.Lock()
			s.ranges = fullPrefixes
			s.lock.Unlock()
		case <-s.ctx.Done():
			ticker.Stop()
			return
		}
	}
}

func (s *CloudflareIPRange) GetIPRanges(_ *http.Request) []netip.Prefix {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.ranges
}

// UnmarshalCaddyfile implements caddyfile.Unmarshaler.
//
//	cloudflare {
//	   interval val
//	   timeout val
//	}
func (m *CloudflareIPRange) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		// No same-line options are supported
		if d.NextArg() {
			return d.ArgErr()
		}

		for d.NextBlock(0) {
			switch d.Val() {
			case "interval":
				if !d.NextArg() {
					return d.ArgErr()
				}
				val, err := caddy.ParseDuration(d.Val())
				if err != nil {
					return err
				}
				m.Interval = caddy.Duration(val)
			case "timeout":
				if !d.NextArg() {
					return d.ArgErr()
				}
				val, err := caddy.ParseDuration(d.Val())
				if err != nil {
					return err
				}
				m.Timeout = caddy.Duration(val)
			default:
				return d.ArgErr()
			}
		}
	}

	return nil
}

// interface guards
var (
	_ caddy.Module            = (*CloudflareIPRange)(nil)
	_ caddy.Provisioner       = (*CloudflareIPRange)(nil)
	_ caddyfile.Unmarshaler   = (*CloudflareIPRange)(nil)
	_ caddyhttp.IPRangeSource = (*CloudflareIPRange)(nil)
)
