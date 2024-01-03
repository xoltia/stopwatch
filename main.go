package main

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"
)

var InitTime = time.Now()

const Version = "0.1.3"

type OutputType uint8

const (
	String OutputType = iota
	Seconds
	Milliseconds
)

type StopwatchEntries map[string]time.Time

func (s StopwatchEntries) Add(id string, startTime time.Time) {
	s[id] = startTime
}

func (s StopwatchEntries) Clear(id string) time.Duration {
	if startTime, ok := s[id]; ok {
		delete(s, id)
		return InitTime.Sub(startTime)
	}

	return 0
}

func StopwatchPath() string {
	if path := os.Getenv("XDG_DATA_HOME"); path != "" {
		return path + "/stopwatch"
	}

	return os.Getenv("HOME") + "/.local/share/stopwatch"
}

func RemoveStopwatchFile() (err error) {
	filename := path.Join(StopwatchPath(), "stopwatch.json")
	err = os.Remove(filename)
	return
}

func OpenStopwatchFile() (file *os.File, err error) {
	filename := path.Join(StopwatchPath(), "stopwatch.json")

	if err = os.MkdirAll(path.Dir(filename), os.ModePerm); err != nil {
		err = fmt.Errorf("error creating directory: %s: %s", path.Dir(filename), err)
		return
	}

	file, err = os.OpenFile(filename, os.O_RDWR|os.O_CREATE, os.ModePerm)
	if err != nil {
		err = fmt.Errorf("error opening file: %s: %s", filename, err)
		return
	}
	err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX)
	if err != nil {
		err = fmt.Errorf("error locking file: %s", err)
	}
	return
}

func ReadStopwatchFile(file *os.File) (entries StopwatchEntries, err error) {
	entries = make(StopwatchEntries)
	stat, err := file.Stat()

	if err != nil {
		err = fmt.Errorf("error getting file stats: %s", err)
		return
	}

	if stat.Size() > 0 {
		err = json.NewDecoder(file).Decode(&entries)
		if err != nil {
			err = fmt.Errorf("error decoding entries: %s", err)
			return
		}
	}

	return
}

func WriteStopwatchFile(file *os.File, entries StopwatchEntries) (err error) {
	if err = file.Truncate(0); err != nil {
		err = fmt.Errorf("error truncating file: %s", err)
		return
	}

	if _, err = file.Seek(0, 0); err != nil {
		err = fmt.Errorf("error seeking file: %s", err)
		return
	}

	err = json.NewEncoder(file).Encode(entries)
	if err != nil {
		err = fmt.Errorf("error encoding entries: %s", err)
	}
	return
}

func CloseStopwatchFile(file *os.File) (err error) {
	err = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	if err != nil {
		err = fmt.Errorf("error unlocking file: %s", err)
		return
	}
	err = file.Close()
	return
}

// Print usage
func Usage() {
	out := flag.CommandLine.Output()
	fmt.Fprintln(out, "Usage:")
	fmt.Fprintln(out, "  stopwatch -start [-n <name>]")
	fmt.Fprintln(out, "  stopwatch -stop <id> [-s | -ms]")
	fmt.Fprintln(out, "  stopwatch -ls [-s | -ms]")
	fmt.Fprintln(out, "  stopwatch -wait [-s | -ms] [-l]")
	fmt.Fprintln(out, "  stopwatch -purge [-y]")
	fmt.Fprintln(out, "  stopwatch -h")
	fmt.Fprintln(out, "  stopwatch -v")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Options:")
	flag.PrintDefaults()
}

// Start a new stopwatch and print id
func Start(id string) int {
	if id == "" {
		b := make([]byte, 8)
		binary.BigEndian.PutUint32(b, uint32(time.Now().Unix()))
		if _, err := rand.Read(b[4:]); err != nil {
			fmt.Fprintf(os.Stderr, "error generating random id: %s\n", err)
			return 1
		}

		id = hex.EncodeToString(b)
	}

	file, err := OpenStopwatchFile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening file %s\n", err)
		return 1
	}

	defer func() {
		if err = CloseStopwatchFile(file); err != nil {
			fmt.Fprintf(os.Stderr, "error closing file %s: %s\n", file.Name(), err)
		}
	}()

	entries, err := ReadStopwatchFile(file)

	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading file %s: %s\n", file.Name(), err)
		return 1
	}

	entries.Add(id, InitTime)

	if err = WriteStopwatchFile(file, entries); err != nil {
		fmt.Fprintf(os.Stderr, "error writing file %s: %s\n", file.Name(), err)
		return 1
	}

	fmt.Println(id)
	return 0
}

// Stop a stopwatch and print duration
func Stop(id string, outputType OutputType) int {
	file, err := OpenStopwatchFile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening file %s\n", err)
		return 1
	}

	defer func() {
		if err = CloseStopwatchFile(file); err != nil {
			fmt.Fprintf(os.Stderr, "error closing file %s: %s\n", file.Name(), err)
		}
	}()

	entries, err := ReadStopwatchFile(file)

	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading file %s: %s\n", file.Name(), err)
		return 1
	}

	duration := entries.Clear(id)

	if duration == 0 {
		fmt.Fprintf(os.Stderr, "no stopwatch with id %s found\n", id)
		return 1
	}

	if err = WriteStopwatchFile(file, entries); err != nil {
		fmt.Fprintf(os.Stderr, "error writing file %s: %s\n", file.Name(), err)
		return 1
	}

	fmt.Println(DurationString(duration, outputType))
	return 0
}

// List all running stopwatches
func List(outputType OutputType) int {
	file, err := OpenStopwatchFile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening file %s\n", err)
		return 1
	}

	defer func() {
		if err = CloseStopwatchFile(file); err != nil {
			fmt.Fprintf(os.Stderr, "error closing file %s: %s\n", file.Name(), err)
		}
	}()

	entries, err := ReadStopwatchFile(file)

	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading file %s: %s\n", file.Name(), err)
		return 1
	}

	for id, startTime := range entries {
		fmt.Printf("%s\t", id)
		fmt.Printf("%s\t", DurationString(InitTime.Sub(startTime), outputType))
		fmt.Println(startTime.Format(time.RFC3339))
	}

	return 0
}

// Wait for SIGINT and print duration
func Wait(live bool, outputType OutputType) int {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	if !live {
		<-signalChan
		fmt.Print("\033[2K\r")
		fmt.Println(DurationString(time.Since(InitTime), outputType))
		return 0
	}

	ticker := time.NewTicker(time.Millisecond * 100)
	defer ticker.Stop()

	for {
		select {
		case <-signalChan:
			fmt.Print("\033[2K\r")
			fmt.Println(DurationString(time.Since(InitTime), outputType))
			return 0
		case <-ticker.C:
			fmt.Print("\033[2K\r")
			fmt.Printf("%s", DurationString(time.Since(InitTime).Round(time.Millisecond*100), outputType))
		}
	}
}

func Purge(skipConfirmation bool) int {
	if !skipConfirmation {
		fmt.Fprintf(os.Stderr, "Are you sure you want to remove the stopwatch file? [y/N] ")
		var answer string

		if _, err := fmt.Scanln(&answer); err != nil {
			fmt.Fprintf(os.Stderr, "error reading input: %s\n", err)
			return 1
		}

		if answer != "y" && answer != "Y" {
			return 0
		}
	}

	if err := RemoveStopwatchFile(); err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(os.Stderr, "error removing stopwatch file: %s\n", err)
		return 1
	}
	return 0
}

// Format duration according to output type
func DurationString(d time.Duration, format OutputType) string {
	if format == Seconds {
		return fmt.Sprintf("%f", d.Seconds())
	} else if format == Milliseconds {
		return fmt.Sprintf("%d", d.Milliseconds())
	} else {
		return fmt.Sprintf("%s", d)
	}
}

var (
	startFlag        = flag.Bool("start", false, "start a new stopwatch")
	idFlag           = flag.String("n", "", "id of the stopwatch")
	stopFlag         = flag.String("stop", "", "stop a stopwatch")
	listFlag         = flag.Bool("ls", false, "list all running stopwatches")
	versionFlag      = flag.Bool("v", false, "print version")
	secondsFlag      = flag.Bool("s", false, "output duration in seconds")
	millisecondsFlag = flag.Bool("ms", false, "output duration in milliseconds")
	waitingFlag      = flag.Bool("wait", false, "start a new stopwatch and wait for SIGINT (does not write to file)")
	liveFlag         = flag.Bool("l", false, "live output (only with -wait)")
	purgeFlag        = flag.Bool("purge", false, "remove stopwatch file")
	confirmFlag      = flag.Bool("y", false, "skip confirmation (only with -purge)")
)

func main() {
	flag.Parse()
	flag.Usage = Usage

	var outputType OutputType

	if *secondsFlag {
		outputType = Seconds
	} else if *millisecondsFlag {
		outputType = Milliseconds
	} else {
		outputType = String
	}

	if *versionFlag {
		fmt.Println(Version)
		os.Exit(0)
	} else if *startFlag {
		os.Exit(Start(*idFlag))
	} else if *stopFlag != "" {
		os.Exit(Stop(*stopFlag, outputType))
	} else if *listFlag {
		os.Exit(List(outputType))
	} else if *waitingFlag {
		os.Exit(Wait(*liveFlag, outputType))
	} else if *purgeFlag {
		os.Exit(Purge(*confirmFlag))
	}

	flag.Usage()
	os.Exit(1)
}
