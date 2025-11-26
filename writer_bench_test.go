package szstd

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
)

var testFiles = []string{
	"testdata/silesia/dickens",
	"testdata/silesia/x-ray",
	"testdata/silesia/ooffice",
	"testdata/silesia/mozilla.tar",
	"testdata/silesia/nci",
	"testdata/silesia/webster",
	"testdata/silesia/reymont",
	"testdata/silesia/mr",
	"testdata/silesia/xml.tar",
	"testdata/silesia/osdb",
	"testdata/silesia/samba.tar",
}
var testCompressionLevels = []zstd.EncoderLevel{
	//zstd.SpeedFastest,
	zstd.SpeedDefault,
	//zstd.SpeedBetterCompression,
	//zstd.SpeedBestCompression,
}

func runZSTDWriterBenchmark(b *testing.B, data []byte, opts ...zstd.EOption) {
	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		writer, err := zstd.NewWriter(io.Discard, opts...)
		if err != nil {
			b.Fatalf("failed to create zstd writer: %v", err)
		}

		if _, err := writer.Write(data); err != nil {
			b.Fatalf("failed to write data to zstd writer: %v", err)
		}

		if err := writer.Close(); err != nil {
			b.Fatalf("failed to close zstd writer: %v", err)
		}
	}
}

func runSZSTDWriterBenchmark(b *testing.B, frameSize int, data []byte, opts ...zstd.EOption) {
	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		writer, err := NewWriter(io.Discard, frameSize, opts...)
		if err != nil {
			b.Fatalf("failed to create szstd writer: %v", err)
		}

		if _, err := writer.Write(data); err != nil {
			b.Fatalf("failed to write data to szstd writer: %v", err)
		}

		if err := writer.Close(); err != nil {
			b.Fatalf("failed to close szstd writer: %v", err)
		}
	}
}

func BenchmarkZSTDWriter(b *testing.B) {

	for _, file := range testFiles {
		data, err := os.ReadFile(file)
		if err != nil {
			b.Fatalf("failed to read test data file %s: %v", file, err)
		}

		for _, level := range testCompressionLevels {
			name := fmt.Sprintf("%s_LV%d", strings.ReplaceAll(file, "/", "_"), level)
			b.Run(name, func(b *testing.B) {
				runZSTDWriterBenchmark(b, data, zstd.WithEncoderLevel(level), zstd.WithEncoderConcurrency(1))
			})
		}
	}
}

func BenchmarkSZSTDWriter(b *testing.B) {
	frameSizes := []int{
		256 * 1024,
		1 * 1024 * 1024,
		4 * 1024 * 1024,
		8 * 1024 * 1024,
	}

	for _, file := range testFiles {
		data, err := os.ReadFile(file)
		if err != nil {
			b.Fatalf("failed to read test data file %s: %v", file, err)
		}

		for _, level := range testCompressionLevels {
			for _, frameSize := range frameSizes {
				name := fmt.Sprintf("%s_LV%d_%dKB", strings.ReplaceAll(file, "/", "_"), level, frameSize/1024)
				b.Run(name, func(b *testing.B) {
					runSZSTDWriterBenchmark(b, frameSize, data, zstd.WithEncoderLevel(level), zstd.WithEncoderConcurrency(1))
				})
			}
		}
	}
}
