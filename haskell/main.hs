import Lexer
import Parser

main = do
    print $ parseExpressions $ lexAll "this (x -> y -> x) 5 stupid"