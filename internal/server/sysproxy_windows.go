//go:build windows

package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows/registry"
)

// Windows keeps the per-user proxy configuration under this key; WinINet,
// WinHTTP's IE-settings import, Edge, Chrome and most apps read it.
const internetSettingsKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

// SetupSystemProxy serves a PAC file that routes traffic through pacDirective
// and saves jobs that point the user's proxy settings at it. The previous
// AutoConfigURL is restored on reset (or removed if there was none), so a
// corporate PAC is not lost. The PAC server stops when ctx is done.
func SetupSystemProxy(
	ctx context.Context,
	pacDirective string,
	stateFile string,
) error {
	pacContent := fmt.Sprintf(
		"function FindProxyForURL(url, host) {\n    return \"%s; DIRECT\";\n}",
		pacDirective,
	)
	pacURL, pac, err := runPACServer(pacContent)
	if err != nil {
		return fmt.Errorf("error creating pac server: %w", err)
	}

	regPath := `HKCU\` + internetSettingsKey
	reset := fmt.Sprintf(`reg delete "%s" /v AutoConfigURL /f`, regPath)
	if k, err := registry.OpenKey(
		registry.CURRENT_USER,
		internetSettingsKey,
		registry.QUERY_VALUE,
	); err == nil {
		if prev, _, err := k.GetStringValue("AutoConfigURL"); err == nil && prev != "" {
			reset = fmt.Sprintf(
				`reg add "%s" /v AutoConfigURL /t REG_SZ /d "%s" /f`,
				regPath,
				prev,
			)
		}
		_ = k.Close()
	}

	jobs := []NetworkJob{
		{
			Description: "set proxy auto-config URL",
			Apply: fmt.Sprintf(
				`reg add "%s" /v AutoConfigURL /t REG_SZ /d "%s" /f`,
				regPath,
				pacURL,
			),
			Reset: reset,
		},
	}
	if err := SaveJobs(stateFile, jobs); err != nil {
		_ = pac.Close()
		return fmt.Errorf("failed to save state: %w", err)
	}
	go func() { <-ctx.Done(); _ = pac.Close() }()
	return nil
}

func runPACServer(content string) (string, *http.Server, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-ns-proxy-autoconfig")
		_, _ = w.Write([]byte(content))
	})
	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(listener) }()
	return fmt.Sprintf(
		"http://127.0.0.1:%d/proxy.pac",
		listener.Addr().(*net.TCPAddr).Port,
	), server, nil
}

var (
	wininet               = syscall.NewLazyDLL("wininet.dll")
	procInternetSetOption = wininet.NewProc("InternetSetOptionW")
)

const (
	internetOptionRefresh         = 37
	internetOptionSettingsChanged = 39
)

// notifyProxyChange tells WinINet (and through it browsers) to reload the
// proxy settings now instead of at their next start.
func notifyProxyChange() {
	if procInternetSetOption.Find() != nil {
		return
	}
	_, _, _ = procInternetSetOption.Call(
		0,
		internetOptionSettingsChanged,
		uintptr(unsafe.Pointer(nil)),
		0,
	)
	_, _, _ = procInternetSetOption.Call(
		0,
		internetOptionRefresh,
		uintptr(unsafe.Pointer(nil)),
		0,
	)
}

func init() { afterNetworkChange = notifyProxyChange }
