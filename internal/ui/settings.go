package ui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/pjanzen/openproject-tracker/internal/config"
)

// showSettings opens a settings window for the application.
func showSettings(a fyne.App, cfg *config.Config, onSave func(*config.Config)) {
	w := a.NewWindow("Settings")
	w.Resize(fyne.NewSize(480, 360))

	baseURLEntry := widget.NewEntry()
	baseURLEntry.SetText(cfg.BaseURL)

	skipTLSCheck := widget.NewCheck("Skip TLS Verification (insecure)", nil)
	skipTLSCheck.SetChecked(cfg.SkipTLSVerify)
	tlsWarning := widget.NewLabel("⚠ Disabling TLS verification is insecure.")

	caPathEntry := widget.NewEntry()
	caPathEntry.SetPlaceHolder("/path/to/ca-cert.pem")
	caPathEntry.SetText(cfg.CAPath)

	// Extra headers as "Key: Value" lines.
	headersEntry := widget.NewMultiLineEntry()
	headersEntry.SetPlaceHolder("Header-Name: value\nAnother-Header: value")
	headersEntry.SetMinRowsVisible(4)
	var headerLines []string
	for k, v := range cfg.ExtraHeaders {
		headerLines = append(headerLines, k+": "+v)
	}
	headersEntry.SetText(strings.Join(headerLines, "\n"))

	saveBtn := widget.NewButton("Save", func() {
		updated := &config.Config{
			BaseURL:       baseURLEntry.Text,
			SkipTLSVerify: skipTLSCheck.Checked,
			CAPath:        caPathEntry.Text,
			ExtraHeaders:  parseHeaders(headersEntry.Text),
		}
		_ = updated.Save()
		onSave(updated)
		w.Close()
	})

	form := container.NewVBox(
		widget.NewLabel("Base URL:"),
		baseURLEntry,
		skipTLSCheck,
		tlsWarning,
		widget.NewLabel("CA Certificate Path:"),
		caPathEntry,
		widget.NewLabel("Extra Headers (one per line, Key: Value):"),
		headersEntry,
		saveBtn,
	)
	w.SetContent(container.NewPadded(form))
	w.Show()
}

// parseHeaders parses "Key: Value" lines into a map.
func parseHeaders(text string) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		k := strings.TrimSpace(line[:idx])
		v := strings.TrimSpace(line[idx+1:])
		if k != "" {
			result[k] = v
		}
	}
	return result
}
