package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"unicode/utf8"
)

type SizedReadSeekCloser interface {
	io.ReadSeekCloser
	Size() (int64, error)
}

type Options struct {
	From        SizedReadSeekCloser
	To          io.WriteCloser
	Conversions map[Converter]struct{}
	Offset      int64
	Limit       uint64
	BlockSize   uint
}

func (opts *Options) close() {
	_ = opts.From.Close()
	_ = opts.To.Close()
}

var (
	illegalFlag      = errors.New("illegal flag")
	invalidOperation = errors.New("invalid operation")
)

func MustParseFlags() *Options {
	opts := Options{
		From:        BetterStream(*os.Stdin),
		To:          BetterStream(*os.Stdout),
		Conversions: map[Converter]struct{}{},
	}

	flag.Func("from", "file to read. by default - stdin", func(param string) error {
		file, err := os.Open(param)
		if err != nil {
			return fmt.Errorf("can't access the specified input file (%v): %w", param, err)
		}
		opts.From = BetterFile(*file)
		return nil
	})
	flag.Func("to", "file to write. by default - stdout", func(param string) error {
		_, err := os.Stat(param)
		if err == nil {
			return fmt.Errorf("specified output file %v already exists", param)
		} else if errors.Is(err, os.ErrNotExist) {
			file, err := os.Create(param)
			if err != nil {
				return fmt.Errorf("can't create the specified output file (%v): %w", param, err)
			}
			opts.To = BetterFile(*file)
			return nil
		}
		return fmt.Errorf("error checking output file existence: %w", err)
	})
	flag.Int64Var(&opts.Offset, "offset", 0,
		"number of bytes to skip from the beginning of the file. by default - 0")
	flag.Uint64Var(&opts.Limit, "limit", math.MaxUint64,
		"number of bytes to read from file. by default - the whole file")
	flag.UintVar(&opts.BlockSize, "block-size", 1024,
		"size of a block that is read from a file before any write operation. Default - 1024")

	flag.Func("conv",
		"one or more text conversions, comma-separated. Available options: upper_case, lower_case, trim_spaces",
		func(s string) error {
			selected := strings.Split(strings.TrimSpace(s), ",")
			for _, conv := range selected {
				switch conv {
				case "upper_case":
					if _, exists := opts.Conversions[LowerCase{}]; exists {
						return fmt.Errorf("%w: upper_case and lower_case at the same time", illegalFlag)
					}
					opts.Conversions[UpperCase{}] = struct{}{}
				case "lower_case":
					if _, exists := opts.Conversions[UpperCase{}]; exists {
						return fmt.Errorf("%w: lower_case and upper_case at the same time", illegalFlag)
					}
					opts.Conversions[LowerCase{}] = struct{}{}
				case "trim_spaces":
					opts.Conversions[&TrimSpaces{}] = struct{}{}
				default:
					return fmt.Errorf("unknown conversion type: %v", conv)
				}
			}
			return nil
		},
	)

	flag.Parse() // Default behaviour ExitOnError

	return &opts
}

func runRead(
	from io.Reader,
	bytesToRead uint64,
	blockSize uint,
	readChunkCallback func([]byte, bool) ([]byte, error),
) error {
	buffer := make([]byte, blockSize)
	var unprocessedTail []byte //  bytes unprocessed from the previous step

	for bytesToRead > 0 || len(unprocessedTail) > 0 {
		if bytesToRead > 0 {
			readN, readError := from.Read(buffer[:min(uint64(len(buffer)), bytesToRead)])
			unprocessedTail = append(unprocessedTail, buffer[:readN]...)
			if readError != nil && readError != io.EOF {
				return fmt.Errorf("unable to read from input file: %w", readError)
			}
			bytesToRead -= uint64(readN)
			if readError == io.EOF {
				bytesToRead = 0
			}
		}

		unprocessed, err := readChunkCallback(unprocessedTail, bytesToRead == 0)
		if err != nil {
			return fmt.Errorf("error processing read chunk: %w", err)
		}
		unprocessedTail = unprocessed
	}
	return nil
}

// End-to-end validation of generated config correctness.
func postValidateParams(opts *Options) error {
	if inpSize, err := opts.From.Size(); err != nil {
		if !errors.Is(err, invalidOperation) { // Special case for streams
			return fmt.Errorf("unable to get input file info: %w", err)
		}
	} else if opts.Offset >= inpSize {
		return fmt.Errorf("invalid offset %v>=%v (input file size)", opts.Offset, inpSize)
	}
	return nil
}

func main() {
	opts := MustParseFlags()
	defer opts.close()

	if err := postValidateParams(opts); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error validating input parameters: %v\n", err)
		if err != nil {
			return
		}
		os.Exit(2)
	}

	if _, err := opts.From.Seek(opts.Offset, 1); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to set the specified offset %v: %v\n", opts.Offset, err)
		os.Exit(1)
	}

	processAndWrite := func(readChunk []byte, isLastChunk bool) ([]byte, error) {
		var decoded strings.Builder

		unprocessedTail := make([]byte, 0)

		for i := 0; i < len(readChunk); {
			letter, width := utf8.DecodeRune(readChunk[i:])
			if letter == utf8.RuneError {
				unprocessedTail = append(unprocessedTail, readChunk[i:i+width]...)
			} else {
				n, err := opts.To.Write(unprocessedTail)
				if n < len(unprocessedTail) {
					return unprocessedTail[n:], err
				}
				unprocessedTail = unprocessedTail[n:]

				decoded.WriteRune(letter)
			}
			i += width
		}

		currentString := decoded.String()

		if decoded.Len() == 0 && isLastChunk {
			n, err := opts.To.Write(unprocessedTail)
			return unprocessedTail[n:], err
		}

		for converter := range opts.Conversions {
			currentString = converter.ConvertChunk(currentString)
		}

		n, err := opts.To.Write([]byte(currentString))
		if n < len(currentString) {
			unprocessedTail = append([]byte(currentString[n:]), unprocessedTail...)
		}

		return unprocessedTail, err
	}

	if err := runRead(opts.From, opts.Limit, opts.BlockSize, processAndWrite); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "Unexpected error: ", err)
		os.Exit(1)
	}
}
