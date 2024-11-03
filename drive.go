package main

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

const (
	logFilePath = "/tmp/progress.log" // Adjust path for Windows if necessary
	pidFilePath = "/tmp/copy_pid"
)

func main() {
	http.HandleFunc("/api/start", startCopyHandler)
	http.HandleFunc("/api/progress", getProgressHandler)
	http.HandleFunc("/api/stop", stopCopyHandler)

	fmt.Println("Starting server on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// startCopyHandler starts the file copy process
func startCopyHandler(w http.ResponseWriter, r *http.Request) {
	source := r.URL.Query().Get("source")
	destination := r.URL.Query().Get("destination")
	if source == "" || destination == "" {
		http.Error(w, "Source and destination are required", http.StatusBadRequest)
		return
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// Use robocopy on Windows
		cmd = exec.Command("robocopy", source, destination, "/MIR", "/NFL", "/NDL", "/NJH", "/NJS", "/NC", "/NP", "/LOG:"+logFilePath)
	} else {
		// Use rsync on macOS and Linux
		cmd = exec.Command("rsync", "-a", "--info=progress2", source, destination)
		// Redirect output to log file
		logFile, err := os.Create(logFilePath)
		if err != nil {
			http.Error(w, "Could not create log file", http.StatusInternalServerError)
			return
		}
		defer logFile.Close()
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	}

	if err := cmd.Start(); err != nil {
		http.Error(w, "Failed to start copy process", http.StatusInternalServerError)
		return
	}

	// Save PID to a file
	if err := os.WriteFile(pidFilePath, []byte(strconv.Itoa(cmd.Process.Pid)), 0644); err != nil {
		http.Error(w, "Could not save PID", http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "Copy started with PID: %d\n", cmd.Process.Pid)
}

// getProgressHandler reads the last few lines from the log file for progress updates
func getProgressHandler(w http.ResponseWriter, r *http.Request) {
	file, err := os.Open(logFilePath)
	if err != nil {
		http.Error(w, "No progress log found", http.StatusNotFound)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lines := []string{}
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > 10 { // Keep only the last 10 lines
			lines = lines[1:]
		}
	}

	if err := scanner.Err(); err != nil {
		http.Error(w, "Error reading progress log", http.StatusInternalServerError)
		return
	}

	progress := strings.Join(lines, "\n")
	fmt.Fprintf(w, "Progress:\n%s", progress)
}

// stopCopyHandler stops the file copy process
func stopCopyHandler(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile(pidFilePath)
	if err != nil {
		http.Error(w, "Copy process not running", http.StatusNotFound)
		return
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		http.Error(w, "Invalid PID file", http.StatusInternalServerError)
		return
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		http.Error(w, "Process not found", http.StatusNotFound)
		return
	}

	if err := process.Kill(); err != nil {
		http.Error(w, "Failed to stop process", http.StatusInternalServerError)
		return
	}

	os.Remove(pidFilePath)
	fmt.Fprintln(w, "Copy process stopped")
}

// CheckStatus checks if the copy process is still running
func statusHandler(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile(pidFilePath)
	if err != nil {
		http.Error(w, "Copy process not running", http.StatusNotFound)
		return
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		http.Error(w, "Invalid PID file", http.StatusInternalServerError)
		return
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		http.Error(w, "Process not found", http.StatusNotFound)
		return
	} else {

		fmt.Fprintln(w, "Copy is in process on"+strconv.Itoa(process.Pid))
	}

}
