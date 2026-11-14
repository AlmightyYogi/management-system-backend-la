package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"math"
	"time"
	"github.com/xuri/excelize/v2"

	"github.com/AlmightyOggy/management-system/internal/config"
	"github.com/AlmightyOggy/management-system/internal/domain"
	"github.com/AlmightyOggy/management-system/internal/repository"
	"github.com/AlmightyOggy/management-system/internal/utils"
	"github.com/gorilla/websocket"
)

var jakartaTZ, _ = time.LoadLocation("Asia/Jakarta")

type VSSMonitorService interface {
	Start(ctx context.Context)
	ListDelays(filter repository.VSSDelayFilter) (*domain.VSSDelayListResult, error)
	ListHistory(filter repository.VSSHistoryFilter) ([]domain.VSSAlertHistory, int64, error)
	ListLive(deviceName, action, status string, onlyIssue bool, page, perPage int, sortBy, sortDir string) *VSSLiveResult
	GetTraffic() *VSSTrafficDetail
	GetDaySummary(date string) (*domain.VSSDaySummary, error)
	GetDayDelayExport(date string) ([]domain.VSSDayDeviceDelayStat, error)
	SendDailyReport(date string) error
	GetDeviceDelayStats(deviceID, rangeKey string) (*domain.VSSDeviceDelayStats, error)
	GetDeviceDelayHistory(deviceID, rangeKey string) ([]domain.VSSDeviceDelayPoint, error)
}

type VSSDeviceLive struct {
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
	NodeID     string `json:"node_id"`
	DTU        string `json:"dtu"`
	ReportTime int64  `json:"report_time"`
	IsLater    bool   `json:"is_later"`
	DelaySec   int64  `json:"delay_sec"`
	LastSeen   string `json:"last_seen"`
	Action     string `json:"action"`
	Status     string `json:"status"`
	Reasons    []string `json:"reasons"`
	CPU        string `json:"cpu,omitempty"`
	StorageFree string `json:"storage_free,omitempty"`
}

type VSSTrafficStats struct {
	TotalMessages int64 `json:"total_messages"`
	Total80003    int64 `json:"total_80003"`
	Total80004    int64 `json:"total_80004"`
	DeviceCount   int   `json:"device_count"`
	NormalCount   int   `json:"normal_count"`
	IssueCount    int   `json:"issue_count"`
}

type MinuteTraffic struct {
	Minute string           `json:"minute"`
	Counts map[string]int64 `json:"counts"`
	Total  int64            `json:"total"`
}

type VSSTrafficDetail struct {
	Totals         map[string]int64 `json:"totals"`
	TotalMessages  int64            `json:"total_messages"`
	CurrentMinute  MinuteTraffic    `json:"current_minute"`
	Series         []MinuteTraffic  `json:"series"`
	TrackedActions []string         `json:"tracked_actions"`
}

type VSSLiveResult struct {
	Traffic VSSTrafficStats  `json:"traffic"`
	Data    []VSSDeviceLive  `json:"data"`
	Total   int              `json:"total"`
}

type vssMonitorService struct {
	repo         repository.VSSDelayRepository
	historyRepo  repository.VSSAlertHistoryRepository
	cfg          config.VSSConfig
	mu           sync.Mutex
	lastSeen     map[string]time.Time
	deviceName   map[string]string
	lastDTU      map[string]string
	lastReportMs map[string]int64
	live         map[string]*VSSDeviceLive
	trafficTotal int64
	traffic80003 int64
	traffic80004 int64
	logReasons   map[string]bool
	emailReasons map[string]bool
	emailMu      sync.Mutex
	lastEmail    map[string]time.Time
	pendingAlerts []domain.VSSDelayEvent
	trafficByAction map[string]int64
	currentMinute   time.Time
	currentCounts   map[string]int64
	minuteHistory   []MinuteTraffic
	recentSamples   map[string][]delaySamplePoint
	liveDirty      bool
	snapshotPath   string
	sampleCh chan domain.VSSDelayEvent
	eventCh  chan domain.VSSDelayEvent
}

type wsEnvelope struct {
	Action  string          `json:"action"`
	Payload json.RawMessage `json:"payload"`
}

type liveSnapshotFile struct {
	SavedAt string                    `json:"saved_at"`
	Devices []VSSDeviceLive           `json:"devices"`
	LastSeen map[string]string        `json:"last_seen"`
	LastDTU  map[string]string        `json:"last_dtu"`
	LastReportMs map[string]int64     `json:"last_report_ms"`
	DeviceName map[string]string      `json:"device_name"`
}

type statusPayload struct {
	DeviceID string `json:"deviceID"`
	NodeID   string `json:"nodeID"`
	DTU      string `json:"dtu"`
	IsLater  string `json:"isLater"`
	Ext      struct {
		IsLater    bool   `json:"isLater"`
		DeviceName string `json:"deviceName"`
		ReportTime int64  `json:"reportTime"`
		AccessMode int    `json:"accessMode"`
	} `json:"ext"`
	Location struct {
		DTU string `json:"dtu"`
	} `json:"location"`
	Storage []struct {
		Name   string `json:"name"`
		Free   string `json:"free"`
		Status string `json:"status"`
	} `json:"storage"`
	DevTemp struct {
		CPU  string `json:"cpu"`
		Disk string `json:"disk"`
	} `json:"devtemp"`
}

type alarmPayload80004 struct {
	DeviceID    string `json:"deviceID"`
	DeviceName  string `json:"deviceName"`
	NodeID      string `json:"nodeID"`
	DTU         string `json:"dtu"`
	IsLater     int    `json:"isLater"`
	EventType   string `json:"eventType"`
	AlarmID     string `json:"alarmID"`
	AlarmDetail string `json:"alarmDetail"`
	Storage     []struct {
		Name string `json:"name"`
		Free string `json:"free"`
	} `json:"storage"`
	DevTemp struct {
		CPU string `json:"cpu"`
	} `json:"devtemp"`
	Payload struct {
		DTU string `json:"dtu"`
		ST  string `json:"st"`
		ET  string `json:"et"`
	} `json:"payload"`
}

type delaySamplePoint struct {
	ReceivedAt time.Time
	DTU        string
	DelaySec   int64
	Action     string
	Reason     string
}

func NewVSSMonitorService(repo repository.VSSDelayRepository, historyRepo repository.VSSAlertHistoryRepository, cfg config.VSSConfig) VSSMonitorService {
	if cfg.DelayThresholdSec <= 0 {
		cfg.DelayThresholdSec = 60
	}
	if cfg.StaleThresholdSec <= 0 {
		cfg.StaleThresholdSec = 300
	}
	if strings.TrimSpace(cfg.LogReasons) == "" {
		cfg.LogReasons = "delayed,stale_timestamp,no_heartbeat,disconnect"
	}
	if cfg.LogRetentionDays <= 0 {
		cfg.LogRetentionDays = 30
	}

	svc := &vssMonitorService{
		repo: 			repo,
		cfg: 			cfg,
		historyRepo: 	historyRepo,
		lastSeen: 		make(map[string]time.Time),
		deviceName: 	make(map[string]string),
		lastDTU:      	make(map[string]string),
		lastReportMs: 	make(map[string]int64),
		logReasons: 	parseReasonSet(cfg.LogReasons),
		live: 			make(map[string]*VSSDeviceLive),
		trafficByAction: make(map[string]int64),
		currentCounts:   make(map[string]int64),
		minuteHistory:   make([]MinuteTraffic, 0, 24*60),
		recentSamples:   make(map[string][]delaySamplePoint),
		sampleCh:        make(chan domain.VSSDelayEvent, 10000),
		eventCh:         make(chan domain.VSSDelayEvent, 5000),
	}

	if cfg.EmailCooldownMin <= 0 {
		cfg.EmailCooldownMin = 15
		svc.cfg = cfg
	}
	svc.emailReasons = parseReasonSet(cfg.EmailReasons)
	svc.lastEmail = make(map[string]time.Time)

	logDir := strings.TrimSpace(cfg.LogDir)
	if logDir == "" {
		logDir = "storage/public/logs/vss"
	}
	_ = os.MkdirAll(logDir, 0o755)
	svc.snapshotPath = filepath.Join(logDir, "vss-live-snapshot.json")

	log.Printf("[VSS] log reasons allowed: %v", keysOf(svc.logReasons))
	return svc
}

func parseReasonSet(s string) map[string]bool {
	m := make(map[string]bool)
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(strings.ToLower(p))
		if p != "" {
			m[p] = true
		}
	}
	return m
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (s *vssMonitorService) reasonAllowedForLog(reason string) bool {
	if len(s.logReasons) == 0 {
		switch strings.ToLower(reason) {
		case "delayed", "stale_timestamp", "no_heartbeat", "disconnect":
			return true
		default:
			return false
		}
	}
	return s.logReasons[strings.ToLower(reason)]
}

func (s *vssMonitorService) reasonAllowedForEmail(reason string) bool {
	if !s.cfg.EmailEnabled {
		return false
	}
	if len(s.emailReasons) == 0 {
		return false
	}
	return s.emailReasons[strings.ToLower(reason)]
}

func (s *vssMonitorService) Start(ctx context.Context) {
	if !s.cfg.Enabled {
		log.Println("[VSS] monitor disabled (VSS_ENABLED=false)")
		return
	}
	now := time.Now()
	log.Printf("[VSS] server time local=%s utc=%s jakarta=%s",
		now.Format(time.RFC3339),
		now.UTC().Format(time.RFC3339),
		now.In(jakartaTZ).Format(time.RFC3339),
	)

	s.loadLiveSnapshot()

	go s.loop(ctx)
	go s.staleChecker(ctx)
	go s.snapshotLoop(ctx)
	go s.logCleanup(ctx)
	go s.emailDigestLoop(ctx)
	go s.dailyReportLoop(ctx)
	go s.sampleWriterLoop(ctx)
	go s.eventWriterLoop(ctx)
	log.Println("[VSS] monitor started")
}

func (s *vssMonitorService) loop(ctx context.Context) {
	delay := 15 * time.Second

	for {
		select {
		case <-ctx.Done():
			log.Println("[VSS] monitor stopped")
			return
		default:
		}

		err := s.connectAndListen(ctx)
		if err != nil {
			log.Printf("[VSS] connection error: %v - retry in %s", err, delay)
			if s.reasonAllowedForLog("disconnect") {
				_ = s.saveSystemEvent("disconnect", fmt.Sprintf("WS error: %v", err))
			}

			if strings.Contains(err.Error(), "too frequently") {
				delay = 3 * time.Minute
			} else {
				delay *= 2
				if delay > 2*time.Minute {
					delay = 2 * time.Minute
				}
				if delay < 15*time.Second {
					delay = 15 * time.Second
				}
			}
		} else {
			delay = 15 * time.Second
		}

		if ctx.Err() != nil {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}
}

func (s *vssMonitorService) ListDelays(filter repository.VSSDelayFilter) (*domain.VSSDelayListResult, error) {
	return s.repo.List(filter)
}

func (s *vssMonitorService) ListHistory(filter repository.VSSHistoryFilter) ([]domain.VSSAlertHistory, int64, error) {
	return s.historyRepo.List(filter)
}

func (s *vssMonitorService) ListLive(deviceName, action, status string, onlyIssue bool, page, perPage int, sortBy, sortDir string) *VSSLiveResult {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 50
	}
	if perPage > 500 {
		perPage = 500
	}

	q := strings.ToLower(strings.TrimSpace(deviceName))
	act := strings.ToLower(strings.TrimSpace(action))
	st := strings.ToUpper(strings.TrimSpace(status))

	s.mu.Lock()
	now := time.Now()
	defer s.mu.Unlock()

	all := make([]VSSDeviceLive, 0, len(s.live))
	normal, issue := 0, 0

	for _, v := range s.live {
		if v == nil {
			continue
		}
		if len(v.Reasons) == 0 {
			normal++
		} else {
			issue++
		}

		if onlyIssue && len(v.Reasons) == 0 {
			continue
		}
		if q != "" {
			name := strings.ToLower(v.DeviceName)
			id := strings.ToLower(v.DeviceID)
			if !strings.Contains(name, q) && !strings.Contains(id, q) {
				continue
			}
		}
		if act != "" && strings.ToLower(v.Action) != act {
			continue
		}
		if st != "" && strings.ToUpper(v.Status) != st {
			continue
		}

		cp := *v

		var seenAge time.Duration
		if ls, ok := s.lastSeen[cp.DeviceID]; ok && !ls.IsZero() {
			cp.LastSeen = ls.UTC().Format(time.RFC3339)
			seenAge = now.Sub(ls)
		} else if cp.LastSeen != "" {
			if t, err := time.Parse(time.RFC3339, cp.LastSeen); err == nil {
				seenAge = now.Sub(t)
			}
		}

		const (
			onlineWithin  = 120 * time.Second
			offlineAfter  = 6 * time.Hour
		)

		switch {
		case seenAge >= offlineAfter:
			cp.Status = "OFFLINE"
		case seenAge >= onlineWithin:
			cp.Status = "STALE"
			if cp.Action == "" || cp.Action == "80003" || cp.Action == "80004" {
				cp.Action = "stale"
			}
		default:
			if len(cp.Reasons) == 0 {
				cp.Status = "NORMAL"
			}
		}

		all = append(all, cp)
	}

	asc := strings.EqualFold(strings.TrimSpace(sortDir), "asc")
	key := strings.ToLower(strings.TrimSpace(sortBy))

	sort.Slice(all, func(i, j int) bool {
		var less bool
		switch key {
		case "device_name", "name":
			less = strings.ToLower(all[i].DeviceName) < strings.ToLower(all[j].DeviceName)
		case "device_id":
			less = all[i].DeviceID < all[j].DeviceID
		case "last_seen", "last_report":
			less = all[i].LastSeen < all[j].LastSeen
		case "dtu", "device_time":
			less = all[i].DTU < all[j].DTU
		case "report_time", "sent_to_middleware":
			less = all[i].ReportTime < all[j].ReportTime
		case "action":
			less = all[i].Action < all[j].Action
		case "status":
			less = all[i].Status < all[j].Status
		case "delay_sec", "delay":
			fallthrough
		default:
			if all[i].DelaySec == all[j].DelaySec {
				less = all[i].DeviceName < all[j].DeviceName
			} else {
				less = all[i].DelaySec < all[j].DelaySec
			}
		}
		if asc {
			return less
		}
		return !less
	})

	total := len(all)
	start := (page - 1) * perPage
	if start > total {
		start = total
	}
	end := start + perPage
	if end > total {
		end = total
	}

	c03 := s.trafficByAction["80003"]
	c04 := s.trafficByAction["80004"]
	return &VSSLiveResult{
		Traffic: VSSTrafficStats{
			TotalMessages: s.trafficTotal,
			Total80003:    c03,
			Total80004:    c04,
			DeviceCount:   len(s.live),
			NormalCount:   normal,
			IssueCount:    issue,
		},
		Data:  all[start:end],
		Total: total,
	}
}

func (s *vssMonitorService) GetTraffic() *VSSTrafficDetail {
	s.mu.Lock()
	defer s.mu.Unlock()

	tracked := []string{"80003", "80004", "80005", "80007", "80030"}

	totals := copyInt64Map(s.trafficByAction)
	series := make([]MinuteTraffic, len(s.minuteHistory))
	copy(series, s.minuteHistory)

	cur := MinuteTraffic{
		Minute: "",
		Counts: map[string]int64{},
		Total:  0,
	}
	if !s.currentMinute.IsZero() {
		cur.Minute = s.currentMinute.Format("15:04")
		cur.Counts = copyInt64Map(s.currentCounts)
		cur.Total = sumInt64Map(s.currentCounts)
	}

	return &VSSTrafficDetail{
		Totals:         totals,
		TotalMessages:  s.trafficTotal,
		CurrentMinute:  cur,
		Series:         series,
		TrackedActions: tracked,
	}
}

func categorizeDelaySec(sec int64) string {
	s := sec
	if s < 0 {
		s = -s
	}
	if s == 0 {
		return "0s"
	}
	if s <= 5 {
		return "1s-5s"
	}
	if s <= 60 {
		return "5s-1m"
	}
	if s <= 300 {
		return "1m-5m"
	}
	return ">5m"
}

func (s *vssMonitorService) GetDaySummary(date string) (*domain.VSSDaySummary, error) {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	if loc == nil {
		loc = time.FixedZone("WIB", 7*3600)
	}

	if strings.TrimSpace(date) == "" {
		date = time.Now().In(loc).AddDate(0, 0, -1).Format("2006-01-02")
	}

	today := time.Now().In(loc).Format("2006-01-02")
	if date >= today {
		return nil, fmt.Errorf("the date must be before today (at the latest, yesterday)")
	}

	events, err := s.repo.ListAllByDate(date)
	if err != nil {
		return nil, err
	}

	dist := map[string]int{
		"0s":    0,
		"1s-5s": 0,
		"5s-1m": 0,
		"1m-5m": 0,
		">5m":   0,
	}

	type agg struct {
		name     string
		min      int64
		max      int64
		sum      int64
		n        int
		hasValid bool
	}
	byDevice := map[string]*agg{}

	for _, ev := range events {
		switch strings.ToLower(ev.Reason) {
		case "delayed", "stale_timestamp", "delay_sample":
		default:
			continue
		}
		abs := ev.DelaySec
		if abs < 0 {
			abs = -abs
		}
		if abs >= 3600 {
			continue
		}

		a, ok := byDevice[ev.DeviceID]
		if !ok {
			a = &agg{name: ev.DeviceName}
			byDevice[ev.DeviceID] = a
		}
		if ev.DeviceName != "" {
			a.name = ev.DeviceName
		}

		if !a.hasValid {
			a.min = ev.DelaySec
			a.max = ev.DelaySec
			a.hasValid = true
		} else {
			if ev.DelaySec < a.min {
				a.min = ev.DelaySec
			}
			if ev.DelaySec > a.max {
				a.max = ev.DelaySec
			}
		}
		a.sum += ev.DelaySec
		a.n++
	}

	const minAvgDelaySec int64 = 3

	type deviceStat struct {
		id     string
		name   string
		min    int64
		max    int64
		avg    float64
		avgSec int64
		n      int
	}

	stats := make([]deviceStat, 0, len(byDevice))
	for id, a := range byDevice {
		if !a.hasValid || a.n == 0 {
			continue
		}
		avg := float64(a.sum) / float64(a.n)
		avgSec := int64(math.Round(avg))
		name := a.name
		if name == "" {
			name = id
		}
		stats = append(stats, deviceStat{
			id: id, name: name,
			min: a.min, max: a.max,
			avg: avg, avgSec: avgSec, n: a.n,
		})
	}

	delayed := make([]deviceStat, 0, len(stats))
	over5Devices := 0
	for _, st := range stats {
		if st.avgSec <= minAvgDelaySec {
			continue
		}
		delayed = append(delayed, st)
		dist[categorizeDelaySec(st.avgSec)]++
		if st.avgSec > 5 {
			over5Devices++
		}
	}

	top := make([]domain.VSSDaySummaryDevice, 0, len(delayed))
	for _, st := range delayed {
		top = append(top, domain.VSSDaySummaryDevice{
			DeviceID:    st.id,
			DeviceName:  st.name,
			MaxDelaySec: st.avgSec,
			EventCount:  st.n,
		})
	}
	sort.Slice(top, func(i, j int) bool {
		if top[i].MaxDelaySec == top[j].MaxDelaySec {
			return top[i].DeviceName < top[j].DeviceName
		}
		return top[i].MaxDelaySec > top[j].MaxDelaySec
	})
	if len(top) > 10 {
		top = top[:10]
	}

	var globalMaxAvg int64
	maxName, maxID := "", ""
	if len(top) > 0 {
		globalMaxAvg = top[0].MaxDelaySec
		maxName = top[0].DeviceName
		maxID = top[0].DeviceID
	}

	return &domain.VSSDaySummary{
		Date: date,
		TotalDevices: len(stats),
		DelayOver5Count: over5Devices,
		TotalEvents:     len(events),
		MaxDelaySec:     globalMaxAvg,
		MaxDelayDevice:  maxName,
		MaxDelayDeviceID: maxID,
		Distribution: dist,
		TopDevices:   top,
	}, nil
}

func (s *vssMonitorService) GetDayDelayExport(date string) ([]domain.VSSDayDeviceDelayStat, error) {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	if loc == nil {
		loc = time.FixedZone("WIB", 7*3600)
	}

	if strings.TrimSpace(date) == "" {
		date = time.Now().In(loc).AddDate(0, 0, -1).Format("2006-01-02")
	}

	today := time.Now().In(loc).Format("2006-01-02")
	if date >= today {
		return nil, fmt.Errorf("the date must be before today (at the latest, yesterday)")
	}

	events, err := s.repo.ListAllByDate(date)
	if err != nil {
		return nil, err
	}

	type agg struct {
		name string
		max  int64
		min  int64
		sum  int64
		n    int
	}
	byDevice := map[string]*agg{}

	for _, ev := range events {
		a, ok := byDevice[ev.DeviceID]
		if !ok {
			a = &agg{
				name: ev.DeviceName,
				max:  ev.DelaySec,
				min:  ev.DelaySec,
			}
			byDevice[ev.DeviceID] = a
		}
		if ev.DeviceName != "" {
			a.name = ev.DeviceName
		}
		a.n++
		a.sum += ev.DelaySec
		if ev.DelaySec > a.max {
			a.max = ev.DelaySec
		}
		if ev.DelaySec < a.min {
			a.min = ev.DelaySec
		}
	}

	out := make([]domain.VSSDayDeviceDelayStat, 0, len(byDevice))
	for id, a := range byDevice {
		name := a.name
		if name == "" {
			name = id
		}
		avg := 0.0
		if a.n > 0 {
			avg = float64(a.sum) / float64(a.n)
		}
		out = append(out, domain.VSSDayDeviceDelayStat{
			DeviceID:    id,
			DeviceName:  name,
			MaxDelaySec: a.max,
			MinDelaySec: a.min,
			AvgDelaySec: avg,
			EventCount:  a.n,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].MaxDelaySec == out[j].MaxDelaySec {
			return out[i].DeviceName < out[j].DeviceName
		}
		return out[i].MaxDelaySec > out[j].MaxDelaySec
	})

	return out, nil
}

func (s *vssMonitorService) login() (token, pid string, err error) {
	body := fmt.Sprintf(
		`{"username": "%s", "password": "%s", "lang": "0", "client": "2"}`,
		s.cfg.Username, s.cfg.PasswordMD5,
	)

	req, err := http.NewRequest(http.MethodPost, s.cfg.LoginURL, strings.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}

	var result struct {
		Status  int    `json:"status"`
		Message string `json:"message"`
		Msg     string `json:"msg"`
		Data    struct {
			Token string `json:"token"`
			PID   string `json:"pid"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", "", fmt.Errorf("decode login response: %w body=%s", err, string(raw))
	}
	if result.Status != 10000 || result.Data.Token == "" {
		return "", "", fmt.Errorf("login failed status=%d message=%s body=%s",
			result.Status, result.Message+result.Msg, string(raw))
	}
	return result.Data.Token, result.Data.PID, nil
}

func (s *vssMonitorService) connectAndListen(ctx context.Context) error {
	token, pid, err := s.login()
	if err != nil {
		return fmt.Errorf("login: %w", err)
	}

	dialer := websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 30 * time.Second,
	}
	conn, _, err := dialer.DialContext(ctx, s.cfg.WSURL, nil)
	if err != nil {
		return fmt.Errorf("ws dial: %w", err)
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(120 * time.Second))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(120 * time.Second))
		return nil
	})

	loginMsg := map[string]interface{}{
		"action": "80000",
		"payload": map[string]string{
			"username": s.cfg.Username,
			"pid":      pid,
			"token":    token,
		},
	}
	if err := conn.WriteJSON(loginMsg); err != nil {
		return fmt.Errorf("ws login write: %w", err)
	}
	log.Printf("[VSS] websocket connected & logged in (pid=%s)", pid)

	pingErr := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteMessage(websocket.PingMessage, []byte("ping")); err != nil {
					select {
					case pingErr <- err:
					default:
					}
					return
				}
			}
		}
	}()

	var (
		msgCount   int64
		lastLog    = time.Now()
		lastDataAt = time.Now()
	)
	const dataSilence = 90 * time.Second

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-pingErr:
			return fmt.Errorf("ws ping: %w", err)
		default:
		}

		_ = conn.SetReadDeadline(time.Now().Add(120 * time.Second))
		_, data, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("ws read: %w", err)
		}

		var env struct {
			Action string `json:"action"`
		}
		if json.Unmarshal(data, &env) == nil {
			switch env.Action {
			case "80003", "80004":
				lastDataAt = time.Now()
			}
		}

		s.handleMessage(data)
		msgCount++

		if time.Since(lastDataAt) > dataSilence {
			return fmt.Errorf("ws data silence: no 80003/80004 for %s", dataSilence)
		}

		if time.Since(lastLog) >= 30*time.Second {
			s.mu.Lock()
			nLive := len(s.live)
			total := s.trafficTotal
			s.mu.Unlock()
			log.Printf("[VSS] stream alive msgs_session=%d traffic_total=%d live_devices=%d data_age=%s",
				msgCount, total, nLive, time.Since(lastDataAt).Round(time.Second))
			lastLog = time.Now()
		}
	}
}

func reportTimeToTime(ms int64) (time.Time, bool) {
	if ms <= 0 {
		return time.Time{}, false
	}
	if ms < 1_000_000_000_000 {
		return time.Unix(ms, 0), true
	}
	return time.UnixMilli(ms), true
}

func parseDTU(dtu string) (time.Time, bool) {
	dtu = strings.TrimSpace(dtu)
	if dtu == "" {
		return time.Time{}, false
	}
	loc, _ := time.LoadLocation("Asia/Jakarta")
	if loc == nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	t, err := time.ParseInLocation("2006-01-02 15:04:05", dtu, loc)
	if err != nil {
		log.Printf("[VSS-DTU-PARSE-FAIL] raw=%q err=%v", dtu, err)
		return time.Time{}, false
	}
	return t, true
}

func (s *vssMonitorService) deviceEventTime(reportMs int64, dtu string) (time.Time, bool) {
	if t, ok := parseDTU(dtu); ok {
		return t, true
	}
	if t, ok := reportTimeToTime(reportMs); ok {
		return t, true
	}
	return time.Time{}, false
}

func (s *vssMonitorService) liveDelayNow(v *VSSDeviceLive, now time.Time) int64 {
	if t, ok := parseDTU(v.DTU); ok {
		d := now.Sub(t).Seconds()
		if d < 0 {
			return 0
		}
		return int64(d)
	}
	if t, ok := reportTimeToTime(v.ReportTime); ok {
		d := now.Sub(t).Seconds()
		if d < 0 {
			return 0
		}
		return int64(d)
	}
	return v.DelaySec
}

func (s *vssMonitorService) handleMessage(data []byte) {
	var env wsEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return
	}

	s.recordTraffic(env.Action)

	switch env.Action {
	case "80003":
		if s.ingest80003(env.Payload) {
			return
		}
		s.handleStatus80003(env.Payload)
	case "80004":
		if s.tryUpsertFromAny(env.Action, env.Payload) {
			return
		}
		s.handleAlarm80004(env.Payload)
	default:
		_ = s.tryUpsertFromAny(env.Action, env.Payload)
	}
}

func (s *vssMonitorService) ingest80003(raw json.RawMessage) bool {
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		return false
	}

	deviceID := anyToString(m["deviceID"])
	if deviceID == "" {
		deviceID = anyToString(m["deviceId"])
	}
	if deviceID == "" {
		return false
	}

	dtu := anyToString(m["dtu"])
	if dtu == "" {
		if loc, ok := m["location"].(map[string]interface{}); ok {
			dtu = anyToString(loc["dtu"])
		}
	}
	if dtu == "" {
		if pl, ok := m["payload"].(map[string]interface{}); ok {
			dtu = anyToString(pl["dtu"])
		}
	}

	deviceName := ""
	var reportMs int64
	if ext, ok := m["ext"].(map[string]interface{}); ok {
		deviceName = anyToString(ext["deviceName"])
		reportMs = anyToInt64(ext["reportTime"])
	}
	if deviceName == "" {
		deviceName = anyToString(m["deviceName"])
	}
	nodeID := anyToString(m["nodeID"])
	if nodeID == "" {
		nodeID = anyToString(m["nodeId"])
	}

	now := time.Now()

	delaySec := int64(0)
	if dtu != "" {
		if t, ok := parseDTU(dtu); ok {
			delaySec = int64(now.Sub(t).Seconds())
		}
	} else if reportMs > 0 {
		if t, ok := reportTimeToTime(reportMs); ok {
			delaySec = int64(now.Sub(t).Seconds())
		}
	}

	var reasons []string
	if arr, ok := m["storage"].([]interface{}); ok {
		for _, it := range arr {
			if sm, ok := it.(map[string]interface{}); ok {
				if anyToString(sm["free"]) == "0" {
					reasons = append(reasons, "storage_full")
					break
				}
			}
		}
	}
	if delaySec >= int64(s.cfg.DelayThresholdSec) {
		reasons = append(reasons, "stale_timestamp")
	}

	s.mu.Lock()
	s.lastSeen[deviceID] = now
	if deviceName != "" {
		s.deviceName[deviceID] = deviceName
	}
	if dtu != "" {
		s.lastDTU[deviceID] = dtu
	}
	if reportMs > 0 {
		s.lastReportMs[deviceID] = reportMs
	}
	s.mu.Unlock()

	p := statusPayload{
		DeviceID: deviceID,
		NodeID:   nodeID,
		DTU:      dtu,
	}
	p.Ext.DeviceName = deviceName
	p.Ext.ReportTime = reportMs

	s.upsertLive(p, "80003", delaySec, false, reasons, now, time.Time{})

	s.persistDelaySample(p, delaySec, now, "80003")

	if s.trafficTotal%200 == 0 || delaySec < -5 || delaySec > 120 {
		log.Printf(
			"[VSS-INGEST] 80003 device=%s name=%q dtu=%q delay=%ds lastSeen=now",
			deviceID, deviceName, dtu, delaySec,
		)
	}
	return true
}

func anyToString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%v", t)
	case json.Number:
		return t.String()
	case int:
		return fmt.Sprintf("%d", t)
	case int64:
		return fmt.Sprintf("%d", t)
	default:
		if v == nil {
			return ""
		}
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
}

func anyToInt64(v interface{}) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case json.Number:
		n, _ := t.Int64()
		return n
	case int64:
		return t
	case int:
		return int64(t)
	case string:
		var n int64
		fmt.Sscanf(strings.TrimSpace(t), "%d", &n)
		return n
	default:
		return 0
	}
}

func (s *vssMonitorService) tryUpsertFromAny(action string, raw json.RawMessage) bool {
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return false
	}

	deviceID := anyToString(m["deviceID"])
	if deviceID == "" {
		deviceID = anyToString(m["deviceId"])
	}
	if deviceID == "" {
		return false
	}

	dtu := anyToString(m["dtu"])
	if dtu == "" {
		if loc, ok := m["location"].(map[string]interface{}); ok {
			dtu = anyToString(loc["dtu"])
		}
	}
	if dtu == "" {
		if pl, ok := m["payload"].(map[string]interface{}); ok {
			dtu = anyToString(pl["dtu"])
		}
	}

	deviceName := ""
	var reportMs int64
	if ext, ok := m["ext"].(map[string]interface{}); ok {
		deviceName = anyToString(ext["deviceName"])
		reportMs = anyToInt64(ext["reportTime"])
	}
	if deviceName == "" {
		deviceName = anyToString(m["deviceName"])
	}

	now := time.Now()

	if dtu == "" && reportMs <= 0 {
		s.mu.Lock()
		s.lastSeen[deviceID] = now
		if deviceName != "" {
			s.deviceName[deviceID] = deviceName
		}
		if prev, ok := s.live[deviceID]; ok && prev != nil {
			prev.LastSeen = now.UTC().Format(time.RFC3339)
			prev.Action = action
			prev.Reasons = nil
			if prev.Status == "STALE" || prev.Status == "OFFLINE" {
				prev.Status = "NORMAL"
			}
			s.liveDirty = true
		}
		s.mu.Unlock()
		return true
	}

	p := statusPayload{
		DeviceID: deviceID,
		DTU:      dtu,
	}
	p.Ext.DeviceName = deviceName
	p.Ext.ReportTime = reportMs
	if node, ok := m["nodeID"].(string); ok {
		p.NodeID = node
	}

	var reasons []string
	if arr, ok := m["storage"].([]interface{}); ok {
		for _, it := range arr {
			if sm, ok := it.(map[string]interface{}); ok {
				if free, _ := sm["free"].(string); free == "0" {
					reasons = append(reasons, "storage_full")
					break
				}
			}
		}
	}
	if action == "80004" {
		reasons = append(reasons, "alarm")
	}

	delaySec := s.computeDelaySec(p, now)

	if s.trafficTotal%200 == 0 || delaySec < -5 || delaySec > 60 {
		log.Printf(
			"[VSS-SKEW] action=%s device=%s name=%q dtu=%q reportMs=%d now=%s delay=%ds lastSeen=now",
			action,
			deviceID,
			deviceName,
			dtu,
			reportMs,
			now.In(jakartaTZ).Format("2006-01-02 15:04:05.000"),
			delaySec,
		)
	}

	if delaySec >= int64(s.cfg.DelayThresholdSec) {
		reasons = append(reasons, "stale_timestamp")
	}

	isLater := false
	if v, ok := m["isLater"]; ok {
		switch t := v.(type) {
		case string:
			isLater = t == "1" || strings.EqualFold(t, "true")
		case float64:
			isLater = t != 0
		case bool:
			isLater = t
		}
	}
	if !isLater {
		if ext, ok := m["ext"].(map[string]interface{}); ok {
			if b, ok := ext["isLater"].(bool); ok {
				isLater = b
			} else if s, ok := ext["isLater"].(string); ok {
				isLater = s == "1" || strings.EqualFold(s, "true")
			}
		}
	}
	if isLater {
		reasons = append(reasons, "delayed")
	}

	s.mu.Lock()
	s.lastSeen[deviceID] = now
	if deviceName != "" {
		s.deviceName[deviceID] = deviceName
	}
	if dtu != "" {
		s.lastDTU[deviceID] = dtu
	}
	if reportMs > 0 {
		s.lastReportMs[deviceID] = reportMs
	}
	s.mu.Unlock()

	s.upsertLive(p, action, delaySec, isLater, reasons, now, time.Time{})
	s.persistDelaySample(p, delaySec, now, action)
	s.recordPacketDelayEvidence(p, delaySec, now, action, isLater)
	return true
}

func copyInt64Map(m map[string]int64) map[string]int64 {
	out := make(map[string]int64, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func sumInt64Map(m map[string]int64) int64 {
	var t int64
	for _, v := range m {
		t += v
	}
	return t
}

func (s *vssMonitorService) recordTraffic(action string) {
	action = strings.TrimSpace(action)
	if action == "" {
		action = "unknown"
	}

	now := time.Now()
	if jakartaTZ != nil {
		now = now.In(jakartaTZ)
	}
	minuteKey := now.Truncate(time.Minute)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.trafficTotal++
	if s.trafficByAction == nil {
		s.trafficByAction = make(map[string]int64)
	}
	s.trafficByAction[action]++

	if s.currentMinute.IsZero() {
		s.currentMinute = minuteKey
		s.currentCounts = make(map[string]int64)
	} else if !s.currentMinute.Equal(minuteKey) {
		s.minuteHistory = append(s.minuteHistory, MinuteTraffic{
			Minute: s.currentMinute.Format("15:04"),
			Counts: copyInt64Map(s.currentCounts),
			Total:  sumInt64Map(s.currentCounts),
		})
		const maxMinuteHistory = 24 * 60
		if len(s.minuteHistory) > maxMinuteHistory {
			s.minuteHistory = s.minuteHistory[len(s.minuteHistory)-maxMinuteHistory:]
		}
		s.currentMinute = minuteKey
		s.currentCounts = make(map[string]int64)
	}

	if s.currentCounts == nil {
		s.currentCounts = make(map[string]int64)
	}
	s.currentCounts[action]++
}

func historyDelaySec(dtuStr string, detectedAt time.Time) (int64, bool) {
	t, ok := parseDTU(dtuStr)
	if !ok {
		return 0, false
	}
	return int64(t.Sub(detectedAt).Seconds()), true
}

func (s *vssMonitorService) computeDelaySec(p statusPayload, now time.Time) int64 {
	var dtuDelay int64
	var hasDTU bool
	if t, ok := parseDTU(p.DTU); ok {
		dtuDelay = int64(now.Sub(t).Seconds())
		hasDTU = true
	}

	var reportDelay int64
	var hasReport bool
	if t, ok := reportTimeToTime(p.Ext.ReportTime); ok {
		reportDelay = int64(now.Sub(t).Seconds())
		hasReport = true
	}

	const staleSkewSec int64 = 3600
	if hasDTU && hasReport {
		absDTU := dtuDelay
		if absDTU < 0 {
			absDTU = -absDTU
		}
		absRep := reportDelay
		if absRep < 0 {
			absRep = -absRep
		}
		if absDTU > staleSkewSec && absRep < absDTU {
			return reportDelay
		}
		return dtuDelay
	}
	if hasDTU {
		return dtuDelay
	}
	if hasReport {
		return reportDelay
	}
	return 0
}

func (s *vssMonitorService) persistIfNotSpam(
	p statusPayload,
	reason string,
	delaySec int64,
	isLater bool,
	now time.Time,
	action string,
	alarmID string,
	alarmDetail string,
	eventType string,
) {
	reason = strings.ToLower(strings.TrimSpace(reason))
	if !s.reasonAllowedForLog(reason) {
		return
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")
	if loc == nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	detectedAt := now.In(loc)
	detected := detectedAt.Format(time.RFC3339)

	storedDelay := delaySec
	metric := "delay_packet_sec"
	switch reason {
	case "no_heartbeat":
		metric = "silent_for_sec"
	case "delayed", "stale_timestamp", "delay_sample":
		if d, ok := historyDelaySec(p.DTU, detectedAt); ok {
			storedDelay = d
		}
		metric = "dtu_minus_detected_sec"
	}

	msg := fmt.Sprintf(
		"VSS | action=%s | device=%s (%s) | reason=%s | %s=%d | dtu=%s | detected=%s | alarmId=%s | detail=%s",
		action, p.DeviceID, p.Ext.DeviceName, reason, metric, storedDelay, p.DTU, detected, alarmID, alarmDetail,
	)

	ev := domain.VSSDelayEvent{
		DetectedAt:  detected,
		DeviceID:    p.DeviceID,
		DeviceName:  p.Ext.DeviceName,
		NodeID:      p.NodeID,
		DTU:         p.DTU,
		ReportTime:  p.Ext.ReportTime,
		IsLater:     isLater,
		DelaySec:    storedDelay,
		Reason:      reason,
		Message:     msg,
		Action:      action,
		AlarmID:     alarmID,
		AlarmDetail: alarmDetail,
		EventType:   eventType,
	}

	if s.eventCh == nil {
		return
	}
	select {
	case s.eventCh <- ev:
	default:
	}
}

func (s *vssMonitorService) recordPacketDelayEvidence(p statusPayload, delaySec int64, now time.Time, action string, isLater bool) {
	th := int64(s.cfg.DelayThresholdSec)
	if th <= 0 {
		th = 60
	}

	histDelay := delaySec
	if d, ok := historyDelaySec(p.DTU, now); ok {
		histDelay = d
	}

	abs := histDelay
	if abs < 0 {
		abs = -abs
	}
	if abs >= 3600 {
		return
	}
	if abs < th && !isLater {
		return
	}

	reason := "delayed"
	if abs >= th {
		reason = "stale_timestamp"
	}
	s.persistIfNotSpam(p, reason, histDelay, isLater, now, action, "", "", "")
}

const (
	sampleLogWindow  = 30 * time.Second
	sampleMemoryKeep = 24 * time.Hour
	sampleMemoryMaxN = 2000
)

func (s *vssMonitorService) pushRecentSample(deviceID string, pt delaySamplePoint) {
	if deviceID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.recentSamples == nil {
		s.recentSamples = make(map[string][]delaySamplePoint)
	}
	arr := append(s.recentSamples[deviceID], pt)
	cut := time.Now().Add(-sampleMemoryKeep)
	i := 0
	for i < len(arr) && arr[i].ReceivedAt.Before(cut) {
		i++
	}
	arr = arr[i:]
	if len(arr) > sampleMemoryMaxN {
		arr = arr[len(arr)-sampleMemoryMaxN:]
	}
	s.recentSamples[deviceID] = arr
}

func (s *vssMonitorService) persistDelaySample(
	p statusPayload,
	delaySec int64,
	now time.Time,
	action string,
) {
	const sampleReason = "delay_sample"

	abs := delaySec
	if abs < 0 {
		abs = -abs
	}
	if abs >= 3600 {
		return
	}

	sampleDelay := delaySec
	if d, ok := historyDelaySec(p.DTU, now); ok {
		sampleDelay = d
	}
	absSample := sampleDelay
	if absSample < 0 {
		absSample = -absSample
	}
	if absSample >= 3600 {
		return
	}

	s.pushRecentSample(p.DeviceID, delaySamplePoint{
		ReceivedAt: now,
		DTU:        p.DTU,
		DelaySec:   sampleDelay,
		Action:     action,
		Reason:     sampleReason,
	})

	loc, _ := time.LoadLocation("Asia/Jakarta")
	if loc == nil {
		loc = time.FixedZone("WIB", 7*3600)
	}

	ev := domain.VSSDelayEvent{
		DetectedAt: now.In(loc).Format(time.RFC3339),
		DeviceID:   p.DeviceID,
		DeviceName: p.Ext.DeviceName,
		NodeID:     p.NodeID,
		DTU:        p.DTU,
		ReportTime: p.Ext.ReportTime,
		IsLater:    p.IsLater == "1" || p.Ext.IsLater,
		DelaySec:   sampleDelay,
		Reason:     sampleReason,
		Message: fmt.Sprintf(
			"VSS | sample | action=%s | device=%s (%s) | dtu_minus_detected=%ds | dtu=%s",
			action, p.DeviceID, p.Ext.DeviceName, sampleDelay, p.DTU,
		),
		Action: action,
	}

	if s.sampleCh == nil {
		return
	}
	select {
	case s.sampleCh <- ev:
	default:
	}
}

func (s *vssMonitorService) sampleWriterLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	buf := make([]domain.VSSDelayEvent, 0, 512)

	flush := func() {
		for _, ev := range buf {
			exists, err := s.repo.ExistsRecent(ev.DeviceID, ev.Reason, sampleLogWindow)
			if err != nil || exists {
				continue
			}
			if err := s.repo.Append(ev); err != nil {
				log.Printf("[VSS] write delay sample error: %v", err)
			}
		}
		buf = buf[:0]
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return
		case ev := <-s.sampleCh:
			buf = append(buf, ev)
			if len(buf) >= 500 {
				flush()
			}
		case <-ticker.C:
			if len(buf) > 0 {
				flush()
			}
		}
	}
}

func (s *vssMonitorService) eventWriterLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	buf := make([]domain.VSSDelayEvent, 0, 256)
	const dedupeWindow = 15 * time.Minute

	flush := func() {
		for _, ev := range buf {
			exists, err := s.repo.ExistsRecent(ev.DeviceID, ev.Reason, dedupeWindow)
			if err != nil || exists {
				continue
			}
			if err := s.repo.Append(ev); err != nil {
				log.Printf("[VSS] write log file error: %v", err)
				continue
			}
			if s.reasonAllowedForLog(ev.Reason) {
				s.saveHistory(ev)
			}
			if s.reasonAllowedForEmail(ev.Reason) {
				s.enqueueAlert(ev)
			}
		}
		buf = buf[:0]
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return
		case ev := <-s.eventCh:
			buf = append(buf, ev)
			if len(buf) >= 200 {
				flush()
			}
		case <-ticker.C:
			if len(buf) > 0 {
				flush()
			}
		}
	}
}

func (s *vssMonitorService) saveSystemEvent(reason, message string) error {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	if loc == nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	reason = strings.ToLower(strings.TrimSpace(reason))
	if !s.reasonAllowedForLog(reason){
		return nil
	}

	ev := domain.VSSDelayEvent{
		DetectedAt: time.Now().In(loc).Format(time.RFC3339),
		DeviceID:   "SYSTEM",
		DeviceName: "VSS-Monitor",
		Reason:     reason,
		Message:    message,
	}
	return s.repo.Append(ev)
}

func (s *vssMonitorService) staleChecker(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	threshold := time.Duration(s.cfg.StaleThresholdSec) * time.Second
	if threshold <= 0 {
		threshold = 5 * time.Minute
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()

			s.mu.Lock()
			ids := make([]string, 0, len(s.lastSeen))
			for id := range s.lastSeen {
				ids = append(ids, id)
			}
			s.mu.Unlock()

			for _, deviceID := range ids {
				s.mu.Lock()
				last, ok := s.lastSeen[deviceID]
				if !ok {
					s.mu.Unlock()
					continue
				}
				silentFor := now.Sub(last)
				if silentFor < threshold {
					s.mu.Unlock()
					continue
				}

				prev := s.live[deviceID]
				var lastPacketDelay int64
				var dtu string
				var reportMs int64
				name := deviceID
				if prev != nil {
					lastPacketDelay = prev.DelaySec
					dtu = prev.DTU
					reportMs = prev.ReportTime
					if prev.DeviceName != "" {
						name = prev.DeviceName
					}
				}
				if n, ok := s.deviceName[deviceID]; ok && n != "" {
					name = n
				}
				if dtu == "" {
					dtu = s.lastDTU[deviceID]
				}
				if reportMs == 0 {
					reportMs = s.lastReportMs[deviceID]
				}
				lastCopy := last
				s.mu.Unlock()

				fake := statusPayload{DeviceID: deviceID, DTU: dtu}
				fake.Ext.DeviceName = name
				fake.Ext.ReportTime = reportMs
				s.upsertLive(fake, "stale", lastPacketDelay, false, []string{"no_heartbeat"}, now, lastCopy)

				if !s.reasonAllowedForLog("no_heartbeat") {
					continue
				}

				silentSec := int64(silentFor.Seconds())
				s.persistIfNotSpam(
					fake,
					"no_heartbeat",
					silentSec,
					false,
					now,
					"stale",
					"",
					"",
					"",
				)
			}
		}
	}
}

func (s *vssMonitorService) handleStatus80003(raw json.RawMessage) {
	var p statusPayload
	if err := json.Unmarshal(raw, &p); err != nil || p.DeviceID == "" {
		return
	}

	if p.DTU == "" && p.Location.DTU != "" {
		p.DTU = p.Location.DTU
	}

	now := time.Now()

	s.mu.Lock()
	if prev, ok := s.live[p.DeviceID]; ok && prev != nil && p.DTU != "" {
		newT, okN := parseDTU(p.DTU)
		oldT, okO := parseDTU(prev.DTU)
		if okN && okO && newT.Before(oldT) {
			// tetap pakai DTU baru dari payload agar "apa adanya"; hapus blok ini jika ingin force monotonic
			// p.DTU = prev.DTU
			// if prev.ReportTime > 0 { p.Ext.ReportTime = prev.ReportTime }
		}
	}

	s.lastSeen[p.DeviceID] = now
	if p.Ext.DeviceName != "" {
		s.deviceName[p.DeviceID] = p.Ext.DeviceName
	}
	if p.DTU != "" {
		s.lastDTU[p.DeviceID] = p.DTU
	}
	if p.Ext.ReportTime > 0 {
		s.lastReportMs[p.DeviceID] = p.Ext.ReportTime
	}
	s.mu.Unlock()

	isLater := p.IsLater == "1" || p.Ext.IsLater
	delaySec := s.computeDelaySec(p, now)

	if s.traffic80003%200 == 0 || delaySec < -5 || delaySec > 60 {
		log.Printf(
			"[VSS-SKEW] 80003 device=%s name=%q dtu=%q reportMs=%d now=%s delay=%ds isLater=%v",
			p.DeviceID,
			p.Ext.DeviceName,
			p.DTU,
			p.Ext.ReportTime,
			now.In(jakartaTZ).Format("2006-01-02 15:04:05.000"),
			delaySec,
			isLater,
		)
	}

	var candidates []string
	if isLater {
		candidates = append(candidates, "delayed")
	}
	if delaySec >= int64(s.cfg.DelayThresholdSec) {
		candidates = append(candidates, "stale_timestamp")
	}
	for _, st := range p.Storage {
		if st.Free == "0" {
			candidates = append(candidates, "storage_full")
			break
		}
	}
	if s.reasonAllowedForLog("high_cpu") {
		var cpu float64
		fmt.Sscanf(p.DevTemp.CPU, "%f", &cpu)
		if cpu >= 95 {
			candidates = append(candidates, "high_cpu")
		}
	}

	seen := map[string]bool{}
	var reasons []string
	for _, r := range candidates {
		r = strings.ToLower(r)
		if seen[r] {
			continue
		}
		seen[r] = true
		reasons = append(reasons, r)
	}

	s.upsertLive(p, "80003", delaySec, isLater, reasons, now, time.Time{})

	s.persistDelaySample(p, delaySec, now, "80003")

	for _, r := range reasons {
		if !s.reasonAllowedForLog(r) {
			continue
		}
		s.persistIfNotSpam(p, r, delaySec, isLater, now, "80003", "", "", "")
	}
}

func (s *vssMonitorService) handleAlarm80004(raw json.RawMessage) {
	var p alarmPayload80004
	if err := json.Unmarshal(raw, &p); err != nil || p.DeviceID == "" {
		return
	}

	now := time.Now()
	s.mu.Lock()
	s.lastSeen[p.DeviceID] = now
	if p.DeviceName != "" {
		s.deviceName[p.DeviceID] = p.DeviceName
	}
	if p.DTU != "" {
		s.lastDTU[p.DeviceID] = p.DTU
	} else if p.Payload.DTU != "" {
		s.lastDTU[p.DeviceID] = p.Payload.DTU
	}
	s.mu.Unlock()

	sp := statusPayload{
		DeviceID: p.DeviceID,
		NodeID:   p.NodeID,
		DTU:      p.DTU,
	}
	if sp.DTU == "" {
		sp.DTU = p.Payload.DTU
	}
	sp.Ext.DeviceName = p.DeviceName
	sp.IsLater = "0"
	if p.IsLater != 0 {
		sp.IsLater = "1"
		sp.Ext.IsLater = true
	}

	delaySec := s.computeDelaySec(sp, now)
	isLater := p.IsLater != 0

	reason := "alarm"

	s.upsertLive(sp, "80004", delaySec, isLater, []string{reason}, now, time.Time{})

	if s.reasonAllowedForLog(reason) || s.reasonAllowedForLog("alarm") {
		if !s.reasonAllowedForLog(reason) {
			reason = "alarm"
		}
		s.persistIfNotSpam(sp, reason, delaySec, isLater, now, "80004", p.AlarmID, p.AlarmDetail, p.EventType)
	}
}

func parseRangeDuration(rangeKey string) time.Duration {
	switch strings.ToLower(strings.TrimSpace(rangeKey)) {
	case "15m":
		return 15 * time.Minute
	case "3h":
		return 3 * time.Hour
	case "6h":
		return 6 * time.Hour
	case "12h":
		return 12 * time.Hour
	case "24h":
		return 24 * time.Hour
	default:
		return time.Hour
	}
}

func categorizeDeviceDelay(sec int64) string {
	s := sec
	if s < 0 {
		s = -s
	}
	if s <= 5 {
		return "0-5s"
	}
	if s <= 60 {
		return "5s-1m"
	}
	return ">1m"
}

func (s *vssMonitorService) samplesForDevice(deviceID string, since time.Time) []delaySamplePoint {
	s.mu.Lock()
	arr := s.recentSamples[deviceID]
	out := make([]delaySamplePoint, 0, len(arr))
	for _, p := range arr {
		if !p.ReceivedAt.Before(since) {
			out = append(out, p)
		}
	}
	s.mu.Unlock()
	return out
}

func (s *vssMonitorService) GetDeviceDelayHistory(deviceID, rangeKey string) ([]domain.VSSDeviceDelayPoint, error) {
	since := time.Now().Add(-parseRangeDuration(rangeKey))
	mem := s.samplesForDevice(deviceID, since)

	if len(mem) < 5 {
		if fileEv, err := s.repo.ListByDeviceSince(deviceID, since); err == nil {
			seen := map[string]struct{}{}
			for _, p := range mem {
				seen[p.ReceivedAt.Format(time.RFC3339Nano)] = struct{}{}
			}
			for _, ev := range fileEv {
				t, err := time.Parse(time.RFC3339, ev.DetectedAt)
				if err != nil {
					continue
				}
				key := t.Format(time.RFC3339Nano)
				if _, ok := seen[key]; ok {
					continue
				}
				mem = append(mem, delaySamplePoint{
					ReceivedAt: t,
					DTU:        ev.DTU,
					DelaySec:   ev.DelaySec,
					Action:     ev.Action,
					Reason:     ev.Reason,
				})
			}
			sort.Slice(mem, func (i, j int) bool {
				return mem[i].ReceivedAt.Before(mem[j].ReceivedAt)
			})
		}
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")
	if loc == nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	out := make([]domain.VSSDeviceDelayPoint, 0, len(mem))
	for _, p := range mem {
		out = append(out, domain.VSSDeviceDelayPoint{
			ReceivedAt: p.ReceivedAt.In(loc).Format(time.RFC3339),
			DTU:        p.DTU,
			DelaySec:   p.DelaySec,
			Action:     p.Action,
			Reason:     p.Reason,
		})
	}
	return out, nil
}

func (s *vssMonitorService) GetDeviceDelayStats(deviceID, rangeKey string) (*domain.VSSDeviceDelayStats, error) {
	points, err := s.GetDeviceDelayHistory(deviceID, rangeKey)
	if err != nil {
		return nil, err
	}

	name := deviceID
	s.mu.Lock()
	if n, ok := s.deviceName[deviceID]; ok && n != "" {
		name = n
	} else if live, ok := s.live[deviceID]; ok && live != nil && live.DeviceName != "" {
		name = live.DeviceName
	}
	s.mu.Unlock()

	cat := map[string]int{"0-5s": 0, "5s-1m": 0, ">1m": 0}
	var sumAbs int64
	var maxAbs int64
	var last domain.VSSDeviceDelayPoint

	for i, p := range points {
		abs := p.DelaySec
		if abs < 0 {
			abs = -abs
		}
		cat[categorizeDeviceDelay(p.DelaySec)]++
		sumAbs += abs
		if abs > maxAbs {
			maxAbs = abs
		}
		last = p
		_ = i
	}
	st := &domain.VSSDeviceDelayStats{
		DeviceID:   deviceID,
		DeviceName: name,
		Range:      rangeKey,
		TotalCount: len(points),
		Categories: []domain.VSSDeviceDelayCategory{
			{Category: "0-5s", Count: cat["0-5s"]},
			{Category: "5s-1m", Count: cat["5s-1m"]},
			{Category: ">1m", Count: cat[">1m"]},
		},
	}
	if len(points) > 0 {
		st.MaxDelaySec = maxAbs
		st.AvgDelaySec = float64(sumAbs) / float64(len(points))
		st.LastDelaySec = last.DelaySec
		st.LastReceivedAt = last.ReceivedAt
	}
	return st, nil
}

func formatDelay(sec int64) string {
	if sec < 60 {
		return fmt.Sprintf("%ds", sec)
	}
	m := sec / 60
	s := sec % 60
	if m < 60 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	h := m / 60
	m = m % 60
	return fmt.Sprintf("%dh %dm", h, m)
}

func (s *vssMonitorService) saveHistory(ev domain.VSSDelayEvent) {
	if s.historyRepo == nil {
		return
	}

	cooldown := 15 * time.Minute
	switch strings.ToLower(ev.Reason) {
	case "delayed", "stale_timestamp":
		cooldown = 10 * time.Minute
	case "no_heartbeat":
		cooldown = 2 * time.Hour
	case "disconnect":
		cooldown = 30 * time.Minute
	default:
		if s.cfg.EmailCooldownMin > 0 {
			cooldown = time.Duration(s.cfg.EmailCooldownMin) * time.Minute
		}
	}

	exists, err := s.historyRepo.ExistsRecent(ev.DeviceID, ev.Reason, cooldown)
	if err != nil || exists {
		return
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")
	if loc == nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	detected, err := time.Parse(time.RFC3339, ev.DetectedAt)
	if err != nil {
		detected = time.Now().In(loc)
	} else {
		detected = detected.In(loc)
	}

	h := &domain.VSSAlertHistory{
		DetectedAt:  detected,
		DeviceID:    ev.DeviceID,
		DeviceName:  ev.DeviceName,
		NodeID:      ev.NodeID,
		DTU:         ev.DTU,
		ReportTime:  ev.ReportTime,
		IsLater:     ev.IsLater,
		DelaySec:    ev.DelaySec,
		Reason:      ev.Reason,
		Message:     ev.Message,
		EmailSent:   s.cfg.EmailEnabled,
		YearMonth:   detected.Format("2006-01"),
		Action:      ev.Action,
		AlarmID:     ev.AlarmID,
		AlarmDetail: ev.AlarmDetail,
		EventType:   ev.EventType,
	}
	if err := s.historyRepo.Create(h); err != nil {
		log.Printf("[VSS] save history error: %v", err)
		return
	}
	log.Printf("[VSS] history saved device=%s reason=%s delaySec=%d month=%s",
		h.DeviceID, h.Reason, h.DelaySec, h.YearMonth)
}

func (s *vssMonitorService) logCleanup(ctx context.Context) {
	s.purgeOldLogs()
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.purgeOldLogs()
		}
	}
}

func (s *vssMonitorService) loadLiveSnapshot() {
	if s.snapshotPath == "" {
		return
	}
	b, err := os.ReadFile(s.snapshotPath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("[VSS] load snapshot: %v", err)
		}
		return
	}
	var snap liveSnapshotFile
	if err := json.Unmarshal(b, &snap); err != nil {
		log.Printf("[VSS] snapshot corrupt: %v", err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, d := range snap.Devices {
		cp := d
		if cp.DeviceID == "" {
			continue
		}
		s.live[cp.DeviceID] = &cp
		if cp.DeviceName != "" {
			s.deviceName[cp.DeviceID] = cp.DeviceName
		}
		if cp.DTU != "" {
			s.lastDTU[cp.DeviceID] = cp.DTU
		}
		if cp.ReportTime > 0 {
			s.lastReportMs[cp.DeviceID] = cp.ReportTime
		}
	}
	for id, ts := range snap.LastSeen {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			s.lastSeen[id] = t
		}
	}
	for id, dtu := range snap.LastDTU {
		if dtu != "" {
			s.lastDTU[id] = dtu
		}
	}
	for id, ms := range snap.LastReportMs {
		if ms > 0 {
			s.lastReportMs[id] = ms
		}
	}
	for id, name := range snap.DeviceName {
		if name != "" {
			s.deviceName[id] = name
		}
	}
	log.Printf("[VSS] live snapshot loaded devices=%d saved_at=%s", len(s.live), snap.SavedAt)
}

func (s *vssMonitorService) saveLiveSnapshot() {
	if s.snapshotPath == "" {
		return
	}

	s.mu.Lock()
	if !s.liveDirty && len(s.live) == 0 {
		s.mu.Unlock()
		return
	}
	s.liveDirty = false

	devices := make([]VSSDeviceLive, 0, len(s.live))
	for _, v := range s.live {
		if v == nil {
			continue
		}
		devices = append(devices, *v)
	}
	lastSeen := make(map[string]string, len(s.lastSeen))
	for id, t := range s.lastSeen {
		lastSeen[id] = t.UTC().Format(time.RFC3339)
	}
	lastDTU := copyStringMap(s.lastDTU)
	lastReport := copyInt64Map(s.lastReportMs)
	names := copyStringMap(s.deviceName)
	s.mu.Unlock()

	snap := liveSnapshotFile{
		SavedAt:      time.Now().In(jakartaTZ).Format(time.RFC3339),
		Devices:      devices,
		LastSeen:     lastSeen,
		LastDTU:      lastDTU,
		LastReportMs: lastReport,
		DeviceName:   names,
	}
	b, err := json.Marshal(snap)
	if err != nil {
		log.Printf("[VSS] snapshot marshal: %v", err)
		return
	}
	tmp := s.snapshotPath + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		log.Printf("[VSS] snapshot write: %v", err)
		return
	}
	if err := os.Rename(tmp, s.snapshotPath); err != nil {
		log.Printf("[VSS] snapshot rename: %v", err)
	}
}

func copyStringMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func (s *vssMonitorService) snapshotLoop(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.saveLiveSnapshot()
			return
		case <-ticker.C:
			s.saveLiveSnapshot()
		}
	}
}

func (s *vssMonitorService) purgeOldLogs() {
	days := s.cfg.LogRetentionDays
	if days <= 0 {
		days = 30
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	dir := s.cfg.LogDir
	if dir == "" {
		dir = "storage/public/logs/vss"
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("[VSS] log cleanup error: %v", err)
		return
	}

	deleted := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "vss-delay-") || !strings.HasSuffix(name, ".log") {
			continue
		}
		datePart := strings.TrimSuffix(strings.TrimPrefix(name, "vss-delay-"), ".log")
		day, err := time.Parse("2006-01-02", datePart)
		if err != nil {
			continue
		}
		if !day.Before(cutoff) {
			continue
		}
		path := filepath.Join(dir, name)
		if err := os.Remove(path); err != nil {
			log.Printf("[VSS] delete log failed %s: %v", name, err)
			continue
		}
		deleted++
		log.Printf("[VSS] deleted old log: %s", name)
	}
	if deleted > 0 {
		log.Printf("[VSS] log cleanup done deleted=%d retention_days=%d", deleted, days)
	}
}

func (s *vssMonitorService) enqueueAlert(ev domain.VSSDelayEvent) {
	if !s.cfg.EmailEnabled {
		return
	}

	key := ev.DeviceID + "|" + ev.Reason
	cooldown := time.Duration(s.cfg.EmailCooldownMin) * time.Minute
	if cooldown <= 0 {
		cooldown = 30 * time.Minute
	}

	s.emailMu.Lock()
	defer s.emailMu.Unlock()

	if t, ok := s.lastEmail[key]; ok && time.Since(t) < cooldown {
		return
	}
	s.lastEmail[key] = time.Now()

	s.pendingAlerts = append(s.pendingAlerts, ev)
}

func (s *vssMonitorService) upsertLive(
	p statusPayload,
	action string,
	delaySec int64,
	isLater bool,
	reasons []string,
	now time.Time,
	lastSeenOverride time.Time,
) {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	if loc == nil {
		loc = time.FixedZone("WIB", 7*3600)
	}

	status := "NORMAL"
	if len(reasons) > 0 {
		switch {
		case containsStr(reasons, "alarm"):
			status = "ALARM"
		case containsStr(reasons, "no_heartbeat"):
			status = "STALE"
		case containsStr(reasons, "storage_full"):
			status = "STORAGE_FULL"
		case containsStr(reasons, "high_cpu"):
			status = "HIGH_CPU"
		case containsStr(reasons, "delayed"), containsStr(reasons, "stale_timestamp"):
			status = "DELAYED"
		default:
			status = "ISSUE"
		}
	}

	name := p.Ext.DeviceName
	if name == "" {
		name = p.DeviceID
	}

	var storageFree, cpu string
	if len(p.Storage) > 0 {
		storageFree = p.Storage[0].Free
	}
	cpu = p.DevTemp.CPU

	seenAt := now
	if !lastSeenOverride.IsZero() {
		seenAt = lastSeenOverride
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.live[p.DeviceID] = &VSSDeviceLive{
		DeviceID:     p.DeviceID,
		DeviceName:   name,
		NodeID:       p.NodeID,
		DTU:          p.DTU,
		ReportTime:   p.Ext.ReportTime,
		IsLater:      isLater,
		DelaySec:     delaySec,
		LastSeen:     seenAt.In(loc).Format(time.RFC3339),
		Action:       action,
		Status:       status,
		Reasons:      append([]string{}, reasons...),
		CPU:          cpu,
		StorageFree:  storageFree,
	}

	s.liveDirty = true
}

func containsStr(ss []string, v string) bool {
	for _, x := range ss {
		if x == v {
			return true
		}
	}
	return false
}

func (s *vssMonitorService) emailDigestLoop(ctx context.Context) {
	if !s.cfg.EmailEnabled {
		return
	}
	min := s.cfg.EmailDigestMin
	if min <= 0 {
		min = 10
	}
	ticker := time.NewTicker(time.Duration(min) * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.flushAlertDigest()
		}
	}
}

func (s *vssMonitorService) dailyReportLoop(ctx context.Context) {
	cfg := config.GetConfig()
	r := cfg.VSSReportEmail
	if !r.Enabled {
		log.Println("[VSS] daily report email disabled")
		return
	}
	if r.SMTPHost == "" || r.FromEmail == "" || strings.TrimSpace(r.ToEmails) == "" {
		log.Println("[VSS] daily report email skipped: incomplete config")
		return
	}

	log.Printf("[VSS] daily report scheduler enabled at %02d:%02d WIB → %s",
		r.Hour, r.Minute, r.ToEmails)

	var lastSentDate string
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			loc, _ := time.LoadLocation("Asia/Jakarta")
			if loc == nil {
				loc = time.FixedZone("WIB", 7*3600)
			}
			now := time.Now().In(loc)
			if now.Hour() != r.Hour || now.Minute() != r.Minute {
				continue
			}
			todayKey := now.Format("2006-01-02")
			if lastSentDate == todayKey {
				continue
			}

			reportDate := now.AddDate(0, 0, -1).Format("2006-01-02")
			if err := s.SendDailyReport(reportDate); err != nil {
				log.Printf("[VSS] daily report failed date=%s: %v", reportDate, err)
				continue
			}
			lastSentDate = todayKey
			log.Printf("[VSS] daily report sent date=%s", reportDate)
		}
	}
}

func (s *vssMonitorService) SendDailyReport(date string) error {
	cfg := config.GetConfig()
	r := cfg.VSSReportEmail
	if !r.Enabled {
		return fmt.Errorf("VSS report email disabled")
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")
	if loc == nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	if strings.TrimSpace(date) == "" {
		date = time.Now().In(loc).AddDate(0, 0, -1).Format("2006-01-02")
	}

	summary, err := s.GetDaySummary(date)
	if err != nil {
		return fmt.Errorf("summary: %w", err)
	}

	rows, err := s.GetDayDelayExport(date)
	if err != nil {
		return fmt.Errorf("export rows: %w", err)
	}

	xlsxBytes, err := buildDayDelayExcel(date, rows)
	if err != nil {
		return fmt.Errorf("excel: %w", err)
	}

	html := buildVSSDailyReportHTML(summary)
	subject := fmt.Sprintf("[VSS Rekap] %s — %d device · %d event",
		summary.Date, summary.TotalDevices, summary.TotalEvents)

	to := utils.ParseEmailList(r.ToEmails)
	if len(to) == 0 {
		return fmt.Errorf("no recipients in VSS_REPORT_TO_EMAILS")
	}
	cc := utils.ParseEmailList(r.CcEmails)

	filename := fmt.Sprintf("vss-rekap-delay-%s.xlsx", date)
	return utils.SendVSSReportEmail(to, cc, subject, html, map[string][]byte{
		filename: xlsxBytes,
	})
}

func buildDayDelayExcel(date string, rows []domain.VSSDayDeviceDelayStat) ([]byte, error) {
	f := excelize.NewFile()
	sheet := "Rekap Delay"
	_ = f.SetSheetName("Sheet1", sheet)

	headers := []string{"No", "Name", "Device ID", "Delay Max", "Delay Min", "Delay Avg"}
	for i, htxt := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, htxt)
	}
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"6366F1"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	_ = f.SetCellStyle(sheet, "A1", "F1", headerStyle)

	formatSec := func(sec int64) string {
		neg := sec < 0
		s := sec
		if s < 0 {
			s = -s
		}
		var out string
		if s < 60 {
			out = fmt.Sprintf("%ds", s)
		} else {
			h := s / 3600
			m := (s % 3600) / 60
			secR := s % 60
			parts := []string{}
			if h > 0 {
				parts = append(parts, fmt.Sprintf("%dh", h))
			}
			if m > 0 {
				parts = append(parts, fmt.Sprintf("%dm", m))
			}
			if secR > 0 || len(parts) == 0 {
				parts = append(parts, fmt.Sprintf("%ds", secR))
			}
			out = strings.Join(parts, " ")
		}
		if neg {
			return "-" + out
		}
		return out
	}

	for i, r := range rows {
		row := i + 2
		_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", row), i+1)
		_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", row), r.DeviceName)
		_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", row), r.DeviceID)
		_ = f.SetCellValue(sheet, fmt.Sprintf("D%d", row), formatSec(r.MaxDelaySec))
		_ = f.SetCellValue(sheet, fmt.Sprintf("E%d", row), formatSec(r.MinDelaySec))
		_ = f.SetCellValue(sheet, fmt.Sprintf("F%d", row), formatSec(int64(r.AvgDelaySec+0.5)))
	}
	_ = f.SetColWidth(sheet, "A", "A", 6)
	_ = f.SetColWidth(sheet, "B", "B", 18)
	_ = f.SetColWidth(sheet, "C", "C", 22)
	_ = f.SetColWidth(sheet, "D", "F", 14)

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func formatDelayHuman(sec int64) string {
	neg := sec < 0
	s := sec
	if s < 0 {
		s = -s
	}
	var out string
	if s < 60 {
		out = fmt.Sprintf("%ds", s)
	} else {
		h := s / 3600
		m := (s % 3600) / 60
		secR := s % 60
		parts := []string{}
		if h > 0 {
			parts = append(parts, fmt.Sprintf("%dh", h))
		}
		if m > 0 {
			parts = append(parts, fmt.Sprintf("%dm", m))
		}
		if secR > 0 || len(parts) == 0 {
			parts = append(parts, fmt.Sprintf("%ds", secR))
		}
		out = strings.Join(parts, " ")
	}
	if neg {
		return "-" + out
	}
	return out
}

func buildVSSDailyReportHTML(sum *domain.VSSDaySummary) string {
	if sum == nil {
		return "<p>Tidak ada data</p>"
	}

	pieTotal := 0
	if sum.Distribution != nil {
		for _, k := range []string{"1s-5s", "5s-1m", "1m-5m", ">5m"} {
			pieTotal += sum.Distribution[k]
		}
	}

	var topRows strings.Builder
	for i, d := range sum.TopDevices {
		if i >= 10 {
			break
		}
		bg := "#ffffff"
		if i%2 == 1 {
			bg = "#f8fafc"
		}
		topRows.WriteString(fmt.Sprintf(`
			<tr style="background:%s;">
				<td style="padding:10px 12px;border-bottom:1px solid #e2e8f0;color:#64748b;font-size:13px;">%d</td>
				<td style="padding:10px 12px;border-bottom:1px solid #e2e8f0;font-weight:600;color:#0f172a;font-size:13px;">%s</td>
				<td style="padding:10px 12px;border-bottom:1px solid #e2e8f0;font-family:Consolas,monospace;font-size:12px;color:#64748b;">%s</td>
				<td style="padding:10px 12px;border-bottom:1px solid #e2e8f0;color:#dc2626;font-weight:700;font-size:13px;text-align:right;">%s</td>
			</tr>`,
			bg,
			i+1,
			htmlEscape(d.DeviceName),
			htmlEscape(d.DeviceID),
			formatDelayHuman(d.MaxDelaySec),
		))
	}
	if topRows.Len() == 0 {
		topRows.WriteString(`
			<tr>
				<td colspan="4" style="padding:20px;text-align:center;color:#94a3b8;font-size:13px;">
					Tidak ada device dengan avg delay &gt; 3 detik
				</td>
			</tr>`)
	}

	maxDeviceBlock := ""
	if sum.MaxDelayDevice != "" {
		maxDeviceBlock = fmt.Sprintf(`
			<div style="margin:0 0 16px;padding:12px 14px;background:#fef2f2;border:1px solid #fecaca;border-radius:8px;font-size:13px;color:#7f1d1d;">
				<span style="font-weight:600;">Delay tertinggi:</span>
				<span style="color:#dc2626;font-weight:700;">%s</span>
				&nbsp;·&nbsp;%s
				<span style="color:#94a3b8;font-family:Consolas,monospace;font-size:12px;">(%s)</span>
			</div>`,
			formatDelayHuman(sum.MaxDelaySec),
			htmlEscape(sum.MaxDelayDevice),
			htmlEscape(sum.MaxDelayDeviceID),
		)
	}

	d15 := 0
	d51 := 0
	d15m := 0
	d5m := 0
	if sum.Distribution != nil {
		d15 = sum.Distribution["1s-5s"]
		d51 = sum.Distribution["5s-1m"]
		d15m = sum.Distribution["1m-5m"]
		d5m = sum.Distribution[">5m"]
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="id">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>VSS Rekap %s</title>
</head>
<body style="margin:0;padding:0;background:#eef2f7;font-family:'Segoe UI',Tahoma,Arial,sans-serif;-webkit-font-smoothing:antialiased;">
  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background:#eef2f7;padding:24px 12px;">
    <tr>
      <td align="center">
        <table role="presentation" width="640" cellpadding="0" cellspacing="0" style="max-width:640px;width:100%%;background:#ffffff;border-radius:12px;overflow:hidden;border:1px solid #e2e8f0;">

          <!-- Header -->
          <tr>
            <td style="background:#1e3a5f;padding:28px 28px 24px;text-align:center;">
              <div style="color:#93c5fd;font-size:12px;letter-spacing:0.6px;text-transform:uppercase;margin-bottom:8px;">
                VSS Monitor · Laporan Harian
              </div>
              <h1 style="margin:0;color:#ffffff;font-size:22px;font-weight:700;line-height:1.3;">
                Rekap Delay %s
              </h1>
              <p style="margin:10px 0 0;color:#bfdbfe;font-size:13px;">
                Periode 00:00 – 23:59 WIB · dikirim otomatis
              </p>
            </td>
          </tr>

          <!-- Body -->
          <tr>
            <td style="padding:24px 28px 8px;">

              <!-- KPI -->
              <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="margin-bottom:20px;">
                <tr>
                  <td width="25%%" style="padding:4px;">
                    <div style="background:#f8fafc;border:1px solid #e2e8f0;border-radius:10px;padding:14px 10px;text-align:center;">
                      <div style="font-size:10px;font-weight:600;color:#94a3b8;text-transform:uppercase;letter-spacing:0.4px;">Total device</div>
                      <div style="margin-top:6px;font-size:22px;font-weight:700;color:#0f172a;">%d</div>
                    </div>
                  </td>
                  <td width="25%%" style="padding:4px;">
                    <div style="background:#f8fafc;border:1px solid #e2e8f0;border-radius:10px;padding:14px 10px;text-align:center;">
                      <div style="font-size:10px;font-weight:600;color:#94a3b8;text-transform:uppercase;letter-spacing:0.4px;">Avg &gt; 3s</div>
                      <div style="margin-top:6px;font-size:22px;font-weight:700;color:#0f172a;">%d</div>
                    </div>
                  </td>
                  <td width="25%%" style="padding:4px;">
                    <div style="background:#fef2f2;border:1px solid #fecaca;border-radius:10px;padding:14px 10px;text-align:center;">
                      <div style="font-size:10px;font-weight:600;color:#f87171;text-transform:uppercase;letter-spacing:0.4px;">Delay tertinggi</div>
                      <div style="margin-top:6px;font-size:18px;font-weight:700;color:#dc2626;">%s</div>
                    </div>
                  </td>
                  <td width="25%%" style="padding:4px;">
                    <div style="background:#f8fafc;border:1px solid #e2e8f0;border-radius:10px;padding:14px 10px;text-align:center;">
                      <div style="font-size:10px;font-weight:600;color:#94a3b8;text-transform:uppercase;letter-spacing:0.4px;">Total event</div>
                      <div style="margin-top:6px;font-size:22px;font-weight:700;color:#0f172a;">%d</div>
                    </div>
                  </td>
                </tr>
              </table>

              <p style="margin:0 0 12px;font-size:13px;line-height:1.55;color:#64748b;">
                <strong style="color:#334155;">Total device</strong> = unit yang tersample hari itu.
                <strong style="color:#334155;">Avg &gt; 3s</strong> = device yang masuk distribusi delay (%d unit).
              </p>

              %s

              <!-- Distribusi -->
              <div style="margin:0 0 20px;padding:14px 16px;background:#f8fafc;border:1px solid #e2e8f0;border-radius:10px;">
                <div style="font-size:11px;font-weight:600;color:#94a3b8;text-transform:uppercase;letter-spacing:0.4px;margin-bottom:10px;">
                  Distribusi avg delay
                </div>
                <table role="presentation" width="100%%" cellpadding="0" cellspacing="0">
                  <tr>
                    <td style="padding:6px 0;font-size:13px;color:#475569;">3–5 detik</td>
                    <td style="padding:6px 0;font-size:13px;font-weight:700;color:#0f172a;text-align:right;">%d</td>
                  </tr>
                  <tr>
                    <td style="padding:6px 0;font-size:13px;color:#475569;">5 detik – 1 menit</td>
                    <td style="padding:6px 0;font-size:13px;font-weight:700;color:#0f172a;text-align:right;">%d</td>
                  </tr>
                  <tr>
                    <td style="padding:6px 0;font-size:13px;color:#475569;">1 – 5 menit</td>
                    <td style="padding:6px 0;font-size:13px;font-weight:700;color:#0f172a;text-align:right;">%d</td>
                  </tr>
                  <tr>
                    <td style="padding:6px 0;font-size:13px;color:#475569;">&gt; 5 menit</td>
                    <td style="padding:6px 0;font-size:13px;font-weight:700;color:#dc2626;text-align:right;">%d</td>
                  </tr>
                </table>
              </div>

              <!-- Top 10 -->
              <div style="font-size:14px;font-weight:700;color:#0f172a;margin:0 0 10px;">
                Top 10 avg delay
              </div>
              <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="border:1px solid #e2e8f0;border-radius:10px;overflow:hidden;border-collapse:separate;">
                <thead>
                  <tr style="background:#f1f5f9;">
                    <th align="left" style="padding:10px 12px;font-size:11px;color:#94a3b8;font-weight:600;text-transform:uppercase;">#</th>
                    <th align="left" style="padding:10px 12px;font-size:11px;color:#94a3b8;font-weight:600;text-transform:uppercase;">Device</th>
                    <th align="left" style="padding:10px 12px;font-size:11px;color:#94a3b8;font-weight:600;text-transform:uppercase;">Device ID</th>
                    <th align="right" style="padding:10px 12px;font-size:11px;color:#94a3b8;font-weight:600;text-transform:uppercase;">Avg delay</th>
                  </tr>
                </thead>
                <tbody>
                  %s
                </tbody>
              </table>

              <p style="margin:16px 0 8px;font-size:12px;line-height:1.5;color:#94a3b8;">
                Lampiran Excel berisi min / max / avg semua device yang tersample pada tanggal tersebut.
              </p>
            </td>
          </tr>

          <!-- Footer -->
          <tr>
            <td style="background:#0f172a;padding:14px 28px;text-align:center;">
              <p style="margin:0;font-size:11px;color:#94a3b8;">
                VSS Monitor · Automated daily report
              </p>
            </td>
          </tr>

        </table>
      </td>
    </tr>
  </table>
</body>
</html>`,
		sum.Date,                       // <title>
		sum.Date,                       // heading
		sum.TotalDevices,               // KPI total device
		pieTotal,                       // KPI avg > 3s
		formatDelayHuman(sum.MaxDelaySec), // KPI delay tertinggi
		sum.TotalEvents,                // KPI total event
		pieTotal,                       // teks penjelasan (%d unit)
		maxDeviceBlock,                 // blok delay tertinggi (HTML utuh)
		d15,                            // distribusi 3–5s
		d51,                            // 5s–1m
		d15m,                           // 1–5m
		d5m,                            // >5m
		topRows.String(),               // top 10 rows
	)
}

func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

func (s *vssMonitorService) flushAlertDigest() {
	s.emailMu.Lock()
	batch := s.pendingAlerts
	s.pendingAlerts = nil
	s.emailMu.Unlock()

	if len(batch) == 0 {
		return
	}

	minDev := s.cfg.EmailMinDevices
	if minDev > 1 {
		uniq := map[string]struct{}{}
		for _, ev := range batch {
			uniq[ev.DeviceID] = struct{}{}
		}
		if len(uniq) < minDev {
			return
		}
	}

	recipients := parseEmails(s.cfg.AlertEmails)
	if len(recipients) == 0 {
		return
	}

	if err := utils.SendVSSDigestAlert(batch, recipients); err != nil {
		log.Printf("[VSS] digest email failed: %v", err)
		return
	}
	log.Printf("[VSS] digest email sent count=%d to=%v", len(batch), recipients)
}

func parseEmails(s string) []string {
	var out[]string
	for _, e := range strings.Split(s, ",") {
		e = strings.TrimSpace(e)
		if e != "" {
			out = append(out, e)
		}
	}
	return out
}