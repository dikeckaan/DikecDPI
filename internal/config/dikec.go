package config

import "fmt"

// Exported parsing helpers used by the Dikec DPI engine and GUI. Kept in a
// separate file to minimise merge conflicts with upstream.

// HTTPSSplitModes lists every accepted split mode name.
func HTTPSSplitModes() []string {
	return append([]string(nil), availableHTTPSModeValues...)
}

func ParseHTTPSSplitMode(s string) (HTTPSSplitModeType, error) {
	if err := checkHTTPSSplitMode(s); err != nil {
		return 0, fmt.Errorf("geçersiz bölme modu %q", s)
	}
	return mustParseHTTPSSplitModeType(s), nil
}

func ParseDNSMode(s string) (DNSModeType, error) {
	if err := checkDNSMode(s); err != nil {
		return 0, fmt.Errorf("geçersiz dns modu %q", s)
	}
	return MustParseDNSModeType(s), nil
}
