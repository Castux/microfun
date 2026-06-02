package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

const (
	ShowDefaultWidth = 100
	ShowDefaultDepth = 50
)

type ListEnding int

const (
	NotAList   ListEnding = iota
	ProperList
	Truncated
	Cyclic
)

func ShowValue(value Value) string {
	var builder strings.Builder
	writeValue(&builder, value, 0, ShowDefaultDepth, ShowDefaultWidth, make(map[*Thunk]bool))
	return builder.String()
}

func ShowValueFull(value Value) string {
	var builder strings.Builder
	writeValue(&builder, value, 0, math.MaxInt, math.MaxInt, make(map[*Thunk]bool))
	return builder.String()
}

func writeValue(builder *strings.Builder, value Value, depth int, maxDepth int, maxWidth int, expanding map[*Thunk]bool) {
	if depth > maxDepth {
		builder.WriteString("…")
		return
	}

	name := ""
	if value.Tag == ThunkTag {
		thunk := value.thunk()
		if expanding[thunk] {
			builder.WriteString(nameOrEllipsis(thunk))
			return
		}
		name = thunk.Name
		expanding[thunk] = true
		defer delete(expanding, thunk)
	}

	forced := WHNF(value)

	switch forced.Tag {
	case NumberTag:
		builder.WriteString(formatNumber(forced.Num))

	case ConsTag:
		writeConsOrList(builder, forced.cons(), depth, maxDepth, maxWidth, expanding)

	case TupleTag:
		writeTuple(builder, forced.tuple(), depth, maxDepth, maxWidth, expanding)

	case ClosureTag, BuiltinTag, CompositionTag, ApplyTag:
		if name != "" {
			builder.WriteString(name)
		} else {
			builder.WriteString("<function>")
		}

	default:
		fmt.Fprintf(builder, "<unknown tag %d>", forced.Tag)
	}
}

func writeConsOrList(builder *strings.Builder, cons *Cons, depth int, maxDepth int, maxWidth int, expanding map[*Thunk]bool) {
	heads, ending, tailName := collectListSpine(cons, expanding, maxWidth)
	if ending != NotAList {
		builder.WriteByte('[')
		for index, head := range heads {
			if index > 0 {
				builder.WriteString("; ")
			}
			writeValue(builder, head, depth+1, maxDepth, maxWidth, expanding)
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

	builder.WriteByte('[')
	writeValue(builder, cons.Head, depth+1, maxDepth, maxWidth, expanding)
	builder.WriteString(", ")
	writeValue(builder, cons.Tail, depth+1, maxDepth, maxWidth, expanding)
	builder.WriteByte(']')
}

func writeTuple(builder *strings.Builder, tuple *Tuple, depth int, maxDepth int, maxWidth int, expanding map[*Thunk]bool) {
	builder.WriteByte('[')
	for index, element := range tuple.Fields {
		if index > 0 {
			builder.WriteString(", ")
		}
		writeValue(builder, element, depth+1, maxDepth, maxWidth, expanding)
	}
	builder.WriteByte(']')
}

func collectListSpine(start *Cons, expanding map[*Thunk]bool, maxWidth int) ([]Value, ListEnding, string) {
	var heads []Value
	seen := make(map[*Thunk]bool)
	var current Value = cons(start.Head, start.Tail)

	for {
		if current.Tag == ThunkTag {
			thunk := current.thunk()
			if expanding[thunk] || seen[thunk] {
				return heads, Cyclic, nameOrEllipsis(thunk)
			}
			seen[thunk] = true
		}

		forced := WHNF(current)
		switch forced.Tag {
		case ConsTag:
			c := forced.cons()
			heads = append(heads, c.Head)
			current = c.Tail
			if len(heads) >= maxWidth {
				return heads, Truncated, ""
			}
		case TupleTag:
			if len(forced.tuple().Fields) == 0 {
				return heads, ProperList, ""
			}
			return nil, NotAList, ""
		default:
			return nil, NotAList, ""
		}
	}
}

func formatNumber(num float64) string {
	return strconv.FormatFloat(num, 'g', -1, 64)
}

func nameOrEllipsis(thunk *Thunk) string {
	if thunk.Name != "" {
		return thunk.Name
	}
	return "…"
}
