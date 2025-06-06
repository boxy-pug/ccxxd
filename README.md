# ccxxd

A hex dump `xxd` clone written in Go, built as a learning project for [CodingChallenges.fyi](https://codingchallenges.fyi/challenges/challenge-xxd).

This tool displays a hex dump of any file or standard input, like the classic `xxd` utility. It supports grouping bytes (`-g`), custom column widths (`-c`), little-endian output (`-e`), offset (`-s`) and length control (`-l`), and can also reverse a hex dump back into binary (`-r`).

## 🛠️ Usage

```sh
ccxxd myfile.bin
# Hex dump of myfile.bin

ccxxd -c 8 -g 4 myfile.bin
# 8 bytes per line, groups of 4

ccxxd -e myfile.bin
# Little-endian hex output

ccxxd -r hex.txt > out.bin
# Convert hex dump back to binary

cat myfile.bin | ccxxd
# Hex dump from stdin

ccxxd -s 10 -l 32 myfile.bin
# Dump 32 bytes starting from offset 10
```

## 📀 Installation

**Build from source:**

Clone this repository and run:
```sh
go build -o ccxxd
```
This will create the `ccxxd` binary in your current directory.

**Or install directly with Go:**

```sh
go install github.com/boxy-pug/ccxxd@latest
```
This will place the `ccxxd` binary in your `$GOPATH/bin` or `$GOBIN` directory. Make sure that directory is in your `PATH` to run `ccxxd` from anywhere.
 

## Testing

**Unit tests:**  
Run with `go test`.  
These check the core logic and formatting in-memory.

**Integration tests:**  
Compare this tool's output to your system's `xxd`.  
Requires `xxd` installed, and if your local implementation differ slightly in formatting the tests will not pass.
Therefore they're not run by default. To run them:

```sh
go test -tags=integration
```                           

## 🧠 What I learned

-   All files are just bytes: I guess i knew this, but building this tool made me realize it more directly: text, images, and programs are just different sequences of bytes on disk. Reverting a hex dump restores the original file exactly, so even executables work after chmod +x. Writing bytes directly (not as text) is what makes the file truly identical to the original.
-   Newlines and special chars in test data caused confusing test failures, invisible trailing newlines etc. Use `echo -n` flag for suppressing trailing newlines.
-   `printf` command could be used instead, more consistent, does not add newline.
-   In the shell, single quotes `'...'` treat everything inside as literal text, so special characters and escape sequences like `\n` are not interpreted. Double quotes `"..."` allow for variable expansion and escape sequences, so `\n` becomes a real newline.
-   I first used `bufio.Reader.Read` for reading lines, but it can return fewer bytes than requested even if there's more data to read, which caused short lines to appear in the middle of the hex dump. `io.ReadFull` keeps reading until the buffer is full or EOF, so now only the last line can be short – this matches the real xxd output.
-   Instead of writing each thing/byte/piece directly to stdout, use `strings.Builder` to build whole line or chunk in memory first. Writing to stdout (or any io.Writer) is expensive compared to working in RAM, especially for lots of small writes. By building the line in RAM I reduce the number of system calls and get more predictable, flicker-free output. Assembling output before printing is generally good.
-  For checking if a number is a power of two: `n > 0 && (n & (n-1)) == 0`, works because only binary nums with one positive bit is power of two. 


## License

This project is for fun and learning.  
Feel free to check out, use, or modify the code as you like!

Licensed under the MIT License.

