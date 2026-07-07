package domain

import "errors"

// Gestiona la cola de reproducción y el índice actual en memoria.
type PlaylistManager struct {
	tracks []Track
	currentIndex int
}

// Inicializa el administrador con una lista de canciones.
func NewPlaylistManager(tracks []Track) *PlaylistManager {
	return &PlaylistManager{
		tracks:	tracks,
		currentIndex:	0,
	}
}

// Devuelve la canción que debería estar sonando actualmente.
func (pm *PlaylistManager) CurrentTrack() (Track, error) {
	if len(pm.tracks) == 0{
		return Track{}, errors.New("La lista de reproducción está vacía.")
	}
	return pm.tracks[pm.currentIndex], nil
}

// Avanza a la siguiente canción. Devuelve true si cambió, o false si no hay más canciones.
func (pm *PlaylistManager) Next() bool{
	if len(pm.tracks) == 0{
		return false
	}

	//Si no es la última canción, avanza normalmente
	if pm.currentIndex < len(pm.tracks)-1 {
		pm.currentIndex++
		return true
	}

	//Comportamiento "Circular": si termina el album, vuelve a la primera cancion
	pm.currentIndex = 0
	return true
}

// Retrocede a la canción anterior
func (pm *PlaylistManager) Prev() bool{
	if len(pm.tracks) == 0{
		return false
	}

	//Si no es la primera, retrocedemos normalmente
	if pm.currentIndex > 0{
		pm.currentIndex--
		return true
	}

	pm.currentIndex = len(pm.tracks)-1
	return true
}

// Devuelve la cantidad total de canciones en la cola.
func (pm *PlaylistManager) Count() int{
	return len(pm.tracks)
}