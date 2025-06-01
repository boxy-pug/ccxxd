package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name   string
		cols   int
		group  int
		endian bool
		input  string
		want   string
	}{
		{
			name:   "Special chars, default config",
			cols:   16,
			group:  2,
			endian: false,
			input:  "Hello123?$€Æ😊",
			want: `00000000: 4865 6c6c 6f31 3233 3f24 e282 acc3 86f0  Hello123?$......
00000010: 9f98 8a                                  ...
`,
		},
		{
			name:   "Little endian default config",
			cols:   16,
			group:  4,
			endian: true,
			input:  "Hello123?$€Æ😊",
			want: `00000000: 6c6c6548 3332316f 82e2243f f086c3ac   Hello123?$......
00000010:   8a989f                              ...
`,
		},
		{
			name:   "Partial line at EOF",
			cols:   8,
			group:  2,
			endian: false,
			input:  "ABCD",
			want:   "00000000: 4142 4344            ABCD\n",
		},
		{
			name:   "Column width 4, group 2",
			cols:   4,
			group:  2,
			endian: false,
			input:  "123456",
			want: `00000000: 3132 3334  1234
00000004: 3536       56
`,
		},
		{
			name:   "Column width 11, group 5",
			cols:   11,
			group:  5,
			endian: false,
			input:  "abcdefghijABCDEFGHIJ",
			want: `00000000: 6162636465 666768696a 41  abcdefghijA
0000000b: 4243444546 4748494a       BCDEFGHIJ
`,
		},
		{
			name:   "Little endian, group 4, cols 8, partial group",
			cols:   8,
			group:  4,
			endian: true,
			input:  "ABCDE",
			want: `00000000: 44434241       45   ABCDE
`,
		},

		/*
					{
						name:   "Seek offset (simulate by skipping bytes)",
						cols:   8,
						group:  2,
						endian: false,
						input:  "1234567890",
						// If seek=2, skips '1' and '2'
						want: `00000000: 3334 3536 3738 3930  34567890
			`,
					},
					{
						name:   "Byte limit (simulate with short input)",
						cols:   8,
						group:  2,
						endian: false,
						input:  "abcdefghij",
						// If len=5, only first 5 bytes
						want: `00000000: 6162 6364 65         abcde
			`,
					},
					{
						name:   "Non-printable bytes",
						cols:   8,
						group:  2,
						endian: false,
						input:  string([]byte{0x00, 0x01, 0x02, 0x41, 0x42, 0x43, 0x7f, 0x80}),
						want: `00000000: 0001 0241 4243 7f80  ...ABC..
			`,
					},
					{
						name:   "All printable ASCII",
						cols:   16,
						group:  4,
						endian: false,
						input:  " !\"#$%&'()*+,-./",
						want: `00000000: 2021 2223 2425 2627 2829 2a2b 2c2d 2e2f   !"#$%&'()*+,-./
			`,
					},
		*/
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			cmd := command{
				output:       &out,
				file:         strings.NewReader(tc.input),
				cols:         tc.cols,
				group:        tc.group,
				littleEndian: tc.endian,
				endByte:      int64(len(tc.input)),
			}
			err := cmd.run()
			assertNoError(t, err)

			got := out.String()
			assertEqual(t, got, tc.want)
		})
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
