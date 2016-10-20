module Parser (parse, parseExpressions) where

import Lexer

type Program = [Binding]
data Binding = Binding String Expr deriving (Show)

data Expr = Constant Int | Identifier String | Lambda String Expr | Application [Expr] deriving (Show)

parse :: [Token] -> Program
parse [] = []
parse tokens = let (binding, rest) = parseBinding tokens in binding : parse rest

parseBinding :: [Token] -> (Binding, [Token])
parseBinding (Ident id : Equal : ts) = let (expr, rest) = parseAtomicExpr ts in (Binding id expr, rest)
parseBinding tokens = error $ "Could not parse binding: " ++ show (take 10 tokens)

parseAtomicExpr :: [Token] -> (Expr, [Token])

parseAtomicExpr (Ident id : Arrow : ts) = let (expr, rest) = parseAtomicExpr ts in ((Lambda id expr), rest)
parseAtomicExpr (Number n : ts) = ((Constant n), ts)
parseAtomicExpr (Ident id : ts) = ((Identifier id), ts)
parseAtomicExpr (Lparens : ts) = case parseAtomicExpr ts of
    (expr, (Rparens : rest)) -> (expr, rest)
    (expr, (bad : rest)) -> error $ "Expected ')', found instead: " ++ show bad
parseAtomicExpr tokens = error $ "Could not parse expression: " ++ show (take 10 tokens)

parseExpressions tokens result = case tokens of
    (Ident id : Arrow : ts) -> treat tokens
    (Number n : ts) -> treat tokens
    (Ident id : ts) -> treat tokens
    (Lparens : ts) -> treat tokens
    _ -> tokens []
    where treat tokens = let (expr, rest) = parseAtomicExpr tokens in parseExpressions rest rest

--parseExpression tokens = case parseExpressions tokens of
--    [e] -> 