package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/xvzc/spoofdpi/internal/engine"
)

// App is the service the frontend talks to (window.Call "main.App.*").
type App struct {
	mu       sync.Mutex
	eng      *engine.Engine
	st       *persisted
	scanning bool
	cancel   context.CancelFunc
	lastErr  string

	// hooks set by main
	onChange func()
	openMain func()
	quit     func()
	emit     func(name string, data any)
}

func NewApp() *App {
	return &App{eng: engine.New(), st: loadState()}
}

// ---- data returned to the UI ----

type ProfileView struct {
	engine.Profile
	Label     string `json:"label"`
	NeedsRoot bool   `json:"needsRoot"`
}

type StateView struct {
	Status        engine.Status  `json:"status"`
	SelectedID    string         `json:"selectedId"`
	Selected      ProfileView    `json:"selected"`
	Profiles      []ProfileView  `json:"profiles"`
	Options       engine.Options `json:"options"`
	Targets       []string       `json:"targets"`
	Scanning      bool           `json:"scanning"`
	LastScan      *scanSummary   `json:"lastScan"`
	StartOnLaunch bool           `json:"startOnLaunch"`
	ShowTrayCount bool           `json:"showTrayCount"`
	FirstRun      bool           `json:"firstRun"`
	UnblockedNow  int            `json:"unblockedNow"`
	Error         string         `json:"error"`
	Version       string         `json:"version"`
}

func view(p engine.Profile) ProfileView {
	return ProfileView{Profile: p, Label: p.Label(), NeedsRoot: p.NeedsRoot()}
}

func (a *App) profilesLocked() []engine.Profile {
	var out []engine.Profile
	if a.st.AutoProfile != nil {
		out = append(out, *a.st.AutoProfile)
	}
	if a.st.CustomProfile != nil {
		out = append(out, *a.st.CustomProfile)
	}
	return append(out, engine.Presets()...)
}

func (a *App) profileLocked(id string) (engine.Profile, error) {
	for _, p := range a.profilesLocked() {
		if p.ID == id {
			return p, nil
		}
	}
	return engine.Profile{}, errNoProfile
}

func (a *App) selectedLocked() engine.Profile {
	if p, err := a.profileLocked(a.st.SelectedID); err == nil {
		return p
	}
	p, _ := engine.PresetByID("tr")
	return p
}

// GetState returns everything the UI renders.
func (a *App) GetState() StateView {
	a.mu.Lock()
	defer a.mu.Unlock()

	ps := a.profilesLocked()
	views := make([]ProfileView, len(ps))
	for i, p := range ps {
		views[i] = view(p)
	}
	st := a.eng.Status()
	errMsg := a.lastErr
	if errMsg == "" {
		errMsg = st.LastError
	}
	return StateView{
		Status:        st,
		SelectedID:    a.selectedLocked().ID,
		Selected:      view(a.selectedLocked()),
		Profiles:      views,
		Options:       a.st.Options,
		Targets:       a.st.Targets,
		Scanning:      a.scanning,
		LastScan:      a.st.LastScan,
		StartOnLaunch: a.st.StartOnLaunch,
		ShowTrayCount: a.st.ShowTrayCount,
		FirstRun:      !a.st.FirstRunDone,
		UnblockedNow:  a.unblockedLocked(),
		Error:         errMsg,
		Version:       version,
	}
}

// unblockedLocked counts blocked sites (per the last scan) that were
// actually used through the proxy in this session.
func (a *App) unblockedLocked() int {
	if a.st.LastScan == nil || !a.eng.Status().Running {
		return 0
	}
	n := 0
	hosts := a.eng.Hosts(0)
	for _, b := range a.st.LastScan.Blocked {
		base := strings.TrimPrefix(b, "www.")
		for _, h := range hosts {
			hn := strings.Split(h.Host, ":")[0]
			if hn == base || strings.HasSuffix(hn, "."+base) {
				n++
				break
			}
		}
	}
	return n
}

// ---- proxy control ----

func (a *App) Start() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	err := a.startLocked()
	a.changed()
	return err
}

func (a *App) startLocked() error {
	if a.eng.Status().Running {
		return nil
	}
	cfg, err := engine.BuildConfig(a.selectedLocked(), a.st.Options)
	if err != nil {
		a.lastErr = err.Error()
		return err
	}
	if err := a.eng.Start(cfg); err != nil {
		a.lastErr = friendlyStartError(err, a.st.Options.ListenAddr)
		return errors.New(a.lastErr)
	}
	a.lastErr = ""
	return nil
}

func friendlyStartError(err error, addr string) string {
	s := err.Error()
	if strings.Contains(s, "address already in use") {
		return fmt.Sprintf(
			"%s adresi başka bir uygulama tarafından kullanılıyor. Ayarlar'dan portu değiştirin.",
			addr,
		)
	}
	return s
}

func (a *App) Stop() {
	a.mu.Lock()
	a.eng.Stop()
	a.mu.Unlock()
	a.changed()
}

func (a *App) Toggle() error {
	if a.eng.Status().Running {
		a.Stop()
		return nil
	}
	return a.Start()
}

func (a *App) restartIfRunningLocked() error {
	if !a.eng.Status().Running {
		return nil
	}
	a.eng.Stop()
	return a.startLocked()
}

// SelectProfile switches profile; a running proxy is restarted with it.
func (a *App) SelectProfile(id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, err := a.profileLocked(id); err != nil {
		return err
	}
	a.st.SelectedID = id
	_ = a.st.save()
	err := a.restartIfRunningLocked()
	a.changed()
	return err
}

// SaveCustomProfile stores the user's own tweak of a profile and selects it.
func (a *App) SaveCustomProfile(p engine.Profile) error {
	p.ID, p.Name, p.Region = "custom", "Özel", "Özel"
	p.Description = "Elle düzenlenmiş ayar."
	p.Verified = false
	if _, err := engine.BuildConfig(p, a.st.Options); err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.st.CustomProfile = &p
	a.st.SelectedID = "custom"
	_ = a.st.save()
	err := a.restartIfRunningLocked()
	a.changed()
	return err
}

func (a *App) SaveOptions(o engine.Options) error {
	if _, err := engine.BuildConfig(engine.Presets()[0], o); err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.st.Options = o
	_ = a.st.save()
	err := a.restartIfRunningLocked()
	a.changed()
	return err
}

func (a *App) SavePreferences(startOnLaunch, showTrayCount bool) {
	a.mu.Lock()
	a.st.StartOnLaunch = startOnLaunch
	a.st.ShowTrayCount = showTrayCount
	_ = a.st.save()
	a.mu.Unlock()
	a.changed()
}

func (a *App) SaveTargets(targets []string) []string {
	var clean []string
	seen := map[string]bool{}
	for _, t := range targets {
		t = strings.ToLower(strings.TrimSpace(t))
		t = strings.TrimPrefix(strings.TrimPrefix(t, "https://"), "http://")
		t = strings.Trim(strings.SplitN(t, "/", 2)[0], ".")
		if t == "" || seen[t] || !strings.Contains(t, ".") {
			continue
		}
		seen[t] = true
		clean = append(clean, t)
	}
	if len(clean) == 0 {
		clean = append([]string(nil), engine.DefaultTargets...)
	}
	a.mu.Lock()
	a.st.Targets = clean
	_ = a.st.save()
	a.mu.Unlock()
	return clean
}

// ---- auto bypass ----

type ScanDone struct {
	Summary *scanSummary `json:"summary"`
	Error   string       `json:"error"`
}

// StartScan runs auto-bypass in the background and streams progress
// events ("scan:progress", "scan:done").
func (a *App) StartScan() error {
	a.mu.Lock()
	if a.scanning {
		a.mu.Unlock()
		return errors.New("tarama zaten sürüyor")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	a.scanning, a.cancel = true, cancel
	targets := append([]string(nil), a.st.Targets...)
	a.mu.Unlock()
	a.changed()

	go func() {
		defer cancel()
		rep, err := engine.Scan(
			ctx,
			engine.ScanOptions{Targets: targets, Thorough: true},
			func(p engine.ScanProgress) {
				a.emit("scan:progress", p)
			},
		)

		a.mu.Lock()
		a.scanning, a.cancel = false, nil
		done := ScanDone{}
		if err != nil {
			done.Error = err.Error()
			if errors.Is(err, context.Canceled) {
				done.Error = "Tarama iptal edildi."
			}
		} else {
			sum := summarize(rep)
			a.st.LastScan = sum
			a.st.FirstRunDone = true
			if rep.Best != nil {
				a.st.AutoProfile = rep.Best
			}
			_ = a.st.save()
			done.Summary = sum
		}
		a.mu.Unlock()

		a.emit("scan:done", done)
		a.changed()
	}()
	return nil
}

func (a *App) CancelScan() {
	a.mu.Lock()
	if a.cancel != nil {
		a.cancel()
	}
	a.mu.Unlock()
}

// ApplyAuto selects the scan winner and starts protection.
func (a *App) ApplyAuto() error {
	a.mu.Lock()
	if a.st.AutoProfile == nil {
		a.mu.Unlock()
		return errors.New("önce Otomatik Bypass taraması yapın")
	}
	a.st.SelectedID = a.st.AutoProfile.ID
	_ = a.st.save()
	a.eng.Stop()
	err := a.startLocked()
	a.mu.Unlock()
	a.changed()
	return err
}

func (a *App) DismissFirstRun() {
	a.mu.Lock()
	a.st.FirstRunDone = true
	_ = a.st.save()
	a.mu.Unlock()
}

// ---- observation ----

func (a *App) Logs(after uint64) []engine.LogEntry { return a.eng.Logs(after) }

func (a *App) Hosts(limit int) []engine.HostStat { return a.eng.Hosts(limit) }

func (a *App) ResetStats() { a.eng.ResetStats() }

func (a *App) OpenMain() {
	if a.openMain != nil {
		a.openMain()
	}
}

func (a *App) Quit() {
	if a.quit != nil {
		a.quit()
	}
}

// ServiceShutdown restores the network settings when the app exits.
func (a *App) ServiceShutdown() error {
	a.CancelScan()
	a.eng.Stop()
	return nil
}

func (a *App) changed() {
	if a.onChange != nil {
		go a.onChange()
	}
}

var _ application.ServiceShutdown = (*App)(nil)
