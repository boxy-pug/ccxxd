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
	//fmt.Println(cfg.seek)

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
	} else if len(args) == 0 {
		cfg.file = os.Stdin
	}

	cfg.endByte = getEndByte(cfg.len, cfg.file)

	cfg.byteGrouping = validateByteGrouping(cfg.byteGrouping, cfg.cols)

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
		bytesRead, err := io.ReadFull(reader, buffer)
		if err != nil {
			fmt.Printf("error: %v", err)
		}

		// Printing offset
		fmt.Printf("%08x: ", offSetHex)

		// Printing hex octs

		for i, byt := range buffer {
			fmt.Printf("%02x", byt)
			if (i+1)%cfg.byteGrouping == 0 {
				fmt.Printf(" ")
			}
			offSetHex++
			if offSetHex == cfg.endByte {
				printExtraSpace(len(buffer), bytesRead)
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
