package main

import (
	"fmt"
	"image/color"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// === Configurable Constants ===
const (
	logFile          = "work_log.txt"
	startAction      = 1
	stopAction       = 0
	logMinuteDivisor = 60
	uiTickInterval   = 1 * time.Second
	defaultWidth     = 350
	defaultHeight    = 300
	logDateFormat    = "20060102T150405"

	appVersion      = "v1.1.1"
	appName         = "mtimelog Lite"
	workdaysPerWeek = 5 // <-- You can change this to 6 or 7 as needed
)

var isRunning bool
var currentSessionStart time.Time

func main() {
	myApp := app.New()
	myWindow := myApp.NewWindow(fmt.Sprintf("%s %s", appName, appVersion))

	summaryLabel := widget.NewLabel("")
	weeklySummaryLabel := widget.NewLabel("")
	stateLabel := widget.NewLabel("")

	var startBtn *widget.Button
	var stopBtn *widget.Button

	startBtn = widget.NewButton("Start", func() {
		logTime(startAction)
		isRunning = true
		currentSessionStart = time.Now()
		updateUI(summaryLabel, weeklySummaryLabel, stateLabel, startBtn, stopBtn)
	})
	stopBtn = widget.NewButton("Stop", func() {
		logTime(stopAction)
		isRunning = false
		currentSessionStart = time.Time{}
		updateUI(summaryLabel, weeklySummaryLabel, stateLabel, startBtn, stopBtn)
	})

	startBtnContainer := styledButtonContainer(startBtn, color.RGBA{R: 144, G: 238, B: 144, A: 255})
	stopBtnContainer := styledButtonContainer(stopBtn, color.RGBA{R: 255, G: 182, B: 193, A: 255})

	showLogBtn := widget.NewButton("Show Log", func() {
		if err := openLogFile(logFile); err != nil {
			fmt.Println("Failed to open log file:", err)
		}
	})

	buttonContainer := container.NewVBox(
		startBtnContainer,
		stopBtnContainer,
		showLogBtn,
		summaryLabel,
		weeklySummaryLabel,
		stateLabel,
	)

	myWindow.SetContent(buttonContainer)

	go func() {
		for range time.Tick(uiTickInterval) {
			updateUI(summaryLabel, weeklySummaryLabel, stateLabel, startBtn, stopBtn)
		}
	}()

	updateUI(summaryLabel, weeklySummaryLabel, stateLabel, startBtn, stopBtn)
	myWindow.Resize(fyne.NewSize(defaultWidth, defaultHeight))
	myWindow.ShowAndRun()
}

func styledButtonContainer(btn *widget.Button, bgColor color.Color) *fyne.Container {
	bg := canvas.NewRectangle(bgColor)
	bg.SetMinSize(fyne.NewSize(80, 40))
	return container.NewMax(bg, btn)
}

func logTime(action int) {
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer f.Close()

	timestamp := time.Now().Format(logDateFormat)
	entry := fmt.Sprintf("%s,%d\n", timestamp, action)
	_, _ = f.WriteString(entry)
}

func getWeeklySummary() string {
	now := time.Now()
	offset := int(now.Weekday())
	if offset == 0 {
		offset = 7
	}
	weekStart := time.Date(now.Year(), now.Month(), now.Day()-offset+1, 0, 0, 0, 0, now.Location())

	mins := calculateWorkTime(func(t time.Time) bool {
		return t.After(weekStart)
	})

	if isRunning && !currentSessionStart.IsZero() && currentSessionStart.After(weekStart) {
		mins += int(time.Since(currentSessionStart).Minutes())
	}

	elapsedWorkDays := getElapsedWorkdaysThisWeek(now)
	if elapsedWorkDays == 0 {
		elapsedWorkDays = 1 // Avoid divide by 0
	}
	perDay := mins / elapsedWorkDays

	return fmt.Sprintf("This Week's Work: %s (%s/day)", formatDuration(mins), formatDuration(perDay))
}

func getElapsedWorkdaysThisWeek(now time.Time) int {
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7 // Treat Sunday as 7
	}
	if weekday > workdaysPerWeek {
		return workdaysPerWeek
	}
	return weekday
}

func getState() string {
	if isRunning {
		return "Status: Running"
	}
	return "Status: Stopped"
}

func updateUI(summaryLabel, weeklySummaryLabel, stateLabel *widget.Label, startBtn, stopBtn *widget.Button) {
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	mins := calculateWorkTime(func(t time.Time) bool {
		return t.After(todayStart)
	})

	if isRunning && !currentSessionStart.IsZero() {
		mins += int(time.Since(currentSessionStart).Minutes())
	}

	summaryLabel.SetText("Today's Work: " + formatDuration(mins))
	weeklySummaryLabel.SetText(getWeeklySummary())
	stateLabel.SetText(getState())

	if isRunning {
		startBtn.SetText("Start ✓")
		stopBtn.SetText("Stop")
	} else {
		startBtn.SetText("Start")
		stopBtn.SetText("Stop ✓")
	}
}

func calculateWorkTime(filterFunc func(time.Time) bool) int {
	data, err := os.ReadFile(logFile)
	if err != nil {
		return 0
	}

	lines := strings.Split(string(data), "\n")
	totalTime := 0
	var startTime time.Time

	for _, line := range lines {
		if len(line) < 15 {
			continue
		}
		parts := strings.SplitN(line, ",", 2)
		if len(parts) < 2 {
			continue
		}

		logTime, err := time.Parse(logDateFormat, parts[0])
		if err != nil || !filterFunc(logTime) {
			continue
		}

		actionCode, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			continue
		}

		switch actionCode {
		case startAction:
			startTime = logTime
		case stopAction:
			if !startTime.IsZero() {
				totalTime += int(logTime.Sub(startTime).Seconds())
				startTime = time.Time{}
			}
		}
	}
	return totalTime / logMinuteDivisor
}

func formatDuration(minutes int) string {
	if minutes >= 60 {
		h := minutes / 60
		m := minutes % 60
		return fmt.Sprintf("%d h %02d", h, m)
	}
	return fmt.Sprintf("%d mins", minutes)
}

func openLogFile(filePath string) error {
	var cmd *exec.Cmd

	switch {
	case isWindows():
		cmd = exec.Command("cmd", "/c", "start", "", filePath)
	case isMac():
		cmd = exec.Command("open", filePath)
	case isLinux():
		cmd = exec.Command("xdg-open", filePath)
	default:
		return fmt.Errorf("unsupported platform")
	}

	return cmd.Start()
}

func isWindows() bool {
	return strings.Contains(strings.ToLower(os.Getenv("OS")), "windows") ||
		strings.Contains(strings.ToLower(runtime.GOOS), "windows")
}

func isMac() bool {
	return runtime.GOOS == "darwin"
}

func isLinux() bool {
	return runtime.GOOS == "linux"
}
