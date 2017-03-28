-- Some arithmetics

let

divides = n -> eq 0 . mod n,
divisors = n -> filter (divides n) < range 1 n,
digits = reverse . map (flip mod 10) . takeWhile (lt 0) . iterate (flip div 10),

isPrime = n -> range 2 (sqrt n) > none (divides n) > and (gte n 2),
primes = upfrom 1 > filter isPrime,
primeDivisors = [
	1 -> {},
	n -> let
		divs = filter (eq 0 . mod n) < range 1 (sqrt n),
		k = head < drop 1 < concat divs {n}
	in
		(k, primeDivisors < div n k)
],

fibonacci = concat {1,1} (zipWith add fibonacci (tail fibonacci)),

let fact = [
	1 -> 1,
	n -> mul n (fact (pred n))
]

in