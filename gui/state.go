package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/xvzc/spoofdpi/internal/engine"
)

// persisted is everything Dikec DPI remembers between launches.
type persisted struct {
	Version       int               `json:"version"`
	SelectedID    string            `json:"selectedId"`
	AutoProfile   *engine.Profile   `json:"autoProfile,omitempty"`
	CustomProfile *engine.Profile   `json:"customProfile,omitempty"`
	Options       engine.Options    `json:"options"`
	Targets       []string          `json:"targets"`
	LastScan      *scanSummary      `json:"lastScan,omitempty"`
	StartOnLaunch bool              `json:"startOnLaunch"`
	ShowTrayCount bool              `json:"showTrayCount"`
	FirstRunDone  bool              `json:"firstRunDone"`
	SavedAt       time.Time         `json:"savedAt"`
	Extra         map[string]string `json:"extra,omitempty"`
}

type scanSummary struct {
	At        time.Time            `json:"at"`
	Duration  string               `json:"duration"`
	Combos    int                  `json:"combos"`
	Blocked   []string             `json:"blocked"`
	Reachable []string             `json:"reachable"`
	Best      *engine.Profile      `json:"best,omitempty"`
	Results   []engine.ComboResult `json:"results"`
	DNS       []dnsSummary         `json:"dns"`
}

type dnsSummary struct {
	Name      string `json:"name"`
	OK        bool   `json:"ok"`
	AvgMillis int64  `json:"avgMillis"`
}

func defaultPersisted() *persisted {
	return &persisted{
		Version:       1,
		SelectedID:    "tr",
		Options:       engine.DefaultOptions(),
		Targets:       append([]string(nil), engine.DefaultTargets...),
		ShowTrayCount: true,
	}
}

func statePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "DikecDPI", "state.json"), nil
}

func loadState() *persisted {
	s := defaultPersisted()
	p, err := statePath()
	if err != nil {
		return s
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return s
	}
	if err := json.Unmarshal(b, s); err != nil {
		// A corrupt file must never prevent the app from starting.
		return defaultPersisted()
	}
	if len(s.Targets) == 0 {
		s.Targets = append([]string(nil), engine.DefaultTargets...)
	}
	if s.Options.ListenAddr == "" {
		s.Options = engine.DefaultOptions()
	}
	return s
}

func (s *persisted) save() error {
	p, err := statePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	s.SavedAt = time.Now()
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	// Write atomically so a crash mid-write cannot corrupt the settings.
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func summarize(r *engine.ScanReport) *scanSummary {
	s := &scanSummary{
		At:        time.Now(),
		Duration:  r.Duration.Round(100 * time.Millisecond).String(),
		Combos:    len(r.Results),
		Blocked:   r.Blocked,
		Reachable: r.Reachable,
		Best:      r.Best,
		Results:   r.Results,
	}
	for _, d := range r.DNS {
		s.DNS = append(s.DNS, dnsSummary{Name: d.Name, OK: d.OK, AvgMillis: d.AvgMillis})
	}
	return s
}

var errNoProfile = errors.New("profil bulunamadı")
