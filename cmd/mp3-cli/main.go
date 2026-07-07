package main

import (
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/ValenRomero24/reproductor-mp3/internal/audio"
	"github.com/ValenRomero24/reproductor-mp3/internal/domain"
	"golang.org/x/term"
)

func main(){

// 1) Verificación de argumentos
	if len(os.Args) < 2{
		fmt.Println("Uso: reproductor-mp3 <ruta-directorio-musica>")
		return
	}	
	dirPath := os.Args[1]

// 2) Escaneo del directorio POSIX
	fmt.Printf("Escaneando la carpeta: %s\n", dirPath)
	tracks, err := audio.ScanDirectory(dirPath)
	if err != nil {
		log.Fatalf("Error al escanear el directorio: %v", err)
	}

// Defensa: si la carpeta está vacía o no tiene MP3 válidos, corta
	if len(tracks) == 0{
		fmt.Println("No se encontraron archivos .mp3/.FLAC/.WAV en el directorio especificado.")
		return
	}
	fmt.Printf("¡Éxito! Se cargaron %d canciones en la cola.\n", len(tracks))

// 3) Inicialización del Administrados de la Playlist (Capa Dom)
	manager := domain.NewPlaylistManager(tracks)

// 4) Inicialización del Motor de Audio (Capa Infra)
	fmt.Println("Inicializando hardware de audio...")
	engine, err := audio.NewOtoEngine()
	if err != nil {
		log.Fatalf("Error al inicializar el audio: %v", err)
	}


// 5) Extraccion del primer Track y Reproducción
	currentTrack, err := manager.CurrentTrack()
	if err != nil{
		log.Fatalf("Error en la playlist: %v", err)
	}
	fmt.Printf("\n Reproduciendo [1/%d] %s\n", manager.Count(), currentTrack.Title)	
	err = engine.Play(currentTrack.Path)
	if err != nil {
		log.Fatalf("Error al reproducir la primera cancion: %v", err)
	}


// Activación MODO RAW 
	oldState, err :=term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		log.Fatalf("Error al activar modo interactivo: %v", err)
	}
	//Devuelve la terminal al estado original
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	fmt.Print("\r\n=== CONTROLES EN TIEMPO REAL ===\r\n")
	fmt.Print("[Espacio]: Pausa/Play | [n]: Sig | [p]: Ant | [+ / -]: volumen | [q]: Salir\r\n")
	fmt.Print("================================\r\n\r\n")

	//Imprimir estado inicial
	printHUD(currentTrack.Title, false, engine.Volume())

// Lógica Asincrónica de Entrada
	keyChan := make(chan byte)
	go func(){
		buf := make([]byte, 1)
		for{
			_, err := os.Stdin.Read(buf)
			if err != nil {
				return
			}
			keyChan <- buf[0] //Tecla capturada
		}
	}()

	paused := false
	running := true

// BUcle de control basasdo en eventos
	for running {
		select{
	// Evento A) El usuario tocó el teclado
		case char := <-keyChan:
			if char == 'q' || char == 'Q' || char== 3{
				running = false
				break
			}
			switch char {
			case ' ':
				if paused {
					engine.Resume()
					paused = false
				} else {
					engine.Pause()
					paused = true
				}
				printHUD(currentTrack.Title, paused, engine.Volume())
			case 'n', 'N':
				manager.Next()
				currentTrack, _=manager.CurrentTrack()
				_ = engine.Play(currentTrack.Path)
				paused = false
				printHUD(currentTrack.Title, paused, engine.Volume())
			case 'p', 'P':
				manager.Prev()
				currentTrack,_ = manager.CurrentTrack()
				_ = engine.Play(currentTrack.Path)
				paused  = false
				printHUD(currentTrack.Title, paused, engine.Volume())
			case '+', '=':
				nuevoVol := engine.Volume() + 0.05 // Sube de a 5%
				engine.SetVolume(nuevoVol)
				printHUD(currentTrack.Title, paused, engine.Volume())
			case '-':
				nuevoVol := engine.Volume() -0.05 // Baja de a 5%
				engine.SetVolume(nuevoVol)
				printHUD(currentTrack.Title, paused, engine.Volume())
			}
	// Evento B) El motor avisa que la canción actual terminó sola
		case <-engine.Done():
		// Avanzar a la siguiente de forma automática
			manager.Next()
			currentTrack, _ = manager.CurrentTrack()
			_ = engine.Play(currentTrack.Path)
			paused = false
			printHUD(currentTrack.Title, paused, engine.Volume())
		}
	}

	term.Restore(int(os.Stdin.Fd()), oldState)
	fmt.Println("\n\nCerrando canales de sonido de forma segura.")
	runtime.KeepAlive(engine)
	runtime.KeepAlive(manager)
}

func printHUD(title string, paused bool, volume float64){
	status:= "▶️  Reproduciendo"
	if paused {
		status = "⏸️  Pausado"
	}
	volPorcentaje := int(volume * 100)
	fmt.Printf("\r\x1b[K%s: %s [Vol: %d%%]", status, title, volPorcentaje)
}
