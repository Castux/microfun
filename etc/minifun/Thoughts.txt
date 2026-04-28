Next version: type system!
==========================

apply : (a -> b) -> a -> b
apply = f -> x -> f x


flip : (a -> b) -> (b -> a)
flip = f -> a -> b -> f b a

id : a -> a
id = x -> x

compose : (b -> c) -> (a -> b) -> (a -> c)

withDefault : a -> Maybe a -> a
withDefault = default -> [
	Some a -> a,
	None -> default
]

andThen : (a -> Maybe b) -> Maybe a -> Maybe b
andThen = f -> [
	Some a -> f a,
	None -> None
]

map : (a -> b) -> Maybe a -> Maybe b
map = f -> [
	Some a -> Some (f a),
	None -> None
]


type List a = Empty | Cons a (List a)

cons : a -> List a -> List a
cons = head -> tail -> head :: tail

length : List a -> Number
length = [
	Empty -> 0,
	Cons head tail -> tail > length > add 1
]

map : (a -> b) -> List a -> List b
map = f -> [
	{} -> {},
	h :: t -> f h :: map f tail
]

foldr : (elem -> acc -> acc) -> acc -> List elem -> acc
foldr = f -> init -> [
	{} -> init,
	h :: t -> f h (foldr f tail)
]

foldl : (elem -> acc -> acc) -> acc -> List elem -> acc
foldl = f -> acc -> [
	{} -> acc,
	h :: t -> foldl f (f h acc) t
]


sum : List Number -> Number
sum = [
	{} -> 0,
	h :: t -> add h (sum tail)
]

sum = foldr add 0


filter : (a -> Bool) -> List a -> List a
filter = f -> [
	{} -> {},
	h :: t -> f h > [
		True -> h :: filter f t,
		False -> filter f t
	]
]


if : Bool -> a -> a -> a
if = p -> t -> f -> p > [
	True -> t,
	False -> f
]


tails : List a -> List (List a)
tails = [
	{} -> {{}},
	l -> l :: (tails (tail l))
]

tails = [
	{} -> {{}},
	h :: t -> cons (h :: t) (tails t)
]


head : List a -> Maybe a
head = [
	{} -> None,
	h :: t -> Some h
]

tail : List a -> List a
tail = [
	{} -> {},
	h :: t -> t
]

upFrom = n -> n > add 1 > upFrom > cons n
n -> cons n (upFrom (add 1 n))

iterate f x = x : (iterate f (f x))
repeat = x -> x : repeat x

reverse = [
	{} -> {},
	h :: t -> concat (reverse tail) {h}
]

reverse = foldl cons {}
concat = foldr cons
flatten = foldr concat []
