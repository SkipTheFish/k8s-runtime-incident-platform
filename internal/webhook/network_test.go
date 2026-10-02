package webhook

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"testing"
)

type staticResolver map[string][]net.IPAddr

func (r staticResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	return r[host], nil
}

func TestNetworkPolicyRejectsUnsafeAddresses(t *testing.T) {
	policy, err := NewNetworkPolicy(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, address := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "192.168.1.2", "::1", "fe80::1", "fc00::1", "2001:db8::1"} {
		policy.resolver = staticResolver{"target": {{IP: net.ParseIP(address)}}}
		if _, err := policy.ResolveAndValidate(context.Background(), "target"); !errors.Is(err, ErrTargetDenied) {
			t.Errorf("address %s err=%v", address, err)
		}
	}
	for _, rawURL := range []string{"ftp://example.com/hook", "http://user:pass@example.com/hook", "http:///missing"} {
		if _, err := policy.ValidateURL(rawURL); !errors.Is(err, ErrTargetDenied) {
			t.Errorf("url=%s err=%v", rawURL, err)
		}
	}
}

func TestNetworkPolicyRejectsMixedDNSAndAllowsExplicitInternalTarget(t *testing.T) {
	policy, _ := NewNetworkPolicy(nil)
	policy.resolver = staticResolver{"mixed.example": {{IP: net.ParseIP("8.8.8.8")}, {IP: net.ParseIP("10.0.0.2")}}}
	if _, err := policy.ResolveAndValidate(context.Background(), "mixed.example"); !errors.Is(err, ErrTargetDenied) {
		t.Fatalf("mixed DNS err=%v", err)
	}

	allowed, err := NewNetworkPolicy([]string{"agent.monitoring.svc", "10.20.0.0/16"})
	if err != nil {
		t.Fatal(err)
	}
	allowed.resolver = staticResolver{"agent.monitoring.svc": {{IP: net.ParseIP("10.20.1.4")}}, "other": {{IP: net.ParseIP("10.20.2.5")}}}
	if _, err := allowed.ResolveAndValidate(context.Background(), "agent.monitoring.svc"); err != nil {
		t.Fatal(err)
	}
	if _, err := allowed.ResolveAndValidate(context.Background(), "other"); err != nil {
		t.Fatal(err)
	}
}

func TestRedirectsAreNotFollowed(t *testing.T) {
	policy, _ := NewNetworkPolicy(nil)
	client := policy.HTTPClient(0, 0)
	request := &http.Request{URL: &url.URL{Scheme: "https", Host: "example.com"}}
	if err := client.CheckRedirect(request, []*http.Request{{}}); !errors.Is(err, http.ErrUseLastResponse) {
		t.Fatalf("err=%v", err)
	}
}
