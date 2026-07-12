package audio

import (
	"io"
	"github.com/mewkiz/flac"
)

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