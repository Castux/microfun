package main

import (
	"fmt"
	"strings"
)

// A RuntimeError is the panic payload raised when a microfun program cannot
// continue: a non-exhaustive match, applying something that is not a function,
// and so on. It carries enough to report the failure in the programmer's terms
// rather than ours: a message, the source span of the offending expression (if
// known), and the chain of named thunks that were being forced when it
// happened.
//
// That chain is the closest thing a lazy language has to a stack trace. The Go
// call stack would only show the machinery of EvaluateToWeakHeadNormalForm; the
// host stack of a lazy evaluator does not correspond to the program's logical
// call structure, because a thunk is built in one place and forced in another.
// The named thunks currently on the reduction stack, on the other hand, are
// exactly the bindings whose evaluation led here, so they read like a trace.
//
// RuntimeError is recovered at the Interpret boundary. Bare panics are reserved
// for interpreter bugs (unimplemented nodes, broken invariants), which should
// keep their Go stack trace rather than be reported as program errors.
type RuntimeError struct {
	Message string
	Pos     SourcePos
	HasPos  bool
	Trace   []string
}

func (e *RuntimeError) Error() string { return e.Message }

// collectTrace walks the reduction stack from the outermost frame inward and
// returns the names of the thunks currently being forced. Anonymous thunks
// (intermediate applications with no binding name) are skipped, leaving a
// skeleton of named bindings, much like Haskell's cost-centre stack.
func collectTrace(stack []StackFrame) []string {
	var trace []string
	for _, frame := range stack {
		if frame.Kind == updateFrame && frame.Thunk.Name != "" {
			trace = append(trace, frame.Thunk.Name)
		}
	}
	return trace
}

// raiseRuntimeError stops the program with a reportable error. pos may be a zero
// SourcePos (File nil), in which case the error is reported without a location.
func (i *Interpreter) raiseRuntimeError(message string, pos SourcePos, stack []StackFrame) {
	panic(&RuntimeError{
		Message: message,
		Pos:     pos,
		HasPos:  pos.File != nil,
		Trace:   collectTrace(stack),
	})
}

// ReportRuntimeError prints a RuntimeError: the message (pointing at the source
// span when one is known) followed by the chain of bindings that led to it.
func ReportRuntimeError(err *RuntimeError) {
	if err.HasPos {
		Log(err.Message, err.Pos, SeverityError)
	} else {
		fmt.Println("runtime error: " + err.Message)
	}
	if len(err.Trace) > 0 {
		fmt.Println("while reducing: " + strings.Join(err.Trace, " → "))
	}
}
