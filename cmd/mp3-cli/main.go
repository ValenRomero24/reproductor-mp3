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
	"github.com/dhowden/tag"
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

	enrichMetadata(tracks)

	manager := domain.NewPlaylistManager(tracks)
	engine, err := audio.NewOtoEngine()
	if err != nil { log.Fatalf("Error hardware: %v", err) }

	currentTrackPtr, ok := playTrackOrSkip(manager, engine)
	if !ok {
		fmt.Println("No se pudo reproducir ninguna canción válida en el directorio.")
		return
	}
	currentTrack := *currentTrackPtr

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
				manager.Next()
				if trackPtr, ok := playTrackOrSkip(manager, engine); ok {
					currentTrack = *trackPtr
					paused = false
				} else {
					running = false
				}
			case 'p', 'P':
				manager.Prev()
				if trackPtr, ok := playTrackOrSkip(manager, engine); ok {
					currentTrack = *trackPtr
					paused = false
				} else {
					running = false
				}
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
			if manager.Next(){
				if trackPtr, ok := playTrackOrSkip(manager, engine); ok{
					currentTrack = *trackPtr
					paused = false
				} else {
					engine.Stop()
					running = false
				}
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


func playTrackOrSkip(manager *domain.PlaylistManager, engine *audio.OtoEngine) (*domain.Track, bool){
	maxIntentos := 5
	intentos := 0

	for intentos < maxIntentos {
		track, err := manager.CurrentTrack()
		if err != nil {
			 return nil, false
		}
		err = engine.Play(track.Path)
		if err == nil {
			return &track, true
		}
		fmt.Printf("\r\x1b[K⚠️ Error al reproducir [%s]: %v. Saltando...\r\n", track.Title, err)

		if !manager.Next(){
			return nil, false
		}
		intentos++
	}

	return nil, false
}

func enrichMetadata(tracks []domain.Track){
	for i := range tracks{
		f, err := os.Open(tracks[i].Path)
		if err != nil {
			continue
		}

		m, err := tag.ReadFrom(f)
		if err == nil{
			artist	:= m.Artist()
			title	:= m.Title()

			if title != " "{
				if artist != " "{
					tracks[i].Title = fmt.Sprintf("%s - %s", artist, title)
				} else {
					tracks[i].Title = title
				}
			}
		}
		f.Close()
	}
}