package audio

import(
	"os"
	"path/filepath"
	"strings"

	"github.com/ValenRomero24/reproductor-mp3/internal/domain"

)

// Busca todos los archivos .mp3, .flac y .wav en la ruta dada y devuelve un slice de Tracks.
func ScanDirectory(dirPath string) ([]domain.Track,error){
// 1) Leer entradas del directorio
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}
	var tracks []domain.Track

// 2) Filtrar y procesar cada entrada
	for _, entry := range entries{
	//Ignora subcarpetas por ahora
		if entry.IsDir(){
			continue
		}
		// Verifica la extensión insensible a mayúsculas (.mp3 o .MP3)
		fileName := entry.Name()
		ext := strings.ToLower(filepath.Ext(fileName))
		if ext == ".mp3" || ext == ".flac" || ext == ".wav"{
		//Contruccion segura de la ruta absoluta POSIX
		fullPath := filepath.Join(dirPath, fileName)
		//El titulo de la cancion es el nombre del archivo sin su extension
		title := strings.TrimSuffix(fileName, filepath.Ext(fileName))

		tracks = append(tracks, domain.Track{
			Path:	fullPath,
			Title:	title,
		})
		}
	}
	return tracks, nil
}