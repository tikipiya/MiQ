package netpolicy

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
)

func Validate(ctx context.Context, value *url.URL, allowPrivate bool) error {
	if value == nil || value.Scheme != "http" && value.Scheme != "https" {
		return fmt.Errorf("URL scheme must be http or https")
	}
	if allowPrivate {
		return nil
	}
	host := strings.TrimSuffix(strings.ToLower(value.Hostname()), ".")
	if host == "" {
		return fmt.Errorf("URL host is empty")
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return fmt.Errorf("private network URL is not allowed")
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("resolve URL host: %w", err)
	}
	if len(addresses) == 0 {
		return fmt.Errorf("URL host has no address")
	}
	for _, address := range addresses {
		ip := address.IP
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
			return fmt.Errorf("private network URL is not allowed")
		}
	}
	return nil
}
