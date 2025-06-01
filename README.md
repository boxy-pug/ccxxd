# ccxxd

## Testing

**Unit tests:**  
Run with `go test`.  
These check the core logic and formatting in-memory—no system `xxd` needed.

**Integration tests:**  
Compare this tool's output to your system's `xxd`.  
Requires `xxd` installed and output might differ slightly on other systems or OSes.  
Not run by default. To run them:

`go test -tags=integration`

## 🧠 What I learned

-  Newlines and special chars in test data caused confusing test failures, invisible trailing newlines etc.
-  Using `echo` in the shell tripped me up when piping to xxd, since `echo` doesn't always handle special characters, Unicode, or escape chars the expected way. Using the echo -n flag for surpressing trailing newlines is one tip, also remember to use single quotes '' to treat everything inside as literal string, with "" special chars are interpreted.
-  `printf` command could be used instead, more consistent, does not add newline.
- I first used `bufio.Reader.Read` for reading lines, but it can return fewer bytes than requested even if there's more data to read, which caused short lines to appear in the middle of the hex dump. `io.ReadFull` keeps reading until the buffer is full or EOF, so now only the last line can be short – this matches what real hex dump tools like xxd do.               
-  Instead of writing each thing directly to stdout, use `strings.Builder` to build the whole line in memory first. Writing to stdout (or any io.Writer) is expensive compared to working in RAM, especially for lots of small writes. By building the line in RAM I reduce the number of system calls and get more predictable, flicker-free output. Assembling output before printing is generally good.
