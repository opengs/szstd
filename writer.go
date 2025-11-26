package szstd

import (
	"errors"
	"io"

	"github.com/klauspost/compress/zstd"
	"github.com/opengs/szstd/seektable"
)

type writer struct {
	w io.Writer

	frameSize   int
	frameBuffer []byte

	encoderBuffer []byte
	encoder       *zstd.Encoder

	seekTable seektable.Table

	isClosed bool
}

// Create new zstd writer that will automatically split input data into frames of the given size.
// Resulting compressed data will be seekable by frame boundaries. `Close` will flush the remaning frames and write the seek table at the end.
func NewWriter(w io.Writer, frameSize int, opts ...zstd.EOption) (io.WriteCloser, error) {
	encoder, err := zstd.NewWriter(nil, append([]zstd.EOption{zstd.WithEncoderConcurrency(1)}, opts...)...)
	if err != nil {
		return nil, errors.Join(errors.New("failed to create zstd encoder"), err)
	}

	return &writer{
		w:             w,
		frameSize:     frameSize,
		frameBuffer:   make([]byte, 0, frameSize),
		encoder:       encoder,
		encoderBuffer: make([]byte, 0, frameSize+frameSize/10), // allocate some extra space for compressed data
	}, nil
}

func (c *writer) Write(data []byte) (n int, err error) {
	for len(data) > 0 {
		// fast path: if we have no data buffered and the incoming data is larger than a frame, encode directly
		if len(c.frameBuffer) == 0 && len(data) >= c.frameSize {
			toEncode := data[:c.frameSize]
			data = data[c.frameSize:]
			c.encoderBuffer = c.encoder.EncodeAll(toEncode, c.encoderBuffer[:0])
			written, err := c.w.Write(c.encoderBuffer)
			if err != nil {
				return n + written, errors.Join(errors.New("error while writing frame"), err)
			}
			n += written
			c.seekTable.AppendEntry(seektable.TableEntry{
				DecompressedSize: uint32(len(toEncode)),
				CompressedSize:   uint32(len(c.encoderBuffer)),
			})
			continue
		}

		// fill frame buffer
		spaceLeft := int(c.frameSize) - len(c.frameBuffer)
		toWrite := min(len(data), spaceLeft)
		c.frameBuffer = append(c.frameBuffer, data[:toWrite]...)
		data = data[toWrite:]
		n += toWrite

		if len(c.frameBuffer) == int(c.frameSize) {
			c.encoderBuffer = c.encoder.EncodeAll(c.frameBuffer, c.encoderBuffer[:0])
			written, err := c.w.Write(c.encoderBuffer)
			if err != nil {
				return n - toWrite + written, errors.Join(errors.New("error while writing frame"), err)
			}
			c.seekTable.AppendEntry(seektable.TableEntry{
				DecompressedSize: uint32(len(c.frameBuffer)),
				CompressedSize:   uint32(len(c.encoderBuffer)),
			})
			c.frameBuffer = c.frameBuffer[:0]
		}
	}

	return n, nil
}

func (c *writer) Close() error {
	if c.isClosed {
		return nil
	}
	c.isClosed = true

	// Write any remaining buffered data
	if len(c.frameBuffer) > 0 {
		c.encoderBuffer = c.encoder.EncodeAll(c.frameBuffer, c.encoderBuffer[:0])
		_, err := c.w.Write(c.encoderBuffer)
		if err != nil {
			return errors.Join(errors.New("error while writing final frame"), err)
		}
		c.seekTable.AppendEntry(seektable.TableEntry{
			DecompressedSize: uint32(len(c.frameBuffer)),
			CompressedSize:   uint32(len(c.encoderBuffer)),
		})
		c.frameBuffer = c.frameBuffer[:0]
	}

	// Write seek table
	if _, err := seektable.WriteTableToWriter(&c.seekTable, c.w); err != nil {
		return errors.Join(errors.New("error while writing seek table"), err)
	}

	c.encoder.Close()

	return nil
}
