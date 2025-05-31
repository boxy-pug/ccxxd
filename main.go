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
	output             io.Writer
	endByte            int64 // Where to stop reading (byte offset)
	littleEndianOutput bool  // -e Output in little-endian order
	byteGrouping       int   // -g <int> default 2
	cols               int   // -c <int> octets per line. default 16
	len                int64 // -l <int> stop writing after len octets
	seek               int64 // -s <offset> (which byte to start reading from)
	revert             bool  // -r Reverse operation: convert (or patch) hex dump into binary
}

func main() {
	cmd, err := loadCommand()
	if err != nil {
		fmt.Println("error loading command:", err)
		os.Exit(1)
	}

	// If -r flag is set, convert hex dump to binary and exit
	if cmd.revert {
		revertToBinary(cmd.file)
		return
	}

	// perform normal hex dump
	cmd.run()
}

// Parses command-line arguments, sets up the command struct, and opens file/stdin
func loadCommand() (command, error) {
	var err error
	cmd := command{
		output: os.Stdout,
	}

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
func (cmd *command) run() error {
	// If input is a file, seek to requested offset
	if cmd.file != os.Stdin {
		_, err := cmd.file.Seek(cmd.seek, 0)
		if err != nil {
			return fmt.Errorf("error setting offset: %v", err)
		}
	}

	reader := bufio.NewReader(cmd.file)
	offset := cmd.seek // Tracks current byte offset for hex display

	// Loop until we've read up to endByte
	for offset < cmd.endByte {

		// Pass in how many bytes were supposed to read
		// which is the smallest of cols or bytes left until endbytes
		length := min(int64(cmd.cols), cmd.endByte-offset)

		lineBytes, err := cmd.readLine(reader, int(length))
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		cmd.printLine(offset, lineBytes)
		offset += int64(len(lineBytes))
	}
	return nil
}

func (cmd *command) readLine(reader *bufio.Reader, length int) ([]byte, error) {
	buf := make([]byte, length) // Buffer for one output line
	n, err := io.ReadFull(reader, buf)
	// end of file, nothing to read:
	if err == io.EOF && n == 0 {
		return nil, io.EOF
	}
	if err == io.ErrUnexpectedEOF || err == nil {
		return buf[:n], nil // partial read at eof
	}
	if err != nil {
		return nil, err // other, real errs
	}
	return buf[:n], nil
}

func (cmd *command) printLine(offset int64, line []byte) {
	var builder strings.Builder
	lineLength := len(line)
	// Print the offset at the start of the line (8 hex digits)
	fmt.Fprintf(&builder, "%08x: ", offset)

	if !cmd.littleEndianOutput {
		cmd.printHex(line, &builder)
	} else {
		// needs to return bytecount bcs of left side padding added
		lineLength = cmd.printLittleEndianHex(line, &builder)
	}
	fmt.Fprint(&builder, " ")
	cmd.printHexPadding(lineLength, &builder)
	cmd.printASCII(line, &builder)
	fmt.Fprintln(cmd.output, builder.String())
}

// printHex prints normal (big-endian) hex output, grouped as specified.
// This function prints each byte as two hex digits, inserting a space after every 'byteGrouping' bytes.
func (cmd *command) printHex(line []byte, builder *strings.Builder) {
	for i, b := range line {
		fmt.Fprintf(builder, "%02x", b)
		if (i+1)%cmd.byteGrouping == 0 {
			builder.WriteString(" ")
		}
	}
	// formattin inconsistencies csn be fixed here i guess
	// res.WriteString(" ")
	if cmd.cols%cmd.byteGrouping != 0 {
		builder.WriteString(" ")
	}
}

// printLittleEndianHex prints the buffer as little-endian hex, grouped by byteGrouping.
// reverses the bytes within each group before printing
func (cmd *command) printLittleEndianHex(line []byte, builder *strings.Builder) int {
	length := len(line)

	for i := 0; i < len(line); i += cmd.byteGrouping {
		// Compute the end index for this group. If we're at the end of the line and don't have a full group,
		// 'end' will be less than i+cmd.byteGrouping.
		start := i
		end := i + cmd.byteGrouping
		end = min(end, len(line))

		// Add left side padding to byte group
		if end-start < cmd.byteGrouping {
			for range cmd.byteGrouping - (end - start) {
				builder.WriteString("  ")
				length++
			}
		}
		// Print the bytes of this group in reverse order (for little-endian display).
		if start < len(line) {
			for j := end - 1; j >= start; j-- {
				fmt.Fprintf(builder, "%02x", line[j]) // Print byte as two hex digits
			}
			// After each group, insert a space to separate groups visually.
			builder.WriteString(" ")
		}
	}
	// to make it line up?
	builder.WriteString(" ")
	return length
}

// Print ASCII representation (print '.' for non-printable)
func (cmd *command) printASCII(line []byte, builder *strings.Builder) {
	for _, b := range line {
		if isValidASCII(b) {
			fmt.Fprintf(builder, "%s", string(b))
		} else {
			fmt.Fprint(builder, ".")
		}
	}
}

// Returns true if b is a printable ASCII character
func isValidASCII(b byte) bool {
	return b >= 32 && b <= 126
}

// Prints extra spaces at end of short lines, so ASCII lines up
func (cmd *command) printHexPadding(bytesRead int, builder *strings.Builder) {
	// For each missing byte, print "  " instead of hex
	for i := bytesRead; i < cmd.cols; i++ {
		builder.WriteString("  ")
		// Add group space if this would have been a group boundary
		if (i+1)%cmd.byteGrouping == 0 {
			builder.WriteString(" ")
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

// Reads hex dump from file, decodes, and writes raw binary to stdout
func revertToBinary(file *os.File) error {
	writer := bufio.NewWriter(os.Stdout)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		// Skip offset (first 10 chars), split at double space between hex and ascii
		line := strings.Split(scanner.Text()[10:], "  ")
		cleanLine := strings.ReplaceAll(line[0], " ", "") // Remove spaces from hex
		hexLine, err := hex.DecodeString(cleanLine)       // Decode hex to bytes
		if err != nil {
			return fmt.Errorf("error decoding string as hex: %v", err)
		}
		_, err = writer.Write(hexLine)
		if err != nil {
			return fmt.Errorf("error writing to stdout: %v", err)
		}
	}
	writer.Flush()
	return nil
}
