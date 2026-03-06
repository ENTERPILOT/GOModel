package providers

import (
	"bytes"
	"io"
)

var responsesDoneMarker = []byte("data: [DONE]\n\n")

var responsesDoneLine = []byte("data: [DONE]")

var responsesCompletionPatterns = [][]byte{
	[]byte(`"type":"response.completed"`),
	[]byte(`"type":"response.done"`),
}

// EnsureResponsesDone normalizes Responses API streams so clients always receive
// a terminal data: [DONE] marker when the upstream stream reaches a completed
// Responses event but closes at EOF before sending the final marker.
func EnsureResponsesDone(stream io.ReadCloser) io.ReadCloser {
	if stream == nil {
		return nil
	}

	return &responsesDoneWrapper{
		ReadCloser: stream,
		tail:       make([]byte, 0, responsesDoneTrackOverlap()),
	}
}

type responsesDoneWrapper struct {
	io.ReadCloser
	tail         []byte
	pending      []byte
	sawDone      bool
	sawCompleted bool
	emitted      bool
}

func (w *responsesDoneWrapper) Read(p []byte) (int, error) {
	if len(w.pending) > 0 {
		n := copy(p, w.pending)
		w.pending = w.pending[n:]
		if len(w.pending) == 0 {
			w.emitted = true
		}
		return n, nil
	}

	if w.emitted {
		return 0, io.EOF
	}

	n, err := w.ReadCloser.Read(p)
	if n > 0 {
		w.trackDone(p[:n])
	}

	if err == io.EOF {
		if w.sawDone {
			if n > 0 {
				return n, nil
			}
			return 0, io.EOF
		}

		if !w.sawCompleted {
			if n > 0 {
				return n, nil
			}
			return 0, io.EOF
		}

		if n > 0 {
			w.pending = append(w.pending[:0], responsesDoneMarker...)
			return n, nil
		}

		n = copy(p, responsesDoneMarker)
		if n < len(responsesDoneMarker) {
			w.pending = append(w.pending[:0], responsesDoneMarker[n:]...)
			return n, nil
		}

		w.emitted = true
		return n, nil
	}

	return n, err
}

func (w *responsesDoneWrapper) trackDone(data []byte) {
	if w.sawDone && w.sawCompleted {
		return
	}

	combined := append(append([]byte(nil), w.tail...), data...)
	if !w.sawDone && bytes.Contains(combined, responsesDoneLine) {
		w.sawDone = true
	}
	if !w.sawCompleted {
		for _, pattern := range responsesCompletionPatterns {
			if bytes.Contains(combined, pattern) {
				w.sawCompleted = true
				break
			}
		}
	}

	overlap := responsesDoneTrackOverlap()
	if len(combined) > overlap {
		combined = combined[len(combined)-overlap:]
	}

	w.tail = append(w.tail[:0], combined...)
}

func responsesDoneTrackOverlap() int {
	overlap := len(responsesDoneLine) - 1
	for _, pattern := range responsesCompletionPatterns {
		if n := len(pattern) - 1; n > overlap {
			overlap = n
		}
	}
	return overlap
}
