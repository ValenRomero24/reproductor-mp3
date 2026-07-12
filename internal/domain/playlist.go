package domain

import (
	"errors"
	"math/rand"
	"time"
)


// Gestiona la cola de reproducción y el índice actual en memoria.
type PlaylistManager struct {
	tracks 			[]Track
	originalTracks	[]Track
	currentIndex	int
	isShuffle		bool
	isLoop			bool
	rng				*rand.Rand
}

// Inicializa el administrador con una lista de canciones.
func NewPlaylistManager(tracks []Track) *PlaylistManager {
	orig := make([]Track, len(tracks))
	copy(orig, tracks)

	return &PlaylistManager{
		tracks:			tracks,
		originalTracks:	orig,
		currentIndex:	0,
		isShuffle:		false,
		isLoop:			false,
		rng:			rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Devuelve la canción que debería estar sonando actualmente.
func (m *PlaylistManager) CurrentTrack() (Track, error) {
	if len(m.tracks) == 0{
		return Track{}, errors.New("La playlist está vacía.")
	}
	return m.tracks[m.currentIndex], nil
}

// Avanza a la siguiente canción. Devuelve true si cambió, o false si no hay más canciones.
func (m *PlaylistManager) Next() bool{
	if len(m.tracks) == 0 {
		return false
	}

	//Si no es la última canción, avanza normalmente
	if m.currentIndex < len(m.tracks)-1 {
		m.currentIndex++
		return true
	}

	if m.isLoop {
		m.currentIndex = 0
		return true
	}

	return true
}

// Retrocede a la canción anterior
func (m *PlaylistManager) Prev(){
	if len(m.tracks) == 0{
		return
	}

	//Si no es la primera, retrocedemos normalmente
	if m.currentIndex > 0{
		m.currentIndex--
	} else {
		m.currentIndex = len(m.tracks)-1
	}
}

func (m * PlaylistManager) ToggleShuffle() {
	if len(m.tracks) <= 1 {
		m.isShuffle = !m.isShuffle
		return
	}

	m.isShuffle = !m.isShuffle
	currentTrack := m.tracks[m.currentIndex]

	if m.isShuffle{
		m.rng.Shuffle(len(m.tracks), func(i,j int){
			m.tracks[i], m.tracks[j] = m.tracks[j], m.tracks[i]
		})

		for i, t := range m.tracks {
			if t.Path == currentTrack.Path {
				m.currentIndex = i
				break
			}
		}
	} else {
		m.tracks = make([]Track, len(m.originalTracks))
		copy(m.tracks, m.originalTracks)

		for i, t := range m.tracks {
			if t.Path == currentTrack.Path{
				m.currentIndex = i
				break
			}
		}
	}
}

func (m *PlaylistManager) ToggleLoop(){
	m.isLoop = !m.isLoop
}

func (m *PlaylistManager) Status() (bool, bool){
	return m.isShuffle, m.isLoop
}