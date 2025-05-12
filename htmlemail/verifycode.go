package htmlemail

import (
	"bytes"
	"html/template"
)

func Verifcode(code string) (string, error) {
	tmpl, err := template.New("email").Parse(`
		<!DOCTYPE html>
		<html>
			<body style="font-family: sans-serif; background-color: #00C896; padding: 20px;">
				<div style="max-width: 500px; margin: auto; background: white; padding: 20px; border-radius: 8px;">
					<h2 style="color: #10b981;">👋 Bienvenue sur Whyspir !</h2>
					<p>Voici ton code de vérification :</p>
					<h1 style="text-align: center; color: #333;">{{.Code}}</h1>
					<p style="color: #777;">Ce code est valable 5 minutes.</p>
				</div>
			</body>
		</html>
	`)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, struct{ Code string }{Code: code})
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
