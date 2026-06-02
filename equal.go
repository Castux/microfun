package main

type comparisonPair struct {
	a, b Value
}

// DeepEqual compares two values by structure, forcing them as needed. It handles
// cyclic structures by tracking pairs of values it has already started comparing.
func DeepEqual(a, b Value, seen map[comparisonPair]bool) bool {
	pair := comparisonPair{a, b}
	if seen[pair] {
		return true
	}
	seen[pair] = true

	forcedA := WHNF(a)
	forcedB := WHNF(b)

	if forcedA.Tag != forcedB.Tag {
		return false
	}

	switch forcedA.Tag {
	case NumberTag:
		return forcedA.Num == forcedB.Num

	case TupleTag:
		tA := forcedA.tuple()
		tB := forcedB.tuple()
		if len(tA.Fields) != len(tB.Fields) {
			return false
		}
		for i := range tA.Fields {
			if !DeepEqual(tA.Fields[i], tB.Fields[i], seen) {
				return false
			}
		}
		return true

	case ConsTag:
		cA := forcedA.cons()
		cB := forcedB.cons()
		return DeepEqual(cA.Head, cB.Head, seen) && DeepEqual(cA.Tail, cB.Tail, seen)

	case ClosureTag, BuiltinTag, CompositionTag, ApplyTag:
		// Functions are only equal if they are the exact same value.
		return forcedA == forcedB

	default:
		return false
	}
}

// FullNormalForm forces a value and all its sub-values. It handles cycles by
// tracking Thunks it has already visited.
func FullNormalForm(value Value, seen map[*Thunk]bool) Value {
	if value.Tag == ThunkTag {
		thunk := value.thunk()
		if seen[thunk] {
			return value
		}
		seen[thunk] = true
	}

	forced := WHNF(value)

	switch forced.Tag {
	case TupleTag:
		t := forced.tuple()
		for i, element := range t.Fields {
			t.Fields[i] = FullNormalForm(element, seen)
		}
		return forced

	case ConsTag:
		c := forced.cons()
		FullNormalForm(c.Head, seen)
		FullNormalForm(c.Tail, seen)
		return forced

	default:
		return forced
	}
}
