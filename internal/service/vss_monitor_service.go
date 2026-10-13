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
	"time"

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
	ListLive(deviceName, action, status string, onlyIssue bool, page, perPage int) *VSSLiveResult
	GetTraffic() *VSSTrafficDetail
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
}

type wsEnvelope struct {
	Action  string          `json:"action"`
	Payload json.RawMessage `json:"payload"`
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
		minuteHistory:   make([]MinuteTraffic, 0, 60),
	}

	if cfg.EmailCooldownMin <= 0 {
		cfg.EmailCooldownMin = 15
		svc.cfg = cfg
	}
	svc.emailReasons = parseReasonSet(cfg.EmailReasons)
	svc.lastEmail = make(map[string]time.Time)

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
	go s.loop(ctx)
	go s.staleChecker(ctx)
	go s.logCleanup(ctx)
	go s.emailDigestLoop(ctx)
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

func (s *vssMonitorService) ListLive(
	deviceName, action, status string,
	onlyIssue bool,
	page, perPage int,
) *VSSLiveResult {
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
		if len(cp.Reasons) == 0 && cp.DelaySec >= int64(s.cfg.DelayThresholdSec) {
			cp.Status = "DELAYED"
		}
		all = append(all, cp)
	}

	sort.Slice(all, func(i, j int) bool {
		if all[i].DelaySec == all[j].DelaySec {
			return all[i].DeviceName < all[j].DeviceName
		}
		return all[i].DelaySec < all[j].DelaySec
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

	log.Println("[VSS] websocket connected & logged in")

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		_ = conn.SetReadDeadline(time.Now().Add(120 * time.Second))
		_, data, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("ws read: %w", err)
		}
		s.handleMessage(data)
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

	if updated := s.tryUpsertFromAny(env.Action, env.Payload); updated {
		return
	}

	switch env.Action {
	case "80003":
		s.handleStatus80003(env.Payload)
	case "80004":
		s.handleAlarm80004(env.Payload)
	}
}

func (s *vssMonitorService) tryUpsertFromAny(action string, raw json.RawMessage) bool {
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return false
	}

	deviceID, _ := m["deviceID"].(string)
	if deviceID == "" {
		deviceID, _ = m["deviceId"].(string)
	}
	if deviceID == "" {
		return false
	}

	dtu, _ := m["dtu"].(string)
	if dtu == "" {
		if loc, ok := m["location"].(map[string]interface{}); ok {
			dtu, _ = loc["dtu"].(string)
		}
	}
	if dtu == "" {
		if pl, ok := m["payload"].(map[string]interface{}); ok {
			dtu, _ = pl["dtu"].(string)
		}
	}

	deviceName := ""
	var reportMs int64
	if ext, ok := m["ext"].(map[string]interface{}); ok {
		deviceName, _ = ext["deviceName"].(string)
		if rt, ok := ext["reportTime"].(float64); ok {
			reportMs = int64(rt)
		}
	}
	if deviceName == "" {
		deviceName, _ = m["deviceName"].(string)
	}

	if dtu == "" && reportMs <= 0 {
		return false
	}

	now := time.Now()
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

	s.upsertLive(p, action, delaySec, false, reasons, now, time.Time{})
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
		if len(s.minuteHistory) > 60 {
			s.minuteHistory = s.minuteHistory[len(s.minuteHistory)-60:]
		}
		s.currentMinute = minuteKey
		s.currentCounts = make(map[string]int64)
	}

	if s.currentCounts == nil {
		s.currentCounts = make(map[string]int64)
	}
	s.currentCounts[action]++
}

func (s *vssMonitorService) computeDelaySec(p statusPayload, now time.Time) int64 {
	if t, ok := parseDTU(p.DTU); ok {
		return int64(now.Sub(t).Seconds())
	}
	if t, ok := reportTimeToTime(p.Ext.ReportTime); ok {
		return int64(now.Sub(t).Seconds())
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

	dedupeWindow := 15 * time.Minute
	exists, err := s.repo.ExistsRecent(p.DeviceID, reason, dedupeWindow)
	if err != nil || exists {
		return
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")
	if loc == nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	detected := now.In(loc).Format(time.RFC3339)

	msg := fmt.Sprintf(
		"VSS | action=%s | device=%s (%s) | reason=%s | delay=%ds | dtu=%s | alarmId=%s | detail=%s",
		action, p.DeviceID, p.Ext.DeviceName, reason, delaySec, p.DTU, alarmID, alarmDetail,
	)

	ev := domain.VSSDelayEvent{
		DetectedAt:  detected,
		DeviceID:    p.DeviceID,
		DeviceName:  p.Ext.DeviceName,
		NodeID:      p.NodeID,
		DTU:         p.DTU,
		ReportTime:  p.Ext.ReportTime,
		IsLater:     isLater,
		DelaySec:    delaySec,
		Reason:      reason,
		Message:     msg,
		Action:      action,
		AlarmID:     alarmID,
		AlarmDetail: alarmDetail,
		EventType:   eventType,
	}

	if err := s.repo.Append(ev); err != nil {
		log.Printf("[VSS] write log file error: %v", err)
		return
	}

	if s.reasonAllowedForLog(reason) {
		s.saveHistory(ev)
	}

	if s.reasonAllowedForEmail(reason) {
		s.enqueueAlert(ev)
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
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	threshold := time.Duration(s.cfg.StaleThresholdSec) * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			s.mu.Lock()
			snapshot := make(map[string]time.Time, len(s.lastSeen))
			for k, v := range s.lastSeen {
				snapshot[k] = v
			}
			s.mu.Unlock()

			for deviceID, last := range snapshot {
				gap := now.Sub(last)
				if gap < threshold {
					continue
				}
				if !s.reasonAllowedForLog("no_heartbeat") {
					continue
				}

				name := deviceID
				dtu := ""
				var reportMs int64
				var delaySec int64

				s.mu.Lock()
				if n, ok := s.deviceName[deviceID]; ok && n != "" {
					name = n
				}
				if v, ok := s.lastDTU[deviceID]; ok {
					dtu = v
				}
				if v, ok := s.lastReportMs[deviceID]; ok {
					reportMs = v
				}
				if prev, ok := s.live[deviceID]; ok && prev != nil {
					delaySec = prev.DelaySec
				} else if t, ok := parseDTU(dtu); ok {
					d := now.Sub(t).Seconds()
					if d < 0 {
						d = 0
					}
					delaySec = int64(d)
				}
				s.mu.Unlock()

				fake := statusPayload{
					DeviceID: deviceID,
					DTU:      dtu,
				}
				fake.Ext.DeviceName = name
				fake.Ext.ReportTime = reportMs

				s.upsertLive(fake, "stale", delaySec, false, []string{"no_heartbeat"}, now, last)

				s.persistIfNotSpam(
					fake,
					"no_heartbeat",
					delaySec,
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

// func (s *vssMonitorService) ProcessIncoming(action string, payload map[string]interface{}) {
// 	receivedAt := time.Now()
// 	s.repo.IncrementCounter(action)

// 	if action != "80003" && action != "80004" {
// 		return
// 	}

// 	deviceID, _   := payload["deviceID"].(string)
// 	nodeID, _     := payload["nodeID"].(string)
// 	dtuStr, _     := payload["dtu"].(string)
// 	isLaterStr, _ := payload["isLater"].(string)

// 	ext, _       := payload["ext"].(map[string]interface{})
// 	deviceName   := ""
// 	reportTime   := int64(0)
// 	if ext != nil {
// 		deviceName, _ = ext["deviceName"].(string)
// 		if rt, ok := ext["reportTime"].(float64); ok {
// 			reportTime = int64(rt)
// 		}
// 	}

// 	var dtuTime time.Time
// 	if dtuStr != "" {
// 		parsed, err := time.ParseInLocation("2006-01-02 15:04:05", dtuStr, jakartaTZ)
// 		if err == nil {
// 			dtuTime = parsed
// 		}
// 	}
// 	if dtuTime.IsZero() {
// 		dtuTime = receivedAt
// 	}

// 	delayMs := receivedAt.UnixMilli() - dtuTime.UnixMilli()
// 	if delayMs < 0 {
// 		delayMs = 0
// 	}
// 	delaySec := delayMs / 1000

// 	isLater := isLaterStr == "1"

// 	status := "NORMAL"
// 	reason := ""
// 	if storageArr, ok := payload["storage"].([]interface{}); ok {
// 		for _, si := range storageArr {
// 			if sm, ok := si.(map[string]interface{}); ok {
// 				free, _ := sm["free"].(string)
// 				if free == "0" {
// 					status = "STORAGE_FULL"
// 					reason = "STORAGE_FULL"
// 					break
// 				}
// 			}
// 		}
// 	}

// 	message := formatDelay(delaySec)

// 	s.repo.UpsertLive(domain.VSSLiveDevice{
// 		DeviceID:   deviceID,
// 		DeviceName: deviceName,
// 		NodeID:     nodeID,
// 		DTU:        dtuStr,
// 		LastSeen:   receivedAt.Format("02/01/2006 15:04:05"),
// 		DelayMs:    delayMs,
// 		DelaySec:   delaySec,
// 		Action:     action,
// 		Reason:     reason,
// 		Status:     status,
// 		IsLater:    isLater,
// 		ReportTime: reportTime,
// 	})

// 	if status != "NORMAL" || delaySec > 60 {
// 		event := domain.VSSDelayEvent{
// 			DetectedAt: receivedAt.Format(time.RFC3339),
// 			DeviceID:   deviceID,
// 			DeviceName: deviceName,
// 			NodeID:     nodeID,
// 			DTU:        dtuStr,
// 			ReportTime: reportTime,
// 			IsLater:    isLater,
// 			DelayMs:    delayMs,
// 			DelaySec:   delaySec,
// 			Reason:     reason,
// 			Message:    message,
// 			Action:     action,
// 		}
// 		_ = s.repo.Append(event)
// 	}
// }

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

	cooldown := time.Duration(s.cfg.EmailCooldownMin) * time.Minute
	if cooldown <= 0 {
		cooldown = 15 * time.Minute
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
	log.Printf("[VSS] history saved device=%s reason=%s month=%s", h.DeviceID, h.Reason, h.YearMonth)
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