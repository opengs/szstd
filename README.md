# szstd - Seekable Zstandard Compression

A Go library for creating and reading seekable Zstandard (zstd) compressed streams. This library allows random access to compressed data without decompressing the entire stream.

## Features

- **Seekable Compression**: Compress data into frames with a seek table for random access
- **Efficient Random Access**: Jump to any position in decompressed data without reading everything
- **Standard Zstandard**: Uses the [klauspost/compress](https://github.com/klauspost/compress) library for zstd compression
- **io.ReadSeekCloser Interface**: Familiar Go I/O interfaces for easy integration
- **Frame-based Architecture**: Data is split into configurable frame sizes for optimal seek performance
- **Comprehensive Testing**: Includes fuzz tests for robustness

## Installation

```bash
go get github.com/opengs/szstd
```

## Usage

### Writing Seekable Compressed Data

```go
package main

import (
    "os"
    "github.com/opengs/szstd"
)

func main() {
    // Create output file
    outFile, err := os.Create("output.zst")
    if err != nil {
        panic(err)
    }
    defer outFile.Close()

    // Create seekable writer with 128KB frames
    writer, err := szstd.NewWriter(outFile, 128*1024)
    if err != nil {
        panic(err)
    }
    defer writer.Close()

    // Write data - it will be automatically split into frames
    data := []byte("Your data here...")
    _, err = writer.Write(data)
    if err != nil {
        panic(err)
    }

    // Close to flush remaining frames and write seek table
    err = writer.Close()
    if err != nil {
        panic(err)
    }
}
```

### Reading Seekable Compressed Data

```go
package main

import (
    "fmt"
    "io"
    "os"
    "github.com/opengs/szstd"
)

func main() {
    // Open compressed file
    file, err := os.Open("output.zst")
    if err != nil {
        panic(err)
    }
    defer file.Close()

    // Create seekable reader
    reader, err := szstd.NewReadSeeker(file)
    if err != nil {
        panic(err)
    }
    defer reader.Close()

    // Seek to specific position (e.g., 1MB into the decompressed data)
    _, err = reader.Seek(1024*1024, io.SeekStart)
    if err != nil {
        panic(err)
    }

    // Read decompressed data from that position
    buffer := make([]byte, 4096)
    n, err := reader.Read(buffer)
    if err != nil && err != io.EOF {
        panic(err)
    }

    fmt.Printf("Read %d bytes from offset 1MB\n", n)
}
```

## How It Works

### Compression

1. **Frame Creation**: Input data is split into fixed-size frames (configurable, e.g., 128KB)
2. **Frame Compression**: Each frame is independently compressed using zstd
3. **Seek Table**: A seek table is appended at the end containing:
   - Number of frames
   - Compressed and decompressed size of each frame
   - Magic numbers for validation

### Decompression

1. **Seek Table Reading**: The seek table is read from the end of the file
2. **Offset Calculation**: When seeking, the appropriate frame is located using the seek table
3. **Frame Decompression**: Only the required frame(s) are decompressed
4. **Random Access**: Data can be read from any position efficiently

## Seek Table Format

The seek table is appended at the end of the compressed data:

```
Header (8 bytes):
  - Magic number (4 bytes): 0x184D2A5E
  - Frame size (4 bytes): Size of entries + footer

Entries (N × 8 bytes):
  - Compressed size (4 bytes)
  - Decompressed size (4 bytes)

Footer (9 bytes):
  - Number of entries (4 bytes)
  - Descriptor (1 byte)
  - Magic number (4 bytes): 0x8F92EAB1
```

## Performance Considerations

### Frame Size Selection

- **Smaller frames** (e.g., 128KB):
  - Faster seeking
  - More granular random access
  - Larger seek table overhead
  - Potentially lower compression ratio

- **Larger frames** (e.g., 4MB):
  - Better compression ratio
  - Smaller seek table overhead
  - Slower seeking (more data to decompress per seek)

## Contributing

Contributions are welcome! Please ensure:
- All tests pass
- Fuzz tests run without failures
- Code follows Go conventions

## References

- [Zstandard Compression](https://github.com/facebook/zstd)
- [Zstandard Seekable Format](https://github.com/facebook/zstd/blob/dev/contrib/seekable_format/zstd_seekable_compression_format.md)
