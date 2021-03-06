package sink

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/geckoboard/prism/profiler"
)

var (
	profilePrefix = "profile-"
	badCharRegex  = regexp.MustCompile(`[\./\\]`)
)

type fileSink struct {
	outputDir string
	sigChan   chan struct{}
	inputChan chan *profiler.Profile
}

// NewFileSink creates a new profile entry sink instance which stores profiles
// to disk at the folder specified by outputDir.
func NewFileSink(outputDir string) profiler.Sink {
	return &fileSink{
		outputDir: outputDir,
		sigChan:   make(chan struct{}, 0),
	}
}

// Initialize the sink.
func (s *fileSink) Open(inputBufferSize int) error {
	// Ensure that ouptut folder exists
	err := os.MkdirAll(s.outputDir, os.ModeDir|os.ModePerm)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "profiler: saving profiles to %s\n", s.outputDir)

	s.inputChan = make(chan *profiler.Profile, inputBufferSize)

	// start worker and wait for ready signal
	go s.worker()
	<-s.sigChan
	return nil
}

// Shutdown the sink.
func (s *fileSink) Close() error {
	// Signal worker to exit and wait for confirmation
	close(s.inputChan)
	<-s.sigChan
	close(s.sigChan)
	return nil
}

// Get a channel for piping profile entries to the sink.
func (s *fileSink) Input() chan<- *profiler.Profile {
	return s.inputChan
}

func (s *fileSink) worker() {
	// Signal that worker has started
	s.sigChan <- struct{}{}
	defer func() {
		// Signal that we have stopped
		s.sigChan <- struct{}{}
	}()

	for {
		profile, sinkOpen := <-s.inputChan
		if !sinkOpen {
			return
		}

		fpath := outputFile(s.outputDir, profile, "json")
		f, err := os.Create(fpath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "profiler: could not create output file %q due to %s; dropping profile\n", fpath, err.Error())
			continue
		}

		data, err := json.Marshal(profile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "profiler: error marshalling profile: %s; dropping profile\n", err.Error())
			continue
		}
		f.Write(data)
		f.Close()
	}
}

// Construct the path to a profile file for this entry. This function will
// also pass the path through filepath.Clean to ensure that the proper slashes
// are used depending on the host OS.
func outputFile(outputDir string, profile *profiler.Profile, extension string) string {
	return filepath.Clean(
		fmt.Sprintf(
			"%s/%s%s-%d-%d.%s",
			outputDir,
			profilePrefix,
			badCharRegex.ReplaceAllString(profile.Target.FnName, "_"),
			profile.CreatedAt.UnixNano(),
			profile.ID,
			extension,
		),
	)
}
