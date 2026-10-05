package webhook

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

var ErrTargetDenied = errors.New("webhook target denied by network policy")

type Resolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

type NetworkPolicy struct {
	resolver     Resolver
	allowedHosts map[string]bool
	allowedCIDRs []netip.Prefix
}

func NewNetworkPolicy(allowlist []string) (*NetworkPolicy, error) {
	policy := &NetworkPolicy{resolver: net.DefaultResolver, allowedHosts: map[string]bool{}}
	for _, entry := range allowlist {
		entry = strings.TrimSpace(strings.ToLower(entry))
		if entry == "" {
			continue
		}
		if prefix, err := netip.ParsePrefix(entry); err == nil {
			policy.allowedCIDRs = append(policy.allowedCIDRs, prefix)
			continue
		}
		if strings.ContainsAny(entry, "/:@") {
			return nil, fmt.Errorf("invalid webhook allowlist entry")
		}
		policy.allowedHosts[entry] = true
	}
	return policy, nil
}

func (p *NetworkPolicy) ValidateURL(rawURL string) (*url.URL, error) {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil {
		return nil, ErrTargetDenied
	}
	return parsed, nil
}

func (p *NetworkPolicy) ResolveAndValidate(ctx context.Context, host string) ([]net.IPAddr, error) {
	normalized := strings.ToLower(strings.TrimSuffix(host, "."))
	addresses, err := p.resolver.LookupIPAddr(ctx, normalized)
	if err != nil {
		return nil, err
	}
	if len(addresses) == 0 {
		return nil, ErrTargetDenied
	}
	hostAllowed := p.allowedHosts[normalized]
	for _, address := range addresses {
		ip, ok := netip.AddrFromSlice(address.IP)
		if !ok {
			return nil, ErrTargetDenied
		}
		ip = ip.Unmap()
		if hostAllowed || p.ipAllowed(ip) {
			continue
		}
		if !safePublicIP(ip) {
			return nil, ErrTargetDenied
		}
	}
	return addresses, nil
}

func (p *NetworkPolicy) ipAllowed(ip netip.Addr) bool {
	for _, prefix := range p.allowedCIDRs {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}

func safePublicIP(ip netip.Addr) bool {
	if !ip.IsValid() || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return false
	}
	for _, raw := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32"} {
		if netip.MustParsePrefix(raw).Contains(ip) {
			return false
		}
	}
	return true
}

func (p *NetworkPolicy) HTTPClient(connectTimeout, requestTimeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: connectTimeout, KeepAlive: 30 * time.Second}
	transport := &http.Transport{DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, ErrTargetDenied
		}
		addresses, err := p.ResolveAndValidate(ctx, host)
		if err != nil {
			return nil, err
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(addresses[0].IP.String(), port))
	}}
	return &http.Client{Transport: transport, Timeout: requestTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}
