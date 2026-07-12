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

// Encapsula de forma independiente el estado de cada archivo
type activeTrack struct {
	player		*oto.Player
	file		*os.File
	volume		float64			// Volumen local de la cancion
	bytesRead	int64
	totalBytes	int64
	sampleRate	int
	isFading	bool			// Si está en fadeou, ignora su propio EOF
	mu			sync.RWMutex	// Protege el estado de esta cancion.
}

type OtoEngine struct {
	context		*oto.Context
	current		*activeTrack // Ranura principal
	fading		*activeTrack // Ranura secundaria para el enganche
	doneChan 	chan struct{}

	mu			sync.Mutex
	state		domain.PlaybackState

	volume		float64
	volMu		sync.RWMutex
}

// Constructor limpio del motor, el contexto se crea al usar Play()
func NewOtoEngine() (*OtoEngine, error){
	return &OtoEngine{
		state:		domain.StateStopped,
		doneChan:	make(chan struct{},1), // Canal buffereado
		volume:		1.0,
	}, nil
}

func (e *OtoEngine) createActiveTrack(path string) (*activeTrack, error){
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	track := &activeTrack{
		file: 	file,
		volume: 1.0,
	}

	var fileSampleRate	int
	var pcmStream		io.Reader
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".mp3":
		dec, err := mp3.NewDecoder(file)
		if err != nil {
			file.Close()
			return nil, err
		}
		fileSampleRate = dec.SampleRate()
		pcmStream = dec
		track.totalBytes = dec.Length()
	case ".WAV":
		dec := wav.NewReader(file)
		format, err := dec.Format()
		if err != nil {
			file.Close()
			return nil, err
		}
		fileSampleRate = int(format.SampleRate)
		pcmStream = dec
		info, err := file.Stat()
		if err == nil {
			track.totalBytes = info.Size() -44
		}
	case ".flac": 
		dec, err := flac.New(file)
		if err != nil {
			file.Close()
			return nil, err
		}
		fileSampleRate = int(dec.Info.SampleRate)
		pcmStream = &flacDecoderWrapper{
			stream: dec,
			bps:	int(dec.Info.BitsPerSample),
		}
		track.totalBytes = int64(dec.Info.NSamples) * 2 * 2
	default:
		file.Close()
		return nil, fmt.Errorf("formato no soportado: %s", ext)
	}
	track.sampleRate = fileSampleRate

	pcmStream = &progressReader{
		source:	pcmStream,
		track:	track,
	}

	pcmStream = &volumeReader{
		source:	pcmStream,
		track:	track,
		engine:	e,
	}
	
	pcmStream = &eofNotifierReader{
		source:	pcmStream,
		track:	track,
		engine: e,
	}

	if e.context == nil {
		options := &oto.NewContextOptions{
			SampleRate:		44100,
			ChannelCount:	2,
			Format:			oto.FormatSignedInt16LE,
		}
		ctx, readyChan, err := oto.NewContext(options)
		if err != nil {
			file.Close()
			return nil, err
		}
		<-readyChan
		e.context = ctx
	}

	track.player = e.context.NewPlayer(pcmStream)
	return track, nil
}

// Play estándar (Corte limpio)
func (e *OtoEngine) Play(path string) error {
	e.mu.Lock()

	if e.current != nil{
		e.current.player.Close()
		e.current.file.Close()
		e.current = nil
	}
	if e.fading != nil {
		e.fading.player.Close()
		e.fading.file.Close()
		e.fading = nil
	}

	e.mu.Unlock()

	track, err := e.createActiveTrack(path)
	if err != nil{
		return err
	}

	e.mu.Lock()

	e.current = track
	e.current.player.Play()
	e.state = domain.StatePlaying

	e.mu.Unlock()
	return nil
}

func (e *OtoEngine) CrossFadeTo(path string) error{
	e.mu.Lock()
	
	if e.current == nil{
		e.mu.Unlock()
		return e.Play(path)
	}
	
	if e.fading != nil {
		e.fading.player.Close()
		e.fading.file.Close()
	}

	e.fading = e.current
	e.fading.mu.Lock()
	e.fading.isFading = true
	e.fading.mu.Unlock()
	e.current = nil
	e.mu.Unlock()

	track, err := e.createActiveTrack(path)
	if err != nil {
		e.mu.Lock()
		e.current = e.fading
		if e.current != nil {
			e.current.isFading = false
		
		}
		e.fading = nil
		e.mu.Unlock()
		return err
	}

	track.volume = 0.0

	e.mu.Lock()
	e.current = track
	e.current.player.Play()
	e.state = domain.StatePlaying
	e.mu.Unlock()

	go func(old, new*activeTrack){
		steps := 50
		duracionFade := 5 * time.Second
		intervaloStep := duracionFade / time.Duration(steps)

		for i := 1; i <= steps; i++ {
			time.Sleep(intervaloStep)
			ratio := float64(i) / float64(steps)

			old.mu.Lock()
			old.volume = 1.0 - ratio
			old.mu.Unlock()

			new.mu.Lock()
			new.volume = ratio
			new.mu.Unlock()
		}

		old.player.Close()
		old.file.Close()

		e.mu.Lock()
		if e.fading == old {
			e.fading = nil
		}
		e.mu.Unlock()
	} (e.fading, e.current)

	return nil

}

// Expone el canal de lectura para que el main sepa cúando termina una canción
func(e *OtoEngine) Done() <-chan struct{}{
	return e.doneChan
}


func (e *OtoEngine) Pause(){
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.current.player != nil && e.state == domain.StatePlaying{
		e.current.player.Pause()
		e.state = domain.StatePaused
	}
}

func (e *OtoEngine) Resume(){
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.current.player != nil && e.state == domain.StatePaused {
		e.current.player.Play() // Reaunda si ya fue inicializado oto.
		e.state = domain.StatePlaying
	}
}

func (e *OtoEngine) Stop(){
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.current.player != nil{
		e.current.player.Pause()
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

func (e *OtoEngine) GetProgress() (time.Duration, time.Duration){
	e.mu.Lock()
	curr := e.current
	e.mu.Unlock()

	if curr == nil {
		return 0, 0
	}

	curr.mu.RLock()
	br := curr.bytesRead
	tb := curr.totalBytes
	sr := curr.sampleRate
	curr.mu.RUnlock()

	bps := int64(sr) * 4
	if bps == 0{
		return 0, 0
	}

	return time.Duration((float64(br) / float64(bps)) * float64(time.Second)),
	       time.Duration((float64(tb) / float64(bps)) * float64(time.Second))
}
