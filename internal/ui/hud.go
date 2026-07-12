package ui

import (
	"fmt"
	"os"
	"time"
	"golang.org/x/term"
)

func formatDuration(d time.Duration) string {
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d", m, s)
}

func PrintHUD(title string, paused bool, volume float64, pos, tot time.Duration){
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		width = 80
	}

	statusIcon := "▶️"
	statusText := "Reproduciendo"
	if paused {
		statusIcon = "⏸️"
		statusText = "Pausado"
	}
	volTxt := fmt.Sprintf("%d%%", int (volume*100))
	posTxt := formatDuration(pos)
	totTxt := formatDuration(tot) 
	fixedLength := 2 + 2 + len(statusText) + 2 + len(posTxt) + 3 + len(totTxt) + 8 + len(volTxt) + 1
	maxTitleLength := width - fixedLength - 3

	if maxTitleLength > 5 && len(title) > maxTitleLength {
		title = title[:maxTitleLength] + "..."
	}
	line := fmt.Sprintf("%s %s: %s [%s / %s] [Vol: %s]", statusIcon, statusText, title, posTxt, totTxt, volTxt)

	if len(line) >= width {
		line = line[:width-1]
	}
	fmt.Printf("\r\x1b[K%s", line)
}