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
	endByte            int64
	littleEndianOutput bool  //-e
	byteGrouping       int   // -g <int> default 2
	cols               int   // -c <int> octets per line. default 16
	len                int64 // -l <int> stop writing after len octets
	seek               int64 // -s <offset> (which byte to start reading from)
	revert             bool  // -r Reverse operation: convert (or patch) hex dump into binary
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
	flag.Int64Var(&cfg.len, "l", -1, "Stop after writing <len> octets.")
	flag.Int64Var(&cfg.seek, "s", 0, "Start at <seek> bytes abs. (or rel.) infile offset.")

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
		if cfg.len >= 0 {
			cfg.endByte = cfg.len
		} else {
			cfg.endByte = info.Size()
		}

	} else if len(args) == 0 {
		cfg.file = os.Stdin
	}

	return cfg
}

func printLines(cfg Config) {
	offSetHex := cfg.seek
	offSetChar := cfg.seek
	reader := bufio.NewReader(cfg.file)
	_, err := cfg.file.Seek(cfg.seek, 0)

	if err != nil {
		fmt.Printf("error setting offset: %v", err)
		os.Exit(1)
	}

	for offSetHex < cfg.endByte {
		buffer := make([]byte, cfg.cols)
		_, err := io.ReadFull(reader, buffer)
		if err != nil {
			fmt.Printf("error: %v", err)
		}

		// Printing offset
		fmt.Printf("%08x: ", offSetHex)

		// Printing hex octs
		for i, byt := range buffer {
			fmt.Printf("%02x", byt)
			if (i+1)%2 == 0 {
				fmt.Printf(" ")
			}
			offSetHex++
			if offSetHex == cfg.endByte {
				printExtraSpace(len(buffer), i)
				break
			}
		}

		// Printing ascii
		for _, charByt := range buffer {
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

		fmt.Println()
	}
}

func isValidASCII(b byte) bool {
	return b >= 32 && b <= 126
}

func printExtraSpace(bufLen, index int) {
	missingBytes := bufLen - (index + 1)
	remainingSpace := missingBytes*2 + (missingBytes+1)/2
	for range remainingSpace {
		fmt.Printf(" ")
	}
}
