package utils

import (
	"fmt"
	"github.com/Romain-GUILLEMOT/WhispyrBack/config"
	"runtime/debug"
	"time"
)

func SendErrorMail(code, file, content, extra string) {
	cfg := config.GetConfig()

	full := fmt.Sprintf(
		"Code erreur : %s\n\nFile: %s\n\nDétails :\n%s\n\nInfos supplémentaires :\n%s",
		code, file, content, extra,
	)

	_ = SendMail(cfg.ErrorReportEmail, "🚨 Erreur ["+code+"]", full)
}
func HandlePanic() {
	if r := recover(); r != nil {
		code := fmt.Sprintf("777-%d", time.Now().Unix()%1000)
		stack := string(debug.Stack())
		SendErrorMail(code, "global", fmt.Sprintf("%v\n\nStacktrace:\n%s", r, stack), "")
		Fatal("Application crashed", "code", code, "reason", r)
	}
}
func ReportError(err error, file, shortCode string, extra string) {
	if err == nil {
		return
	}
	code := fmt.Sprintf("888-%s", shortCode)
	SendErrorMail(code, file, err.Error(), extra)
	Error("Handled error", "code", code, "err", err)
}
