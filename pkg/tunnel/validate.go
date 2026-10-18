package tunnel

import (
	"fmt"
	"net"
	"regexp"
	"strings"
)

// hostnameLabel is one RFC 1123 DNS label: alphanumerics and inner hyphens.
var hostnameLabel = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?$`)

// ValidateHost accepts an IP address or an RFC 1123 hostname. Anything else is
// rejected, so a host can never be read by socat as an option or extra address.
func ValidateHost(host string) error {
	if net.ParseIP(host) != nil {
		return nil
	}
	if host == "" || len(host) > 253 {
		return fmt.Errorf("invalid host %q: must be a hostname or IP address", host)
	}
	for _, label := range strings.Split(host, ".") {
		if !hostnameLabel.MatchString(label) {
			return fmt.Errorf("invalid host %q: must be a hostname or IP address", host)
		}
	}
	return nil
}

// ValidatePort accepts a TCP port number.
func ValidatePort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid port %d: must be between 1 and 65535", port)
	}
	return nil
}

// maxUserLen keeps "synacklab-tunnel-<user>-<suffix>" short and the user label
// well inside Kubernetes' 63-character label limit.
const maxUserLen = 20

var nonNameChars = regexp.MustCompile(`[^a-z0-9]+`)

// SanitizeUser turns a local OS username into a lowercase DNS-1123 fragment
// usable in a pod name and label value, falling back to "user".
func SanitizeUser(name string) string {
	s := strings.Trim(nonNameChars.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if len(s) > maxUserLen {
		s = strings.TrimRight(s[:maxUserLen], "-")
	}
	if s == "" {
		return "user"
	}
	return s
}

// Validate checks the target before it is used in socat's arguments.
func (t Target) Validate() error {
	if err := ValidateHost(t.Host); err != nil {
		return err
	}
	return ValidatePort(t.Port)
}
