Lexical
=======

Equal := =
Lparens := (
Rparens := )
Arrow := ->

Id := [^\w()]+
Number := \d+

Grammar (PEG)
=============

Program := Expr

Expr := Binding | Application | Lambda | AtomicExpr
Binding := 'let' Id '=' Expr 'in' Expr
Lambda := Id '->' Expr
Application := (Application | AtomicExpr) AtomicExpr
AtomicExpr := Id | Number | '(' Expr ')'

ListX := [ X { ',' X } ]

Advanced
========

Boolean
-------

Just 0 and non 0 like in C

Tuple and list creation
-----------------------

AtomicExpr := ... | '(' ListExpr ')'

(), (a,b), (a,()), (a, (b, ()))

cons = x -> xs -> (x, xs)

compose = f -> g -> f g
id = x -> x
swap = f -> x -> y -> f y x

Pattern matching
----------------

It's like a lambda, but with several possible matches and expressions. It compares its argument to each pattern in succession, binding the free names in them.

AtomicExpr := ... | Matching

Matching := '[' ListMatch ']'
Match := Pattern '->' Expr
Pattern := Id | Number | '(' ListPattern ')'

Actually, a lambda is just a matching with only one option (in which case we just don't write the brackets).
At first though, it looks like the only tuple pattern needed is (x,xs). Not recursive.

Examples
========

length = [
	() -> 0,
	(h,t) -> add 1 (length t)
]

if = cond -> t -> f -> [ 0 -> f, else -> t ] cond

fact = [
	1 -> 1,
	n -> mul n (fact (sub n 1))
]

head = (h,t) -> h
tail = (h,t) -> t

empty = [ () -> 1, l -> 0 ]

concat = [
	() -> v -> v,
	(h,t) -> v -> (h, (concat t v))
]

reverse = [
	() -> (),
	(h,t) -> concat (reverse t) (h,())
]

reverse = l ->
	let help = [
		() -> res -> res,
		(h,t) -> res -> help t (h,res)
	]
	in help l ()

map = f -> [
	() -> (),
	(h,t) -> (f h, map f t)
]

filter = f -> [
	() -> (),
	(h,t) -> let rest = filter f t in if (f h) (h, rest) rest
]

foldr = f -> acc -> [
	() -> acc,
	(h,t) -> f h (foldr f acc t)
]

foldl = f -> acc -> [
	() -> acc,
	(h,t) -> foldl f (f acc h) t
]

sum = foldr add 0
product = foldr mul 1

zipWith = f -> [
	()		-> b -> (),
	(a,as)	-> [
		()		-> (),
		(b,bs)	-> (f a b, zipWith f as bs)
	]
]

zip = zipWith cons

-- works with booleans as 0 / >0
and = mul
or = add

-- Lua like:
and = a -> b -> if a b 0
or = a -> b -> if a 1 b
not = [ 0 -> 1, else -> 0]

andList = foldr and 1
orList = foldr or 0

all = f -> andList (map f)
any = f -> orList (map f)

succ = add 1
pred = (flip sub) 1

range = a -> b -> if (lt a b)
	(a, range (succ a) b)
	()

isPrime = n -> and (gt n 1) (all (k -> neq (div n k) 0) (range 2 (sqrt n)))


take = n -> [
	() -> (),
	(h,t) -> cons h (take (pred n) t)
]

drop = [
	0 -> l -> l,
	n -> [
		() -> (),
		(h,t) -> drop (pred n) t
	]
]