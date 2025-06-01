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

const (
	defaultGroup             = 2
	defaultGroupLittleEndian = 4
	defaultCols              = 16
)

type command struct {
	file         io.Reader // Input file (or stdin)
	output       io.Writer
	endByte      int64 // Where to stop reading (byte offset)
	littleEndian bool  // -e Output in little-endian order
	group        int   // -g <int> default 2, byte grouping
	cols         int   // -c <int> octets per line. default 16
	len          int64 // -l <int> stop writing after len octets
	seek         int64 // -s <offset> (which byte to start reading from)
	revert       bool  // -r Reverse operation: convert (or patch) hex dump into binary
}

func main() {
	cmd, err := loadCommand()
	if err != nil {
		fmt.Println("error loading command:", err)
		os.Exit(1)
	}

	// If -r flag is set, convert hex dump to binary and exit
	if cmd.revert {
		err := revertToBinary(cmd.file)
		if err != nil {
			fmt.Println("error reverting to binary:", err)
			os.Exit(1)
		}
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

	flag.BoolVar(&cmd.littleEndian, "e", false, "Switch to little-endian hex dump.")
	flag.BoolVar(&cmd.revert, "r", false, "Reverse operation: convert (or patch) hex dump into binary.")
	flag.IntVar(&cmd.group, "g", defaultGroup, "Separate the output of every <bytes> bytes (two hex characters or eight bit digits each) by a whitespace.")
	flag.IntVar(&cmd.cols, "c", defaultCols, "Format <cols> octets per line. Default 16")
	flag.Int64Var(&cmd.len, "l", -1, "Stop after writing <len> octets.")
	flag.Int64Var(&cmd.seek, "s", 0, "Start at <seek> bytes abs. (or rel.) infile offset.")

	flag.Parse()
	args := flag.Args()

	switch len(args) {
	case 0:
		cmd.file = os.Stdin
	case 1:
		cmd.file, err = os.Open(args[0])
		if err != nil {
			fmt.Printf("error opening %v as file: %v", args[0], err)
			os.Exit(1)
		}
	default:
		flag.Usage()
		return cmd, fmt.Errorf("too many args, check usage: %v", args)
	}

	cmd.endByte, err = getEndByte(cmd.len, cmd.file)
	if err != nil {
		return cmd, err
	}

	// Validate and fix up byte grouping as needed
	cmd.group = validateByteGrouping(cmd.group, cmd.cols)

	// If little-endian output is requested and grouping=2, set grouping to 4 (xxd -e default)
	if cmd.littleEndian && cmd.group == defaultGroup {
		cmd.group = defaultGroupLittleEndian
	}

	return cmd, nil
}

// Main hex dump loop: reads bytes, formats, and prints each line
func (cmd *command) run() error {
	// If input is a file, seek to requested offset
	if seeker, ok := cmd.file.(io.Seeker); ok && cmd.seek > 0 {
		_, err := seeker.Seek(cmd.seek, io.SeekStart)
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

// I first used bufio.Reader.Read here, but it can return fewer bytes than requested
// even if there's more data to read, which caused short lines to appear in the middle of the hex dump.
// io.ReadFull keeps reading until the buffer is full or EOF, so now only the last line can be short—
// this matches what real hex dump tools like xxd do.
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

// Instead of writing each thing directly to stdout, I use a strings.Builder to build
// the whole line in memory first. Writing to stdout (or any io.Writer)
// is expensive compared to working in RAM, especially for lots of small writes. 
// By building the line in RAM I reduce the number of system calls and get more
// predictable, flicker-free output. Assembling output before printing is generally good.
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
		if (i+1)%cmd.group == 0 {
			builder.WriteString(" ")
		}
	}
	// ensures a double space before ascii if
	if cmd.cols%cmd.group != 0 {
		builder.WriteString(" ")
	}
}

// printLittleEndianHex prints the buffer as little-endian hex, grouped by byteGrouping.
// reverses the bytes within each group before printing
func (cmd *command) printLittleEndianHex(line []byte, builder *strings.Builder) int {
	length := len(line)

	for i := 0; i < len(line); i += cmd.group {
		// Compute the end index for this group. If we're at the end of the line and don't have a full group,
		// 'end' will be less than i+cmd.byteGrouping.
		start := i
		end := i + cmd.group
		end = min(end, len(line))

		// Add left side padding to byte group
		if end-start < cmd.group {
			for range cmd.group - (end - start) {
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
		if (i+1)%cmd.group == 0 {
			builder.WriteString(" ")
		}
	}
}

// Returns the end byte offset for the dump (either file size or user-specified length)
func getEndByte(len int64, file io.Reader) (int64, error) {
	if len >= 0 {
		return len, nil
	}
	if f, ok := file.(*os.File); ok {
		info, err := f.Stat()
		if err != nil {
			return 0, err
		}
		return info.Size(), nil
	}
	return 0, nil
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
func revertToBinary(file io.Reader) error {
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
