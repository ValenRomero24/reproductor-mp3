package audio

import (
	"io"
	"time"
)

type progressReader struct {
	source	io.Reader
	track	*activeTrack
}

func (pr *progressReader) Read(p []byte) (int, error){
	n, err := pr.source.Read(p)
	if n > 0 {
		pr.track.mu.Lock()
		pr.track.bytesRead += int64(n)
		pr.track.mu.Unlock()
	}
	return n, err 
}

// volumeReader: Modifica la amplitud matemática de los bytes PCM
type volumeReader struct{
	source	io.Reader
	track 	*activeTrack
	engine	*OtoEngine
}

func (v *volumeReader) Read(p []byte) (int, error){
	n, err := v.source.Read(p)
	if n > 0 {
		v.track.mu.RLock()
		localVol := v.track.volume
		v.track.mu.RUnlock()

		globalVol := v.engine.Volume()
		effVol := localVol * globalVol

		if effVol < 0.99{
			for i:=0; i < n; i+=2{
				if i+1 >= n{
					break // Proteccion por si llega un byte al final del buffer
				}
			// 1) Reconstrucción del int16 a partir de 2 bytes (Little Endian)
				sample := int16(p[i]) | (int16(p[i+1]) << 8)
			
			// 2) Aplicar ganancia (atenuación)
				newSample := int16(float64(sample) * effVol)
			
			// 3) Separacion del int16 a 2 bytes individuales
				p[i]	= byte(newSample)
				p[i+1]	= byte(newSample >> 8)
			}
		}
	}
	return n, err
}

// eofNotifierReader: Monitorea el flujo y gatilla el fin de la canción
type eofNotifierReader struct{
	source	io.Reader
	track	*activeTrack
	engine	*OtoEngine
	sent 	bool
}

func(r *eofNotifierReader) Read(p []byte) (int, error){
	n, err := r.source.Read(p)
	if err == io.EOF && !r.sent{
		r.sent = true
		r.track.mu.RLock()
		fading := r.track.isFading
		r.track.mu.RUnlock()

		if !fading {
			go func(){
				time.Sleep(400 * time.Millisecond)
				r.engine.doneChan <- struct{}{}
			}()
		}
	}
	return n, err
}