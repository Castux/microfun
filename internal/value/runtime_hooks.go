package value

// Forcing a value to weak head normal form means running compiled code, which is
// the backend's job (it re-enters the G-machine). Value-level operations that must
// force — structural equality, full-normal-form eval, display, the structural
// builtins — cannot import the backend without creating an import cycle, so the
// backend installs these hooks at startup (see NewMachine). They are non-nil for
// the entire duration of execution; nothing forces before the machine is built.

// Force reduces v to weak head normal form.
var Force func(v Value) Value

// RaiseBuiltinError aborts execution from inside a structural builtin, locating
// the error at the active builtin's application span and tracing the reduction
// stack that was live when the builtin was entered.
var RaiseBuiltinError func(message string)
