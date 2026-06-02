package value

import (
	"fmt"
	"strings"
)

// ShowConst renders a compile-time constant Value without forcing anything.
// Core/bytecode constants are already fully built (numbers, prebuilt code-point
// lists for string literals, the empty tuple, builtins), so this walks them
// directly rather than routing through Force, keeping the dumps independent of the
// machine.
func ShowConst(v Value) string {
	switch v.Tag {
	case NumberTag:
		return formatNumber(v.Num)

	case BuiltinTag:
		return "<builtin " + v.Builtin().Name + ">"

	case ConsTag:
		// A string literal lowers to a cons list of code points; render it as a
		// quoted string when every element is one, else as a literal list.
		if s, ok := codePointString(v); ok {
			return fmt.Sprintf("%q", s)
		}
		return showConstList(v)

	case TupleTag:
		t := v.Tuple()
		if len(t.Fields) == 0 {
			return "[]"
		}
		parts := make([]string, len(t.Fields))
		for i, f := range t.Fields {
			parts[i] = ShowConst(f)
		}
		return "(" + strings.Join(parts, ", ") + ")"

	default:
		return fmt.Sprintf("<tag %d>", v.Tag)
	}
}

// codePointString reports whether v is a proper cons list of integral code points
// and, if so, returns the decoded string.
func codePointString(v Value) (string, bool) {
	var runes []rune
	for {
		switch v.Tag {
		case TupleTag:
			if len(v.Tuple().Fields) == 0 {
				return string(runes), true
			}
			return "", false
		case ConsTag:
			head := v.Cons().Head
			if head.Tag != NumberTag || head.Num != float64(int32(head.Num)) {
				return "", false
			}
			runes = append(runes, rune(int32(head.Num)))
			v = v.Cons().Tail
		default:
			return "", false
		}
	}
}

func showConstList(v Value) string {
	var parts []string
	for v.Tag == ConsTag {
		parts = append(parts, ShowConst(v.Cons().Head))
		v = v.Cons().Tail
	}
	return "[" + strings.Join(parts, "; ") + "]"
}
