package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	defaultGroupSize             = 2
	defaultGroupSizeLittleEndian = 4
	defaultCols                  = 16
	offsetCharWidth              = 10
)

type command struct {
	input          io.Reader // Input file (or stdin)
	output         io.Writer
	endOffset      int64 // Where to stop reading (byte offset)
	littleEndian   bool  // -e Output in little-endian order
	groupSize      int   // -g <int> default 2, byte grouping
	bytesPerLine   int   // -c <int> octets per line. default 16
	maxBytes       int64 // -l <int> stop writing after len octets
	startOffset    int64 // -s <offset> (which byte to start reading from)
	revert         bool  // -r Reverse operation: convert (or patch) hex dump into binary
	wantedHexWidth int   // Helper for little endian formatting
}

func main() {
	cmd, err := loadCommand()
	if err != nil {
		fmt.Println("error loading command:", err)
		os.Exit(1)
	}

	// If -r flag is set, convert hex dump to binary and exit
	if cmd.revert {
		err := revertToBinary(cmd.input, cmd.output)
		if err != nil {
			fmt.Fprintln(cmd.output, "error reverting to binary:", err)
			os.Exit(1)
		}
		return
	}

	// perform normal hex dump
	err = cmd.run()
	if err != nil {
		fmt.Fprintln(cmd.output, "error running command:", err)
		os.Exit(1)
	}
}

// Parses command-line arguments, sets up the command struct, and opens file/stdin
func loadCommand() (command, error) {
	var err error
	cmd := command{
		output: os.Stdout,
	}

	flag.BoolVar(&cmd.littleEndian, "e", false, "Print hex output in little-endian order within each group.")
	flag.BoolVar(&cmd.revert, "r", false, "Convert a hex dump back into binary (reverse operation).")
	flag.IntVar(&cmd.groupSize, "g", defaultGroupSize, "Group hex output every <bytes> bytes, separated by a space")
	flag.IntVar(&cmd.bytesPerLine, "c", defaultCols, "Number of bytes to display per line in the hex dump")
	flag.Int64Var(&cmd.maxBytes, "l", -1, "Limit output to <len> bytes and then stop (default: dump entire input).")
	flag.Int64Var(&cmd.startOffset, "s", 0, "Skip <seek> bytes from the start before dumping (default 0, i.e., start at beginning).")

	flag.Parse()
	args := flag.Args()

	switch len(args) {
	case 0:
		cmd.input = os.Stdin
	case 1:
		cmd.input, err = os.Open(args[0])
		if err != nil {
			fmt.Printf("error opening %v as file: %v", args[0], err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "too many args: %v\n", args)
		flag.Usage()
		os.Exit(1)
	}

	// Validate and fix up byte grouping as needed
	cmd.groupSize, err = validateByteGrouping(cmd.groupSize, cmd.bytesPerLine, cmd.littleEndian)
	if err != nil {
		return cmd, err
	}

	return cmd, nil
}

// Main hex dump loop: reads bytes, formats, and prints each line
func (cmd *command) run() error {
	var err error
	// determine where reading should end
	cmd.endOffset, err = getEndByte(cmd.maxBytes, cmd.startOffset, cmd.input)
	if err != nil {
		return err
	}

	if cmd.littleEndian {
		cmd.wantedHexWidth = hexFieldWidth(cmd.bytesPerLine, cmd.groupSize)
	}

	// If input is a file, seek to requested offset
	if seeker, ok := cmd.input.(io.Seeker); ok && cmd.startOffset > 0 {
		_, err := seeker.Seek(cmd.startOffset, io.SeekStart)
		if err != nil {
			return fmt.Errorf("error setting offset: %v", err)
		}
	}

	reader := bufio.NewReader(cmd.input)
	offset := cmd.startOffset // Tracks current byte offset for hex display

	// Loop until we've read up to endByte
	for offset < cmd.endOffset {

		// Pass in how many bytes were supposed to read
		// which is the smallest of cols or bytes left until endbytes
		length := min(int64(cmd.bytesPerLine), cmd.endOffset-offset)

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

// readLine: Use io.ReadFull to ensure each line is filled unless at EOF, matching xxd behavior.
func (cmd *command) readLine(reader *bufio.Reader, length int) ([]byte, error) {
	buf := make([]byte, length) // Buffer for one output line
	n, err := io.ReadFull(reader, buf)

	switch {
	case err == nil:
		// successful full line read
		return buf, nil
	case err == io.EOF && n == 0:
		// reached eof, nothing read
		return nil, io.EOF
	case err == io.ErrUnexpectedEOF || (err == io.EOF && n > 0):
		// partial read at last line
		return buf[:n], nil
	default:
		// any other err
		return nil, err
	}
}

// Printline builds the whole line in memory with strings.Builder, then writes it once for efficiency.
func (cmd *command) printLine(offset int64, line []byte) {
	var builder strings.Builder
	lineLength := len(line)
	// Print the offset at the start of the line (8 hex digits)
	fmt.Fprintf(&builder, "%08x: ", offset)

	if !cmd.littleEndian {
		cmd.printHex(line, &builder)
	} else {
		// needs to return bytecount bcs of left side padding added
		lineLength = cmd.printLittleEndianHex(line, &builder)
	}
	cmd.printHexPadding(lineLength, &builder)
	cmd.printASCII(line, &builder)
	fmt.Fprintln(cmd.output, builder.String())
}

// printHex prints normal (big-endian) hex output, grouped as specified.
// This function prints each byte as two hex digits, inserting a space after every 'byteGrouping' bytes.
func (cmd *command) printHex(line []byte, builder *strings.Builder) {
	for i, b := range line {
		fmt.Fprintf(builder, "%02x", b)
		if (i+1)%cmd.groupSize == 0 {
			builder.WriteString(" ")
		}
	}
	// ensures a double space before ascii if
	if cmd.bytesPerLine%cmd.groupSize != 0 {
		builder.WriteString(" ")
	}
}

// printLittleEndianHex prints the buffer as little-endian hex, grouped by byteGrouping.
// reverses the bytes within each group before printing
func (cmd *command) printLittleEndianHex(line []byte, builder *strings.Builder) int {
	length := len(line)

	for i := 0; i < len(line); i += cmd.groupSize {
		// Compute the end index for this group. If we're at the end of the line and don't have a full group,
		// 'end' will be less than i+cmd.byteGrouping.
		start := i
		end := i + cmd.groupSize
		end = min(end, len(line))

		// Add left side padding to byte group
		if end-start < cmd.groupSize {
			for range cmd.groupSize - (end - start) {
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
	builder.WriteString(" ")

	if cmd.littleEndian {
		// fmt.Printf("builder len is %v and cmd wanted width is %v\n", builder.Len(), cmd.wantedWidth)
		// the 12 is the chars for offset printing
		for builder.Len() < cmd.wantedHexWidth {
			// fmt.Printf("builder len is %v and cmd wanted width is %v\n", builder.Len(), cmd.wantedWidth)
			builder.WriteString(" ")
		}
	} else {
		// For each missing byte, print "  " instead of hex
		for i := bytesRead; i < cmd.bytesPerLine; i++ {
			builder.WriteString("  ")
			// Add group space if this would have been a group boundary
			if (i+1)%cmd.groupSize == 0 {
				builder.WriteString(" ")
			}
		}
	}
}

// Returns the end byte offset for the dump (either file size or user-specified length)
func getEndByte(maxBytes, startOffset int64, file io.Reader) (int64, error) {
	var totalLen int64

	switch r := file.(type) {
	case *os.File:
		info, err := r.Stat()
		if err != nil {
			return 0, err
		}
		totalLen = info.Size()
	case *strings.Reader:
		totalLen = int64(r.Len())
	case *bytes.Buffer:
		totalLen = int64(r.Len())
	case *bytes.Reader:
		totalLen = int64(r.Len())
	default:
		// fallback: assume "infinite" (read until EOF)
		totalLen = 1<<63 - 1
	}

	if maxBytes >= 0 {
		return startOffset + maxBytes, nil
	}
	return totalLen, nil
}

// Ensures byte grouping is valid (positive, <= cols, etc.)
// In little endian number of octets must be a power of 2
// If little-endian output is requested and grouping=2, set grouping to 4 (xxd -e default)
func validateByteGrouping(groupSize, bytesPerLine int, littleEndian bool) (int, error) {
	switch {
	case littleEndian && !isPowerOfTwo(groupSize):
		return 0, fmt.Errorf("number of octets per group must be a power of 2 with -e")
	case littleEndian && groupSize == defaultGroupSize:
		return defaultGroupSizeLittleEndian, nil
	case groupSize < 0:
		return defaultGroupSize, nil
	case groupSize == 0:
		return bytesPerLine, nil
	case groupSize > bytesPerLine:
		return bytesPerLine, nil
	default:
		return groupSize, nil
	}
}

// isPowerOfTwo returns true if n is a positive power of two.
// checks that n has only one bit set in binary.
// For example, 8 (1000 in binary) is a power of two, but 6 (0110) is not.
//
//   - A power of two in binary has exactly one '1' bit (e.g. 8 = 1000).
//   - Subtracting 1 flips all bits after the first '1' (e.g. 8-1 = 7 = 0111).
//   - The bitwise AND of n and n-1 is zero only for powers of two.
func isPowerOfTwo(n int) bool {
	return n > 0 && (n&(n-1)) == 0
}

// hexFieldWidth calculates the total width of the hex field before the ASCII panel.
// This ensures the ASCII column always aligns,
// especially in little-endian mode with odd group/column sizes.
//
// The width includes:
//   - 2 hex digits per byte
//   - 1 space after each group
//   - 2 extra spaces for the gap before ASCII (as xxd does)
//   - offsetCharWidth, which accounts for the "00000000: " offset prefix
//
// Example:
//
//	For cols=11, group=2:
//	  numGroups = (11 + 2 - 1) / 2 = 6
//	  width = 6 * (2*2 + 1) = 6 * 5 = 30
//	  width += 2 (extra spaces) = 32
//	  return width + offsetCharWidth
//
// Helper for problematic little endian spacing before ascii
func hexFieldWidth(cols, group int) int {
	numGroups := (cols + group - 1) / group
	width := numGroups * (group*2 + 1)
	// gap before ascii
	width += 2

	return width + offsetCharWidth
}

// revertToBinary reads a hex dump and writes the decoded binary to output.
func revertToBinary(file io.Reader, output io.Writer) error {
	writer := bufio.NewWriter(output)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		// Skip offset (first 10 chars), split at double space between hex and ascii
		line := strings.Split(scanner.Text()[offsetCharWidth:], "  ")
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
