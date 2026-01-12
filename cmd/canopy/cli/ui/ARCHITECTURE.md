# UI Architecture

This document provides a comprehensive overview of Canopy's UI architecture, explaining how events flow from the test runner to the terminal, how components interact, and how state is managed throughout the system.

## Table of Contents

1. [Overview](#overview)
2. [CoreUI vs TeaUI](#coreui-vs-teaui)
3. [Component Architecture](#component-architecture)
4. [Event Propagation](#event-propagation)
5. [State Management](#state-management)
6. [Output Streams: Stderr vs Stdout](#output-streams-stderr-vs-stdout)
7. [Putting It All Together](#putting-it-all-together)

---

## Overview

Canopy's UI system is built on an **event-driven, layered architecture** that cleanly separates:

- **Event production** (test runner)
- **Event routing** (event bus)
- **Event processing** (handlers)
- **Data transformation** (adapters)
- **Output formatting** (presenters)
- **Display rendering** (UI layer)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Test Runner                                    │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐                     │
│  │  go test    │───▶│   JSONL     │───▶│   Event     │                     │
│  │  -json      │    │   Parser    │    │   Stream    │                     │
│  └─────────────┘    └─────────────┘    └──────┬──────┘                     │
└────────────────────────────────────────────────┼────────────────────────────┘
                                                 │
                                                 ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Event Bus (partybus)                           │
│                                                                             │
│    GoTestType ──────┬────────────────────────────────────────────────────   │
│    GoTestRunType ───┤                                                       │
│    PrintType ───────┤                                                       │
│    CLINotification ─┘                                                       │
└─────────────────────────────────────────────────────────────────────────────┘
                                                 │
                         ┌───────────────────────┴───────────────────────┐
                         │                                               │
                         ▼                                               ▼
              ┌──────────────────┐                            ┌──────────────────┐
              │     CoreUI       │                            │      TeaUI       │
              │  (non-interactive)│                            │   (interactive)  │
              └──────────────────┘                            └──────────────────┘
```

---

## CoreUI vs TeaUI

The UI system provides two implementations of the `clio.UI` interface, each optimized for different environments.

### CoreUI (`core_ui.go`)

**CoreUI** is the foundational event processor designed for **non-interactive environments** such as CI/CD pipelines, piped output, or any context where a TTY is not available.

```go
type coreUI struct {
    presenters     []presenter.Presenter   // Format final output
    handlers       []partybus.Handler      // Process events in real-time
    stdout         io.WriteCloser          // Output stream for reports
    stderr         io.WriteCloser          // Output stream for notifications
    teardownCalled bool                    // Prevents duplicate teardown
}
```

**Characteristics:**

| Aspect | CoreUI Behavior |
|--------|-----------------|
| **Output Model** | Buffered - handlers accumulate data, presenters write at teardown |
| **Interactivity** | None - events only, no keyboard/mouse input |
| **Display Updates** | No real-time updates, output is final when written |
| **Lifecycle** | `Setup()` → `Handle()` (repeated) → `Teardown()` |
| **Use Case** | CI/CD, file output, piped commands |

**Builder Pattern:**

```go
ux := newCoreUI().
    withNotifications().           // Handle CLINotification events
    withReports().                 // Handle CLIReport events
    withHandlers(testHandler).     // Add custom event handlers
    withHandledPresenters(adapter) // Add handler+presenter combinations
    withStdout(reportWriter).      // Set output destination
    withStderr(notificationWriter)
```

### TeaUI (`tea_ui.go`)

**TeaUI** wraps CoreUI and adds **interactive terminal capabilities** using the [Bubble Tea](https://github.com/charmbracelet/bubbletea) framework. It's designed for **TTY environments** where real-time updates enhance the user experience.

```go
type UI struct {
    config         TeaUIConfig           // Configuration including embedded CoreUI
    program        *tea.Program          // Bubble Tea program for rendering
    running        *sync.WaitGroup       // Tracks goroutine lifecycle
    subscription   partybus.Unsubscribable
    teardownCalled bool
}

type TeaUIConfig struct {
    handler       *bubbly.HandlerCollection   // Body event handlers (creates models)
    footerHandler *bubbly.HandlerCollection   // Footer event handlers
    coreUI        *coreUI                     // Embedded CoreUI for base functionality
    printReaders  []io.Reader                 // Streams handler output to display
    spinner       *syncspinner.Model          // Animation spinner
    frame         frameWithFooter             // Body + footer layout
}
```

**Characteristics:**

| Aspect | TeaUI Behavior |
|--------|----------------|
| **Output Model** | Real-time - renders to stderr, updates continuously |
| **Interactivity** | Full - keyboard input, escape to cancel |
| **Display Updates** | Live spinner, progress, running test indicators |
| **Lifecycle** | Same as CoreUI + Bubble Tea program lifecycle |
| **Use Case** | Interactive terminal sessions |

**Key Insight:** TeaUI **wraps** CoreUI rather than replacing it. This ensures:
- Consistent event handling across both modes
- CoreUI handles data accumulation and final presentation
- TeaUI adds visual polish and interactivity on top

```go
// TeaUI delegates to CoreUI for core event handling
func (m *UI) Handle(e partybus.Event) error {
    if m.program != nil {
        m.program.Send(e)           // Send to Bubble Tea for visual updates
    }
    return m.config.coreUI.Handle(e) // Delegate to CoreUI for processing
}
```

### Comparison Table

| Feature | CoreUI | TeaUI |
|---------|--------|-------|
| **Primary Output** | stdout/stderr directly | stderr via Bubble Tea |
| **Animation** | None | Spinner, progress indicators |
| **Keyboard Input** | None | Ctrl+C, Esc to interrupt |
| **Running Tests Display** | Not shown | Shown in footer |
| **Memory Model** | Event handlers write or buffer | Same + frame buffer for display |
| **Dependency** | Standard library | Requires Bubble Tea |

### When Each is Used

```go
// test_go_ui.go
func NewTestGoUI(cfg TestUIConfig, maxPkgNameLength int) clio.UI {
    if cfg.IsTTY && cfg.Writer == nil {
        return newDynamicGoUI(cfg, maxPkgNameLength)  // → TeaUI
    }
    return newSafeGoUI(cfg, maxPkgNameLength)         // → CoreUI
}
```

---

## Component Architecture

The UI system uses four main component types that form a processing pipeline:

```
Events → Handlers → Adapters → Presenters → Output
```

### Handlers

**Purpose:** Process events in real-time, track state, transform raw events into structured data.

**Interface:** (`format/handler/handler.go`)

```go
type Handler interface {
    BusHandler           // Handle(partybus.Event) error
    TestEventHandler     // OnGoTestEvent(gotest.Event) error
    fmt.Stringer         // String() - returns buffered output
}
```

**Implementations:**

| Handler | Location | Purpose |
|---------|----------|---------|
| `gostd.QuietHandler` | `format/handler/gostd/quiet_handler.go` | Shows only failures and package summaries |
| `gostd.VerboseHandler` | `format/handler/gostd/verbose_handler.go` | Shows all test events (RUN, PASS, FAIL) |
| `goxx.QuietPackage` | `format/handler/goxx/quiet_handler.go` | Enhanced quiet mode with better formatting |
| `handler.TestRun` | `format/handler/test_run.go` | Captures final test run result |
| `handler.Aggregator` | `format/handler/aggregator.go` | Collects all events of a specific type |

**Event Processing Example:**

```go
// gostd/quiet_handler.go
func (h *quietHandler) OnGoTestEvent(event gotest.Event) error {
    h.result.Update(event)  // Update internal state

    // When a package completes, render its output
    if event.Action == gotest.PassAction && event.Reference.IsPackage() {
        h.render(event.Reference)
    }
    return nil
}
```

### Presenters

**Purpose:** Format processed data into output strings with styling and structure.

**Interface:** (`format/presenter/presenter.go`)

```go
type Presenter interface {
    Present(stdout, stderr io.Writer) error
}
```

**Factory Pattern:**

```go
// Presenter factories allow configuration-time binding
type EventFactory func(e partybus.Event) Presenter
type TestRunFactory func(tr ...gotest.Run) Presenter
```

**Implementations:**

| Presenter | Location | Purpose |
|-----------|----------|---------|
| `goQuietEvent` | `format/presenter/go_quiet_event.go` | Formats individual events for quiet mode |
| `goVerboseEvent` | `format/presenter/go_verbose_event.go` | Formats events for verbose mode |
| `goSummary` | `format/presenter/go_summary.go` | Formats test run summary |
| `PrintEvent` | `format/presenter/print_event.go` | Simple string output |

### Adapters

**Purpose:** Bridge handlers and presenters by implementing both interfaces.

**Interface:** (`format/adapter/handled_presenters.go`)

```go
type HandledPresenter interface {
    partybus.Handler     // Receives and processes events
    presenter.Presenter  // Formats output
}
```

**This pattern eliminates coupling:** The adapter accumulates events during `Handle()` and formats them during `Present()`.

**Implementations:**

| Adapter | Location | Purpose |
|---------|----------|---------|
| `AllEvents` | `format/adapter/all_events.go` | Collects events, presents each with factory |
| `TestRun` | `format/adapter/test_run.go` | Captures test run, presents summary |

**Example:**

```go
// adapter/all_events.go
type AllEvents struct {
    *handler.Aggregator           // Embedded handler (collects events)
    factory presenter.EventFactory // Factory to create presenters
}

func (p AllEvents) Handle(e partybus.Event) error {
    return p.Aggregator.Handle(e)  // Delegate to embedded handler
}

func (p AllEvents) Present(stdout, stderr io.Writer) error {
    for _, e := range p.Events() {
        pres := p.factory(e)       // Create presenter for each event
        pres.Present(stdout, stderr)
    }
    return nil
}
```

### Components (Bubble Tea Models)

**Purpose:** Interactive UI elements for TeaUI mode.

**Location:** `format/model/bubble/`

| Component | Purpose |
|-----------|---------|
| `syncspinner` | Animated spinner synchronized across renders |
| `gosummary` | Live test statistics footer |
| `jestsummary` | Jest-style summary display |

**How Components Connect to TeaUI:**

```go
// TeaUI creates models via handlers when events arrive
func (m UI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case partybus.Event:
        // Body handlers create new models
        models, cmd := m.config.handler.Handle(msg)
        for _, newModel := range models {
            m.config.frame.body.AppendModel(newModel)
        }

        // Footer handlers create footer models
        if m.config.footerHandler != nil {
            footerModels, cmd := m.config.footerHandler.Handle(msg)
            for _, newModel := range footerModels {
                m.config.frame.footer.AppendModel(newModel)
            }
        }
    }
}
```

### Component Assembly Example

Here's how all components wire together in `test_go_ui.go`:

```go
func newDynamicGoUI(cfg TestUIConfig, maxPkgNameLength int) clio.UI {
    // 1. Create shared state
    spin := syncspinner.New()

    // 2. Create I/O pipes (handlers write, UI reads)
    reportReader, reportWriter := readerWriterPair()
    notificationReader, notificationWriter := readerWriterPair()

    // 3. Create handler (processes events, writes to reportWriter)
    var h handler.Handler
    if cfg.Verbose > 0 {
        h = gostd.NewVerboseHandler(reportWriter, config)
    } else {
        h = gostd.NewQuietHandler(reportWriter, config)
    }

    // 4. Build CoreUI with handlers
    ux := newCoreUI().
        withNotifications().
        withReports().
        withHandlers(h).
        withStdout(reportWriter).
        withStderr(notificationWriter)

    // 5. Create footer handler (creates Bubble Tea models)
    summaryHandler := gosummary.NewFactory(config, common)

    // 6. Wrap in TeaUI
    c := NewTeaUIConfig().
        WithCoreUI(ux).                                   // Embed CoreUI
        WithSyncSpinner(spin).                            // Add animation
        WithPrintReader(reportReader, notificationReader). // Read handler output
        WithFooter(summaryHandler)                        // Add footer

    return NewTeaUI(c)
}
```

---

## Event Propagation

### Event Types

Defined in `internal/bus/event/types.go`:

```go
const (
    GoTestRunRequestType  // Test run initiated (Value: RunnerConfig, Source: UUID)
    GoTestType            // Individual test event (Value: gotest.Event)
    GoTestRunType         // Test run completed (Value: gotest.Run)
    PrintType             // Debug/informational output
    CLIReport             // Longer-form CLI output
    CLINotification       // Short status messages
)
```

### Test Action Lifecycle

Defined in `internal/gotest/action.go`:

```
┌─────────┐     ┌─────────┐     ┌─────────────┐
│  start  │────▶│   run   │────▶│   output    │──┐
│(package)│     │ (test)  │     │  (multiple) │  │
└─────────┘     └─────────┘     └─────────────┘  │
                                                  │
                     ┌────────────────────────────┘
                     │
                     ▼
              ┌─────────────┐
              │ pass/fail/  │  ◀── Terminal states
              │    skip     │
              └─────────────┘
```

- **`start`**: First event for a package (package-level only)
- **`run`**: First event for a test/subtest
- **`output`**: Test produces output (can occur multiple times)
- **`pass`/`fail`/`skip`**: Terminal state (exactly one per test)

### Complete Event Flow

```
┌──────────────────────────────────────────────────────────────────────────────┐
│ 1. TEST RUNNER (internal/gotest/runner.go)                                   │
│                                                                              │
│    go test -json  ──▶  stdout (JSONL)  ──▶  Parse  ──▶  gotest.Event        │
│                   ──▶  stderr (errors) ──▶  Buffer ──▶  ErrRunStderr        │
└──────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│ 2. TEST MANAGER (internal/test/manager.go)                                   │
│                                                                              │
│    onEvent callback:                                                         │
│      ├── logEvent(event)           // Log for debugging                      │
│      ├── publishTestEvent(event)   // Publish to event bus                   │
│      └── runModel.addEvent(event)  // Persist to storage                     │
│                                                                              │
│    On completion:                                                            │
│      └── publishTestRun(run)       // Publish final run state                │
└──────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│ 3. EVENT BUS (internal/bus/bus.go)                                           │
│                                                                              │
│    bus.Publish(partybus.Event{                                               │
│        Type:  event.GoTestType,                                              │
│        Value: gotest.Event,                                                  │
│    })                                                                        │
│                                                                              │
│    Subscribers receive events via subscription                               │
└──────────────────────────────────────────────────────────────────────────────┘
                                    │
                    ┌───────────────┴───────────────┐
                    ▼                               ▼
┌────────────────────────────┐    ┌────────────────────────────────────────────┐
│ 4a. COREUI                 │    │ 4b. TEAUI                                   │
│                            │    │                                            │
│ Handle(event):             │    │ Handle(event):                             │
│   for _, h := range        │    │   program.Send(event) // Bubble Tea        │
│       handlers {           │    │   coreUI.Handle(event) // Delegate         │
│     h.Handle(event)        │    │                                            │
│   }                        │    │ Update(msg):                               │
│                            │    │   case partybus.Event:                     │
│ Teardown():                │    │     // Create models via handlers          │
│   for _, p := range        │    │     // Append to frame                     │
│       presenters {         │    │                                            │
│     p.Present(stdout,      │    │ View():                                    │
│               stderr)      │    │   return frame.View() // Render to term    │
│   }                        │    │                                            │
└────────────────────────────┘    └────────────────────────────────────────────┘
```

### Event Flow Example: Test Failure

```
1. go test outputs:  {"Action":"fail","Package":"pkg","Test":"TestFoo",...}
                              │
2. Runner parses to:  gotest.Event{Action: FailAction, Reference: {Package: "pkg", FuncName: "TestFoo"}}
                              │
3. Manager publishes: partybus.Event{Type: GoTestType, Value: event}
                              │
4. CoreUI routes to:  handler.Handle(event)
         │                    │
         │                    ▼
         │            quietHandler.OnGoTestEvent(event)
         │              └── result.Update(event)
         │              └── (buffers failure output)
         │
         └───────────▶ adapter.Handle(event)
                         └── aggregator.events = append(events, event)

5. On teardown:       presenter.Present(stdout, stderr)
                         └── Writes formatted failure output
```

---

## State Management

### The Result Struct (`internal/gotest/result.go`)

The `Result` struct is the **central state aggregator** for test execution. It maintains thread-safe, queryable state with multiple indices optimized for different access patterns.

```go
type Result struct {
    lock   *sync.RWMutex    // Thread safety
    config ResultConfig

    // Primary indices
    references *orderedset.OrderedSet[Reference]   // All test references (ordered)
    packages   *orderedset.OrderedSet[Reference]   // Package-level refs only
    children   map[Reference]*orderedset.OrderedSet[Reference]  // Hierarchy tree

    // Event storage
    testEventsByReference  map[Reference][]Event   // All events per reference
    testOutputByReference  map[Reference][]Event   // Only "output" actions

    // Action indices (for quick lookups by state)
    referencesByAction     map[Action]*orderedset.OrderedSet[Reference]
    testReferencesByAction map[Action]*orderedset.OrderedSet[Reference]

    // Terminal state
    conclusionEvent        map[Reference]Event     // Final event per reference

    // Timing
    start         time.Time
    lastEventTime time.Time
    totalElapsed  time.Duration

    // Coverage
    coverage *float64
}
```

### State Update Flow

```go
func (r *Result) Update(e Event) {
    r.lock.Lock()
    defer r.lock.Unlock()

    // 1. Update timing
    if r.start.IsZero() {
        r.start = e.Time
    }
    r.lastEventTime = e.Time

    // 2. Maintain hierarchy (parent-child relationships)
    parentRef := e.Reference.ParentRef()
    if parentRef != nil {
        r.children[*parentRef].Add(e.Reference)
    }

    // 3. Process by action type
    switch e.Action {
    case OutputAction:
        // Accumulate output
        r.testOutputByReference[e.Reference] = append(r.testOutputByReference[e.Reference], e)

    case PassAction, SkipAction:
        // Clear output for passing tests (memory optimization)
        if !r.config.TrackOtherOutput {
            r.testOutputByReference[e.Reference] = nil
        }
        fallthrough

    case FailAction:
        // Record conclusion
        r.conclusionEvent[e.Reference] = e
        r.referencesByAction[RunAction].Delete(e.Reference)  // No longer "running"
    }

    // 4. Update action indices
    r.referencesByAction[e.Action].Add(e.Reference)
    r.references.Add(e.Reference)
}
```

### Determining Test Status

```go
func (r Result) Passed() (passed bool, stillRunning bool) {
    r.lock.RLock()
    defer r.lock.RUnlock()

    runningTestRefs := r.testReferencesByAction[RunAction]
    failedTestRefs := r.testReferencesByAction[FailAction]

    // Tests are still running if:
    // 1. Any tests are in RunAction state
    // 2. Not all references have conclusion events
    hasMirroredRefs := len(r.conclusionEvent) == r.references.Size()
    isStillRunning := runningTestRefs.Size() > 0 || !hasMirroredRefs

    // Passed is true only if no tests failed
    passed = failedTestRefs.Size() == 0

    return passed, isStillRunning
}
```

### ResultConfig Options

```go
type ResultConfig struct {
    TrackOtherOutput   bool  // Keep output from passing/skipped tests
    TrackFailingOutput bool  // Keep output from failing tests
}
```

| Use Case | TrackOtherOutput | TrackFailingOutput |
|----------|------------------|-------------------|
| Live testing (memory efficient) | `false` | `false` |
| Replay/history (full data) | `true` | `true` |

---

## Output Streams: Stderr vs Stdout

Canopy uses stderr and stdout strategically to separate **active/updating content** from **final/immutable content**.

### The Two-Stream Model

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           STDERR (Active State)                         │
│                                                                         │
│  • Continuously rewritten by Bubble Tea                                 │
│  • Contains spinner, progress, running tests                            │
│  • Ephemeral - content changes with each render                         │
│  • User sees "live" updates                                             │
│                                                                         │
│  Example: "⠋ Running tests... 3 passed, 0 failed (2.3s)"               │
└─────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────┐
│                           STDOUT (Final State)                          │
│                                                                         │
│  • Written once, never rewritten                                        │
│  • Contains completed test results, final summaries                     │
│  • Permanent - safe to pipe to files                                    │
│  • Appears "above" the active stderr content                            │
│                                                                         │
│  Example: "✓ pkg/foo           (0.42s)"                                 │
└─────────────────────────────────────────────────────────────────────────┘
```

### Implementation in TeaUI

```go
// tea_ui.go
func (m *UI) Setup(subscription partybus.Unsubscribable) error {
    // Bubble Tea renders to stderr (active, rewritable)
    m.program = tea.NewProgram(m, tea.WithOutput(os.Stderr), ...)

    // Handler output appears via program.Println (permanent)
    go func(reader io.Reader) {
        scanner := bufio.NewScanner(reader)
        for scanner.Scan() {
            m.program.Println(scanner.Text())  // Writes above the active UI
        }
    }(printReader)

    return m.config.coreUI.Setup(subscription)
}
```

### Visual Representation

```
Terminal Display:
┌──────────────────────────────────────────────────────────────────────┐
│ ✓ github.com/user/pkg/foo                              0.42s        │ ◀─ stdout (permanent)
│ ✓ github.com/user/pkg/bar                              0.31s        │
│ ✗ github.com/user/pkg/baz                              1.02s        │
│     --- FAIL: TestBaz (1.02s)                                       │
│         baz_test.go:42: expected 1, got 2                           │
├──────────────────────────────────────────────────────────────────────┤
│ ⠋ Running tests... 2 passed, 1 failed, 3 running (3.2s)             │ ◀─ stderr (updating)
│   └─ TestQux, TestCorge, TestGrault                                 │
└──────────────────────────────────────────────────────────────────────┘
        ▲                                                      ▲
        │                                                      │
   Scrolls up as                                        Stays at bottom,
   new results arrive                                   continuously updated
```

### Source of stdout vs stderr Content

| Content Type | Stream | Source |
|--------------|--------|--------|
| Package results | stdout | Handler writes to `reportWriter` |
| Test failure details | stdout | Handler writes to `reportWriter` |
| Notifications | stderr | CoreUI writes to `notificationWriter` |
| Spinner/progress | stderr | TeaUI renders via Bubble Tea |
| Running test list | stderr | Footer model in TeaUI |
| Final summary | stdout | Presenter at teardown (CoreUI mode) |
| Final summary | stderr | Footer model (TeaUI mode) |

### From go test Perspective

The test runner itself produces two streams:

```go
// runner.go
func (r *Runner) startEventStream() (<-chan JSONL, error) {
    stdout, _ := cmd.StdoutPipe()  // JSONL test events (structured)
    stderr, _ := cmd.StderrPipe()  // Compilation errors, warnings

    // stdout is parsed line-by-line into events
    go jsonLFromReader(stdout, events)

    // stderr is accumulated and reported as error at END
    var sb strings.Builder
    go func() {
        reader := bufio.NewReader(stderr)
        for {
            line, _ := reader.ReadString('\n')
            sb.WriteString(line)
        }
    }()

    // stderr only surfaces after run completes
    go func() {
        wg.Wait()
        if sb.Len() > 0 {
            events <- JSONL{Index: math.MaxInt64, Error: ErrRunStderr{Output: sb.String()}}
        }
        close(events)
    }()
}
```

**Key insight:** Test runner stderr (compilation errors) is **deferred** until the run completes, then surfaced as a special error event. This prevents interleaving of error messages with structured test output.

---

## Putting It All Together

### Complete Data Flow Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                             │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐  │
│  │  go test    │───▶│   JSONL     │───▶│  gotest.    │───▶│   Manager   │  │
│  │   -json     │    │   Parser    │    │   Event     │    │  onEvent()  │  │
│  └─────────────┘    └─────────────┘    └─────────────┘    └──────┬──────┘  │
│                                                                   │         │
│                              TEST RUNNER                          │         │
└───────────────────────────────────────────────────────────────────┼─────────┘
                                                                    │
                                                   publishTestEvent(e)
                                                                    │
                                                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                              EVENT BUS                                      │
│                                                                             │
│                    partybus.Event{Type: GoTestType, Value: e}               │
│                                                                             │
└───────────────────────────────────────────────────────────────────┬─────────┘
                                                                    │
                                    ┌───────────────────────────────┤
                                    │                               │
                                    ▼                               ▼
┌───────────────────────────────────────────────┐ ┌───────────────────────────┐
│               CoreUI.Handle(e)                │ │  TeaUI.Update(msg)        │
│                                               │ │                           │
│  ┌─────────────────────────────────────────┐  │ │  ┌─────────────────────┐  │
│  │  for _, h := range handlers:            │  │ │  │ handler.Handle(e)   │  │
│  │    h.Handle(e)                          │  │ │  │   ↓                 │  │
│  │      │                                  │  │ │  │ Create new models   │  │
│  │      ├── quietHandler.OnGoTestEvent(e)  │  │ │  │   ↓                 │  │
│  │      │     └── result.Update(e)         │  │ │  │ frame.AppendModel() │  │
│  │      │     └── (buffer or write)        │  │ │  └─────────────────────┘  │
│  │      │                                  │  │ │                           │
│  │      └── adapter.Handle(e)              │  │ │  ┌─────────────────────┐  │
│  │            └── aggregator.append(e)     │  │ │  │ View()              │  │
│  │                                         │  │ │  │   ↓                 │  │
│  └─────────────────────────────────────────┘  │ │  │ Render to stderr    │  │
│                                               │ │  │ (active UI)         │  │
│  ┌─────────────────────────────────────────┐  │ │  └─────────────────────┘  │
│  │  Teardown():                            │  │ │                           │
│  │    for _, p := range presenters:        │  │ │  ┌─────────────────────┐  │
│  │      p.Present(stdout, stderr)          │  │ │  │ printReaders scan   │  │
│  │        │                                │  │ │  │   ↓                 │  │
│  │        └── Write final output           │  │ │  │ program.Println()   │  │
│  │                                         │  │ │  │ (permanent output)  │  │
│  └─────────────────────────────────────────┘  │ │  └─────────────────────┘  │
│                                               │ │                           │
└───────────────────────────────────────────────┘ └───────────────────────────┘
                    │                                           │
                    ▼                                           ▼
            ┌───────────────┐                          ┌───────────────┐
            │    stdout     │                          │    stderr     │
            │   (final)     │                          │   (active)    │
            └───────────────┘                          └───────────────┘
```

### Lifecycle Summary

1. **Setup Phase**
   - UI registers with event bus subscription
   - TeaUI starts Bubble Tea program (renders to stderr)
   - Print readers start scanning handler output pipes

2. **Event Processing Phase** (repeated for each event)
   - Test runner produces JSONL
   - Manager publishes to event bus
   - CoreUI routes to handlers
   - Handlers update state, may write output
   - TeaUI creates/updates models, re-renders

3. **Teardown Phase**
   - Close handler output pipes
   - Wait for print readers to complete
   - Call presenters to write final output
   - Quit Bubble Tea program
   - Final state written to stdout

### Key Architectural Principles

1. **Separation of Concerns**: Each component has a single responsibility
2. **Composition over Inheritance**: Adapters compose handlers and presenters
3. **Event-Driven**: Loose coupling via event bus
4. **Factory Pattern**: Configurability without tight coupling
5. **Builder Pattern**: Fluent API for UI construction
6. **Dual-Stream Output**: Active (stderr) vs Final (stdout) content

---

## File Reference

| File | Purpose |
|------|---------|
| `core_ui.go` | CoreUI implementation |
| `tea_ui.go` | TeaUI implementation |
| `test_go_ui.go` | UI factory for go test format |
| `format/handler/handler.go` | Handler interface |
| `format/handler/gostd/` | Standard Go test handlers |
| `format/presenter/presenter.go` | Presenter interface |
| `format/adapter/handled_presenters.go` | Adapter interface |
| `format/model/bubble/` | Bubble Tea components |
| `internal/gotest/runner.go` | Test runner |
| `internal/gotest/result.go` | State aggregator |
| `internal/gotest/action.go` | Test action types |
| `internal/bus/event/types.go` | Event type definitions |
| `internal/test/manager.go` | Test session manager |
