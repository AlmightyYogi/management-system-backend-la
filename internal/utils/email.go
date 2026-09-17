package utils

import (
	"fmt"
	"net/smtp"
	"strings"
	"time"

	"github.com/AlmightyOggy/management-system/internal/config"
	"github.com/AlmightyOggy/management-system/internal/domain"
)

const appBaseURL = "http://10.49.192.13:9999"
const appName    = "Management System"
const teamName   = "B2B IT Operations Team"

func SendOpenTicketReminder(tickets []domain.Report) error {
	cfg := config.GetConfig()
	if cfg.Email.SMTPHost == "" {
		return fmt.Errorf("SMTP configuration not set")
	}

	subject := fmt.Sprintf("[REMINDER] %d Open Ticket Belum Ditutup (>3 Hari) - %s",
		len(tickets),
		time.Now().Format("02 Jan 2006 15:04"),
	)

	var rows strings.Builder
	for _, t := range tickets {
		hours := time.Since(t.CreatedAt).Hours()
		urgentBadge := ""
		if hours >= 8 {
			urgentBadge = `<span style="display:inline-block;background:#dc2626;color:#fff;font-size:10px;font-weight:700;padding:2px 7px;border-radius:10px;margin-left:6px;">KRITIS</span>`
		} else if hours >= 4 {
			urgentBadge = `<span style="display:inline-block;background:#d97706;color:#fff;font-size:10px;font-weight:700;padding:2px 7px;border-radius:10px;margin-left:6px;">URGENT</span>`
		}

		viewURL := fmt.Sprintf("%s/reports/%s", appBaseURL, t.UUID.String())

		rows.WriteString(fmt.Sprintf(`
			<tr>
				<td style="padding:14px 16px;border-bottom:1px solid #f1f5f9;vertical-align:top;">
					<a href="%s" style="color:#2563eb;font-weight:700;font-size:13px;text-decoration:none;">%s</a>%s
				</td>
				<td style="padding:14px 16px;border-bottom:1px solid #f1f5f9;vertical-align:top;font-size:13px;color:#374151;">%s</td>
				<td style="padding:14px 16px;border-bottom:1px solid #f1f5f9;vertical-align:top;font-size:13px;color:#374151;">%s</td>
				<td style="padding:14px 16px;border-bottom:1px solid #f1f5f9;vertical-align:top;font-size:13px;color:#6b7280;">%s</td>
				<td style="padding:14px 16px;border-bottom:1px solid #f1f5f9;vertical-align:top;font-size:13px;color:%s;font-weight:600;">%.1f jam</td>
				<td style="padding:14px 16px;border-bottom:1px solid #f1f5f9;vertical-align:top;text-align:center;">
					<a href="%s" style="display:inline-block;padding:6px 14px;background:#2563eb;color:#fff;font-size:12px;font-weight:600;border-radius:6px;text-decoration:none;">View</a>
				</td>
			</tr>`,
			viewURL, t.Incident, urgentBadge,
			t.Requestor,
			t.Apps,
			t.CreatedAt.Format("02 Jan 2006 15:04"),
			durationColor(hours), hours,
			viewURL,
		))
	}

	htmlBody := buildReminderHTML(len(tickets), rows.String())
	return SendEmail(cfg.Email.AdminEmail, subject, htmlBody)
}

func durationColor(hours float64) string {
	if hours >= 8 { return "#dc2626" }
	if hours >= 4 { return "#d97706" }
	return "#16a34a"
}

func buildReminderHTML(count int, tableRows string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="id">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Open Ticket Reminder</title>
</head>
<body style="margin:0;padding:0;background:#f3f4f6;font-family:'Segoe UI',Arial,sans-serif;">

<table width="100%%" cellpadding="0" cellspacing="0" style="background:#f3f4f6;padding:32px 0;">
  <tr>
    <td align="center">
      <table width="620" cellpadding="0" cellspacing="0" style="max-width:620px;width:100%%;background:#fff;border-radius:12px;overflow:hidden;box-shadow:0 4px 20px rgba(0,0,0,0.08);">

        <!-- ── Header ── -->
        <tr>
          <td style="background:linear-gradient(135deg,#1e3a5f 0%%,#2563eb 100%%);padding:28px 32px;text-align:center;">
            <div style="display:inline-block;background:rgba(255,255,255,0.15);border-radius:10px;padding:8px 20px;margin-bottom:14px;">
              <span style="color:#fff;font-size:15px;font-weight:700;letter-spacing:0.5px;">%s</span>
            </div>
            <h1 style="color:#fff;margin:0;font-size:20px;font-weight:700;line-height:1.3;">
              %d Open Ticket Belum Ditutup
            </h1>
            <p style="color:rgba(255,255,255,0.75);margin:8px 0 0;font-size:13px;">
              Ticket-ticket berikut sudah lebih dari 3 hari berstatus <strong>Open</strong>
            </p>
          </td>
        </tr>

        <!-- ── Alert banner ── -->
        <tr>
          <td style="background:#fef2f2;border-left:4px solid #dc2626;padding:14px 24px;">
            <table cellpadding="0" cellspacing="0">
              <tr>
                <td style="font-size:18px;padding-right:10px;">⚠️</td>
                <td style="font-size:13px;color:#991b1b;line-height:1.5;">
                  Terdapat <strong>%d ticket</strong> yang masih terbuka lebih dari 3 hari dan memerlukan perhatian segera.
                  Harap segera tindaklanjuti atau tutup ticket yang sudah selesai ditangani.
                </td>
              </tr>
            </table>
          </td>
        </tr>

        <!-- ── Body ── -->
        <tr>
          <td style="padding:28px 32px;">

            <p style="font-size:14px;color:#374151;margin:0 0 20px;line-height:1.6;">
              Berikut adalah daftar ticket yang belum ditutup beserta informasi detailnya.
              Klik tombol <strong>View</strong> untuk melihat dan menindaklanjuti ticket.
            </p>

            <!-- Ticket table -->
            <table width="100%%" cellpadding="0" cellspacing="0" style="border-collapse:collapse;border:1px solid #e5e7eb;border-radius:8px;overflow:hidden;font-size:13px;">
              <thead>
                <tr style="background:#f8fafc;">
                  <th style="padding:11px 16px;text-align:left;font-size:11px;font-weight:700;color:#6b7280;text-transform:uppercase;letter-spacing:0.5px;border-bottom:1px solid #e5e7eb;">Incident</th>
                  <th style="padding:11px 16px;text-align:left;font-size:11px;font-weight:700;color:#6b7280;text-transform:uppercase;letter-spacing:0.5px;border-bottom:1px solid #e5e7eb;">Requestor</th>
                  <th style="padding:11px 16px;text-align:left;font-size:11px;font-weight:700;color:#6b7280;text-transform:uppercase;letter-spacing:0.5px;border-bottom:1px solid #e5e7eb;">Aplikasi</th>
                  <th style="padding:11px 16px;text-align:left;font-size:11px;font-weight:700;color:#6b7280;text-transform:uppercase;letter-spacing:0.5px;border-bottom:1px solid #e5e7eb;">Dibuat</th>
                  <th style="padding:11px 16px;text-align:left;font-size:11px;font-weight:700;color:#6b7280;text-transform:uppercase;letter-spacing:0.5px;border-bottom:1px solid #e5e7eb;">Durasi</th>
                  <th style="padding:11px 16px;text-align:center;font-size:11px;font-weight:700;color:#6b7280;text-transform:uppercase;letter-spacing:0.5px;border-bottom:1px solid #e5e7eb;">Aksi</th>
                </tr>
              </thead>
              <tbody>
                %s
              </tbody>
            </table>

            <!-- Legend -->
            <table cellpadding="0" cellspacing="0" style="margin-top:16px;">
              <tr>
                <td style="font-size:12px;color:#6b7280;padding-right:16px;">
                  <span style="display:inline-block;width:10px;height:10px;background:#dc2626;border-radius:50%;margin-right:5px;vertical-align:middle;"></span>
                  Kritis (≥ 8 jam)
                </td>
                <td style="font-size:12px;color:#6b7280;padding-right:16px;">
                  <span style="display:inline-block;width:10px;height:10px;background:#d97706;border-radius:50%;margin-right:5px;vertical-align:middle;"></span>
                  Urgent (≥ 4 jam)
                </td>
                <td style="font-size:12px;color:#6b7280;">
                  <span style="display:inline-block;width:10px;height:10px;background:#16a34a;border-radius:50%;margin-right:5px;vertical-align:middle;"></span>
                  Normal (&lt; 4 jam)
                </td>
              </tr>
            </table>

            <!-- CTA Button -->
            <table cellpadding="0" cellspacing="0" style="margin-top:28px;width:100%%;">
              <tr>
                <td align="center">
                  <a href="%s/reports" style="display:inline-block;padding:12px 32px;background:#2563eb;color:#fff;font-size:14px;font-weight:600;border-radius:8px;text-decoration:none;letter-spacing:0.3px;">
                    Lihat Semua Ticket →
                  </a>
                </td>
              </tr>
            </table>

          </td>
        </tr>

        <!-- ── Divider ── -->
        <tr>
          <td style="padding:0 32px;">
            <div style="height:1px;background:#f1f5f9;"></div>
          </td>
        </tr>

        <!-- ── Footer ── -->
        <tr>
          <td style="padding:24px 32px;background:#f8fafc;">
            <table width="100%%" cellpadding="0" cellspacing="0">
              <tr>
                <td>
                  <p style="margin:0 0 6px;font-size:13px;font-weight:600;color:#374151;">Thank you,</p>
                  <p style="margin:0;font-size:13px;color:#6b7280;">%s</p>
                </td>
                <td align="right" style="vertical-align:bottom;">
                  <p style="margin:0;font-size:11px;color:#9ca3af;">
                    Email ini dikirim otomatis setiap jam.<br>
                    %s
                  </p>
                </td>
              </tr>
            </table>
          </td>
        </tr>

        <!-- ── Bottom bar ── -->
        <tr>
          <td style="background:#1e3a5f;padding:14px 32px;text-align:center;">
            <p style="margin:0;font-size:11px;color:rgba(255,255,255,0.5);">
              © %d %s · Automated Notification System
            </p>
          </td>
        </tr>

      </table>
    </td>
  </tr>
</table>

</body>
</html>`,
		appName,
		count,
		count,
		tableRows,
		appBaseURL,
		teamName,
		time.Now().Format("02 Jan 2006 15:04 WIB"),
		time.Now().Year(),
		appName,
	)
}

func SendNewTicketNotification(report domain.Report) error {
	cfg := config.GetConfig()
	if cfg.Email.SMTPHost == "" {
		return nil
	}

	subject := fmt.Sprintf("[NEW TICKET] %s - %s", report.Incident, report.Apps)
	viewURL  := fmt.Sprintf("%s/reports/%s", appBaseURL, report.UUID.String())

	htmlBody := fmt.Sprintf(`<!DOCTYPE html>
<html lang="id">
<head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background:#f3f4f6;font-family:'Segoe UI',Arial,sans-serif;">

<table width="100%%" cellpadding="0" cellspacing="0" style="background:#f3f4f6;padding:32px 0;">
  <tr>
    <td align="center">
      <table width="580" cellpadding="0" cellspacing="0" style="max-width:580px;width:100%%;background:#fff;border-radius:12px;overflow:hidden;box-shadow:0 4px 20px rgba(0,0,0,0.08);">

        <!-- Header -->
        <tr>
          <td style="background:linear-gradient(135deg,#1e3a5f 0%%,#2563eb 100%%);padding:28px 32px;text-align:center;">
            <div style="display:inline-block;background:rgba(255,255,255,0.15);border-radius:10px;padding:6px 18px;margin-bottom:12px;">
              <span style="color:#fff;font-size:13px;font-weight:700;">%s</span>
            </div>
            <h1 style="color:#fff;margin:0;font-size:19px;font-weight:700;line-height:1.3;">
              Incident has been assigned to %s
            </h1>
          </td>
        </tr>

        <!-- Body -->
        <tr>
          <td style="padding:28px 32px;">
            <p style="font-size:14px;color:#374151;margin:0 0 20px;line-height:1.6;">
              <strong style="color:#2563eb;">%s</strong> was recently assigned to
              <strong>%s</strong>.
            </p>
            <p style="font-size:14px;color:#374151;margin:0 0 24px;line-height:1.6;">
              You can view the incident to track updates and make changes.
              <br>
              <a href="%s" style="color:#2563eb;font-weight:600;">View incident</a>
            </p>

            <!-- Info card -->
            <table width="100%%" cellpadding="0" cellspacing="0" style="background:#f8fafc;border:1px solid #e5e7eb;border-radius:8px;overflow:hidden;margin-bottom:24px;">
              <tr>
                <td style="padding:16px 20px;border-bottom:1px solid #e5e7eb;">
                  <p style="margin:0;font-size:14px;font-weight:700;color:#111827;">About this incident</p>
                </td>
              </tr>
              <tr>
                <td style="padding:16px 20px;">
                  <table cellpadding="0" cellspacing="0">
                    <tr><td style="font-size:13px;color:#6b7280;padding:4px 0;width:140px;">Short description:</td><td style="font-size:13px;color:#111827;font-weight:600;padding:4px 0;">%s</td></tr>
                    <tr><td style="font-size:13px;color:#6b7280;padding:4px 0;">Requestor:</td><td style="font-size:13px;color:#111827;font-weight:600;padding:4px 0;">%s</td></tr>
                    <tr><td style="font-size:13px;color:#6b7280;padding:4px 0;">Application:</td><td style="font-size:13px;color:#111827;font-weight:600;padding:4px 0;">%s</td></tr>
                    <tr><td style="font-size:13px;color:#6b7280;padding:4px 0;">Severity:</td><td style="font-size:13px;color:#111827;font-weight:600;padding:4px 0;">%s</td></tr>
                    <tr><td style="font-size:13px;color:#6b7280;padding:4px 0;">Scope:</td><td style="font-size:13px;color:#111827;font-weight:600;padding:4px 0;">%s</td></tr>
                    <tr><td style="font-size:13px;color:#6b7280;padding:4px 0;">Created:</td><td style="font-size:13px;color:#111827;font-weight:600;padding:4px 0;">%s</td></tr>
                  </table>
                </td>
              </tr>
            </table>

            <!-- CTA -->
            <table cellpadding="0" cellspacing="0" style="width:100%%;">
              <tr>
                <td align="center">
                  <a href="%s" style="display:inline-block;padding:12px 32px;background:#2563eb;color:#fff;font-size:14px;font-weight:600;border-radius:8px;text-decoration:none;">
                    View Incident →
                  </a>
                </td>
              </tr>
            </table>
          </td>
        </tr>

        <!-- Footer -->
        <tr>
          <td style="padding:20px 32px;background:#f8fafc;border-top:1px solid #f1f5f9;">
            <p style="margin:0 0 4px;font-size:13px;font-weight:600;color:#374151;">Thank you,</p>
            <p style="margin:0;font-size:13px;color:#6b7280;">%s</p>
          </td>
        </tr>

        <!-- Bottom bar -->
        <tr>
          <td style="background:#1e3a5f;padding:12px 32px;text-align:center;">
            <p style="margin:0;font-size:11px;color:rgba(255,255,255,0.5);">
              © %d %s · Automated Notification System
            </p>
          </td>
        </tr>

      </table>
    </td>
  </tr>
</table>
</body>
</html>`,
		appName,
		report.AssignedTo,
		report.Incident, report.AssignedTo,
		viewURL,
		report.Description,
		report.Requestor,
		report.Apps,
		report.Severity,
		report.Scope,
		report.CreatedAt.Format("02 Jan 2006 15:04"),
		viewURL,
		teamName,
		time.Now().Year(), appName,
	)

	return SendEmail(cfg.Email.AdminEmail, subject, htmlBody)
}

func SendVSSDelayAlert(ev domain.VSSDelayEvent, recipients []string) error {
	cfg := config.GetConfig()
	if cfg.Email.SMTPHost == "" {
		return nil
	}
	if len(recipients) == 0 {
		return nil
	}

	reasonLabel := vssReasonLabel(ev.Reason)
	reasonColor := vssReasonColor(ev.Reason)

	subject := fmt.Sprintf("[VSS ALERT] %s — %s (%s)",
		reasonLabel, ev.DeviceName, ev.DeviceID)

	monitorURL := fmt.Sprintf("%s/vss/delays?device_id=%s", appBaseURL, ev.DeviceID)

	htmlBody := fmt.Sprintf(`<!DOCTYPE html>
<html lang="id">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>VSS Alert</title>
</head>
<body style="margin:0;padding:0;background:#f3f4f6;font-family:'Segoe UI',Arial,sans-serif;">

<table width="100%%" cellpadding="0" cellspacing="0" style="background:#f3f4f6;padding:32px 0;">
  <tr>
    <td align="center">
      <table width="580" cellpadding="0" cellspacing="0" style="max-width:580px;width:100%%;background:#fff;border-radius:12px;overflow:hidden;box-shadow:0 4px 20px rgba(0,0,0,0.08);">

        <!-- Header -->
        <tr>
          <td style="background:linear-gradient(135deg,#1e3a5f 0%%,#2563eb 100%%);padding:28px 32px;text-align:center;">
            <div style="display:inline-block;background:rgba(255,255,255,0.15);border-radius:10px;padding:6px 18px;margin-bottom:12px;">
              <span style="color:#fff;font-size:13px;font-weight:700;">%s</span>
            </div>
            <h1 style="color:#fff;margin:0;font-size:19px;font-weight:700;line-height:1.3;">
              VSS Device Alert Detected
            </h1>
            <p style="color:rgba(255,255,255,0.75);margin:8px 0 0;font-size:13px;">
              Monitor otomatis mendeteksi kondisi abnormal pada perangkat VSS
            </p>
          </td>
        </tr>

        <!-- Alert banner -->
        <tr>
          <td style="background:#fef2f2;border-left:4px solid %s;padding:14px 24px;">
            <table cellpadding="0" cellspacing="0">
              <tr>
                <td style="font-size:18px;padding-right:10px;">⚠️</td>
                <td style="font-size:13px;color:#991b1b;line-height:1.5;">
                  <strong>%s</strong> pada device
                  <strong>%s</strong> (%s).
                  Harap segera dicek di VSS / lapangan.
                </td>
              </tr>
            </table>
          </td>
        </tr>

        <!-- Body -->
        <tr>
          <td style="padding:28px 32px;">
            <p style="font-size:14px;color:#374151;margin:0 0 20px;line-height:1.6;">
              Detail deteksi sebagai berikut. Gunakan informasi di bawah untuk investigasi.
            </p>

            <!-- Info card -->
            <table width="100%%" cellpadding="0" cellspacing="0" style="background:#f8fafc;border:1px solid #e5e7eb;border-radius:8px;overflow:hidden;margin-bottom:24px;">
              <tr>
                <td style="padding:16px 20px;border-bottom:1px solid #e5e7eb;">
                  <p style="margin:0;font-size:14px;font-weight:700;color:#111827;">Detail VSS Event</p>
                </td>
              </tr>
              <tr>
                <td style="padding:16px 20px;">
                  <table cellpadding="0" cellspacing="0" width="100%%">
                    <tr>
                      <td style="font-size:13px;color:#6b7280;padding:6px 0;width:140px;">Detected At</td>
                      <td style="font-size:13px;color:#111827;font-weight:600;padding:6px 0;">%s</td>
                    </tr>
                    <tr>
                      <td style="font-size:13px;color:#6b7280;padding:6px 0;">Device ID</td>
                      <td style="font-size:13px;color:#111827;font-weight:600;padding:6px 0;">%s</td>
                    </tr>
                    <tr>
                      <td style="font-size:13px;color:#6b7280;padding:6px 0;">Device Name</td>
                      <td style="font-size:13px;color:#111827;font-weight:600;padding:6px 0;">%s</td>
                    </tr>
                    <tr>
                      <td style="font-size:13px;color:#6b7280;padding:6px 0;">Reason</td>
                      <td style="font-size:13px;padding:6px 0;">
                        <span style="display:inline-block;background:%s;color:#fff;font-size:11px;font-weight:700;padding:3px 10px;border-radius:10px;">%s</span>
                      </td>
                    </tr>
                    <tr>
                      <td style="font-size:13px;color:#6b7280;padding:6px 0;">Delay</td>
                      <td style="font-size:13px;color:#111827;font-weight:600;padding:6px 0;">%d detik</td>
                    </tr>
                    <tr>
                      <td style="font-size:13px;color:#6b7280;padding:6px 0;">DTU</td>
                      <td style="font-size:13px;color:#111827;font-weight:600;padding:6px 0;">%s</td>
                    </tr>
                    <tr>
                      <td style="font-size:13px;color:#6b7280;padding:6px 0;">Report Time</td>
                      <td style="font-size:13px;color:#111827;font-weight:600;padding:6px 0;">%d</td>
                    </tr>
                    <tr>
                      <td style="font-size:13px;color:#6b7280;padding:6px 0;">Is Later</td>
                      <td style="font-size:13px;color:#111827;font-weight:600;padding:6px 0;">%v</td>
                    </tr>
                    <tr>
                      <td style="font-size:13px;color:#6b7280;padding:6px 0;vertical-align:top;">Message</td>
                      <td style="font-size:13px;color:#374151;padding:6px 0;line-height:1.5;">%s</td>
                    </tr>
                  </table>
                </td>
              </tr>
            </table>

            <!-- CTA -->
            <table cellpadding="0" cellspacing="0" style="width:100%%;">
              <tr>
                <td align="center">
                  <a href="%s" style="display:inline-block;padding:12px 32px;background:#2563eb;color:#fff;font-size:14px;font-weight:600;border-radius:8px;text-decoration:none;">
                    Buka VSS Monitor →
                  </a>
                </td>
              </tr>
            </table>
          </td>
        </tr>

        <!-- Footer -->
        <tr>
          <td style="padding:20px 32px;background:#f8fafc;border-top:1px solid #f1f5f9;">
            <table width="100%%" cellpadding="0" cellspacing="0">
              <tr>
                <td>
                  <p style="margin:0 0 4px;font-size:13px;font-weight:600;color:#374151;">Thank you,</p>
                  <p style="margin:0;font-size:13px;color:#6b7280;">%s</p>
                </td>
                <td align="right" style="vertical-align:bottom;">
                  <p style="margin:0;font-size:11px;color:#9ca3af;">
                    Notifikasi otomatis dari VSS Monitor<br>
                    %s
                  </p>
                </td>
              </tr>
            </table>
          </td>
        </tr>

        <!-- Bottom bar -->
        <tr>
          <td style="background:#1e3a5f;padding:12px 32px;text-align:center;">
            <p style="margin:0;font-size:11px;color:rgba(255,255,255,0.5);">
              © %d %s · Automated Notification System
            </p>
          </td>
        </tr>

      </table>
    </td>
  </tr>
</table>
</body>
</html>`,
		appName,
		reasonColor,
		reasonLabel, ev.DeviceName, ev.DeviceID,
		ev.DetectedAt,
		ev.DeviceID,
		ev.DeviceName,
		reasonColor, reasonLabel,
		ev.DelaySec,
		ev.DTU,
		ev.ReportTime,
		ev.IsLater,
		ev.Message,
		monitorURL,
		teamName,
		time.Now().Format("02 Jan 2006 15:04 WIB"),
		time.Now().Year(),
		appName,
	)

	var lastErr error
	for _, to := range recipients {
		to = strings.TrimSpace(to)
		if to == "" {
			continue
		}
		if err := SendEmail(to, subject, htmlBody); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func vssReasonLabel(reason string) string {
    switch strings.ToLower(reason) {
    case "delayed":
      return "Data Delayed (isLater)"
    case "stale_timestamp":
      return "Stale Timestamp"
    case "no_heartbeat":
      return "No Heartbeat"
    case "disconnect":
      return "WebSocket Disconnect"
    case "storage_full":
      return "Storage Full"
    case "high_cpu":
      return "High CPU"
    default:
      return reason
    }
}

func vssReasonColor(reason string) string {
    switch strings.ToLower(reason) {
    case "delayed", "stale_timestamp":
      return "#d97706"
    case "no_heartbeat", "disconnect":
      return "#dc2626"
    case "storage_full", "high_cpu":
      return "#7c3aed"
    default:
      return "#2563eb"
    }
}

func SendEmail(to, subject, htmlBody string) error {
	cfg := config.GetConfig()

	auth := smtp.PlainAuth(
		"",
		cfg.Email.SMTPUser,
		cfg.Email.SMTPPass,
		cfg.Email.SMTPHost,
	)

	msg := "From: " + cfg.Email.FromEmail + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/html; charset=UTF-8\r\n\r\n" +
		htmlBody

	err := smtp.SendMail(
		fmt.Sprintf("%s:%d", cfg.Email.SMTPHost, cfg.Email.SMTPPort),
		auth,
		cfg.Email.FromEmail,
		[]string{to},
		[]byte(msg),
	)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

func SendVSSDigestAlert(events []domain.VSSDelayEvent, recipients []string) error {
	cfg := config.GetConfig()
	if cfg.Email.SMTPHost == "" || len(events) == 0 || len(recipients) == 0 {
		return nil
	}

	uniq := map[string]struct{}{}
	for _, ev := range events {
		uniq[ev.DeviceID] = struct{}{}
	}

	subject := fmt.Sprintf("[VSS Alert] %d event / %d device (digest) — %s",
		len(events), len(uniq), time.Now().Format("02 Jan 2006 15:04"))

	var rows strings.Builder
	for _, ev := range events {
		reasonColor := vssReasonColor(ev.Reason)
		reasonLabel := vssReasonLabel(ev.Reason)
		name := ev.DeviceName
		if name == "" {
			name = "-"
		}
		rows.WriteString(fmt.Sprintf(`
			<tr>
				<td style="padding:10px 12px;border-bottom:1px solid #f1f5f9;font-size:12px;color:#374151;">%s</td>
				<td style="padding:10px 12px;border-bottom:1px solid #f1f5f9;font-size:12px;color:#111827;font-weight:600;">%s</td>
				<td style="padding:10px 12px;border-bottom:1px solid #f1f5f9;font-size:12px;color:#6b7280;">%s</td>
				<td style="padding:10px 12px;border-bottom:1px solid #f1f5f9;">
					<span style="display:inline-block;background:%s;color:#fff;font-size:10px;font-weight:700;padding:2px 8px;border-radius:10px;">%s</span>
				</td>
				<td style="padding:10px 12px;border-bottom:1px solid #f1f5f9;font-size:12px;color:#111827;font-weight:600;">%ds</td>
			</tr>`,
			ev.DetectedAt,
			name,
			ev.DeviceID,
			reasonColor,
			reasonLabel,
			ev.DelaySec,
		))
	}

	htmlBody := fmt.Sprintf(`<!DOCTYPE html>
<html lang="id">
<head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"></head>
<body style="margin:0;padding:0;background:#f3f4f6;font-family:'Segoe UI',Arial,sans-serif;">
<table width="100%%" cellpadding="0" cellspacing="0" style="background:#f3f4f6;padding:32px 0;">
  <tr><td align="center">
    <table width="640" cellpadding="0" cellspacing="0" style="max-width:640px;width:100%%;background:#fff;border-radius:12px;overflow:hidden;box-shadow:0 4px 20px rgba(0,0,0,0.08);">
      <tr>
        <td style="background:linear-gradient(135deg,#1e3a5f 0%%,#2563eb 100%%);padding:28px 32px;text-align:center;">
          <div style="display:inline-block;background:rgba(255,255,255,0.15);border-radius:10px;padding:6px 18px;margin-bottom:12px;">
            <span style="color:#fff;font-size:13px;font-weight:700;">%s</span>
          </div>
          <h1 style="color:#fff;margin:0;font-size:19px;font-weight:700;">VSS Alert Digest</h1>
          <p style="color:rgba(255,255,255,0.75);margin:8px 0 0;font-size:13px;">
            %d event dari %d device dalam jendela digest
          </p>
        </td>
      </tr>
      <tr>
        <td style="background:#fef2f2;border-left:4px solid #dc2626;padding:14px 24px;">
          <p style="margin:0;font-size:13px;color:#991b1b;line-height:1.5;">
            Ringkasan otomatis. Detail lengkap ada di History VSS (database).
          </p>
        </td>
      </tr>
      <tr>
        <td style="padding:24px 28px;">
          <table width="100%%" cellpadding="0" cellspacing="0" style="border-collapse:collapse;border:1px solid #e5e7eb;border-radius:8px;overflow:hidden;">
            <thead>
              <tr style="background:#f8fafc;">
                <th style="padding:10px 12px;text-align:left;font-size:11px;color:#6b7280;text-transform:uppercase;">Waktu</th>
                <th style="padding:10px 12px;text-align:left;font-size:11px;color:#6b7280;text-transform:uppercase;">Name</th>
                <th style="padding:10px 12px;text-align:left;font-size:11px;color:#6b7280;text-transform:uppercase;">Device ID</th>
                <th style="padding:10px 12px;text-align:left;font-size:11px;color:#6b7280;text-transform:uppercase;">Reason</th>
                <th style="padding:10px 12px;text-align:left;font-size:11px;color:#6b7280;text-transform:uppercase;">Delay</th>
              </tr>
            </thead>
            <tbody>%s</tbody>
          </table>
          <table cellpadding="0" cellspacing="0" style="margin-top:24px;width:100%%;">
            <tr><td align="center">
              <a href="%s/vss/history" style="display:inline-block;padding:12px 28px;background:#2563eb;color:#fff;font-size:14px;font-weight:600;border-radius:8px;text-decoration:none;">Buka VSS History →</a>
            </td></tr>
          </table>
        </td>
      </tr>
      <tr>
        <td style="padding:20px 28px;background:#f8fafc;border-top:1px solid #f1f5f9;">
          <p style="margin:0 0 4px;font-size:13px;font-weight:600;color:#374151;">Thank you,</p>
          <p style="margin:0;font-size:13px;color:#6b7280;">%s</p>
        </td>
      </tr>
      <tr>
        <td style="background:#1e3a5f;padding:12px 28px;text-align:center;">
          <p style="margin:0;font-size:11px;color:rgba(255,255,255,0.5);">© %d %s · Automated Notification System</p>
        </td>
      </tr>
    </table>
  </td></tr>
</table>
</body>
</html>`,
		appName,
		len(events),
		len(uniq),
		rows.String(),
		appBaseURL,
		teamName,
		time.Now().Year(),
		appName,
	)

	var lastErr error
	for _, to := range recipients {
		to = strings.TrimSpace(to)
		if to == "" {
			continue
		}
		if err := SendEmail(to, subject, htmlBody); err != nil {
			lastErr = err
		}
	}
	return lastErr
}