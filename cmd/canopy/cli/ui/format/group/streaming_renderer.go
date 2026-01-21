package group

import "io"

// OutputFunc outputs a reference to the given writer.
// This is the handler-specific behavior that varies between output modes.
type OutputFunc[R any] func(ref R, writer io.Writer)

// StatusFunc returns the grouping status for a reference.
// Returns (shouldGroup, completed) where:
// - shouldGroup: true if this reference should be grouped with others of the same status
// - completed: true if this reference has finished processing and is ready to output
type StatusFunc[R any] func(ref R) (shouldGroup bool, completed bool)

// DeleteFunc removes a reference from tracking and returns the updated slice.
type DeleteFunc[R any] func(ref R) []R

// StreamingGroupRenderer handles streaming output with optional grouping.
// It renders items incrementally, grouping consecutive items together when their
// status indicates they should be grouped.
//
// Unlike buffered approaches, this streams output incrementally:
// - Group header is written when first groupable item is ready
// - Item output streams as each item completes
// - Group footer is written when the group ends (non-groupable item or all done)
type StreamingGroupRenderer[R any] struct {
	writer         io.Writer
	config         Config
	streamingGroup *StreamingGroupWriter

	statusFn StatusFunc[R]
	outputFn OutputFunc[R]
}

// NewStreamingGroupRenderer creates a new renderer for streaming grouped output.
// - writer: destination for output
// - config: grouping configuration
// - statusFn: returns (shouldGroup, completed) for a reference
// - outputFn: outputs a reference to a writer
func NewStreamingGroupRenderer[R any](
	writer io.Writer,
	config Config,
	statusFn StatusFunc[R],
	outputFn OutputFunc[R],
) *StreamingGroupRenderer[R] {
	return &StreamingGroupRenderer[R]{
		writer:   writer,
		config:   config,
		statusFn: statusFn,
		outputFn: outputFn,
	}
}

// RenderWithGrouping processes references with streaming output grouping.
// It iterates through refs, outputting completed items and grouping consecutive
// items that have the same groupable status.
//
// Processing stops when an incomplete item is encountered (the streaming group
// stays open for subsequent calls). The deleteFn is called for each completed
// item to remove it from tracking and get the updated slice.
//
// Parameters:
// - refs: current slice of references to process
// - deleteFn: removes a processed reference and returns updated slice
func (r *StreamingGroupRenderer[R]) RenderWithGrouping(refs []R, deleteFn DeleteFunc[R]) {
	for len(refs) > 0 {
		ref := refs[0]
		shouldGroup, completed := r.statusFn(ref)

		if !completed {
			// item not done yet - return (streaming group stays open)
			return
		}

		if shouldGroup {
			// ensure streaming group is started
			if r.streamingGroup == nil {
				title := r.config.GroupedStatusLabel() + " packages"
				r.streamingGroup = NewStreamingGroupWriter(
					r.writer, title, r.config.Formatter)
			}
			if !r.streamingGroup.Started() {
				_ = r.streamingGroup.Start()
			}
			// output directly to streaming writer (incremental)
			r.outputFn(ref, r.streamingGroup)
		} else {
			// non-groupable: close any open group first
			if r.streamingGroup != nil && r.streamingGroup.Started() {
				_ = r.streamingGroup.End()
				r.streamingGroup = nil
			}
			// output directly to main writer
			r.outputFn(ref, r.writer)
		}

		refs = deleteFn(ref)
	}
	// note: we do NOT close the streaming group here because refs being empty
	// doesn't mean all packages are done - it just means all currently known
	// packages have been processed. More packages may arrive later (e.g., with
	// glob patterns like ./...). Call Close() explicitly when done.
}

// Close closes any open streaming group. Should be called when rendering is complete.
func (r *StreamingGroupRenderer[R]) Close() {
	if r.streamingGroup != nil && r.streamingGroup.Started() {
		_ = r.streamingGroup.End()
		r.streamingGroup = nil
	}
}
