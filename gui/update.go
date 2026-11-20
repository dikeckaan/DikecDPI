package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	releasesAPI  = "https://api.github.com/repos/dikeckaan/DikecDPI/releases/latest"
	releasesPage = "https://github.com/dikeckaan/DikecDPI/releases/latest"
	caskName     = "dikec-dpi"
)

// UpdateInfo is what the UI shows about new versions.
type UpdateInfo struct {
	Current    string    `json:"current"`
	Latest     string    `json:"latest"`
	Available  bool      `json:"available"`
	Notes      string    `json:"notes"`
	URL        string    `json:"url"`
	ViaBrew    bool      `json:"viaBrew"`
	CheckedAt  time.Time `json:"checkedAt"`
	Installing bool      `json:"installing"`
	Error      string    `json:"error,omitempty"`
}

type updater struct {
	mu   sync.Mutex
	info UpdateInfo
}

func newUpdater() *updater {
	return &updater{info: UpdateInfo{Current: version, URL: releasesPage}}
}

func (u *updater) get() UpdateInfo {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.info
}

// check asks GitHub for the latest release. Only the version metadata is
// read; nothing is downloaded or executed here.
func (u *updater) check(ctx context.Context) UpdateInfo {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	info := u.get()
	info.Current = version
	info.CheckedAt = time.Now()
	info.ViaBrew = brewPath() != "" && installedViaBrew()
	info.Error = ""

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, releasesAPI, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "DikecDPI/"+version)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		info.Error = "güncelleme denetlenemedi: " + err.Error()
	} else {
		defer func() { _ = resp.Body.Close() }()
		var rel struct {
			TagName string `json:"tag_name"`
			Body    string `json:"body"`
			HTMLURL string `json:"html_url"`
		}
		switch {
		case resp.StatusCode != http.StatusOK:
			info.Error = "güncelleme denetlenemedi: " + resp.Status
		case json.NewDecoder(resp.Body).Decode(&rel) != nil:
			info.Error = "güncelleme yanıtı okunamadı"
		default:
			info.Latest = strings.TrimPrefix(rel.TagName, "v")
			info.Notes = rel.Body
			if rel.HTMLURL != "" {
				info.URL = rel.HTMLURL
			}
			info.Available = newerThan(info.Latest, version)
		}
	}

	u.mu.Lock()
	installing := u.info.Installing
	u.info = info
	u.info.Installing = installing
	out := u.info
	u.mu.Unlock()
	return out
}

// newerThan compares dotted numeric versions ("1.10.0" > "1.9.3").
// Development builds ("dev") never report updates.
func newerThan(latest, current string) bool {
	l, okL := parseVersion(latest)
	c, okC := parseVersion(current)
	if !okL || !okC {
		return false
	}
	for i := 0; i < 3; i++ {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return false
}

func parseVersion(s string) ([3]int, bool) {
	var v [3]int
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	s = strings.SplitN(s, "-", 2)[0] // drop pre-release suffix
	parts := strings.Split(s, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return v, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return v, false
		}
		v[i] = n
	}
	return v, true
}

func brewPath() string {
	for _, p := range []string{"/opt/homebrew/bin/brew", "/usr/local/bin/brew"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// installedViaBrew reports whether this app bundle is the one Homebrew
// installed (so `brew upgrade` will replace it).
func installedViaBrew() bool {
	for _, room := range []string{"/opt/homebrew/Caskroom", "/usr/local/Caskroom"} {
		if _, err := os.Stat(filepath.Join(room, caskName)); err == nil {
			exe, err := os.Executable()
			return err == nil && strings.HasPrefix(exe, "/Applications/Dikec DPI.app/")
		}
	}
	return false
}

func updateLogPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "DikecDPI", "update.log")
}

// updateScript waits for the app (pid) to exit, upgrades the cask and reopens
// the app. Homebrew refuses to quit an app it is running inside of, so the
// app quits itself first and the upgrade runs only after it is gone (the
// script is reparented to launchd by then). Homebrew verifies the
// download's SHA-256 from the tap.
func updateScript(brew, openCmd, logFile string, pid int) string {
	return fmt.Sprintf(
		`n=0; while kill -0 %d 2>/dev/null && [ $n -lt 150 ]; do sleep 0.2; n=$((n+1)); done; `+
			`%s update >>%s 2>&1; %s upgrade --cask %s >>%s 2>&1; status=$?; `+
			`%s; exit $status`,
		pid,
		shQuote(brew), shQuote(logFile), shQuote(brew), caskName, shQuote(logFile),
		openCmd,
	)
}

// install starts the detached updater; the caller must then quit the app.
func (u *updater) install() error {
	brew := brewPath()
	if brew == "" || !installedViaBrew() {
		return errNotBrew
	}
	u.mu.Lock()
	if u.info.Installing {
		u.mu.Unlock()
		return errors.New("güncelleme zaten sürüyor")
	}
	u.info.Installing = true
	u.mu.Unlock()

	logFile := updateLogPath()
	_ = os.MkdirAll(filepath.Dir(logFile), 0o755)
	script := updateScript(brew, `/usr/bin/open -a "Dikec DPI"`, logFile, os.Getpid())
	cmd := exec.Command("/bin/sh", "-c", script)
	// Own process group: it must survive this app being quit by brew.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Env = append(os.Environ(), "HOMEBREW_NO_ENV_HINTS=1")
	if err := cmd.Start(); err != nil {
		u.mu.Lock()
		u.info.Installing = false
		u.mu.Unlock()
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

var errNotBrew = errors.New("bu kurulum Homebrew ile yapılmamış")
