package runtime

import (
	"context"
	"io"
	"time"
)

// Logs also observes container state because Podman can miss the exit event
// between opening a follow stream and subscribing to container events.
func (p *Podman) Logs(ctx context.Context, id string, w io.Writer) error {
	container, err := p.Inspect(ctx, id)
	if err != nil {
		return err
	}
	if !container.Running {
		return p.logs(ctx, id, w, false)
	}
	followCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	counted := &countedLogs{writer: w}
	done := make(chan error, 1)
	go func() { done <- p.logs(followCtx, id, counted, true) }()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case err := <-done:
			return err
		case <-ctx.Done():
			cancel()
			<-done
			return ctx.Err()
		case <-ticker.C:
			container, err := p.Inspect(ctx, id)
			if err != nil {
				cancel()
				<-done
				return err
			}
			if container.Running {
				continue
			}
			cancel()
			<-done
			if counted.err != nil {
				return counted.err
			}
			// Read the final log without following. Skip the prefix already sent,
			// including any partial frame interrupted when the container exited.
			return p.logs(ctx, id, &remainingLogs{writer: w, skip: counted.bytes}, false)
		}
	}
}

type countedLogs struct {
	writer io.Writer
	bytes  int64
	err    error
}

func (w *countedLogs) Write(data []byte) (int, error) {
	n, err := w.writer.Write(data)
	w.bytes += int64(n)
	if err != nil {
		w.err = err
	}
	return n, err
}

type remainingLogs struct {
	writer io.Writer
	skip   int64
}

func (w *remainingLogs) Write(data []byte) (int, error) {
	skipped := min(w.skip, int64(len(data)))
	w.skip -= skipped
	if skipped == int64(len(data)) {
		return len(data), nil
	}
	n, err := w.writer.Write(data[skipped:])
	return int(skipped) + n, err
}
