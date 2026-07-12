package main

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/ValenRomero24/reproductor-mp3/internal/audio"
	"github.com/ValenRomero24/reproductor-mp3/internal/domain"
	"github.com/ValenRomero24/reproductor-mp3/internal/ui"
	"golang.org/x/term"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Uso: reproductor-mp3 <ruta-directorio-musica>")
		return
	}	
	dirPath := os.Args[1]

	tracks, err := audio.ScanDirectory(dirPath)
	if err != nil { log.Fatalf("Error: %v", err) }
	if len(tracks) == 0 { fmt.Println("No se encontraron canciones."); return }

	manager := domain.NewPlaylistManager(tracks)
	engine, err := audio.NewOtoEngine()
	if err != nil { log.Fatalf("Error hardware: %v", err) }

	currentTrack, _ := manager.CurrentTrack()
	_ = engine.Play(currentTrack.Path)

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil { log.Fatalf("Error raw terminal: %v", err) }
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	fmt.Print("\r\n=== CONTROLES EN TIEMPO REAL ===\r\n")
	fmt.Print("[Espacio]: Pausa | [n]: Sig | [p]: Ant | [s]: Shuffle | [l]: Loop | [+ / -]: Vol | [q]: Salir\r\n")
	fmt.Print("================================\r\n\r\n")

	uiTicker := time.NewTicker(200 * time.Millisecond)
	defer uiTicker.Stop()

	pos, tot := engine.GetProgress()
	shuf, lp := manager.Status()
	ui.PrintHUD(currentTrack.Title, false, shuf, lp, engine.Volume(), pos, tot)

	keyChan := make(chan byte)
	go func() {
		buf := make([]byte, 1)
		for {
			_, err := os.Stdin.Read(buf)
			if err != nil { return }
			keyChan <- buf[0] 
		}
	}()

	paused := false
	running := true 
	transitioning := false 

	for running {
		select {
		case <-uiTicker.C:
			if !paused {
				pos, tot := engine.GetProgress()
				shuf, lp := manager.Status()
				ui.PrintHUD(currentTrack.Title, paused, shuf, lp, engine.Volume(), pos, tot)

				if tot > 0 {
					tiempoRestante := tot - pos
					if tiempoRestante <= 5*time.Second && !transitioning {
						transitioning = true 
						
						// Intentamos pasar al siguiente tema respetando las reglas de negocio
						if manager.Next() {
							currentTrack, _ = manager.CurrentTrack()
							_ = engine.CrossFadeTo(currentTrack.Path)
						} else {
							// Si dio false, se acabó el disco y el loop está OFF. 
							// Dejamos que terminen los últimos 5 segundos del tema actual en paz.
						}
					} else if tiempoRestante > 5*time.Second {
						transitioning = false 
					}
				}
			}

		case char := <-keyChan:
			if char == 'q' || char == 'Q' || char == 3 { running = false; break }
			switch char {
			case ' ':
				if paused { engine.Resume(); paused = false } else { engine.Pause(); paused = true }
			case 'n', 'N':
				if manager.Next() {
					currentTrack, _ = manager.CurrentTrack()
					_ = engine.Play(currentTrack.Path)
					paused = false
				}
			case 'p', 'P':
				manager.Prev()
				currentTrack, _ = manager.CurrentTrack()
				_ = engine.Play(currentTrack.Path)
				paused = false
			case 's', 'S':
				manager.ToggleShuffle()
			case 'l', 'L':
				manager.ToggleLoop()
			case '+', '=':
				engine.SetVolume(engine.Volume() + 0.05)
			case '-':
				engine.SetVolume(engine.Volume() - 0.05)
			}
			pos, tot := engine.GetProgress()
			shuf, lp := manager.Status()
			ui.PrintHUD(currentTrack.Title, paused, shuf, lp, engine.Volume(), pos, tot)

		case <-engine.Done():
			// Fuego de seguridad si la canción termina por completo
			if manager.Next() {
				currentTrack, _ = manager.CurrentTrack()
				_ = engine.Play(currentTrack.Path)
				paused = false
			} else {
				engine.Stop()
				running = false
			}
		}
	}

	term.Restore(int(os.Stdin.Fd()), oldState)
	fmt.Println("\n\nPlaylist finalizada de forma limpia.")
	runtime.KeepAlive(engine)
	runtime.KeepAlive(manager)
}