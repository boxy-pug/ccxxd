package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestXxdUnitRun(t *testing.T) {
	tests := []struct {
		name         string
		bytesPerLine int
		groupSize    int
		littleEndian bool
		maxBytes     int64
		startOffset  int64
		input        string
		want         string
	}{
		{
			name:         "Special chars, default config",
			bytesPerLine: 16,
			groupSize:    2,
			littleEndian: false,
			maxBytes:     -1,
			startOffset:  0,
			input:        "Hello123?$€Æ😊",
			want: `00000000: 4865 6c6c 6f31 3233 3f24 e282 acc3 86f0  Hello123?$......
00000010: 9f98 8a                                  ...
`,
		},
		{
			name:         "Little endian default config",
			bytesPerLine: 16,
			groupSize:    4,
			littleEndian: true,
			maxBytes:     -1,
			startOffset:  0,
			input:        "Hello123?$€Æ😊",
			want: `00000000: 6c6c6548 3332316f 82e2243f f086c3ac   Hello123?$......
00000010:   8a989f                              ...
`,
		},
		{
			name:         "Partial line at EOF",
			bytesPerLine: 8,
			groupSize:    2,
			littleEndian: false,
			maxBytes:     -1,
			startOffset:  0,
			input:        "ABCD",
			want:         "00000000: 4142 4344            ABCD\n",
		},
		{
			name:         "Column width 4, group 2",
			bytesPerLine: 4,
			groupSize:    2,
			littleEndian: false,
			maxBytes:     -1,
			startOffset:  0,
			input:        "123456",
			want: `00000000: 3132 3334  1234
00000004: 3536       56
`,
		},
		{
			name:         "Column width 11, group 5",
			bytesPerLine: 11,
			groupSize:    5,
			littleEndian: false,
			maxBytes:     -1,
			startOffset:  0,
			input:        "abcdefghijABCDEFGHIJ",
			want: `00000000: 6162636465 666768696a 41  abcdefghijA
0000000b: 4243444546 4748494a       BCDEFGHIJ
`,
		},
		{
			name:         "Little endian, group 4, cols 8, partial group",
			bytesPerLine: 8,
			groupSize:    4,
			littleEndian: true,
			maxBytes:     -1,
			startOffset:  0,
			input:        "ABCDE",
			want: `00000000: 44434241       45   ABCDE
`,
		},
		{
			name:         "Byte limit 5",
			bytesPerLine: 16,
			groupSize:    2,
			littleEndian: false,
			maxBytes:     5,
			startOffset:  0,
			input:        "abcdefghij",
			// If len=5, only first 5 bytes
			want: "00000000: 6162 6364 65                             abcde\n",
		},
		{
			name:         "Seek to byte 3",
			bytesPerLine: 16,
			groupSize:    2,
			littleEndian: false,
			maxBytes:     -1,
			startOffset:  3,
			input:        "abcdefghij",
			// If len=5, only first 5 bytes
			want: "00000003: 6465 6667 6869 6a                        defghij\n",
		},

		{
			name:         "Non-printable bytes",
			bytesPerLine: 8,
			groupSize:    2,
			littleEndian: false,
			maxBytes:     -1,
			startOffset:  0,
			input:        string([]byte{0x00, 0x01, 0x02, 0x41, 0x42, 0x43, 0x7f, 0x80}),
			want: `00000000: 0001 0241 4243 7f80  ...ABC..
`,
		},
		{
			name:         "All printable ASCII",
			bytesPerLine: 16,
			groupSize:    2,
			littleEndian: false,
			maxBytes:     -1,
			startOffset:  0,
			input:        " !\"#$%&'()*+,-./",
			want: `00000000: 2021 2223 2425 2627 2829 2a2b 2c2d 2e2f   !"#$%&'()*+,-./
`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			cmd := command{
				output:       &out,
				input:        strings.NewReader(tc.input),
				bytesPerLine: tc.bytesPerLine,
				groupSize:    tc.groupSize,
				littleEndian: tc.littleEndian,
				maxBytes:     tc.maxBytes,
				startOffset:  tc.startOffset,
			}
			err := cmd.run()
			assertNoError(t, err)

			got := out.String()
			assertEqual(t, got, tc.want)
		})
	}
}

func TestRevertToBinary(t *testing.T) {
	original := []byte("Hello, world!\n")
	hexDump := "00000000: 4865 6c6c 6f2c 2077 6f72 6c64 210a       Hello, world!.\n"

	input := strings.NewReader(hexDump)
	var output bytes.Buffer

	err := revertToBinary(input, &output)
	assertNoError(t, err)

	got := output.Bytes()
	if !bytes.Equal(got, original) {
		t.Errorf("output does not match original\nGOT:  %q\nWANT: %q", got, original)
	}
}

func assertNoError(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("did not expect error: %v", err)
	}
}

func assertEqual(t testing.TB, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("GOT:\n%s\n\nWANT:\n%s\n", got, want)
	}
}
