//go:build windows

package state

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"os/exec"
	"strconv"
)

func processRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	out, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH").Output()
	if err != nil {
		return false
	}
	reader := csv.NewReader(bytes.NewReader(out))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return false
	}
	expected := strconv.Itoa(pid)
	for _, record := range records {
		if len(record) >= 2 && record[1] == expected {
			return true
		}
	}
	return false
}
