package domain

// PlaybackState representa el estado actual del reproductor
type PlaybackState int

const (
	StateStopped PlaybackState = iota
	StatePlaying
	StatePaused
)

// Track representa una canción en el sistema.
type Track struct{
	Path string //Ruta absoluta en el sistema de archivos POSIX
	Title string //Nombre del archivo o titulo del tag ID3
}

// AudioEngine define el contrato que cualquier morot de audio de Linux debe cumplir.
type AudioEngine interface {
	Play(path string) error
	Pause()
	Resume()
	Stop()
	Close() error
}