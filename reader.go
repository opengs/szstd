package szstd

import (
	"errors"
	"fmt"
	"io"

	"github.com/klauspost/compress/zstd"
	"github.com/opengs/szstd/seektable"
)

type reader struct {
	r io.ReadSeeker

	decoder   *zstd.Decoder
	seekTable *seektable.Table

	offset uint64

	totalCompressedDataSize   uint64 // without seek table
	totalUncompressedDataSize uint64

	currentFrameIndex     int
	currentFrameLoaded    bool
	currentFrameBuffer    []byte
	currentFrameReaded    int // number of bytes already readed from the current frame buffer
	currentFrameAvailable int // total number of bytes available in the current frame buffer (readed + un-readed)

	compressedDataBuffer []byte
}

func NewReadSeeker(r io.ReadSeeker, opts ...zstd.DOption) (io.ReadSeekCloser, error) {
	decoder, err := zstd.NewReader(nil, append([]zstd.DOption{zstd.WithDecoderConcurrency(1)}, opts...)...)
	if err != nil {
		return nil, errors.Join(errors.New("failed to create zstd decoder"), err)
	}

	seekTable, err := seektable.ReadTableFromReadSeeker(r)
	if err != nil {
		return nil, errors.Join(errors.New("failed to read seek table"), err)
	}

	// Calculate total uncompressed size
	lastOffsets := seekTable.OffsetsByIndex(seekTable.NumEntries() - 1)
	lastEntry := seekTable.GetEntry(seekTable.NumEntries() - 1)
	totalUncompressedDataSize := lastOffsets.EntryOffsetInDecompressed + uint64(lastEntry.DecompressedSize)

	// Calculate total compressed size
	_, err = r.Seek(0, io.SeekStart)
	if err != nil {
		return nil, errors.Join(errors.New("failed to seek to the beginning of the data to calculate total compressed size"), err)
	}
	totalDataSize, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, errors.Join(errors.New("failed to seek to end to calculate total compressed size"), err)
	}
	totalCompressedDataSize := uint64(totalDataSize) - uint64(seekTable.Size())

	// Make sure the seek table is consistent with the underlying reader size
	if seekTable.NumEntries() > 0 {
		expectedSize := uint64(lastOffsets.EntryOffsetInCompressed) + uint64(lastEntry.CompressedSize)
		if totalCompressedDataSize < expectedSize { // size can be greater because of possible empty frames as per ZSTD spec
			return nil, fmt.Errorf("seek table last entry size mismatch: expected total compressed size %d, got %d", expectedSize, totalCompressedDataSize)
		}
	}

	return &reader{r: r, decoder: decoder, seekTable: seekTable, totalUncompressedDataSize: totalUncompressedDataSize, totalCompressedDataSize: totalCompressedDataSize}, nil
}

func (r *reader) Read(p []byte) (int, error) {
	if r.currentFrameIndex >= r.seekTable.NumEntries() {
		return 0, io.EOF
	}

	if !r.currentFrameLoaded {
		tableOffsets, offsetFounded := r.seekTable.Find(r.offset)
		if !offsetFounded {
			return 0, fmt.Errorf("failed to find frame for offset %d", r.offset)
		}
		if _, err := r.r.Seek(int64(tableOffsets.EntryOffsetInCompressed), io.SeekStart); err != nil {
			return 0, errors.Join(fmt.Errorf("failed to seek to the frame offset %d", r.offset), err)
		}
		entry := r.seekTable.GetEntry(tableOffsets.EntryIndex)
		if uint64(entry.CompressedSize) > uint64(cap(r.compressedDataBuffer)) {
			r.compressedDataBuffer = make([]byte, entry.CompressedSize)
		} else {
			r.compressedDataBuffer = r.compressedDataBuffer[:entry.CompressedSize]
		}
		if _, err := io.ReadFull(r.r, r.compressedDataBuffer); err != nil {
			return 0, errors.Join(fmt.Errorf("failed to read compressed frame data for offset %d", r.offset), err)
		}

		var err error
		r.currentFrameBuffer, err = r.decoder.DecodeAll(r.compressedDataBuffer, r.currentFrameBuffer[:0])
		if err != nil {
			return 0, errors.Join(fmt.Errorf("failed to decode frame for offset %d", r.offset), err)
		}
		r.currentFrameLoaded = true
		r.currentFrameAvailable = len(r.currentFrameBuffer)
	}

	availableToRead := r.currentFrameAvailable - r.currentFrameReaded
	toRead := min(len(p), availableToRead)
	copy(p[:toRead], r.currentFrameBuffer[r.currentFrameReaded:r.currentFrameReaded+toRead])
	r.currentFrameReaded += toRead
	r.offset += uint64(toRead)

	// If we have readed the entire current frame, move to the next one
	if r.currentFrameReaded >= r.currentFrameAvailable {
		r.currentFrameIndex++
		r.currentFrameLoaded = false
		r.currentFrameReaded = 0
		r.currentFrameAvailable = 0
	}

	return toRead, nil
}

func (r *reader) Seek(offset int64, whence int) (int64, error) {
	// Calculate the new offset
	var newOffset uint64
	switch whence {
	case io.SeekStart:
		if offset < 0 {
			return 0, errors.New("negative offset")
		}
		newOffset = uint64(offset)
	case io.SeekCurrent:
		if offset == 0 { // must report current offset even if offset id beyond the end of the file. standard golang iotest behaviour
			return int64(r.offset), nil
		}
		if offset < 0 && uint64(-offset) > r.offset {
			return 0, errors.New("negative offset")
		}
		newOffset = r.offset + uint64(offset)
	case io.SeekEnd:
		if offset < 0 && uint64(-offset) > r.totalUncompressedDataSize {
			return 0, errors.New("negative offset")
		}
		newOffset = r.totalUncompressedDataSize + uint64(offset)
	default:
		return 0, errors.New("invalid whence")
	}

	if newOffset > r.totalUncompressedDataSize {
		return 0, errors.New("offset beyond end of data")
	}

	tableOffsets, found := r.seekTable.Find(newOffset)
	if !found {
		return 0, errors.New("offset beyond end of data")
	}
	frameStartOffset := tableOffsets.EntryOffsetInDecompressed

	if tableOffsets.EntryIndex != r.currentFrameIndex { // Only load new frame if the index is different. If we seek in the same frame, we can just adjust the readed offset
		r.currentFrameIndex = tableOffsets.EntryIndex
		r.currentFrameLoaded = false
	}

	r.currentFrameReaded = int(newOffset - frameStartOffset)
	r.offset = newOffset

	return int64(newOffset), nil
}

func (r *reader) Close() error {
	r.decoder.Close()
	return nil
}
