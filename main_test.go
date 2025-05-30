package main

import (
	"log"
	"os"
	"os/exec"
	"testing"
)

var testFiles = getTestFiles("./testdata/")

func TestXxdUnit(t *testing.T) {
	t.Run("xxd no flags", func(t *testing.T) {
		for _, testFile := range testFiles {
			cmd := exec.Command("./ccxxd", testFile)
			got, err := cmd.Output()
			assertNoError(t, err)

			unixCmd := exec.Command("xxd", testFile)
			want, err := unixCmd.Output()
			assertNoError(t, err)

			assertEqual(t, string(got), string(want))
		}
	})

	// Failing bcs doesnt support right to left printing
	t.Run("little endian -e", func(t *testing.T) {
		for _, testFile := range testFiles {
			cmd := exec.Command("./ccxxd", "-e", testFile)
			got, err := cmd.Output()
			assertNoError(t, err)

			unixCmd := exec.Command("xxd", "-e", testFile)
			want, err := unixCmd.Output()
			assertNoError(t, err)

			assertEqual(t, string(got), string(want))
		}
	})

	t.Run("column flag -c 4", func(t *testing.T) {
		for _, testFile := range testFiles {
			cmd := exec.Command("./ccxxd", "-c", "4", testFile)
			got, err := cmd.Output()
			assertNoError(t, err)

			unixCmd := exec.Command("xxd", "-c", "4", testFile)
			want, err := unixCmd.Output()
			assertNoError(t, err)

			assertEqual(t, string(got), string(want))
		}
	})

	t.Run("stop writing after -l octets", func(t *testing.T) {
		for _, testFile := range testFiles {
			cmd := exec.Command("./ccxxd", "-l", "6", testFile)
			got, err := cmd.Output()
			assertNoError(t, err)

			unixCmd := exec.Command("xxd", "-l", "6", testFile)
			want, err := unixCmd.Output()
			assertNoError(t, err)

			assertEqual(t, string(got), string(want))
		}
	})

	t.Run("combining little endian -e and bytegrouping flag -g 4", func(t *testing.T) {
		for _, testFile := range testFiles {
			cmd := exec.Command("./ccxxd", "-e", "-g", "4", testFile)
			got, err := cmd.Output()
			assertNoError(t, err)

			unixCmd := exec.Command("xxd", "-e", "-g", "4", testFile)
			want, err := unixCmd.Output()
			assertNoError(t, err)

			assertEqual(t, string(got), string(want))
		}
	})

	t.Run("combining -g 6 and -c 10", func(t *testing.T) {
		for _, testFile := range testFiles {
			cmd := exec.Command("./ccxxd", "-c", "10", "-g", "5", testFile)
			got, err := cmd.Output()
			assertNoError(t, err)

			unixCmd := exec.Command("xxd", "-c", "10", "-g", "5", testFile)
			want, err := unixCmd.Output()
			assertNoError(t, err)

			assertEqual(t, string(got), string(want))
		}
	})

	t.Run("combining -g 5 and -c 13", func(t *testing.T) {
		for _, testFile := range testFiles {
			cmd := exec.Command("./ccxxd", "-c", "13", "-g", "5", testFile)
			got, err := cmd.Output()
			assertNoError(t, err)

			unixCmd := exec.Command("xxd", "-c", "13", "-g", "5", testFile)
			want, err := unixCmd.Output()
			assertNoError(t, err)

			assertEqual(t, string(got), string(want))
		}
	})

	t.Run("combining -g 3 and -c 8", func(t *testing.T) {
		for _, testFile := range testFiles {
			cmd := exec.Command("./ccxxd", "-c", "8", "-g", "3", testFile)
			got, err := cmd.Output()
			assertNoError(t, err)

			unixCmd := exec.Command("xxd", "-c", "8", "-g", "3", testFile)
			want, err := unixCmd.Output()
			assertNoError(t, err)

			assertEqual(t, string(got), string(want))
		}
	})

	// TODO failing cus formatting spaces before ascii, or error?
	// failing if you add -g 3 to this
	t.Run("combining -c 8 and endian -e", func(t *testing.T) {
		for _, testFile := range testFiles {
			cmd := exec.Command("./ccxxd", "-e", "-c", "8", testFile)
			got, err := cmd.Output()
			assertNoError(t, err)

			unixCmd := exec.Command("xxd", "-e", "-c", "8", testFile)
			want, err := unixCmd.Output()
			assertNoError(t, err)

			assertEqual(t, string(got), string(want))
		}
	})

	t.Run("seeking to specific byte start with -s", func(t *testing.T) {
		for _, testFile := range testFiles {
			cmd := exec.Command("./ccxxd", "-s", "10", testFile)
			got, err := cmd.Output()
			assertNoError(t, err)

			unixCmd := exec.Command("xxd", "-s", "10", testFile)
			want, err := unixCmd.Output()
			assertNoError(t, err)

			assertEqual(t, string(got), string(want))
		}
	})
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

func getTestFiles(testFolder string) []string {
	var res []string

	files, err := os.ReadDir(testFolder)
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		res = append(res, testFolder+file.Name())
	}
	return res
}
