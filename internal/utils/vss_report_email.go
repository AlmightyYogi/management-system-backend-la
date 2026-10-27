package utils

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"mime/multipart"
	"net"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"

	"github.com/AlmightyOggy/management-system/internal/config"
)

func SendVSSReportEmail(to []string, subject, htmlBody string, attachments map[string][]byte) error {
	cfg := config.GetConfig()
	r := cfg.VSSReportEmail

	if !r.Enabled {
		return fmt.Errorf("VSS report email disabled")
	}
	if r.SMTPHost == "" || r.FromEmail == "" {
		return fmt.Errorf("VSS report SMTP not configured")
	}
	if len(to) == 0 {
		return fmt.Errorf("no recipients")
	}

	from := r.FromEmail
	fromHeader := from
	if r.FromName != "" {
		fromHeader = fmt.Sprintf("%s <%s>", r.FromName, from)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	htmlHeader := textproto.MIMEHeader{}
	htmlHeader.Set("Content-Type", "text/html; charset=UTF-8")
	htmlHeader.Set("Content-Transfer-Encoding", "8bit")
	part, err := writer.CreatePart(htmlHeader)
	if err != nil {
		return err
	}
	if _, err := part.Write([]byte(htmlBody)); err != nil {
		return err
	}

	for name, data := range attachments {
		if name == "" || len(data) == 0 {
			continue
		}
		ah := textproto.MIMEHeader{}
		ah.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		ah.Set("Content-Transfer-Encoding", "base64")
		ah.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
		ap, err := writer.CreatePart(ah)
		if err != nil {
			return err
		}
		encoded := make([]byte, base64.StdEncoding.EncodedLen(len(data)))
		base64.StdEncoding.Encode(encoded, data)
		for i := 0; i < len(encoded); i += 76 {
			end := i + 76
			if end > len(encoded) {
				end = len(encoded)
			}
			if _, err := ap.Write(encoded[i:end]); err != nil {
				return err
			}
			if _, err := ap.Write([]byte("\r\n")); err != nil {
				return err
			}
		}
	}

	if err := writer.Close(); err != nil {
		return err
	}

	boundary := writer.Boundary()
	var msg bytes.Buffer
	msg.WriteString(fmt.Sprintf("From: %s\r\n", fromHeader))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(to, ", ")))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=%s\r\n", boundary))
	msg.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))
	msg.WriteString("\r\n")
	msg.Write(body.Bytes())

	addr := fmt.Sprintf("%s:%d", r.SMTPHost, r.SMTPPort)
	if r.SMTPPort <= 0 {
		addr = net.JoinHostPort(r.SMTPHost, "25")
	}

	c, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	defer c.Close()

	if err := c.Hello("xpjdighbbapp01"); err != nil {
		_ = c.Hello("localhost")
	}

	if r.SMTPSecurity == "starttls" {
		if ok, _ := c.Extension("STARTTLS"); ok {
			// butuh import crypto/tls jika dipakai
			// untuk Indosat port 25 plain: biarkan none
		}
	}

	if err := c.Mail(from); err != nil {
		return fmt.Errorf("mail from: %w", err)
	}
	for _, rcpt := range to {
		rcpt = strings.TrimSpace(rcpt)
		if rcpt == "" {
			continue
		}
		if err := c.Rcpt(rcpt); err != nil {
			return fmt.Errorf("rcpt %s: %w", rcpt, err)
		}
	}

	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("data: %w", err)
	}
	if _, err := w.Write(msg.Bytes()); err != nil {
		_ = w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

func ParseEmailList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}