package utils

import (
	"errors"
	"fmt"
	"github.com/Romain-GUILLEMOT/WhispyrBack/config"
	"github.com/disposable/disposable"
	"gopkg.in/gomail.v2"
	"strconv"
	"strings"
	"time"
)

func InitMailer() {
	cfg := config.GetConfig()
	num, err := strconv.Atoi(cfg.SmtpPort)
	if err != nil {
		Fatal("Invalid SMTP port", "err", err)
		return
	}
	d := gomail.NewDialer(cfg.SmtpHost, num, cfg.SmtpUser, cfg.SmtpPass)
	s, err := d.Dial()
	if err != nil {
		Fatal("Mailer unreachable", "err", err)
		return
	}
	_ = s.Close()
	Success("Mailer connection OK")
}

func SendMail(to string, subject string, content string) error {
	cfg := config.GetConfig()
	num, err := strconv.Atoi(cfg.SmtpPort)
	if err != nil {
		return err
	}
	m := gomail.NewMessage()
	m.SetHeader("From", cfg.SmtpUser)
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", content)
	m.SetHeader("Date", time.Now().Format(time.RFC1123Z))
	m.SetHeader("Message-ID", fmt.Sprintf("<%d@%s>", time.Now().UnixNano(), "romain-guillemot.dev"))

	d := gomail.NewDialer(cfg.SmtpHost, num, cfg.SmtpUser, cfg.SmtpPass)

	if err := d.DialAndSend(m); err != nil {
		Error("Failed to send htmlemail", "err", err)
		return err
	}

	Info("📧 Email sent", "to", to, "subject", subject)
	return nil
}
func GetEmailDomain(email string) error {
	if strings.Contains(email, "+") {
		return errors.New("Les adresses email avec alias (symbole '+') ne sont pas autorisées.")
	}

	atIndex := strings.LastIndex(email, "@")
	if atIndex == -1 || atIndex == len(email)-1 {
		return errors.New("L'adresse email est invalide : caractère '@' manquant ou mal placé.")
	}

	domain := strings.ToLower(email[atIndex+1:])
	if disposable.Domain(domain) {
		return errors.New("Les adresses email jetables ne sont pas autorisées.")
	}
	return nil
}
