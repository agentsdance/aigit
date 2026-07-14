package main

import (
	"bytes"
	"fmt"
	"regexp"
)

const maxDiffBytes = 1_800_000

var binaryFileRe = regexp.MustCompile(`(?m)^Binary files (.+) and (.+) differ$`)

// processBinaryDiff removes git's "Binary files a/x and b/x differ" lines from
// the diff so the LLM only sees textual changes.
func processBinaryDiff(diff []byte) []byte {
	return binaryFileRe.ReplaceAll(diff, nil)
}

// truncateDiff trims the diff to maxDiffBytes when it exceeds the API's
// character limit, keeping line boundaries intact.
func truncateDiff(diff []byte) []byte {
	if len(diff) <= maxDiffBytes {
		return diff
	}

	truncated := diff[:maxDiffBytes]
	if last := bytes.LastIndexByte(truncated, '\n'); last > 0 {
		truncated = truncated[:last]
	}
	note := fmt.Sprintf("\n\n... diff truncated (%d/%d bytes). Only the first portion is shown.\n", maxDiffBytes, len(diff))
	return append(truncated, note...)
}