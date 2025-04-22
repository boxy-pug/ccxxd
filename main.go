package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
)

type Config struct {
	file               *os.File
	fileBytes          int64
	littleEndianOutput bool //-e
	byteGrouping       int  // -g <int> default 2
	cols               int  // -c <int> octets per line. default 16
	len                int  // -l <int> stop writing after len octets
	seek               int  // -s <offset> (which byte to start reading from)
	revert             bool // -r Reverse operation: convert (or patch) hex dump into binary
}

func main() {
	cfg := loadConfig()

	printLines(cfg)

}

func loadConfig() Config {
	var err error
	var cfg Config

	flag.BoolVar(&cfg.littleEndianOutput, "e", false, "Switch to little-endian hex dump.")
	flag.BoolVar(&cfg.revert, "r", false, "Reverse operation: convert (or patch) hex dump into binary.")
	flag.IntVar(&cfg.byteGrouping, "g", 2, "Separate the output of every <bytes> bytes (two hex characters or eight bit digits each) by a whitespace.")
	flag.IntVar(&cfg.cols, "c", 16, "Format <cols> octets per line. Default 16")
	flag.IntVar(&cfg.len, "l", -1, "Stop after writing <len> octets.")
	flag.IntVar(&cfg.seek, "s", -1, "Start at <seek> bytes abs. (or rel.) infile offset.")

	flag.Parse()

	args := flag.Args()

	if len(args) == 1 {
		cfg.file, err = os.Open(args[0])
		if err != nil {
			fmt.Printf("error opening %v as file: %v", args[0], err)
			os.Exit(1)
		}
		info, err := cfg.file.Stat()
		if err != nil {
			fmt.Printf("couldn't retrieve file stat for %v: %v", cfg.file, err)
			os.Exit(1)
		}
		cfg.fileBytes = info.Size()
	} else if len(args) == 0 {
		cfg.file = os.Stdin
	}

	return cfg
}

func printLines(cfg Config) {
	offSet := int64(0)
	reader := bufio.NewReader(cfg.file)

	for offSet < cfg.fileBytes {
		buffer := make([]byte, 16)
		bt, err := io.ReadFull(reader, buffer)
		if err != nil {
			fmt.Printf("error: %v", err)
		}

		fmt.Printf("%08x: ", offSet)
		for i, byt := range buffer {
			fmt.Printf("%02x", byt)
			if (i+1)%2 == 0 {
				fmt.Printf(" ")
			}
		}

		for _, charByt := range buffer {
			if isValidASCII(charByt) {
				fmt.Printf("%s", string(charByt))
			} else {
				fmt.Printf(".")
			}

		}

		fmt.Println()
		offSet += int64(bt)
	}
}

func isValidASCII(b byte) bool {
	return b >= 32 && b <= 126
}

/*

r := strings.NewReader("some io.Reader stream to be read\n")

	buf := make([]byte, 4)
	if _, err := io.ReadFull(r, buf); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s\n", buf)

	// minimal read size bigger than io.Reader stream
	longBuf := make([]byte, 64)
	if _, err := io.ReadFull(r, longBuf); err != nil {
		fmt.Println("error:", err)
	}

*/
