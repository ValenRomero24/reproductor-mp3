package audio

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ValenRomero24/reproductor-mp3/internal/domain"
	"github.com/ebitengine/oto/v3"
	"github.com/hajimehoshi/go-mp3"
	"github.com/mewkiz/flac"
	"github.com/youpy/go-wav"	
)

// OtoEngine implementa la interfaz domain.AudioEngine usando Ebitengine/Oto
type OtoEngine struct {
	context			*oto.Context
	sampleRate 		int
	currentFormat	oto.Format //Guardar formato correspondiente 
	player			*oto.Player
	pcmStream		io.Reader // <---Interfaz genérica: Soporta MP3, FLAC o WAV
	currentFile		*os.File
	doneChan		chan struct{}//<--- Canal de Eventos

	mu				sync.Mutex // Protege el estado interno contra accesos simultaneos
	state 			domain.PlaybackState

	volume float64		// control de volumen (Min: 0.0 - Max: 1.0)
	volMu sync.RWMutex // Mutex dedicado 
}

// Constructor limpio del motor, el contexto se crea al usar Play()
func NewOtoEngine() (*OtoEngine, error){
	return &OtoEngine{
		state:		domain.StateStopped,
		doneChan:	make(chan struct{},1), // Canal buffereado
		volume:		1.0,
	}, nil
}

// Expone el canal de lectura para que el main sepa cúando termina una canción
func(e *OtoEngine) Done() <-chan struct{}{
	return e.doneChan
}

// Play detiene cualquier reproduccion previa, abre el archivo MP3, lo decodifica y comienza el audio.
func (e *OtoEngine) Play(path string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

// 1) Limpieza de recursos previos
	if e.currentFile != nil {
		e.currentFile.Close()
		e.currentFile = nil
	}
	e.player = nil //liberamos el player anterior
	e.pcmStream = nil //liberamos el decoder anterior

// Limpiar cualquier señal vieja del canal doneChan
	select{
	case <-e.doneChan:
	default:
	}

// 2) Abrir el archivo fisico
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	e.currentFile = file

	var fileSampleRate int
	fileFormat:= oto.FormatSignedInt16LE

	ext := strings.ToLower(filepath.Ext(path))

// 3) Switch polimórfico: cada formato se decodifica a PCM crudo
	switch ext {
	case ".mp3":
		dec, err:=mp3.NewDecoder(file)
		if err != nil{
			file.Close()
			return err
		}
		fileSampleRate = dec.SampleRate()
		e.pcmStream = dec
	case ".wav":
		dec := wav.NewReader(file)
		format, err := dec.Format()
		if err != nil{
			file.Close()
			return err
		}
		fileSampleRate = int(format.SampleRate)
		e.pcmStream = dec
	case ".flac":
		dec, err := flac.New(file)
		if err != nil{
			file.Close()
			return err
		}
		fileSampleRate = int(dec.Info.SampleRate)

		e.pcmStream = &flacDecoderWrapper{
			stream: dec,
			bps: int(dec.Info.BitsPerSample),
		}

	default:
		file.Close()
		return fmt.Errorf("Formato de archivo no soportado: %s", ext)
	}

// 1) Primer decorador: Detecta el fin de la cancion
	e.pcmStream = &eofNotifierReader{
		source: e.pcmStream,
		done: 	e.doneChan,
	}
// 2) Segundo decorador: Modifica el volumen de los bytes resultantes
	e.pcmStream = &volumeReader{
		source: e.pcmStream,
		engine: e,
	}

// 4) Control dinámico de Hardware (sample rate & formato)
	if e.context == nil || e.sampleRate != fileSampleRate || e.currentFormat != fileFormat{
		options := &oto.NewContextOptions{
			SampleRate:		fileSampleRate,
			ChannelCount: 	2,
			Format: 		fileFormat,
		}

		ctx, readyChan, err := oto.NewContext(options)
		if err != nil {
			file.Close()
			return fmt.Errorf("Error al inicializar hardware para: %s: %v", ext, err)
		}
		<-readyChan

		e.context 		= ctx
		e.sampleRate 	= fileSampleRate
		e.currentFormat	= fileFormat
	}

// 5) Iniciar la reproducción usando la interfaz genérica
	e.player = e.context.NewPlayer(e.pcmStream)
	e.player.Play()
	e.state = domain.StatePlaying

	return nil
}

func (e *OtoEngine) Pause(){
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.player != nil && e.state == domain.StatePlaying{
		e.player.Pause()
		e.state = domain.StatePaused
	}
}

func (e *OtoEngine) Resume(){
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.player != nil && e.state == domain.StatePaused {
		e.player.Play() // Reaunda si ya fue inicializado oto.
		e.state = domain.StatePlaying
	}
}

func (e *OtoEngine) Stop(){
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.player != nil{
		e.player.Pause()
		e.state = domain.StateStopped
	}
}

func (e *OtoEngine) SetVolume(vol float64){
	e.volMu.Lock()
	defer e.volMu.Unlock()
	if vol < 0.0 { vol = 0.0}
	if vol > 1.0 { vol = 1.0}
	e.volume = vol
}

func (e *OtoEngine) Volume() float64{
	e.volMu.RLock()
	defer e.volMu.RUnlock()
	return e.volume
}

// flacDecoderWrapper: Transforma frame de FLAC en un flujo io.Reader PCM
type flacDecoderWrapper struct{
	stream	*flac.Stream
	buf		[]byte
	off		int
	bps		int
}

func (w *flacDecoderWrapper) Read(p []byte) (int, error){
	//Si ya se consumió buffer interno trae el siguiente frame
	if w.off >= len(w.buf){
		frame, err := w.stream.ParseNext()
		if err != nil {
			return 0, err
		}

		numChannels := len(frame.Subframes)
		if numChannels == 0{
			return 0, io.EOF
		}
		numSamples := len(frame.Subframes[0].Samples)

		w.buf = make([]byte, numSamples*numChannels*2)
		idx :=0
	//Interpolar los canales (L R L R L R) y converir a Little Endian
		for i:=0; i < numSamples; i++{
			for ch:=0; ch < numChannels; ch++ {
				sample := frame.Subframes[ch].Samples[i]

				var s16 int16
				if w.bps == 24{
					s16 = int16(sample >> 8) //DownSamplig seguro de 24-bit a 16.bit
				} else if w.bps == 32{
					s16 = int16(sample >> 16)
				} else {
					s16 = int16(sample) //16-bit nativo
				}

			//Escribir los bytes en formato Little Endian
				w.buf[idx] = byte(s16)
				w.buf[idx+1] = byte(s16 >> 8)
				idx += 2
			}
		}
		w.off = 0
	}

	n:= copy(p, w.buf[w.off:])
	w.off += n
	return n, nil
}

// eofNotifierReader: Monitorea el flujo y gatilla el fin de la canción
type eofNotifierReader struct{
	source	io.Reader
	done 	chan struct{}
	sent 	bool
}

func(r *eofNotifierReader) Read(p []byte) (int, error){
	n, err := r.source.Read(p)

	// Si el decoder original se queda sin datos y no se envió el aviso
	if err == io.EOF && !r.sent {
		r.sent = true

		go func(){
			time.Sleep(100 * time.Millisecond)
			r.done <- struct{}{}
		}()
	}
	return n, err
}

// volumeReader: Modifica la amplitud matemática de los bytes PCM
type volumeReader struct{
	source	io.Reader
	engine	*OtoEngine
}

func (v *volumeReader) Read(p []byte) (int, error){
	n, err := v.source.Read(p)
	if n > 0 {
		vol := v.engine.Volume()
	// Si el volumen está al maximo (1.0), saltea el cálculo para ahorrar CPU
		if vol < 0.99{
		// Avanzar de a 2 Bytes (cada muestra = int16)
			for i:=0; i < n; i+=2{
				if i+1 >= n{
					break // Proteccion por si llega un byte al final del buffer
				}
			// 1) Reconstrucción del int16 a partir de 2 bytes (Little Endian)
				sample := int16(p[i]) | (int16(p[i+1]) << 8)
			
			// 2) Aplicar ganancia (atenuación)
				newSample := int16(float64(sample) * vol)
			
			// 3) Separacion del int16 a 2 bytes individuales
				p[i]	= byte(newSample)
				p[i+1]	= byte(newSample >> 8)
			}
		}
	}
	return n, err
}
