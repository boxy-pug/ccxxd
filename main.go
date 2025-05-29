package main

import (
	"bufio"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

type command struct {
	file               *os.File // Input file (or stdin)
	endByte            int64    // Where to stop reading (byte offset)
	littleEndianOutput bool     //-e Output in little-endian order
	byteGrouping       int      // -g <int> default 2
	cols               int      // -c <int> octets per line. default 16
	len                int64    // -l <int> stop writing after len octets
	seek               int64    // -s <offset> (which byte to start reading from)
	revert             bool     // -r Reverse operation: convert (or patch) hex dump into binary
}

func main() {
	cfg, err := loadCommand()
	if err != nil {
		fmt.Println("error loading command:", err)
		os.Exit(1)
	}

	// If -r flag is set, convert hex dump to binary and exit
	if cfg.revert {
		revertToBinary(cfg.file)
		return
	}

	// perform normal hex dump
	run(cfg)
}

// Parses command-line arguments, sets up the command struct, and opens file/stdin
func loadCommand() (command, error) {
	var err error
	var cmd command

	flag.BoolVar(&cmd.littleEndianOutput, "e", false, "Switch to little-endian hex dump.")
	flag.BoolVar(&cmd.revert, "r", false, "Reverse operation: convert (or patch) hex dump into binary.")
	flag.IntVar(&cmd.byteGrouping, "g", 2, "Separate the output of every <bytes> bytes (two hex characters or eight bit digits each) by a whitespace.")
	flag.IntVar(&cmd.cols, "c", 16, "Format <cols> octets per line. Default 16")
	flag.Int64Var(&cmd.len, "l", -1, "Stop after writing <len> octets.")
	flag.Int64Var(&cmd.seek, "s", 0, "Start at <seek> bytes abs. (or rel.) infile offset.")

	flag.Parse()
	args := flag.Args()

	if len(args) == 1 {
		cmd.file, err = os.Open(args[0])
		if err != nil {
			fmt.Printf("error opening %v as file: %v", args[0], err)
			os.Exit(1)
		}
	} else if len(args) == 0 {
		cmd.file = os.Stdin
	} else {
		flag.Usage()
		return cmd, fmt.Errorf("too many args, check usage: %v", args)
	}

	// Compute where to stop reading (endByte)
	cmd.endByte = getEndByte(cmd.len, cmd.file)

	// Validate and fix up byte grouping as needed
	cmd.byteGrouping = validateByteGrouping(cmd.byteGrouping, cmd.cols)

	// If little-endian output is requested and grouping=2, set grouping to 4 (xxd -e default)
	if cmd.littleEndianOutput && cmd.byteGrouping == 2 {
		cmd.byteGrouping = 4
	}

	return cmd, nil
}

// Main hex dump loop: reads bytes, formats, and prints each line
func run(cfg command) {
	offSetHex := cfg.seek  // Tracks current byte offset for hex display
	offSetChar := cfg.seek // Tracks current byte offset for ASCII display
	reader := bufio.NewReader(cfg.file)

	// If input is a file, seek to requested offset
	if cfg.file != os.Stdin {
		_, err := cfg.file.Seek(cfg.seek, 0)
		if err != nil {
			fmt.Printf("error setting offset: %v", err)
			os.Exit(1)
		}
	}

	// Loop until we've read up to endByte
	for offSetHex < cfg.endByte {
		buffer := make([]byte, cfg.cols) // Buffer for one output line
		bytesRead, err := io.ReadFull(reader, buffer)
		if err != nil {
			if err == io.EOF && bytesRead == 0 {
				break // end of file, nothing to read
			}
			if err != io.EOF && err != io.ErrUnexpectedEOF {
				fmt.Printf("error: %v", err)
				break
			}
			// For io.ErrUnexpectedEOF, we still want to print the partial buffer
		}

		// Print the offset at the start of the line (8 hex digits)
		fmt.Printf("%08x: ", offSetHex)

		// Printing hex bytes
		if !cfg.littleEndianOutput {
			// Normal hex output
			for i, byt := range buffer[:bytesRead] {
				fmt.Printf("%02x", byt) // Print byte as two hex digits
				if (i+1)%cfg.byteGrouping == 0 {
					fmt.Printf(" ") // Space after each group
				}
				offSetHex++
				// Stop if we've reached the end
				if offSetHex == cfg.endByte {
					printExtraSpace(cfg.cols, bytesRead, cfg.byteGrouping)
					break
				}
			}
		} else {
			// Little-endian output: print each group reversed
			offSetHex += printLittleEndianHex(buffer[:bytesRead], cfg.byteGrouping)
			if offSetHex == cfg.endByte {
				printExtraSpace(cfg.cols, bytesRead, cfg.byteGrouping)
				break
			}
		}

		// Print space between hex and ascii
		fmt.Printf(" ")
		// Print ASCII representation (print '.' for non-printable)
		for _, charByt := range buffer[:bytesRead] {
			if isValidASCII(charByt) {
				fmt.Printf("%s", string(charByt))
			} else {
				fmt.Printf(".")
			}
			offSetChar++
			if offSetChar == cfg.endByte {
				break
			}
		}
		fmt.Println() // End of line
	}
}

// Returns true if b is a printable ASCII character
func isValidASCII(b byte) bool {
	return b >= 32 && b <= 126
}

// Prints extra spaces at end of short lines, so ASCII lines up
func printExtraSpace(totalCols, bytesRead, byteGrouping int) {
	// For each missing byte, print "  " instead of hex
	for i := bytesRead; i < totalCols; i++ {
		fmt.Print("  ")
		// Add group space if this would have been a group boundary
		if (i+1)%byteGrouping == 0 {
			fmt.Print(" ")
		}
	}
}

// Returns the end byte offset for the dump (either file size or user-specified length)
func getEndByte(len int64, file *os.File) int64 {
	var res int64
	info, err := file.Stat()
	if err != nil {
		fmt.Printf("couldn't retrieve file stat for %v: %v", file, err)
		os.Exit(1)
	}
	if len >= 0 {
		res = len
	} else {
		res = info.Size()
	}
	return res
}

// Ensures byte grouping is valid (positive, <= cols, etc.)
func validateByteGrouping(bg, cols int) int {
	if bg < 0 {
		return 2
	} else if bg == 0 {
		return cols
	} else if bg > cols {
		return cols
	} else {
		return bg
	}
}

// Prints buffer as little-endian hex, grouped by byteGrouping
func printLittleEndianHex(buffer []byte, byteGrouping int) int64 {
	for i := 0; i < len(buffer); i += byteGrouping {
		groupSize := byteGrouping
		if i+byteGrouping > len(buffer) {
			groupSize = len(buffer) - i
		}
		currentBuffer := make([]byte, groupSize)
		copy(currentBuffer, buffer[i:i+groupSize])

		// Reverse the bytes in the group (for little-endian)
		for j := 0; j < groupSize/2; j++ {
			currentBuffer[j], currentBuffer[groupSize-1-j] = currentBuffer[groupSize-1-j], currentBuffer[j]
		}

		fmt.Print(hex.EncodeToString(currentBuffer) + " ")
	}

	return int64(len(buffer))
}

// Reads hex dump from file, decodes, and writes raw binary to stdout
func revertToBinary(file *os.File) {
	writer := bufio.NewWriter(os.Stdout)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		// Skip offset (first 10 chars), split at double space between hex and ascii
		line := strings.Split(scanner.Text()[10:], "  ")
		cleanLine := strings.ReplaceAll(line[0], " ", "") // Remove spaces from hex
		hexLine, err := hex.DecodeString(cleanLine)       // Decode hex to bytes
		if err != nil {
			fmt.Errorf("error decoding string as hex: %v", err)
		}
		_, err = writer.Write(hexLine)
		if err != nil {
			fmt.Errorf("error writing to stdout: %v", err)
		}
	}
	writer.Flush()
}
