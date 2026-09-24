package repository

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/AlmightyOggy/management-system/internal/domain"
)

type VSSDelayFilter struct {
	DeviceID	string
	DeviceName	string
	Reason		string
	Date		string
	Page		int
	PerPage		int
}

type VSSDelayRepository interface {
	Append(event domain.VSSDelayEvent) error
	List(filter VSSDelayFilter) (*domain.VSSDelayListResult, error)
	ExistsRecent(deviceID, reason string, within time.Duration) (bool, error)

	UpsertLive(device domain.VSSLiveDevice)
	ListLive(filter VSSLiveFilter) domain.VSSLiveResult
	IncrementCounter(action string)
}

type VSSLiveFilter struct {
	DeviceName string
	Action     string
	Status     string
	OnlyIssue  bool
	Page       int
	PerPage    int
}

type vssDelayRepository struct {
	dir	string
	mu 	sync.Mutex
	liveMu    sync.RWMutex
	liveState map[string]domain.VSSLiveDevice
	counterMu   sync.Mutex
	totalWS     int
	count80003  int
	count80004  int
}

func NewVSSDelayRepository(logDir string) VSSDelayRepository {
	_ = os.MkdirAll(logDir, 0o755)
	return &vssDelayRepository{dir: logDir}
}

func (r *vssDelayRepository) UpsertLive(device domain.VSSLiveDevice) {
	r.liveMu.Lock()
	defer r.liveMu.Unlock()
	r.liveState[device.DeviceID] = device
}

func (r *vssDelayRepository) ListLive(filter VSSLiveFilter) domain.VSSLiveResult {
	r.liveMu.RLock()
	all := make([]domain.VSSLiveDevice, 0, len(r.liveState))
	for _, d := range r.liveState {
		all = append(all, d)
	}
	r.liveMu.RUnlock()

	filtered := all[:0]
	for _, d := range all {
		if filter.DeviceName != "" && !strings.Contains(strings.ToLower(d.DeviceName), strings.ToLower(filter.DeviceName)) {
			continue
		}
		if filter.Action != "" && d.Action != filter.Action {
			continue
		}
		if filter.Status != "" && d.Status != filter.Status {
			continue
		}
		if filter.OnlyIssue && d.Status == "NORMAL" {
			continue
		}
		filtered = append(filtered, d)
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].DelayMs > filtered[j].DelayMs
	})

	normal, issue := 0, 0
	for _, d := range all {
		if d.Status == "NORMAL" {
			normal++
		} else {
			issue++
		}
	}

	r.counterMu.Lock()
	totalWS, c80003, c80004 := r.totalWS, r.count80003, r.count80004
	r.counterMu.Unlock()

	total := len(filtered)
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PerPage < 1 {
		filter.PerPage = 50
	}
	start := (filter.Page - 1) * filter.PerPage
	if start > total {
		start = total
	}
	end := start + filter.PerPage
	if end > total {
		end = total
	}

	return domain.VSSLiveResult{
		Data:       filtered[start:end],
		Total:      total,
		Page:       filter.Page,
		PerPage:    filter.PerPage,
		TotalWS:    totalWS,
		Count80003: c80003,
		Count80004: c80004,
		Normal:     normal,
		Issue:      issue,
	}
}

func (r *vssDelayRepository) IncrementCounter(action string) {
	r.counterMu.Lock()
	defer r.counterMu.Unlock()
	r.totalWS++
	switch action {
	case "80003":
		r.count80003++
	case "80004":
		r.count80004++
	}
}

func (r *vssDelayRepository) filePath(day time.Time) string {
	name := "vss-delay-" + day.Format("2006-01-02") + ".log"
	return filepath.Join(r.dir, name)
}

func (r *vssDelayRepository) Append(event domain.VSSDelayEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	path := r.filePath(time.Now())
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	b, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = f.Write(append(b, '\n'))
	return err
}

func (r *vssDelayRepository) List(filter VSSDelayFilter) (*domain.VSSDelayListResult, error) {
	day := time.Now()
	if filter.Date != "" {
		if t, err := time.Parse("2006-01-02", filter.Date); err == nil {
			day = t
		}
	}

	path := r.filePath(day)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &domain.VSSDelayListResult{
				Data: []domain.VSSDelayEvent{}, Page: 1, PerPage: filter.PerPage,
			}, nil
		}
		return nil, err
	}
	defer f.Close()

	var all []domain.VSSDelayEvent
	sc := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var ev domain.VSSDelayEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		if filter.DeviceID != "" && ev.DeviceID != filter.DeviceID {
			continue
		}
		if filter.DeviceName != "" && !strings.Contains(
			strings.ToLower(ev.DeviceName), strings.ToLower(filter.DeviceName),
		) {
			continue
		}
		if filter.Reason != "" && ev.Reason != filter.Reason {
			continue
		}
		all = append(all, ev)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].DetectedAt > all[j].DetectedAt
	})

	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PerPage < 1 {
		filter.PerPage = 20
	}
	total := len(all)
	start := (filter.Page - 1) * filter.PerPage
	if start > total {
		start = total
	}
	end := start + filter.PerPage
	if end > total {
		end = total
	}

	return &domain.VSSDelayListResult{
		Data: 		all[start:end],
		Total:		total,
		Page:		filter.Page,
		PerPage:	filter.PerPage,
	}, nil
}

func (r *vssDelayRepository) ExistsRecent(deviceID, reason string, within time.Duration) (bool, error) {
	path := r.filePath(time.Now())
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	defer f.Close()

	cutoff := time.Now().Add(-within)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var ev domain.VSSDelayEvent
		if json.Unmarshal(sc.Bytes(), &ev) != nil {
			continue
		}
		if ev.DeviceID != deviceID || ev.Reason != reason {
			continue
		}
		t, err := time.Parse(time.RFC3339, ev.DetectedAt)
		if err != nil {
			continue
		}
		if !t.Before(cutoff) {
			return true, nil
		}
	}
	return false, sc.Err()
}