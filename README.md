# Stopwatch
A simple CLI stopwatch written in Go.

## Installation
```bash
go install github.com/xoltia/stopwatch
```

## Usage
```bash
# Start the stopwatch
sid=$(stopwatch -start)
# Stop the stopwatch and output string (ex. 1h2m3s4ms)
stopwatch -stop $sid
# Stop the stopwatch and output milliseconds
stopwatch -ms -stop $sid
# Stop the stopwatch and output seconds
stopwatch -s -stop $sid

# Start stopwatch with a custom name
stopwatch -start -n "My Stopwatch"
# Stop the stopwatch with a custom name
stopwatch -stop "My Stopwatch"

# List all (non-blocking) stopwatches
stopwatch -ls

# Start a blocking stopwatch
# Prints elapsed time when stopped
stopwatch -wait

# Start a blocking stopwatch and show live time
stopwath -wait -l
```
