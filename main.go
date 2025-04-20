package main

import (
	"fmt"
	"image/color"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

const logFile = "work_log.txt"

var isRunning bool
var currentSessionStart time.Time

func main() {
	myApp := app.New()
	myWindow := myApp.NewWindow("gtimelog lite")

	summaryLabel := widget.NewLabel("")
	weeklySummaryLabel := widget.NewLabel("")
	stateLabel := widget.NewLabel("")

	var startBtn *widget.Button
	var stopBtn *widget.Button

	startBtn = widget.NewButton("Start", func() {
		logTime("START")
		isRunning = true
		currentSessionStart = time.Now()
		updateUI(summaryLabel, weeklySummaryLabel, stateLabel, startBtn, stopBtn)
	})
	stopBtn = widget.NewButton("Stop", func() {
		logTime("STOP")
		isRunning = false
		currentSessionStart = time.Time{}
		updateUI(summaryLabel, weeklySummaryLabel, stateLabel, startBtn, stopBtn)
	})

	startBtnContainer := styledButtonContainer(startBtn, color.RGBA{R: 144, G: 238, B: 144, A: 255})
	stopBtnContainer := styledButtonContainer(stopBtn, color.RGBA{R: 255, G: 182, B: 193, A: 255})

	showLogBtn := widget.NewButton("Show Log", func() {
		err := openLogFile(logFile)
		if err != nil {
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
		for range time.Tick(time.Second) {
			updateUI(summaryLabel, weeklySummaryLabel, stateLabel, startBtn, stopBtn)
		}
	}()

	updateUI(summaryLabel, weeklySummaryLabel, stateLabel, startBtn, stopBtn)
	myWindow.Resize(fyne.NewSize(350, 300))
	myWindow.ShowAndRun()
}

func styledButtonContainer(btn *widget.Button, bgColor color.Color) *fyne.Container {
	bg := canvas.NewRectangle(bgColor)
	bg.SetMinSize(fyne.NewSize(80, 40))
	return container.NewMax(bg, btn)
}

func logTime(action string) {
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer f.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	entry := fmt.Sprintf("%s %s\n", timestamp, action)
	_, _ = f.WriteString(entry)
}

func getWeeklySummary() string {
	now := time.Now()
	offset := int(now.Weekday())
	if offset == 0 {
		offset = 7
	}
	weekStart := now.AddDate(0, 0, -offset+1).Truncate(24 * time.Hour)
	mins := calculateWorkTime(func(t time.Time) bool {
		return t.After(weekStart)
	})

	if isRunning && !currentSessionStart.IsZero() && currentSessionStart.After(weekStart) {
		mins += int(time.Since(currentSessionStart).Minutes())
	}

	perDay := mins / 5
	return fmt.Sprintf("This Week's Work: %s (%s/day)", formatDuration(mins), formatDuration(perDay))
}

func getState() string {
	if isRunning {
		return "Status: Running"
	}
	return "Status: Stopped"
}

func updateUI(summaryLabel, weeklySummaryLabel, stateLabel *widget.Label, startBtn, stopBtn *widget.Button) {
	mins := calculateWorkTime(func(t time.Time) bool {
		return t.After(time.Now().Truncate(24 * time.Hour))
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
		if len(line) < 19 {
			continue
		}
		parts := strings.SplitN(line, " ", 3)
		if len(parts) < 3 {
			continue
		}

		timeStr, action := parts[0]+" "+parts[1], parts[2]
		logTime, err := time.Parse("2006-01-02 15:04:05", timeStr)
		if err != nil || !filterFunc(logTime) {
			continue
		}

		if action == "START" {
			startTime = logTime
		} else if action == "STOP" && !startTime.IsZero() {
			totalTime += int(logTime.Sub(startTime).Seconds())
			startTime = time.Time{}
		}
	}
	return totalTime / 60
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
