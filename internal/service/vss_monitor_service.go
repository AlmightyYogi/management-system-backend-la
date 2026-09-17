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

type VSSMonitorService interface {
	Start(ctx context.Context)
	ListDelays(filter repository.VSSDelayFilter) (*domain.VSSDelayListResult, error)
	ListHistory(filter repository.VSSHistoryFilter) ([]domain.VSSAlertHistory, int64, error)
}

type vssMonitorService struct {
	repo		 	repository.VSSDelayRepository
	historyRepo  	repository.VSSAlertHistoryRepository
	cfg			 	config.VSSConfig
	mu			 	sync.Mutex
	lastSeen	 	map[string]time.Time
	logReasons   	map[string]bool
	emailReasons 	map[string]bool
	emailMu      	sync.Mutex
	lastEmail    	map[string]time.Time
	pendingAlerts 	[]domain.VSSDelayEvent
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
		cfg.StaleThresholdSec = 90
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
		logReasons: 	parseReasonSet(cfg.LogReasons),
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

func (s *vssMonitorService) handleMessage(data []byte) {
	var env wsEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return
	}

	switch env.Action {
	case "80003":
		s.handleStatus80003(env.Payload)
	case "80004":
		s.handleAlarm80004(env.Payload)
	default:
		return
	}
}

func (s *vssMonitorService) computeDelaySec(p statusPayload, now time.Time) int64 {
	if p.Ext.ReportTime > 0 {
		rt := time.UnixMilli(p.Ext.ReportTime)
		d := now.Sub(rt).Seconds()
		if d < 0 {
			d = -d
		}
		return int64(d)
	}
	if p.DTU != "" {
		loc, _ := time.LoadLocation("Asia/Jakarta")
		if loc == nil {
			loc = time.FixedZone("WIB", 7*3600)
		}
		t, err := time.ParseInLocation("2006-01-02 15:04:05", p.DTU, loc)
		if err == nil {
			d := now.Sub(t).Seconds()
			if d < 0 {
				d = -d
			}
			return int64(d)
		}
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

	if s.reasonAllowedForEmail(reason) {
		s.saveHistory(ev)
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
				fake := statusPayload{DeviceID: deviceID}
				fake.Ext.DeviceName = deviceID
				s.persistIfNotSpam(fake, "no_heartbeat", int64(gap.Seconds()), false, now, "stale", "", "", "")
			}
		}
	}
}

func (s *vssMonitorService) handleStatus80003(raw json.RawMessage) {
	var p statusPayload
	if err := json.Unmarshal(raw, &p); err != nil || p.DeviceID == "" {
		return
	}

	now := time.Now()
	s.mu.Lock()
	s.lastSeen[p.DeviceID] = now
	s.mu.Unlock()

	isLater := p.IsLater == "1" || p.Ext.IsLater
	delaySec := s.computeDelaySec(p, now)

	var candidates []string
	if isLater {
		candidates = append(candidates, "delayed")
	}
	if delaySec >= s.cfg.DelayThresholdSec {
		candidates = append(candidates, "stale_timestamp")
	}
	if s.reasonAllowedForLog("storage_full") {
		for _, st := range p.Storage {
			if st.Free == "0" {
				candidates = append(candidates, "storage_full")
				break
			}
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
	for _, r := range candidates {
		r = strings.ToLower(r)
		if seen[r] || !s.reasonAllowedForLog(r) {
			continue
		}
		seen[r] = true
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
	s.mu.Unlock()

	sp := statusPayload{
		DeviceID: 	p.DeviceID,
		NodeID: 	p.NodeID,
		DTU: 		p.DTU,
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
	if et := strings.ToLower(strings.TrimSpace(p.EventType)); et != "" {
		reason = "alarm"
	}

	if !s.reasonAllowedForLog(reason) && !s.reasonAllowedForLog("alarm") {
		return
	}
	if !s.reasonAllowedForLog(reason) {
		reason = "alarm"
	}

	s.persistIfNotSpam(
		sp,
		reason,
		delaySec,
		isLater,
		now,
		"80004",
		p.AlarmID,
		p.AlarmDetail,
		p.EventType,
	)
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