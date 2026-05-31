package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Limits that keep show terminating on infinite or self-referential values.
const (
	ShowDefaultWidth = 100 // most elements rendered in a single list or tuple
	ShowDefaultDepth = 50  // deepest nesting rendered before an ellipsis
)

// ListEnding describes how a chain of cons cells ended, as discovered while
// walking it in collectListSpine.
type ListEnding int

const (
	NotAList   ListEnding = iota // the chain ended in a value that is not [], so it is really nested tuples
	ProperList                   // the chain ended in the empty tuple
	Truncated                    // the width limit was reached first
	Cyclic                       // the chain looped back to a named value
)

// ShowValue renders a runtime value as a readable string. It forces the value
// only as far as it needs to, guesses which tuples are lists, labels functions
// and cycles with the name of the binding they came from, and caps width and
// depth so that even an infinite or self-referential value still prints.
func (i *Interpreter) ShowValue(value RuntimeValue) string {
	var builder strings.Builder
	i.WriteValue(&builder, value, 0, ShowDefaultDepth, ShowDefaultWidth, make(map[*NamedValue]bool))
	return builder.String()
}

// ShowValueFull is like ShowValue but without width or depth limits.
func (i *Interpreter) ShowValueFull(value RuntimeValue) string {
	var builder strings.Builder
	i.WriteValue(&builder, value, 0, math.MaxInt, math.MaxInt, make(map[*NamedValue]bool))
	return builder.String()
}

// WriteValue renders one value. expanding holds the named values currently being
// rendered on the path above this point, so a value that refers back to itself
// prints its name instead of looping forever.
func (i *Interpreter) WriteValue(builder *strings.Builder, value RuntimeValue, depth int, maxDepth int, maxWidth int, expanding map[*NamedValue]bool) {

	if depth > maxDepth {
		builder.WriteString("…")
		return
	}

	// If this is a named value, keep its name (to label a function or a cycle)
	// and guard against expanding the same binding twice on one path.
	name := ""
	if named, isNamed := value.(*NamedValue); isNamed {
		if expanding[named] {
			builder.WriteString(NameOrEllipsis(named))
			return
		}
		name = named.Name
		expanding[named] = true
		defer delete(expanding, named)
	}

	switch forced := i.EvaluateToWeakHeadNormalForm(value).(type) {
	case RuntimeNumber:
		builder.WriteString(FormatNumber(forced))

	case RuntimeCons:
		i.WriteConsOrList(builder, forced, depth, maxDepth, maxWidth, expanding)

	case RuntimeTuple:
		i.WriteTuple(builder, forced, depth, maxDepth, maxWidth, expanding)

	case RuntimeClosure, RuntimeBuiltin, RuntimeComposition, RuntimePartial:
		if name != "" {
			builder.WriteString(name)
		} else {
			builder.WriteString("<function>")
		}

	default:
		fmt.Fprintf(builder, "%+v", forced)
	}
}

// WriteConsOrList renders a cons cell as a list [a; b; c] when its tail chain
// ends in the empty list, and as a plain 2-tuple [a, b] otherwise.
func (i *Interpreter) WriteConsOrList(builder *strings.Builder, cons RuntimeCons, depth int, maxDepth int, maxWidth int, expanding map[*NamedValue]bool) {

	heads, ending, tailName := i.CollectListSpine(cons, expanding, maxWidth)
	if ending != NotAList {
		builder.WriteByte('[')
		for index, head := range heads {
			if index > 0 {
				builder.WriteString("; ")
			}
			i.WriteValue(builder, head, depth+1, maxDepth, maxWidth, expanding)
		}
		if ending == Truncated {
			builder.WriteString("; …")
		} else if ending == Cyclic {
			builder.WriteString("; ")
			builder.WriteString(tailName)
		} else if ending == ProperList && len(heads) == 1 {
			builder.WriteString(";")
		}
		builder.WriteByte(']')
		return
	}

	// The tail is not a list, so this cons is really a 2-tuple pair.
	builder.WriteByte('[')
	i.WriteValue(builder, cons.Head, depth+1, maxDepth, maxWidth, expanding)
	builder.WriteString(", ")
	i.WriteValue(builder, cons.Tail, depth+1, maxDepth, maxWidth, expanding)
	builder.WriteByte(']')
}

// WriteTuple renders a non-cons tuple (arity 0, 1, 3, 4, …) as [a, b, c]. The
// empty tuple, which is also the empty list, prints as [].
func (i *Interpreter) WriteTuple(builder *strings.Builder, tuple RuntimeTuple, depth int, maxDepth int, maxWidth int, expanding map[*NamedValue]bool) {

	builder.WriteByte('[')
	for index, element := range tuple {
		if index > 0 {
			builder.WriteString(", ")
		}
		i.WriteValue(builder, element, depth+1, maxDepth, maxWidth, expanding)
	}
	builder.WriteByte(']')
}

// CollectListSpine walks a chain of cons cells, gathering the (still unforced)
// head of each cell and reporting how the chain ended. It forces only the spine,
// so the heads stay lazy until they are rendered. A repeated named value, or one
// already being expanded above, is reported as a cycle rather than followed.
func (i *Interpreter) CollectListSpine(start RuntimeCons, expanding map[*NamedValue]bool, maxWidth int) ([]RuntimeValue, ListEnding, string) {

	var heads []RuntimeValue
	seen := make(map[*NamedValue]bool)
	var current RuntimeValue = start

	for {
		if named, isNamed := current.(*NamedValue); isNamed {
			if expanding[named] || seen[named] {
				return heads, Cyclic, NameOrEllipsis(named)
			}
			seen[named] = true
		}

		switch forced := i.EvaluateToWeakHeadNormalForm(current).(type) {
		case RuntimeCons:
			heads = append(heads, forced.Head)
			current = forced.Tail
			if len(heads) >= maxWidth {
				return heads, Truncated, ""
			}
		case RuntimeTuple:
			if len(forced) == 0 {
				return heads, ProperList, ""
			}
			return nil, NotAList, ""
		default:
			return nil, NotAList, ""
		}
	}
}

func FormatNumber(number RuntimeNumber) string {
	return strconv.FormatFloat(float64(number), 'g', -1, 64)
}

func NameOrEllipsis(named *NamedValue) string {
	if named.Name != "" {
		return named.Name
	}
	return "…"
}
