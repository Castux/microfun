module Lexer (lexAll, Token(..)) where

import Data.List
import Data.Char
import Data.Maybe

data Token = Equal | Lparens | Rparens | Arrow | Ident String | Number Int deriving (Show)

tokens = [
    ("->", Arrow),
    ("(", Lparens),
    (")", Rparens),
    ("=", Equal)
    ]

lexers = [lexSpecial, lexNumber, lexIdentifier]

lexAll :: String -> [Token]
lexAll [] = []
lexAll str = let striped = dropWhile isSpace str in case lexOne striped of
    Just (tok, rest) -> tok : lexAll rest
    Nothing -> error $ "Could not lex: " ++ (take 10 str)

lexOne :: String -> Maybe (Token, String)
lexOne str = listToMaybe $ mapMaybe (\f -> f str) lexers

lexTokenString :: String -> (String, Token) -> Maybe (Token, String)
lexTokenString str (tokStr, token) = case stripPrefix tokStr str of
    Just rest -> Just (token, rest)
    Nothing -> Nothing

lexSpecial :: String -> Maybe (Token, String)
lexSpecial str = listToMaybe $ mapMaybe (lexTokenString str) tokens

lexNumber :: String -> Maybe (Token, String)
lexNumber [] = Nothing
lexNumber str = let (digits,rest) = span isDigit str in case digits of
    [] -> Nothing
    _ -> Just (Number (read digits), rest)

isValidIdChar c = isAlphaNum c || c == '_'

lexIdentifier :: String -> Maybe (Token, String)
lexIdentifier [] = Nothing
lexIdentifier str = let (chars, rest) = span isValidIdChar str in case chars of
    [] -> Nothing
    (c:_) | isDigit c -> Nothing
    _ -> Just (Ident chars, rest)