package ui

import (
	"fmt"
	"time"
)

func PrintHUD(title string, paused, shuffle, loop bool, volume float64, pos, tot time.Duration) {
	status := "PLAY"
	if paused {
		status = "PAUSE"
	}

	shufStr := "OFF"
	if shuffle {
		shufStr = "ON"
	}

	loopStr := "OFF"
	if loop {
		loopStr = "ON"
	}

	minP, secP := int(pos.Minutes()), int(pos.Seconds())%60
	minT, secT := int(tot.Minutes()), int(tot.Seconds())%60

	fmt.Printf("\r\x1b[K[%s] %s | %02d:%02d / %02d:%02d | Vol: %d%% | Shuffle: %s | Loop: %s", 
		status, title, minP, secP, minT, secT, int(volume*100), shufStr, loopStr)
}