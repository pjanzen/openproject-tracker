package ui

import (
	"fmt"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/pjanzen/openproject-tracker/internal/auth"
	"github.com/pjanzen/openproject-tracker/internal/client"
	"github.com/pjanzen/openproject-tracker/internal/config"
	"github.com/pjanzen/openproject-tracker/internal/storage"
)

// Run initialises and starts the Fyne application.
func Run(cfg *config.Config, jar *auth.PersistentJar) {
	a := app.NewWithID("com.github.pjanzen.openproject-tracker")
	w := a.NewWindow("OpenProject Time Tracker")
	w.Resize(fyne.NewSize(520, 480))

	tracker := &appState{
		cfg:    cfg,
		jar:    jar,
		client: client.NewClient(cfg, jar),
		window: w,
	}
	tracker.build(a)
	w.ShowAndRun()
}

type appState struct {
	cfg    *config.Config
	jar    *auth.PersistentJar
	client *client.Client
	window fyne.Window

	workPackages []client.WorkPackage
	activities   []client.Activity

	selectedWP       *client.WorkPackage
	selectedActivity *client.Activity

	running   bool
	startTime time.Time
	stopCh    chan struct{}

	// UI elements that need updating
	statusLabel  *widget.Label
	timerLabel   *widget.Label
	startBtn     *widget.Button
	stopBtn      *widget.Button
	wpSelect     *widget.Select
	actSelect    *widget.Select
	commentEntry *widget.Entry
	loginStatus  *widget.Label
}

func (s *appState) build(a fyne.App) {
	s.loginStatus = widget.NewLabel("Status: Not logged in")
	s.statusLabel = widget.NewLabel("Status: Ready")
	s.timerLabel = widget.NewLabel("00:00:00")
	s.commentEntry = widget.NewMultiLineEntry()
	s.commentEntry.SetPlaceHolder("Optional comment...")
	s.commentEntry.SetMinRowsVisible(3)

	loginBtn := widget.NewButton("Login via SSO", s.onLogin)

	s.wpSelect = widget.NewSelect([]string{"--- Select task ---"}, func(val string) {
		s.onWorkPackageSelected(val)
	})
	s.wpSelect.PlaceHolder = "--- Select task ---"

	refreshBtn := widget.NewButton("Refresh", s.onRefresh)

	s.actSelect = widget.NewSelect([]string{"--- Select activity ---"}, func(val string) {
		s.onActivitySelected(val)
	})
	s.actSelect.PlaceHolder = "--- Select activity ---"

	s.startBtn = widget.NewButton("Start", s.onStart)
	s.stopBtn = widget.NewButton("Stop", s.onStop)
	s.stopBtn.Disable()

	settingsBtn := widget.NewButton("Settings", func() {
		showSettings(a, s.cfg, func(updated *config.Config) {
			s.cfg = updated
			s.client = client.NewClient(updated, s.jar)
		})
	})

	loginRow := container.NewHBox(loginBtn, s.loginStatus)
	wpRow := container.NewBorder(nil, nil, nil, refreshBtn, s.wpSelect)
	timerRow := container.NewHBox(s.timerLabel)
	ctrlRow := container.NewHBox(s.startBtn, s.stopBtn)

	content := container.NewVBox(
		loginRow,
		widget.NewSeparator(),
		container.NewHBox(widget.NewLabel("Task:"), wpRow),
		container.NewHBox(widget.NewLabel("Activity:"), s.actSelect),
		widget.NewSeparator(),
		timerRow,
		ctrlRow,
		widget.NewSeparator(),
		widget.NewLabel("Comment:"),
		s.commentEntry,
		widget.NewSeparator(),
		s.statusLabel,
		settingsBtn,
	)

	s.window.SetContent(container.NewPadded(content))

	// Attempt to verify existing session on startup.
	go func() {
		if err := s.client.CheckAuth(); err == nil {
			s.loginStatus.SetText("Status: Logged in")
			s.onRefresh()
		}
	}()
}

func (s *appState) onLogin() {
	s.loginStatus.SetText("Status: Opening browser...")
	go func() {
		if err := auth.Login(s.cfg, s.jar); err != nil {
			s.loginStatus.SetText("Status: Login failed")
			s.setStatus("Login error: " + err.Error())
			return
		}
		s.loginStatus.SetText("Status: Logged in")
		s.setStatus("Login successful")
		s.onRefresh()
	}()
}

func (s *appState) onRefresh() {
	s.setStatus("Loading work packages...")
	go func() {
		wps, err := s.client.GetMyWorkPackages()
		if err != nil {
			s.setStatus("Error loading work packages: " + err.Error())
			return
		}
		acts, err := s.client.GetActivities()
		if err != nil {
			s.setStatus("Error loading activities: " + err.Error())
			return
		}
		s.workPackages = wps
		s.activities = acts

		wpLabels := make([]string, len(wps))
		for i, wp := range wps {
			wpLabels[i] = fmt.Sprintf("[%d] %s (%s)", wp.ID, wp.Subject, wp.Project)
		}
		actLabels := make([]string, len(acts))
		for i, a := range acts {
			actLabels[i] = a.Name
		}

		s.wpSelect.Options = wpLabels
		s.wpSelect.Refresh()
		s.actSelect.Options = actLabels
		s.actSelect.Refresh()
		s.setStatus(fmt.Sprintf("Loaded %d tasks, %d activities", len(wps), len(acts)))
	}()
}

func (s *appState) onWorkPackageSelected(val string) {
	for i, wp := range s.workPackages {
		label := fmt.Sprintf("[%d] %s (%s)", wp.ID, wp.Subject, wp.Project)
		if label == val {
			s.selectedWP = &s.workPackages[i]
			return
		}
	}
	s.selectedWP = nil
}

func (s *appState) onActivitySelected(val string) {
	for i, a := range s.activities {
		if a.Name == val {
			s.selectedActivity = &s.activities[i]
			return
		}
	}
	s.selectedActivity = nil
}

func (s *appState) onStart() {
	if s.selectedWP == nil {
		s.setStatus("Please select a task first")
		return
	}
	if s.selectedActivity == nil {
		s.setStatus("Please select an activity first")
		return
	}
	s.running = true
	s.startTime = time.Now()
	s.startBtn.Disable()
	s.stopBtn.Enable()
	s.setStatus("Timer running...")

	s.stopCh = make(chan struct{})
	go s.runTimer(s.stopCh)
}

func (s *appState) onStop() {
	if !s.running {
		return
	}
	close(s.stopCh)
	s.running = false
	s.startBtn.Enable()
	s.stopBtn.Disable()

	elapsed := time.Since(s.startTime)
	hours := elapsed.Hours()

	spentOn := s.startTime.Format("2006-01-02")
	te := client.TimeEntry{
		WorkPackageID: s.selectedWP.ID,
		ActivityID:    s.selectedActivity.ID,
		Hours:         hours,
		Comment:       s.commentEntry.Text,
		SpentOn:       spentOn,
	}

	s.setStatus("Saving time entry...")
	go func() {
		if err := s.client.CreateTimeEntry(te); err != nil {
			s.setStatus("Error saving time entry: " + err.Error())
			dialog.ShowError(err, s.window)
			return
		}
		dir, _ := storage.ConfigDir()
		_ = s.jar.Save(filepath.Join(dir, "cookies.json"))
		s.setStatus(fmt.Sprintf("Saved %.2f hours for %s", hours, s.selectedWP.Subject))
		s.commentEntry.SetText("")
		s.timerLabel.SetText("00:00:00")
	}()
}

func (s *appState) runTimer(stop <-chan struct{}) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			elapsed := time.Since(s.startTime)
			h := int(elapsed.Hours())
			m := int(elapsed.Minutes()) % 60
			sec := int(elapsed.Seconds()) % 60
			s.timerLabel.SetText(fmt.Sprintf("%02d:%02d:%02d", h, m, sec))
		}
	}
}

func (s *appState) setStatus(msg string) {
	s.statusLabel.SetText("Status: " + msg)
}
