package audio

import (
	"fmt"
	"io"
	"os"
	"github.com/mewkiz/flac"
)

// flacDecoderWrapper: Transforma frame de FLAC en un flujo io.Reader PCM
type flacDecoderWrapper struct{
	file	*os.File
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

func (w *flacDecoderWrapper) SeekSamples(targetSamples uint64) error{
	if _, err := w.file.Seek(0, io.SeekStart); err != nil {
		return err
	}

	newStream, err := flac.New(w.file)
	if err != nil {
		return err
	}
	w.stream	= newStream
	w.buf 		= nil
	w.off 		= 0

	var currentSample uint64 = 0
	for currentSample < targetSamples {
		frame, err := w.stream.ParseNext()
		if err != nil {
			if err == io.EOF { break }
			return err
		}
		currentSample += uint64(len(frame.Subframes[0].Samples))
	}
	return nil
}

func (w *flacDecoderWrapper) Seek(offset int64, whence int) (int64, error) {
	if whence != io.SeekStart {
		return 0, fmt.Errorf("solo se soporta SeekStart para búsquedas en FLAC")
	}

	targetSample := uint64(offset / 4)

	_, err := w.stream.Seek(targetSample)
	if err != nil {
		return 0, fmt.Errorf("error buscando muestra en FLAC: %v", err)
	}

	w.buf = nil
	w.off = 0
	return offset, nil

}