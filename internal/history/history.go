package history

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/oSEAItic/tidal/internal/runner"
)

// Record is a single entry in the history log.
type Record struct {
	Timestamp string          `json:"ts"`
	Command   string          `json:"command"`
	Summary   *runner.Summary `json:"summary"`
}

// Append writes a run result to the history file.
func Append(dir string, env runner.Envelope) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path := filepath.Join(dir, "history.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	rec := Record{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Command:   env.Command,
		Summary:   env.Summary,
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(f, "%s\n", data)
	return err
}

// Read returns the last N records from the history file.
func Read(dir string, limit int) ([]Record, error) {
	path := filepath.Join(dir, "history.jsonl")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var all []Record
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var r Record
		if err := json.Unmarshal(scanner.Bytes(), &r); err != nil {
			continue
		}
		all = append(all, r)
	}

	if limit > 0 && len(all) > limit {
		all = all[len(all)-limit:]
	}
	return all, scanner.Err()
}
